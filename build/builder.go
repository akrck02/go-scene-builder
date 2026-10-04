// Package build turns a map, a relief and style rules into pieces of a scene: stage 1 of the scene
// builder (basic blocks). Each feature gets a role from its style ("shape.role"): ground, water,
// volume, line, point or none; the relief becomes the terrain.
package build

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
	"github.com/twpayne/go-geos"
)

// chunkSize is the side (km) of the squares the pieces are grouped in, so the viewer only draws
// the ones in sight.
const chunkSize = 0.5

// Materials of the basic blocks.
var (
	groundMaterial = mesh.Material{Name: "ground", Kind: "lit", DoubleSided: true, DepthBias: 0.001}
	waterMaterial  = mesh.Material{Name: "water surface", Kind: "lit", DoubleSided: true, DepthBias: 0.001}
	solidMaterial  = mesh.Material{Name: "solid", Kind: "lit"}
)

// Input is what a build needs.
type Input struct {
	Map    *mapdata.Map
	Relief *relief.Relief // nil = flat ground
	Rules  *style.Rules
}

// Result is the pieces of the scene and what happened.
type Result struct {
	Pieces   []*mesh.Piece
	Frame    mesh.Frame
	Roles    map[string]int // features per role
	Warnings []string
}

// Build runs stage 1 for every feature, spread over the processor's cores.
func Build(input Input) *Result {
	result := &Result{Frame: frameOf(input), Roles: map[string]int{}}
	if input.Relief != nil {
		result.Pieces = append(result.Pieces, terrainPieces(input.Relief, input.Rules.Terrain, result.Frame)...)
	}
	workers := runtime.NumCPU()
	outputs := make([]*workerOutput, workers)
	var group sync.WaitGroup
	for worker := range workers {
		outputs[worker] = newWorkerOutput()
		group.Add(1)
		go func() {
			defer group.Done()
			builder := &featureBuilder{input: input, frame: result.Frame, context: geos.NewContext(), output: outputs[worker]}
			for index := worker; index < len(input.Map.Features); index += workers {
				builder.build(input.Map.Features[index])
			}
		}()
	}
	group.Wait()
	result.gather(outputs)
	return result
}

func frameOf(input Input) mesh.Frame {
	if input.Map.Origin != nil {
		return mesh.Frame{OriginEast: input.Map.Origin[0], OriginNorth: input.Map.Origin[1]}
	}
	if input.Relief != nil {
		east, north := input.Relief.Centre()
		return mesh.Frame{OriginEast: east, OriginNorth: north}
	}
	return mesh.Frame{}
}

// workerOutput is what one worker built: pieces by material and chunk, roles and warnings.
type workerOutput struct {
	pieces   map[pieceKey]*mesh.Piece
	roles    map[string]int
	warnings []string
}

type pieceKey struct {
	material    string
	column, row int
}

func newWorkerOutput() *workerOutput {
	return &workerOutput{pieces: map[pieceKey]*mesh.Piece{}, roles: map[string]int{}}
}

// piece returns the piece of a material for the chunk a map position falls in.
func (output *workerOutput) piece(material mesh.Material, east, north float64) *mesh.Piece {
	key := pieceKey{material.Name, int(math.Floor(east / chunkSize)), int(math.Floor(north / chunkSize))}
	if piece, known := output.pieces[key]; known {
		return piece
	}
	piece := &mesh.Piece{Name: fmt.Sprintf("%s %d,%d", material.Name, key.column, key.row), Material: material}
	output.pieces[key] = piece
	return piece
}

// gather merges the workers' pieces of the same material and chunk, in a stable order.
func (result *Result) gather(outputs []*workerOutput) {
	merged := map[pieceKey]*mesh.Piece{}
	for _, output := range outputs {
		for key, piece := range output.pieces {
			if existing, known := merged[key]; known {
				existing.Append(piece)
			} else {
				merged[key] = piece
			}
		}
		for role, count := range output.roles {
			result.Roles[role] += count
		}
		result.Warnings = append(result.Warnings, output.warnings...)
	}
	keys := make([]pieceKey, 0, len(merged))
	for key := range merged {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(first, second int) bool {
		if keys[first].material != keys[second].material {
			return keys[first].material < keys[second].material
		}
		if keys[first].column != keys[second].column {
			return keys[first].column < keys[second].column
		}
		return keys[first].row < keys[second].row
	})
	for _, key := range keys {
		result.Pieces = append(result.Pieces, merged[key])
	}
	sort.Strings(result.Warnings)
}

// featureBuilder builds the blocks of features with one GEOS context (contexts are not shared
// between goroutines).
type featureBuilder struct {
	input   Input
	frame   mesh.Frame
	context *geos.Context
	output  *workerOutput
}

func (builder *featureBuilder) warn(feature *mapdata.Feature, format string, values ...any) {
	builder.output.warnings = append(builder.output.warnings, fmt.Sprintf("%s (%s): ", feature.Identifier, feature.Kind)+fmt.Sprintf(format, values...))
}

func (builder *featureBuilder) build(feature *mapdata.Feature) {
	resolved, matched := builder.input.Rules.Resolve(feature.Kind, feature.Properties)
	if !matched {
		builder.output.roles["unstyled"]++
		return
	}
	shape := resolved.Section("shape")
	role := shape.Text("role", "none")
	builder.output.roles[role]++
	var err error
	switch role {
	case "ground":
		err = builder.ground(feature, resolved, shape)
	case "water":
		err = builder.water(feature, resolved, shape)
	case "volume":
		err = builder.volume(feature, resolved, shape)
	case "line":
		err = builder.line(feature, resolved, shape)
	case "none", "point":
	default:
		err = fmt.Errorf("unknown role %q", role)
	}
	if err != nil {
		builder.warn(feature, "%v", err)
	}
}

// height is the relief height in metres at a map position.
func (builder *featureBuilder) height(east, north float64) float64 {
	return builder.input.Relief.Height(east, north)
}

// largestSide is the longest side a draped triangle may have: about a relief cell.
func (builder *featureBuilder) largestSide() float64 {
	if builder.input.Relief == nil {
		return math.Inf(1)
	}
	return math.Max(0.012, builder.input.Relief.Cell)
}

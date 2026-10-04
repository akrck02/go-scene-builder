// Package build turns a map, a relief and style rules into pieces of a scene. It runs stage 1 of the
// scene builder (basic blocks), then stage 2 (relations) and stage 4 (recipes) on the volumes. Each
// feature gets a role from its style ("shape.role"): ground, water, volume, line, point or none;
// the relief becomes the terrain.
package build

import (
	"fmt"
	"math"
	"runtime"
	"sort"
	"sync"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/models"
	"github.com/akrck02/go-scene-builder/recipes"
	"github.com/akrck02/go-scene-builder/relations"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
	"github.com/twpayne/go-geos"
)

// defaultChunk is the side (km) of the squares the pieces are grouped in, so the viewer only draws
// the ones in sight; the style's "chunk" changes it (larger for larger maps).
const defaultChunk = 0.5

// Materials of the basic blocks.
var (
	groundMaterial = mesh.Material{Name: "ground", Kind: "lit", DoubleSided: true, DepthBias: 0.001}
	waterMaterial  = mesh.Material{Name: "water surface", Kind: "lit", DoubleSided: true, DepthBias: 0.001}
	solidMaterial  = mesh.Material{Name: "solid", Kind: "lit"}
)

// Input is what a build needs.
type Input struct {
	Map    *mapdata.Map
	Relief *relief.Relief // the terrain to build, and the ground heights; nil = no terrain
	Rules  *style.Rules
	// Heights gives the ground height (metres) instead of the relief, for scenes whose terrain
	// exists already (a live scene); nil = the relief's, or flat.
	Heights HeightSource
	// Detail is the longest side (km) of a draped triangle when there is no relief; 0 = no limit.
	Detail float64
	// Context are features that take part in the relations (streets a house faces, roads that open
	// gates) without being built.
	Context []*mapdata.Feature
	// Models are the models the "model" role and the scatter recipe place.
	Models *models.Library
	// Layers are grids of categories over the map, by name; "biome" colours the terrain and sows
	// the vegetation of each biome (terrain.biomes in the style).
	Layers map[string]*relief.Relief
}

// HeightSource gives the height of the ground in metres at a map position (km east and north).
type HeightSource interface {
	Height(east, north float64) float64
}

// HeightFunction turns a function into a HeightSource.
type HeightFunction func(east, north float64) float64

// Height calls the function.
func (function HeightFunction) Height(east, north float64) float64 { return function(east, north) }

// related are the features the relations look at: the map's and the context's.
func (input Input) related() []*mapdata.Feature {
	if len(input.Context) == 0 {
		return input.Map.Features
	}
	return append(append([]*mapdata.Feature{}, input.Map.Features...), input.Context...)
}

// Result is the pieces of the scene and what happened.
type Result struct {
	Pieces    []*mesh.Piece
	Instances []*mesh.Instances // models placed many times
	Frame     mesh.Frame
	Roles     map[string]int // features per role
	Blocks    int            // volumes decorated by the recipes
	Facing    int            // blocks with a side onto a street
	Shared    int            // walls a neighbour leans against
	Warnings  []string
}

// Build runs stage 1 for every feature, spread over the processor's cores.
func Build(input Input) *Result {
	result := &Result{Frame: frameOf(input), Roles: map[string]int{}}
	waters, dug := digChannels(input)
	if dug != nil {
		input.Relief = dug
	}
	if input.Relief != nil {
		result.Pieces = append(result.Pieces, terrainPieces(input.Relief, input.Layers["biome"], input.Rules.Terrain, result.Frame)...)
	}
	openings := relations.FindOpenings(input.related(), func(feature *mapdata.Feature) map[string]any {
		resolved, _ := input.Rules.Resolve(feature.Kind, feature.Properties)
		where, _ := resolved.Section("openings")["where"].(map[string]any)
		return where
	})
	chunk := defaultChunk
	if input.Rules.Chunk > 0 {
		chunk = input.Rules.Chunk
	}
	workers := runtime.NumCPU()
	outputs := make([]*workerOutput, workers)
	var group sync.WaitGroup
	for worker := range workers {
		outputs[worker] = newWorkerOutput(chunk)
		group.Add(1)
		go func() {
			defer group.Done()
			builder := &featureBuilder{input: input, frame: result.Frame, context: geos.NewContext(), output: outputs[worker], openings: openings, channels: waters}
			for index := worker; index < len(input.Map.Features); index += workers {
				builder.build(input.Map.Features[index])
			}
		}()
	}
	group.Wait()
	var blocks []*block.Block
	var plantings []planting
	for _, output := range outputs {
		blocks = append(blocks, output.blocks...)
		plantings = append(plantings, output.plantings...)
	}
	relations.Compute(blocks, input.related(), input.Rules.Relations)
	result.count(blocks)
	outputs = append(outputs, decorate(blocks, result.Frame, workers, chunk)...)
	outputs = append(outputs, sow(plantings, blocks, input, result.Frame, chunk)...)
	outputs = append(outputs, sowBiomes(input, blocks, result.Frame, chunk)...)
	result.gather(outputs)
	return result
}

func (result *Result) count(blocks []*block.Block) {
	result.Blocks = len(blocks)
	for _, current := range blocks {
		if current.Facing != nil {
			result.Facing++
		}
		for side := range current.Polygon.Exterior {
			if current.SharedUpTo(side) > current.Bottom {
				result.Shared++
			}
		}
	}
}

// decorate runs the recipes of every block, spread over the processor's cores.
func decorate(blocks []*block.Block, frame mesh.Frame, workers int, chunk float64) []*workerOutput {
	outputs := make([]*workerOutput, workers)
	var group sync.WaitGroup
	for worker := range workers {
		outputs[worker] = newWorkerOutput(chunk)
		group.Add(1)
		go func() {
			defer group.Done()
			decorator := &recipes.Decorator{Frame: frame, Context: geos.NewContext()}
			for index := worker; index < len(blocks); index += workers {
				current := blocks[index]
				first := current.Polygon.Exterior[0]
				pieceFor := func(material mesh.Material) *mesh.Piece { return outputs[worker].piece(material, first[0], first[1]) }
				if err := decorator.Decorate(current, pieceFor); err != nil {
					outputs[worker].warnings = append(outputs[worker].warnings, fmt.Sprintf("%s (%s): %v", current.Feature.Identifier, current.Feature.Kind, err))
				}
			}
		}()
	}
	group.Wait()
	return outputs
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
	chunk     float64
	instances map[pieceKey]*mesh.Instances
	blocks    []*block.Block
	plantings []planting
	pieces    map[pieceKey]*mesh.Piece
	roles     map[string]int
	warnings  []string
}

type pieceKey struct {
	material    string
	column, row int
}

func newWorkerOutput(chunk float64) *workerOutput {
	return &workerOutput{pieces: map[pieceKey]*mesh.Piece{}, instances: map[pieceKey]*mesh.Instances{}, roles: map[string]int{}, chunk: chunk}
}

// piece returns the piece of a material for the chunk a map position falls in.
func (output *workerOutput) piece(material mesh.Material, east, north float64) *mesh.Piece {
	key := pieceKey{material.Name, int(math.Floor(east / output.chunk)), int(math.Floor(north / output.chunk))}
	if piece, known := output.pieces[key]; known {
		return piece
	}
	piece := &mesh.Piece{Name: fmt.Sprintf("%s %d,%d", material.Name, key.column, key.row), Material: material}
	output.pieces[key] = piece
	return piece
}

// copiesOf returns the instances of a model for the chunk a map position falls in.
func (output *workerOutput) copiesOf(model *models.Model, east, north float64) *mesh.Instances {
	key := pieceKey{model.Name, int(math.Floor(east / output.chunk)), int(math.Floor(north / output.chunk))}
	if copies, known := output.instances[key]; known {
		return copies
	}
	copies := &mesh.Instances{Name: fmt.Sprintf("%s %d,%d", model.Name, key.column, key.row), Model: model.Pieces}
	output.instances[key] = copies
	return copies
}

// gather merges the workers' pieces of the same material and chunk, in a stable order.
func (result *Result) gather(outputs []*workerOutput) {
	merged := map[pieceKey]*mesh.Piece{}
	mergedInstances := map[pieceKey]*mesh.Instances{}
	for _, output := range outputs {
		for key, piece := range output.pieces {
			if existing, known := merged[key]; known {
				existing.Append(piece)
			} else {
				merged[key] = piece
			}
		}
		for key, copies := range output.instances {
			if existing, known := mergedInstances[key]; known {
				existing.Append(copies)
			} else {
				mergedInstances[key] = copies
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
		piece := merged[key]
		if piece.Material.HideBelow > 0 {
			piece.Details = append(piece.Details, mesh.Detail{ScreenSize: piece.Material.HideBelow})
		}
		result.Pieces = append(result.Pieces, piece)
	}
	instanceKeys := make([]pieceKey, 0, len(mergedInstances))
	for key := range mergedInstances {
		instanceKeys = append(instanceKeys, key)
	}
	sort.Slice(instanceKeys, func(first, second int) bool {
		return fmt.Sprint(instanceKeys[first]) < fmt.Sprint(instanceKeys[second])
	})
	for _, key := range instanceKeys {
		result.Instances = append(result.Instances, mergedInstances[key])
	}
	sort.Strings(result.Warnings)
}

// featureBuilder builds the blocks of features with one GEOS context (contexts are not shared
// between goroutines).
type featureBuilder struct {
	input    Input
	frame    mesh.Frame
	context  *geos.Context
	output   *workerOutput
	openings relations.Openings
	channels *channels // the rivers dug into the relief; nil without a relief
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
	case "point":
		err = builder.point(feature, resolved, shape)
	case "model":
		err = builder.model(feature, shape)
	case "none":
	default:
		err = fmt.Errorf("unknown role %q", role)
	}
	if err != nil {
		builder.warn(feature, "%v", err)
	}
}

// height is the relief height in metres at a map position.
func (builder *featureBuilder) height(east, north float64) float64 {
	if builder.input.Heights != nil {
		return builder.input.Heights.Height(east, north)
	}
	return builder.input.Relief.Height(east, north)
}

// largestSide is the longest side a draped triangle may have: about a relief cell.
func (builder *featureBuilder) largestSide() float64 {
	if builder.input.Relief == nil {
		if builder.input.Detail > 0 {
			return builder.input.Detail
		}
		return math.Inf(1)
	}
	return math.Max(0.012, builder.input.Relief.Cell)
}

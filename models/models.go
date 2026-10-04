// Package models is a library of small 3D models (flowers, lamps, benches, trees…) that maps
// place many times: each model is built once and drawn as instances. A model is made of simple
// parts written in YAML, in metres, with the origin on the ground, x to the east, y up and z to
// the south:
//
//	models:
//	  lily:
//	    parts:
//	      - {shape: cylinder, radius: 0.01, height: 0.35, color: "#4f6b35"}               # stem
//	      - {shape: petals, count: 6, length: 0.07, width: 0.035, tilt: 35, at: [0, 0.35, 0],
//	         color: ["#7da7e0", "#f2f0e8"], material: {kind: glow}}
//	  farol:
//	    file: farol.glb            # or a model made in Blender, say (glTF binary)
//
// Shapes: box (size [x, y, z]), cylinder and cone (radius, height, sides; a cylinder can narrow to
// top_radius), blob (radius or size [x, y, z], sides, rings; smooth unless smooth: false; wobble:
// a share of its size that bends it out of round, like a tree crown), petals (count, length, width,
// tilt in degrees from flat), ogive (a pointed arch: width, straight sides, depth; with border, a
// frame around an opening of the same shape). Every part can be placed (at: [x, y, z]) and turned around the
// vertical axis (turn: degrees), and has a colour (or a palette: each part takes one) and a
// material.
//
// Variety: a model can be built in several variants (variants: 3), whose parts move ("move",
// metres) and change size ("grow", a share) a little each time, and every copy placed on a map
// can change its colour, warmth and size (vary):
//
//	caducifolio:
//	  variants: 4
//	  vary: {color: 0.12, warmth: 0.06, scale: [0.8, 1.25]}
//	  parts:
//	    - {shape: blob, size: [6, 6, 6], at: [0, 4, 0], wobble: 0.15, move: 0.8, grow: 0.2, color: "#56713a"}
package models

import (
	"fmt"
	"hash/fnv"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"

	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/recipes"
	"github.com/akrck02/go-scene-builder/style"
	"gopkg.in/yaml.v3"
)

// Model is built once: a piece per material, in scene units (km) around the origin.
type Model struct {
	Name     string
	Pieces   []*mesh.Piece
	Variants []*Model // the model's variants, itself first; nil = only itself
	Vary     Variation
}

// Variation is how much each placed copy of a model changes.
type Variation struct {
	Color  float64    // share of lightness, up or down
	Warmth float64    // share of red against blue, up or down
	Scale  [2]float64 // range of sizes; zero = always 1
}

// Copy chooses how one more copy of the model looks: which variant, a size factor and a tint.
func (model *Model) Copy(random *rand.Rand) (*Model, float64, [4]float32) {
	chosen := model
	if len(model.Variants) > 1 {
		chosen = model.Variants[random.IntN(len(model.Variants))]
	}
	scale := 1.0
	if model.Vary.Scale[1] > 0 {
		scale = model.Vary.Scale[0] + (model.Vary.Scale[1]-model.Vary.Scale[0])*random.Float64()
	}
	lightness := 1 + model.Vary.Color*(2*random.Float64()-1)
	warmth := model.Vary.Warmth * (2*random.Float64() - 1)
	return chosen, scale, [4]float32{float32(lightness * (1 + warmth)), float32(lightness), float32(lightness * (1 - warmth)), 1}
}

// Library holds the models by name.
type Library struct {
	models map[string]*Model
}

// Model returns a model by name, or nil.
func (library *Library) Model(name string) *Model {
	if library == nil {
		return nil
	}
	return library.models[name]
}

// Names lists the models of the library.
func (library *Library) Names() []string {
	if library == nil {
		return nil
	}
	names := make([]string, 0, len(library.models))
	for name := range library.models {
		names = append(names, name)
	}
	return names
}

// NewLibrary is a library of models built in code.
func NewLibrary(models ...*Model) *Library {
	library := &Library{models: map[string]*Model{}}
	for _, model := range models {
		library.models[model.Name] = model
	}
	return library
}

type libraryFile struct {
	Models map[string]modelFile `yaml:"models"`
}

type modelFile struct {
	Parts    []map[string]any `yaml:"parts"`
	File     string           `yaml:"file"`
	Levels   []levelFile      `yaml:"levels"`
	Variants int              `yaml:"variants"`
	Vary     struct {
		Color  float64   `yaml:"color"`
		Warmth float64   `yaml:"warmth"`
		Scale  []float64 `yaml:"scale"`
	} `yaml:"vary"`
}

// levelFile is a simpler version of a model, drawn when it looks smaller than "below" (a share of
// the screen); without parts, the model is not drawn at all:
//
//	levels:
//	  - {below: 0.05, parts: [{shape: blob, size: [6, 8, 6], at: [0, 2, 0], color: "#56713a"}]}
//	  - {below: 0.01}
type levelFile struct {
	Below float64          `yaml:"below"`
	Parts []map[string]any `yaml:"parts"`
}

// Load reads one or more library files (later files add or replace models).
func Load(paths ...string) (*Library, error) {
	library := &Library{models: map[string]*Model{}}
	for _, path := range paths {
		if err := library.load(path); err != nil {
			return nil, fmt.Errorf("models %s: %w", path, err)
		}
	}
	return library, nil
}

func (library *Library) load(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var file libraryFile
	if err := yaml.Unmarshal(content, &file); err != nil {
		return err
	}
	for name, definition := range file.Models {
		model, err := buildModel(name, definition, filepath.Dir(path))
		if err != nil {
			return fmt.Errorf("model %q: %w", name, err)
		}
		library.models[name] = model
	}
	return nil
}

// buildModel builds a model and its variants (a model from a file has no variants).
func buildModel(name string, definition modelFile, directory string) (*Model, error) {
	model := &Model{Name: name, Vary: Variation{Color: definition.Vary.Color, Warmth: definition.Vary.Warmth}}
	if scale := definition.Vary.Scale; len(scale) == 2 {
		model.Vary.Scale = [2]float64{scale[0], scale[1]}
	}
	if definition.File != "" {
		pieces, err := readGLB(filepath.Join(directory, definition.File))
		if err != nil {
			return nil, err
		}
		model.Pieces = pieces
		return model, addLevels(model, definition.Levels)
	}
	for variant := range max(1, definition.Variants) {
		current := model
		if variant > 0 {
			current = &Model{Name: fmt.Sprintf("%s %d", name, variant+1)}
		}
		pieces, err := buildParts(name, definition.Parts, variant)
		if err != nil {
			return nil, err
		}
		current.Pieces = pieces
		if err := addLevels(current, definition.Levels); err != nil {
			return nil, err
		}
		if definition.Variants > 1 {
			model.Variants = append(model.Variants, current)
		}
	}
	return model, nil
}

// addLevels gives every piece of a model its simpler versions: the level's piece of the same
// material, or nothing.
func addLevels(model *Model, levels []levelFile) error {
	for _, level := range levels {
		simpler, err := buildParts(model.Name, level.Parts, 0)
		if err != nil {
			return err
		}
		byMaterial := map[string]*mesh.Piece{}
		for _, piece := range simpler {
			byMaterial[piece.Material.Name] = piece
		}
		for _, piece := range model.Pieces {
			piece.Details = append(piece.Details, mesh.Detail{Piece: byMaterial[piece.Material.Name], ScreenSize: level.Below})
		}
	}
	return nil
}

// buildParts builds a model's parts, each into the piece of its material. Variants after the first
// move and resize their parts a little ("move" and "grow"); every variant bends its blobs its own way.
func buildParts(name string, parts []map[string]any, variant int) ([]*mesh.Piece, error) {
	pieces := map[string]*mesh.Piece{}
	var order []*mesh.Piece
	for index, values := range parts {
		seed := seedOf(name, index, variant)
		part := style.Style(values)
		if variant > 0 {
			part = varied(part, rand.New(rand.NewPCG(seed, 11)))
		}
		material := recipes.MaterialOf(mesh.Material{Name: "model", Kind: "lit"}, part)
		piece := pieces[material.Name]
		if piece == nil {
			piece = &mesh.Piece{Name: name + " " + material.Name, Material: material}
			pieces[material.Name] = piece
			order = append(order, piece)
		}
		color, err := part.Color("color", map[string]any{"id": fmt.Sprint(name, index)}, [4]float32{0.7, 0.7, 0.7, 1})
		if err != nil {
			return nil, err
		}
		if err := addPart(piece, part, color, seed); err != nil {
			return nil, fmt.Errorf("part %d: %w", index+1, err)
		}
	}
	return order, nil
}

// placement turns a part's local metres (x east, y up, z south) into model coordinates (km).
type placement struct {
	at   [3]float64
	turn float64 // radians around the vertical
}

func (place placement) point(x, y, z float64) mesh.Vector {
	cosine, sine := math.Cos(place.turn), math.Sin(place.turn)
	return mesh.Vector{(place.at[0] + x*cosine - z*sine) / 1000, (place.at[1] + y) / 1000, (place.at[2] + x*sine + z*cosine) / 1000}
}

// direction turns a local direction like the placement turns points (for normals).
func (place placement) direction(x, y, z float64) mesh.Vector {
	cosine, sine := math.Cos(place.turn), math.Sin(place.turn)
	return mesh.Vector{x*cosine - z*sine, y, x*sine + z*cosine}
}

func placementOf(part style.Style) placement {
	place := placement{}
	if values, isList := part["at"].([]any); isList && len(values) == 3 {
		for axis := range 3 {
			place.at[axis], _ = number(values[axis])
		}
	}
	turn, _ := part.Number("turn", nil, 0)
	place.turn = turn * math.Pi / 180
	return place
}

// seedOf is a repeatable seed for one part of one variant of a model.
func seedOf(name string, part, variant int) uint64 {
	hash := fnv.New64a()
	fmt.Fprintf(hash, "%s/%d/%d", name, part, variant)
	return hash.Sum64()
}

// varied is a copy of a part moved by up to "move" metres and resized by up to "grow" (a share).
func varied(part style.Style, random *rand.Rand) style.Style {
	move, _ := part.Number("move", nil, 0)
	grow, _ := part.Number("grow", nil, 0)
	if move == 0 && grow == 0 {
		return part
	}
	copied := style.Style{}
	for key, value := range part {
		copied[key] = value
	}
	shift := func() float64 { return move * (2*random.Float64() - 1) }
	at := triple(part["at"], [3]float64{})
	copied["at"] = []any{at[0] + shift(), at[1] + shift()/2, at[2] + shift()}
	factor := 1 + grow*(2*random.Float64()-1)
	if _, sized := part["size"]; sized {
		size := triple(part["size"], [3]float64{0.1, 0.1, 0.1})
		copied["size"] = []any{size[0] * factor, size[1] * factor, size[2] * factor}
	}
	for _, key := range []string{"radius", "top_radius", "height", "length"} {
		if value, present := number(part[key]); present {
			copied[key] = value * factor
		}
	}
	return copied
}

func addPart(piece *mesh.Piece, part style.Style, color [4]float32, seed uint64) error {
	place := placementOf(part)
	sides, _ := part.Number("sides", nil, 8)
	switch shape := part.Text("shape", ""); shape {
	case "box":
		size := triple(part["size"], [3]float64{0.1, 0.1, 0.1})
		addBox(piece, place, size, color)
	case "cylinder", "cone":
		radius, _ := part.Number("radius", nil, 0.05)
		height, _ := part.Number("height", nil, 0.2)
		top, _ := part.Number("top_radius", nil, radius)
		if shape == "cone" {
			top = 0
		}
		addCylinder(piece, place, radius, top, height, int(sides), color)
	case "blob":
		radius, _ := part.Number("radius", nil, 0.1)
		size := triple(part["size"], [3]float64{2 * radius, 2 * radius, 2 * radius})
		rings, _ := part.Number("rings", nil, math.Max(4, math.Round(sides/2)))
		wobble, _ := part.Number("wobble", nil, 0)
		smooth := part["smooth"] != false
		addBlob(piece, place, blobShape{size: size, sides: int(sides), rings: int(rings), wobble: wobble, seed: seed, smooth: smooth}, color)
	case "ogive":
		width, _ := part.Number("width", nil, 1)
		straight, _ := part.Number("straight", nil, 1)
		depth, _ := part.Number("depth", nil, 0.2)
		border, _ := part.Number("border", nil, 0)
		steps, _ := part.Number("steps", nil, 6)
		addOgive(piece, place, width, straight, depth, border, int(steps), color)
	case "petals":
		count, _ := part.Number("count", nil, 5)
		length, _ := part.Number("length", nil, 0.05)
		width, _ := part.Number("width", nil, 0.025)
		tilt, _ := part.Number("tilt", nil, 30)
		addPetals(piece, place, int(count), length, width, tilt*math.Pi/180, color)
	default:
		return fmt.Errorf("unknown shape %q", shape)
	}
	return nil
}

func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	}
	return 0, false
}

func triple(value any, fallback [3]float64) [3]float64 {
	values, isList := value.([]any)
	if !isList || len(values) != 3 {
		return fallback
	}
	var result [3]float64
	for axis := range 3 {
		result[axis], _ = number(values[axis])
	}
	return result
}

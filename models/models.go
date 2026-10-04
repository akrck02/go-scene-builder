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
// Shapes: box (size [x, y, z]), cylinder and cone (radius, height, sides), blob (radius or size
// [x, y, z], sides), petals (count, length, width, tilt in degrees from flat). Every part can be
// placed (at: [x, y, z]) and turned around the vertical axis (turn: degrees), and has a colour
// (or a palette: each part takes one) and a material.
package models

import (
	"fmt"
	"math"
	"os"
	"path/filepath"

	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/recipes"
	"github.com/akrck02/go-scene-builder/style"
	"gopkg.in/yaml.v3"
)

// Model is built once: a piece per material, in scene units (km) around the origin.
type Model struct {
	Name   string
	Pieces []*mesh.Piece
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
	Parts []map[string]any `yaml:"parts"`
	File  string           `yaml:"file"`
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
		model := &Model{Name: name}
		if definition.File != "" {
			model.Pieces, err = readGLB(filepath.Join(filepath.Dir(path), definition.File))
		} else {
			model.Pieces, err = buildParts(name, definition.Parts)
		}
		if err != nil {
			return fmt.Errorf("model %q: %w", name, err)
		}
		library.models[name] = model
	}
	return nil
}

// buildParts builds a model's parts, each into the piece of its material.
func buildParts(name string, parts []map[string]any) ([]*mesh.Piece, error) {
	pieces := map[string]*mesh.Piece{}
	var order []*mesh.Piece
	for index, values := range parts {
		part := style.Style(values)
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
		if err := addPart(piece, part, color); err != nil {
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

func addPart(piece *mesh.Piece, part style.Style, color [4]float32) error {
	place := placementOf(part)
	sides, _ := part.Number("sides", nil, 8)
	switch shape := part.Text("shape", ""); shape {
	case "box":
		size := triple(part["size"], [3]float64{0.1, 0.1, 0.1})
		addBox(piece, place, size, color)
	case "cylinder", "cone":
		radius, _ := part.Number("radius", nil, 0.05)
		height, _ := part.Number("height", nil, 0.2)
		top := radius
		if shape == "cone" {
			top = 0
		}
		addCylinder(piece, place, radius, top, height, int(sides), color)
	case "blob":
		radius, _ := part.Number("radius", nil, 0.1)
		size := triple(part["size"], [3]float64{2 * radius, 2 * radius, 2 * radius})
		addBlob(piece, place, size, int(sides), color)
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

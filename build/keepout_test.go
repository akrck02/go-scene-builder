package build

import (
	"math"
	"testing"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/models"
	"github.com/akrck02/go-scene-builder/style"
)

// stall is a model 1 m across: a flat square around its origin.
func stall() *models.Model {
	piece := &mesh.Piece{Name: "stall", Material: mesh.Material{Name: "model", Kind: "lit"}}
	half := 0.0005
	piece.AddQuad(mesh.Vector{-half, 0.001, -half}, mesh.Vector{half, 0.001, -half}, mesh.Vector{half, 0.001, half}, mesh.Vector{-half, 0.001, half},
		mesh.Vector{0, 1, 0}, [4]float32{1, 1, 1, 1})
	return &models.Model{Name: "stall", Pieces: []*mesh.Piece{piece}}
}

func TestDecorationsKeepTheirRules(test *testing.T) {
	rules, err := style.Parse([]byte(`
units: m
rules:
  - when: {kind: square}
    shape: {role: ground}
    decorate: [{recipe: scatter, density: 0.5, models: {stall: 1}, spacing: 0.5}]
  - when: {kind: street}
    shape: {role: ground, width: 6}
  - when: {kind: fountain}
    shape: {role: model, model: fountain}
`))
	if err != nil {
		test.Fatal(err)
	}
	square := &mapdata.Feature{Identifier: "square", Kind: "square", Polygons: []mapdata.Polygon{square(0.04)}}
	street := &mapdata.Feature{Identifier: "street", Kind: "street", Lines: [][]mapdata.Point{{{0, 0.01}, {0.04, 0.01}}}}
	fountain := &mapdata.Feature{Identifier: "fountain", Kind: "fountain", Points: []mapdata.Point{{0.025, 0.028}}}
	library := models.NewLibrary(stall(), &models.Model{Name: "fountain", Pieces: []*mesh.Piece{scaled(stall().Pieces[0], 8)}})
	result := Build(Input{Map: &mapdata.Map{Features: []*mapdata.Feature{square, street, fountain}}, Rules: rules, Models: library})
	var spots []mapdata.Point
	for _, copies := range result.Instances {
		if copies.Name[:5] != "stall" {
			continue
		}
		for index := 0; index+2 < len(copies.Positions); index += 3 {
			spots = append(spots, mapdata.Point{float64(copies.Positions[index]) + result.Frame.OriginEast, result.Frame.OriginNorth - float64(copies.Positions[index+2])})
		}
	}
	if len(spots) < 50 {
		test.Fatalf("only %d stalls placed", len(spots))
	}
	for index, spot := range spots {
		if math.Abs(spot[1]-0.01) < 0.003+0.0008+0.0005 {
			test.Errorf("a stall on the street at %v", spot)
		}
		if mapdata.Distance(spot, fountain.Points[0]) < 0.004*math.Sqrt2+0.0005 {
			test.Errorf("a stall against the fountain at %v", spot)
		}
		for _, other := range spots[index+1:] {
			if mapdata.Distance(spot, other) < 2*0.0005*math.Sqrt2 {
				test.Fatalf("two stalls overlap at %v and %v", spot, other)
			}
		}
	}
}

func scaled(piece *mesh.Piece, factor float32) *mesh.Piece {
	copied := *piece
	copied.Positions = make([]float32, len(piece.Positions))
	for index, value := range piece.Positions {
		copied.Positions[index] = value * factor
	}
	return &copied
}

package build

import (
	"testing"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
)

const testRules = `
units: m
rules:
  - when: {kind: building}
    shape: {role: volume, height: "3 * floors", sink: 0}
  - when: {kind: street}
    shape: {role: ground, width: 6}
    color: "#e6dcc4"
  - when: {kind: wall}
    shape: {role: line, height: 9, thickness: 2, segment: 10, sink: 0}
`

func testInput(test *testing.T, features ...*mapdata.Feature) Input {
	rules, err := style.Parse([]byte(testRules))
	if err != nil {
		test.Fatal(err)
	}
	return Input{Map: &mapdata.Map{Features: features}, Rules: rules}
}

func square(side float64) mapdata.Polygon {
	return mapdata.Polygon{Exterior: mapdata.Ring{{0, 0}, {side, 0}, {side, side}, {0, side}}}
}

func TestVolumeFacesOutwards(test *testing.T) {
	building := &mapdata.Feature{Identifier: "house", Kind: "building", Properties: map[string]any{"floors": 2.0},
		Polygons: []mapdata.Polygon{square(0.01)}}
	result := Build(testInput(test, building))
	if len(result.Pieces) != 1 {
		test.Fatalf("one piece expected, got %d", len(result.Pieces))
	}
	piece := result.Pieces[0]
	if piece.TriangleCount() != 4*2+2 {
		test.Errorf("4 walls and a top: %d triangles", piece.TriangleCount())
	}
	centre := mesh.Vector{0.005, 0.003, -0.005}
	for triangle := 0; triangle < piece.TriangleCount(); triangle++ {
		first := piece.Indices[3*triangle]
		position := mesh.Vector{float64(piece.Positions[3*first]), float64(piece.Positions[3*first+1]), float64(piece.Positions[3*first+2])}
		normal := mesh.Vector{float64(piece.Normals[3*first]), float64(piece.Normals[3*first+1]), float64(piece.Normals[3*first+2])}
		if normal.Dot(position.Subtract(centre)) <= 0 {
			test.Errorf("triangle %d faces inwards", triangle)
		}
	}
	if top := maxHeight(piece); top < 0.0059 || top > 0.0061 {
		test.Errorf("top at %v km, want 0.006 (2 floors × 3 m)", top)
	}
}

func TestGroundLineAndWall(test *testing.T) {
	street := &mapdata.Feature{Identifier: "street", Kind: "street", Lines: [][]mapdata.Point{{{0, 0}, {0.1, 0}}}}
	wall := &mapdata.Feature{Identifier: "wall", Kind: "wall", Lines: [][]mapdata.Point{{{0, 0.05}, {0.1, 0.05}}}}
	unknown := &mapdata.Feature{Identifier: "tree", Kind: "tree"}
	result := Build(testInput(test, street, wall, unknown))
	if result.Roles["ground"] != 1 || result.Roles["line"] != 1 || result.Roles["unstyled"] != 1 {
		test.Errorf("roles %v", result.Roles)
	}
	triangles := map[string]int{}
	for _, piece := range result.Pieces {
		triangles[piece.Material.Name] += piece.TriangleCount()
	}
	if triangles["ground"] == 0 {
		test.Error("the street should be draped")
	}
	if triangles["solid"] != 10*(4*2+2) {
		test.Errorf("a 100 m wall in 10 m pieces: %d triangles, want %d", triangles["solid"], 10*(4*2+2))
	}
}

func maxHeight(piece *mesh.Piece) float64 {
	highest := 0.0
	for index := 1; index < len(piece.Positions); index += 3 {
		highest = max(highest, float64(piece.Positions[index]))
	}
	return highest
}

func TestNothingOutsideTheRelief(test *testing.T) {
	street := &mapdata.Feature{Identifier: "street", Kind: "street", Lines: [][]mapdata.Point{{{0.005, 0.01}, {0.5, 0.01}}}}
	house := &mapdata.Feature{Identifier: "far", Kind: "building", Properties: map[string]any{"floors": 1.0},
		Polygons: []mapdata.Polygon{{Exterior: mapdata.Ring{{1, 1}, {1.01, 1}, {1.01, 1.01}, {1, 1.01}}}}}
	input := testInput(test, street, house)
	input.Relief = &relief.Relief{Columns: 2, Rows: 2, Cell: 0.01, West: 0, North: 0.02, Heights: make([]float32, 4)}
	input.Map.Origin = &mapdata.Point{0, 0}
	result := Build(input)
	for _, piece := range result.Pieces {
		if piece.Ground {
			continue // terrain
		}
		for index := 0; index < len(piece.Positions); index += 3 {
			if east := float64(piece.Positions[index]); east > 0.0201 {
				test.Fatalf("%s reaches %v km east, beyond the relief", piece.Name, east)
			}
		}
		if piece.Material.Name == "solid" {
			test.Error("the house outside the relief should be skipped")
		}
	}
}

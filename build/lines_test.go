package build

import (
	"math"
	"testing"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/style"
)

const gateRules = `
units: m
rules:
  - when: {kind: road}
    shape: {role: none}
  - when: {kind: wall}
    shape: {role: line, height: 9, thickness: 2, segment: 1000, sink: 0}
    openings: {where: {kind: road}, width: 6}
    decorate: [{recipe: gate_towers, size: 4, height: 12, sink: 0}]
  - when: {kind: bridge}
    shape: {role: line, follow: level, height: 0.5, thickness: 4, sink: 1}
  - when: {kind: tower}
    shape: {role: point, size: 6, height: 15, form: round, sink: 0}
`

func gateInput(test *testing.T, features ...*mapdata.Feature) Input {
	rules, err := style.Parse([]byte(gateRules))
	if err != nil {
		test.Fatal(err)
	}
	return Input{Map: &mapdata.Map{Features: features}, Rules: rules}
}

func TestGateInAWall(test *testing.T) {
	wall := &mapdata.Feature{Identifier: "wall", Kind: "wall", Lines: [][]mapdata.Point{{{0, 0}, {0.1, 0}}}}
	road := &mapdata.Feature{Identifier: "road", Kind: "road", Lines: [][]mapdata.Point{{{0.05, -0.02}, {0.05, 0.02}}}}
	result := Build(gateInput(test, wall, road))
	solid := result.Pieces[0]
	// two stretches of wall and two towers: four prisms of 10 triangles
	if solid.TriangleCount() != 4*10 {
		test.Fatalf("%d triangles, want 40", solid.TriangleCount())
	}
	for index := 0; index < len(solid.Positions); index += 3 {
		east, height := float64(solid.Positions[index]), float64(solid.Positions[index+1])
		if height < 0.0089 && east > 0.0471 && east < 0.0529 {
			test.Errorf("wall inside the 6 m gate at %v km", east)
		}
	}
	if top := maxHeight(solid); math.Abs(top-0.012) > 1e-6 {
		test.Errorf("towers should reach 12 m, highest point %v km", top)
	}
}

func TestBridgeAndRoundTower(test *testing.T) {
	bridge := &mapdata.Feature{Identifier: "bridge", Kind: "bridge", Lines: [][]mapdata.Point{{{0, 0}, {0.03, 0}}}}
	tower := &mapdata.Feature{Identifier: "tower", Kind: "tower", Points: []mapdata.Point{{0.1, 0.1}}}
	result := Build(gateInput(test, bridge, tower))
	if result.Blocks != 1 {
		test.Errorf("the tower should be a block, got %d blocks", result.Blocks)
	}
	triangles := 0
	for _, piece := range result.Pieces {
		triangles += piece.TriangleCount()
	}
	// bridge prism 10; round tower 16 walls (32) and a flat top (14)
	if triangles != 10+32+14 {
		test.Errorf("%d triangles, want %d", triangles, 10+32+14)
	}
}

func TestBattlementsAndArchedGate(test *testing.T) {
	rules, err := style.Parse([]byte(`
units: m
rules:
  - when: {kind: road}
    shape: {role: none}
  - when: {kind: wall}
    shape: {role: line, height: 30, thickness: 8, segment: 1000, sink: 0, battlements: {height: 2, width: 2, gap: 2}}
    openings: {where: {kind: road}, width: 10}
    decorate: [{recipe: gate_towers, size: 10, height: 40, sink: 0, arch: 18}]
`))
	if err != nil {
		test.Fatal(err)
	}
	wall := &mapdata.Feature{Identifier: "wall", Kind: "wall", Lines: [][]mapdata.Point{{{0, 0}, {0.1, 0}}}}
	road := &mapdata.Feature{Identifier: "road", Kind: "road", Lines: [][]mapdata.Point{{{0.05, -0.02}, {0.05, 0.02}}}}
	solid := Build(Input{Map: &mapdata.Map{Features: []*mapdata.Feature{wall, road}}, Rules: rules}).Pieces[0]
	merlonTops, overTheGate := 0, false
	for index := 0; index < len(solid.Positions); index += 3 {
		east, height := float64(solid.Positions[index]), float64(solid.Positions[index+1])
		if math.Abs(height-0.032) < 1e-6 {
			merlonTops++
		}
		if east > 0.0451 && east < 0.0549 && height < 0.018-1e-6 {
			test.Errorf("something under the 18 m arch at %v km, %v m", east, height*1000)
		}
		if east > 0.0449 && east < 0.0551 && math.Abs(height-0.018) < 1e-6 {
			overTheGate = true // the underside of the arch, from side to side of the gate
		}
	}
	if merlonTops == 0 {
		test.Error("no battlements on top of the wall")
	}
	if !overTheGate {
		test.Error("the gate is not closed above its arch")
	}
}

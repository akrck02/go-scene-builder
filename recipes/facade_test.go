package recipes

import (
	"math"
	"testing"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/style"
	"github.com/twpayne/go-geos"
)

func TestFacadeOnTheStreetSide(test *testing.T) {
	ring := mapdata.Ring{{0, 0}, {0.008, 0}, {0.008, 0.01}, {0, 0.01}}
	facade := map[string]any{
		"door":         map[string]any{"color": "#553322"},
		"windows":      map[string]any{"shutters": []any{"#335544", "#884433"}},
		"flower_boxes": map[string]any{"share": 1.0, "flowers": []any{"#dd3344"}, "glowing": map[string]any{"share": 0.5, "color": "#ddeeff"}},
		"emblem":       map[string]any{"size": 0.6, "color": "#d9dee6"},
		"lamp":         map[string]any{"share": 1.0},
	}
	current := &block.Block{Feature: &mapdata.Feature{Identifier: "house", Properties: map[string]any{"id": "house"}},
		Polygon: mapdata.Polygon{Exterior: ring}, Ground: 0, Bottom: -1, Eaves: 6.5,
		Facing: &block.Facing{Side: 0, Outward: mapdata.Point{0, -1}},
		Style:  style.Style{"facade": facade, "roof": map[string]any{"recipe": "flat"}}}
	pieces := map[string]*mesh.Piece{}
	pieceFor := func(material mesh.Material) *mesh.Piece {
		if pieces[material.Name] == nil {
			pieces[material.Name] = &mesh.Piece{Material: material}
		}
		return pieces[material.Name]
	}
	if err := (&Decorator{Context: geos.NewContext()}).Decorate(current, pieceFor); err != nil {
		test.Fatal(err)
	}
	for _, name := range []string{"glass", "details", "glowing details", "walls", "roofs"} {
		if pieces[name] == nil || pieces[name].TriangleCount() == 0 {
			test.Errorf("no %s", name)
		}
	}
	if pieces["flowers"] == nil && pieces["glowing details"] == nil {
		test.Error("upper windows should have flower boxes")
	}
	// everything of the facade stands out of the south side (z grows to the south in the scene)
	for _, name := range []string{"glass", "details"} {
		for index := 2; index < len(pieces[name].Positions); index += 3 {
			if pieces[name].Positions[index] < -0.0000001 {
				test.Fatalf("%s behind the facade at z = %v", name, pieces[name].Positions[index])
			}
		}
	}
}

func TestRoundDoorBasketsAndIvy(test *testing.T) {
	ring := mapdata.Ring{{0, 0}, {0.008, 0}, {0.008, 0.01}, {0, 0.01}}
	facade := map[string]any{
		"door":    map[string]any{"round": 1.0},
		"baskets": map[string]any{"share": 1.0, "flowers": []any{"#dd3344"}},
		"ivy":     map[string]any{"share": 1.0, "color": []any{"#3f6b32"}, "height": []any{3.0, 5.0}},
	}
	current := &block.Block{Feature: &mapdata.Feature{Identifier: "house", Properties: map[string]any{"id": "house"}},
		Polygon: mapdata.Polygon{Exterior: ring}, Ground: 0, Bottom: -1, Eaves: 6.5,
		Facing: &block.Facing{Side: 0, Outward: mapdata.Point{0, -1}},
		Style:  style.Style{"facade": facade, "roof": map[string]any{"recipe": "flat"}}}
	pieces := map[string]*mesh.Piece{}
	pieceFor := func(material mesh.Material) *mesh.Piece {
		if pieces[material.Name] == nil {
			pieces[material.Name] = &mesh.Piece{Material: material}
		}
		return pieces[material.Name]
	}
	if err := (&Decorator{Context: geos.NewContext()}).Decorate(current, pieceFor); err != nil {
		test.Fatal(err)
	}
	if ivy := pieces["ivy"]; ivy == nil || ivy.TriangleCount() == 0 || !ivy.Material.Foliage {
		test.Fatal("the ivy is missing or does not take the season")
	}
	if pieces["flowers"] == nil {
		test.Error("no flowers in the baskets")
	}
	top := 0.0
	for index := 1; index < len(pieces["details"].Positions); index += 3 {
		top = max(top, float64(pieces["details"].Positions[index]))
	}
	if top < 0.0021 { // the hooks of the baskets reach 2.15 m; a round door 2.3 m
		test.Errorf("details reach only %v m", top*1000)
	}
}

func TestChimneyRisesAboveTheRidge(test *testing.T) {
	ring := mapdata.Ring{{0, 0}, {0.01, 0}, {0.01, 0.006}, {0, 0.006}}
	current := &block.Block{Feature: &mapdata.Feature{Identifier: "house", Properties: map[string]any{"id": "house"}},
		Polygon: mapdata.Polygon{Exterior: ring}, Ground: 0, Bottom: -1, Eaves: 6,
		Style: style.Style{"roof": map[string]any{"recipe": "gable", "pitch": 40.0, "chimneys": map[string]any{"share": 1.0, "above": 1.0}}}}
	pieces := map[string]*mesh.Piece{}
	pieceFor := func(material mesh.Material) *mesh.Piece {
		if pieces[material.Name] == nil {
			pieces[material.Name] = &mesh.Piece{Material: material}
		}
		return pieces[material.Name]
	}
	if err := (&Decorator{Context: geos.NewContext()}).Decorate(current, pieceFor); err != nil {
		test.Fatal(err)
	}
	chimney := pieces["chimneys"]
	if chimney == nil || chimney.TriangleCount() == 0 {
		test.Fatal("no chimney")
	}
	top := 0.0
	for index := 1; index < len(chimney.Positions); index += 3 {
		top = max(top, float64(chimney.Positions[index]))
	}
	ridge := 6 + 3*math.Tan(40*math.Pi/180) // half the 6 m width
	if top*1000 < ridge+0.9 {
		test.Errorf("the chimney tops out at %.2f m, the ridge is at %.2f m", top*1000, ridge)
	}
}

func TestIrregularHouseKeepsItsWalls(test *testing.T) {
	// a pentagon: far from its rectangle, so walls stay on it and a tent roof covers it
	ring := mapdata.Ring{{0, 0}, {0.01, 0}, {0.012, 0.005}, {0.006, 0.009}, {0, 0.006}}
	current := &block.Block{Feature: &mapdata.Feature{Identifier: "house", Properties: map[string]any{"id": "house"}},
		Polygon: mapdata.Polygon{Exterior: ring}, Ground: 0, Bottom: -1, Eaves: 6,
		Style: style.Style{"roof": map[string]any{"recipe": "gable", "pitch": 40.0, "eaves": 0.0}}}
	pieces := map[string]*mesh.Piece{}
	pieceFor := func(material mesh.Material) *mesh.Piece {
		if pieces[material.Name] == nil {
			pieces[material.Name] = &mesh.Piece{Material: material}
		}
		return pieces[material.Name]
	}
	if err := (&Decorator{Context: geos.NewContext()}).Decorate(current, pieceFor); err != nil {
		test.Fatal(err)
	}
	walls := pieces["walls"]
	if walls == nil || walls.TriangleCount() != 2*len(ring) {
		test.Fatalf("one wall per side of the footprint expected, got %d triangles", walls.TriangleCount())
	}
	if roofs := pieces["roofs"]; roofs == nil || roofs.TriangleCount() != len(ring) {
		test.Errorf("a tent roof has a slope per side")
	}
}

func TestSideWindowsOnOpenWallsOnly(test *testing.T) {
	ring := mapdata.Ring{{0, 0}, {0.008, 0}, {0.008, 0.01}, {0, 0.01}}
	facade := map[string]any{"sides": map[string]any{"spacing": 3.0}}
	// a house at the back of its plot: no street near, a neighbour against its west side
	current := &block.Block{Feature: &mapdata.Feature{Identifier: "back", Properties: map[string]any{"id": "back"}},
		Polygon: mapdata.Polygon{Exterior: ring}, Ground: 0, Bottom: -1, Eaves: 6.5,
		Covered: []float64{math.Inf(-1), math.Inf(-1), math.Inf(-1), 6.5},
		Style:   style.Style{"facade": facade, "roof": map[string]any{"recipe": "flat"}}}
	pieces := map[string]*mesh.Piece{}
	pieceFor := func(material mesh.Material) *mesh.Piece {
		if pieces[material.Name] == nil {
			pieces[material.Name] = &mesh.Piece{Material: material}
		}
		return pieces[material.Name]
	}
	if err := (&Decorator{Context: geos.NewContext()}).Decorate(current, pieceFor); err != nil {
		test.Fatal(err)
	}
	if pieces["glass"] == nil {
		test.Fatal("open walls should have windows")
	}
	outside := map[string]int{}
	for index := 0; index < len(pieces["glass"].Positions); index += 3 {
		east, south := float64(pieces["glass"].Positions[index]), float64(pieces["glass"].Positions[index+2])
		switch {
		case east < -0.000001:
			outside["west"]++
		case east > 0.008000001:
			outside["east"]++
		case south > 0.000001:
			outside["south"]++
		case south < -0.010000001:
			outside["north"]++
		}
	}
	for _, side := range []string{"east", "south", "north"} {
		if outside[side] == 0 {
			test.Errorf("no windows on the open %s wall", side)
		}
	}
	if outside["west"] > 0 {
		test.Error("windows on the wall the neighbour covers")
	}
}

func TestStepsDownToALowerStreet(test *testing.T) {
	ring := mapdata.Ring{{0, 0}, {0.008, 0}, {0.008, 0.01}, {0, 0.01}}
	details := func(height func(east, north float64) float64) int {
		current := &block.Block{Feature: &mapdata.Feature{Identifier: "house", Properties: map[string]any{"id": "house"}},
			Polygon: mapdata.Polygon{Exterior: ring}, Ground: 0, Bottom: -1, Eaves: 6.5,
			Facing: &block.Facing{Side: 0, Outward: mapdata.Point{0, -1}},
			Style:  style.Style{"facade": map[string]any{"door": map[string]any{}}, "roof": map[string]any{"recipe": "flat"}}}
		pieces := map[string]*mesh.Piece{}
		pieceFor := func(material mesh.Material) *mesh.Piece {
			if pieces[material.Name] == nil {
				pieces[material.Name] = &mesh.Piece{Material: material}
			}
			return pieces[material.Name]
		}
		if err := (&Decorator{Context: geos.NewContext(), Height: height}).Decorate(current, pieceFor); err != nil {
			test.Fatal(err)
		}
		return pieces["details"].TriangleCount()
	}
	level := details(func(east, north float64) float64 { return 0 })
	below := details(func(east, north float64) float64 { return -0.9 })
	if steps := (below - level) / 8; steps != 5 { // four sides and a top each
		test.Errorf("a door 0.9 m above the street should have 5 steps, it has %d", steps)
	}
}

func TestPlinthLeavesTheDoorFree(test *testing.T) {
	ring := mapdata.Ring{{0, 0}, {0.008, 0}, {0.008, 0.01}, {0, 0.01}}
	current := &block.Block{Feature: &mapdata.Feature{Identifier: "house", Properties: map[string]any{"id": "house"}},
		Polygon: mapdata.Polygon{Exterior: ring}, Ground: 0, Bottom: -1, Eaves: 6.5,
		Facing: &block.Facing{Side: 0, Outward: mapdata.Point{0, -1}},
		Style: style.Style{"facade": map[string]any{"door": map[string]any{"width": 1.0}},
			"stonework": map[string]any{"plinth": map[string]any{"height": 1.5}}, "roof": map[string]any{"recipe": "flat"}}}
	pieces := map[string]*mesh.Piece{}
	pieceFor := func(material mesh.Material) *mesh.Piece {
		if pieces[material.Name] == nil {
			pieces[material.Name] = &mesh.Piece{Material: material}
		}
		return pieces[material.Name]
	}
	if err := (&Decorator{Context: geos.NewContext()}).Decorate(current, pieceFor); err != nil {
		test.Fatal(err)
	}
	stone := pieces[stoneMaterial.Name]
	if stone == nil {
		test.Fatal("no plinth")
	}
	// the door stands in the middle of the 8 m front (the south side, z >= 0): nothing of the plinth crosses 4 m
	for index := 0; index+2 < len(stone.Indices); index += 3 {
		west, east, south := math.Inf(1), math.Inf(-1), math.Inf(1)
		for corner := range 3 {
			vertex := stone.Indices[index+corner] * 3
			west = math.Min(west, float64(stone.Positions[vertex]))
			east = math.Max(east, float64(stone.Positions[vertex]))
			south = math.Min(south, float64(stone.Positions[vertex+2]))
		}
		if south >= 0 && west < 0.004 && east > 0.004 {
			test.Fatalf("the plinth runs across the door, from %.2f to %.2f m", west*1000, east*1000)
		}
	}
}

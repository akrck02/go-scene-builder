package build

import (
	"testing"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/models"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
)

func TestBiomesColourAndSow(test *testing.T) {
	rules, err := style.Parse([]byte(`
units: m
terrain:
  color: "#000000"
  biomes:
    forest: {color: "#00ff00", decorate: [{recipe: scatter, density: 0.002, models: {tree: 1}}]}
    meadow: {color: "#ffff00"}
`))
	if err != nil {
		test.Fatal(err)
	}
	ground := &relief.Relief{Columns: 4, Rows: 4, Cell: 0.1, West: 0, North: 0.4, Heights: make([]float32, 16)}
	for index := range ground.Heights {
		ground.Heights[index] = 10
	}
	// west half forest (0), east half meadow (1)
	biomes := &relief.Relief{Columns: 2, Rows: 2, Cell: 0.2, West: 0, North: 0.4, Heights: []float32{0, 1, 0, 1}, Labels: []string{"forest", "meadow"}}
	tree := &models.Model{Name: "tree", Pieces: []*mesh.Piece{{Name: "tree", Material: mesh.Material{Name: "tree"}}}}
	library := models.NewLibrary(tree)
	place := &mapdata.Feature{Identifier: "village", Kind: "place", Polygons: []mapdata.Polygon{{Exterior: mapdata.Ring{{0, 0.2}, {0.1, 0.2}, {0.1, 0.4}, {0, 0.4}}}}}
	result := Build(Input{Map: &mapdata.Map{Features: []*mapdata.Feature{place}}, Relief: ground, Rules: rules, Models: library,
		Layers: map[string]*relief.Relief{"biome": biomes}})
	copies := 0
	for _, instances := range result.Instances {
		for index := 0; index < instances.Count(); index++ {
			east, south := instances.Positions[3*index], instances.Positions[3*index+2]
			if east > 0 { // scene x = east − 0.2 (the origin is the relief's centre): the meadow is east of 0
				test.Errorf("a tree in the meadow at %v", east)
			}
			if east < -0.1 && south < 0 {
				test.Errorf("a tree on the village at %v, %v", east, south)
			}
			copies++
		}
	}
	if copies == 0 {
		test.Error("the forest should have trees")
	}
	terrain := result.Pieces[0]
	west, east := terrain.Colors[0:4], terrain.Colors[4*3:4*3+4]
	if west[1] < 0.9 || east[0] < 0.9 {
		test.Errorf("west green, east yellow: %v %v", west, east)
	}
}

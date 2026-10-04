package build

import (
	"testing"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/style"
)

func TestPlantsAvoidHouses(test *testing.T) {
	rules, err := style.Parse([]byte(`
units: m
rules:
  - when: {kind: parcel}
    shape: {role: ground}
    decorate: [{recipe: plants, density: 0.2, trees: 0.3, beds: 0.3, clear: 1}]
  - when: {kind: building}
    shape: {role: volume, height: 6}
`))
	if err != nil {
		test.Fatal(err)
	}
	garden := &mapdata.Feature{Identifier: "garden", Kind: "parcel", Properties: map[string]any{"id": "garden"},
		Polygons: []mapdata.Polygon{square(0.03)}}
	house := &mapdata.Feature{Identifier: "house", Kind: "building", Polygons: []mapdata.Polygon{square(0.01)}}
	result := Build(Input{Map: &mapdata.Map{Features: []*mapdata.Feature{garden, house}}, Rules: rules})
	plants := 0
	for _, piece := range result.Pieces {
		if piece.Material.Name != "plants" && piece.Material.Name != "flower beds" {
			continue
		}
		plants += piece.TriangleCount()
		for index := 0; index < len(piece.Positions); index += 3 {
			east, south := float64(piece.Positions[index]), float64(piece.Positions[index+2])
			if east > 0.0005 && east < 0.0095 && -south > 0.0005 && -south < 0.0095 {
				test.Fatalf("a plant grows inside the house at %v, %v", east, -south)
			}
		}
	}
	if plants == 0 {
		test.Error("the garden should have plants")
	}
}

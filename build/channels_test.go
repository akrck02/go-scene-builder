package build

import (
	"math"
	"strings"
	"testing"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
)

const riverRules = `
units: m
rules:
  - when: {kind: river}
    shape: {role: water, width: 20}
  - when: {kind: bridge}
    shape: {role: line, follow: level, thickness: 4, height: 0.5, sink: 0.5}
`

// slopedRelief is half a kilometre of ground in cells of 12.5 m, rising 20 m per km to the east.
func slopedRelief() *relief.Relief {
	grid := &relief.Relief{Columns: 40, Rows: 40, Cell: 0.0125, West: 0, North: 0.5}
	grid.Heights = make([]float32, grid.Columns*grid.Rows)
	for row := range grid.Rows {
		for column := range grid.Columns {
			east, _ := grid.CellCentre(row, column)
			grid.Heights[row*grid.Columns+column] = float32(50 + 20*east)
		}
	}
	return grid
}

// sceneVertices are the vertices of the pieces whose material starts with a name, in map
// coordinates (km east and north) and metres.
func sceneVertices(result *Result, material string) [][3]float64 {
	var vertices [][3]float64
	for _, piece := range result.Pieces {
		if !strings.HasPrefix(piece.Material.Name, material) {
			continue
		}
		for index := 0; index+2 < len(piece.Positions); index += 3 {
			vertices = append(vertices, [3]float64{float64(piece.Positions[index]) + result.Frame.OriginEast,
				result.Frame.OriginNorth - float64(piece.Positions[index+2]), 1000 * float64(piece.Positions[index+1])})
		}
	}
	return vertices
}

func TestRiversRunBelowTheGroundAndBridgesOverThem(test *testing.T) {
	rules, err := style.Parse([]byte(riverRules))
	if err != nil {
		test.Fatal(err)
	}
	river := &mapdata.Feature{Identifier: "river", Kind: "river", Lines: [][]mapdata.Point{{{0.02, 0.25}, {0.48, 0.25}}}}
	bridge := &mapdata.Feature{Identifier: "bridge", Kind: "bridge", Lines: [][]mapdata.Point{{{0.25, 0.235}, {0.25, 0.265}}}}
	natural := slopedRelief()
	result := Build(Input{Map: &mapdata.Map{Features: []*mapdata.Feature{river, bridge}}, Relief: natural, Rules: rules})

	water := sceneVertices(result, waterMaterial.Name)
	if len(water) == 0 {
		test.Fatal("no water")
	}
	highestWater := math.Inf(-1)
	for _, vertex := range water {
		if ground := natural.Height(vertex[0], vertex[1]); vertex[2] > ground-defaultFreeboard/2 {
			test.Fatalf("water at %.1f m over natural ground at %.1f m (%.3f, %.3f km)", vertex[2], ground, vertex[0], vertex[1])
		}
		if math.Abs(vertex[0]-0.25) < 0.02 {
			highestWater = math.Max(highestWater, vertex[2])
		}
	}

	terrain := &mesh.Piece{}
	for _, piece := range result.Pieces {
		if piece.Ground {
			terrain.Append(piece)
		}
	}
	lowestGround := math.Inf(1)
	for index := 0; index+2 < len(terrain.Positions); index += 3 {
		east := float64(terrain.Positions[index]) + result.Frame.OriginEast
		north := result.Frame.OriginNorth - float64(terrain.Positions[index+2])
		if math.Abs(north-0.25) < 0.007 && math.Abs(east-0.25) < 0.02 { // the cell centres next to the axis
			lowestGround = math.Min(lowestGround, 1000*float64(terrain.Positions[index+1]))
		}
	}
	if lowestGround >= highestWater {
		test.Errorf("the river bed (%.1f m) is not under the water (%.1f m)", lowestGround, highestWater)
	}

	deck := sceneVertices(result, solidMaterial.Name)
	if len(deck) == 0 {
		test.Fatal("no bridge")
	}
	southmost, northmost := math.Inf(1), math.Inf(-1)
	for _, vertex := range deck {
		southmost, northmost = math.Min(southmost, vertex[1]), math.Max(northmost, vertex[1])
		if math.Abs(vertex[1]-0.25) < 0.01 && vertex[2] < highestWater+1.5-0.01 {
			test.Errorf("the bridge goes down to %.2f m over water at %.2f m", vertex[2], highestWater)
		}
	}
	if northmost-southmost <= 0.03 {
		test.Errorf("the bridge does not reach out to the banks: %.0f m long", 1000*(northmost-southmost))
	}
}

// TestBridgesClearTheWaterOfALiveScene: without a relief to dig, the water lies over the ground of
// the scene, and bridges still pass over it.
func TestBridgesClearTheWaterOfALiveScene(test *testing.T) {
	rules, err := style.Parse([]byte(riverRules + `
    # above_bed comes from the default: 1 m
`))
	if err != nil {
		test.Fatal(err)
	}
	river := &mapdata.Feature{Identifier: "river", Kind: "river", Lines: [][]mapdata.Point{{{0.02, 0.25}, {0.48, 0.25}}}}
	bridge := &mapdata.Feature{Identifier: "bridge", Kind: "bridge", Lines: [][]mapdata.Point{{{0.25, 0.235}, {0.25, 0.265}}}}
	flat := HeightFunction(func(float64, float64) float64 { return 10 })
	result := Build(Input{Map: &mapdata.Map{Features: []*mapdata.Feature{river, bridge}}, Heights: flat, Rules: rules})
	for _, vertex := range sceneVertices(result, solidMaterial.Name) {
		if math.Abs(vertex[1]-0.25) < 0.01 && vertex[2] < 10+1+1.5-0.01 {
			test.Fatalf("the bridge goes down to %.2f m over water at 11 m", vertex[2])
		}
	}
}

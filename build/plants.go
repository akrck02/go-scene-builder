package build

import (
	"math"
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/models"
	"github.com/akrck02/go-scene-builder/style"
)

// The scatter recipe does the same with models of the library instead of plants, chosen by weight:
//
//	decorate: [{recipe: scatter, density: 0.05, models: {lily: 3, poppy: 1}, scale: [0.8, 1.2], turn: random,
//	            group: 8, spread: 0.5}]     # optional: copies in groups, like flower beds
//
// The plants recipe sows ground features with shrubs, trees and flower beds, never on a block or
// closer to it than "clear" metres. In the style of a ground feature:
//
//	decorate:
//	  - recipe: plants
//	    density: 0.01          # plants per square metre
//	    trees: 0.15            # share of trees; beds: share of flower beds; the rest are shrubs
//	    beds: 0.25
//	    tree_height: [4, 8]    # metres
//	    clear: 1.5             # metres kept free around blocks
//	    foliage: ["#4f6b35", "#5d7a3c"]
//	    flowers: ["#d2453c", "#e8c547", "#f2f0e8"]
//	    trunk: "#5a4030"
//
// Each feature's plants are always the same (seeded by its id).

var (
	plantMaterial  = mesh.Material{Name: "plants", Kind: "lit"}
	bloomsMaterial = mesh.Material{Name: "flower beds", Kind: "lit"}
)

// planting is a ground feature to sow, found in stage 1 and sown once the blocks are known.
type planting struct {
	feature  *mapdata.Feature
	settings style.Style
	polygons []mapdata.Polygon
}

// plantingsOf records the plants recipes of a ground feature.
func (builder *featureBuilder) plantingsOf(feature *mapdata.Feature, resolved style.Style) {
	decorations, _ := resolved["decorate"].([]any)
	for _, entry := range decorations {
		settings := style.Style(asMap(entry))
		recipe := settings.Text("recipe", "")
		if (recipe == "plants" || recipe == "scatter") && len(feature.Polygons) > 0 {
			builder.output.plantings = append(builder.output.plantings, planting{feature, settings, feature.Polygons})
		}
	}
}

// sow plants every planting, spread over the processor's cores.
func sow(plantings []planting, blocks []*block.Block, input Input, frame mesh.Frame, chunk float64) []*workerOutput {
	occupied := newFootprintIndex(blocks)
	workers := runtime.NumCPU()
	outputs := make([]*workerOutput, workers)
	var group sync.WaitGroup
	for worker := range workers {
		outputs[worker] = newWorkerOutput(chunk)
		group.Add(1)
		go func() {
			defer group.Done()
			gardener := &gardener{input: input, frame: frame, occupied: occupied, output: outputs[worker]}
			for index := worker; index < len(plantings); index += workers {
				gardener.sow(plantings[index])
			}
		}()
	}
	group.Wait()
	return outputs
}

type gardener struct {
	input    Input
	frame    mesh.Frame
	occupied *footprintIndex
	output   *workerOutput
}

func (gardener *gardener) height(east, north float64) float64 {
	if gardener.input.Heights != nil {
		return gardener.input.Heights.Height(east, north)
	}
	return gardener.input.Relief.Height(east, north)
}

func (gardener *gardener) sow(current planting) {
	values, err := numbers(current.settings, current.feature.Properties, map[string]float64{"density": 0.01, "trees": 0.15, "beds": 0.25, "clear": 1.5})
	if err != nil {
		gardener.output.warnings = append(gardener.output.warnings, current.feature.Identifier+": "+err.Error())
		return
	}
	random := rand.New(rand.NewPCG(style.Seed(current.feature.Properties, "plants"), 7))
	scatter := current.settings.Text("recipe", "") == "scatter"
	chosen, weights, missing := weightedModels(current.settings, gardener.input.Models)
	for _, name := range missing {
		gardener.output.warnings = append(gardener.output.warnings, current.feature.Identifier+": no model "+name)
	}
	if scatter && len(chosen) == 0 {
		return
	}
	for _, polygon := range current.polygons {
		area := math.Abs(polygon.Exterior.SignedArea()) * 1e6
		count := int(area*values["density"] + random.Float64())
		for range count {
			point, found := pointInside(polygon, random)
			if !found || gardener.occupied.near(point, values["clear"]/1000) {
				continue
			}
			if scatter {
				gardener.place(point, pickWeighted(chosen, weights, random), current.settings, current.feature.Properties, random)
			} else {
				gardener.plant(point, current.settings, current.feature.Properties, random, values)
			}
		}
	}
}

// place puts copies of a model on the ground (the scatter recipe: models instead of plants): one,
// or a group ("group" copies within "spread" metres, like a flower bed).
func (gardener *gardener) place(point mapdata.Point, model *models.Model, settings style.Style, properties map[string]any, random *rand.Rand) {
	group, _ := settings.Number("group", properties, 1)
	spread, _ := settings.Number("spread", properties, 0.5)
	for copy := range max(1, int(group)) {
		spot := point
		if copy > 0 {
			angle, reach := random.Float64()*2*math.Pi, spread*math.Sqrt(random.Float64())
			spot = mapdata.Point{point[0] + reach*math.Cos(angle)/1000, point[1] + reach*math.Sin(angle)/1000}
		}
		scale, turn, err := sizeAndTurn(settings, properties, random)
		if err != nil {
			return
		}
		position := gardener.frame.Scene(spot[0], spot[1], gardener.height(spot[0], spot[1]))
		gardener.output.copiesOf(model, spot[0], spot[1]).Add(position, turn, scale, [4]float32{1, 1, 1, 1})
	}
}

func (gardener *gardener) plant(point mapdata.Point, settings style.Style, properties map[string]any, random *rand.Rand, values map[string]float64) {
	ground := gardener.height(point[0], point[1])
	choice := random.Float64()
	switch {
	case choice < values["trees"]:
		low, high := rangeOf(settings["tree_height"], 4, 8)
		height := low + (high-low)*random.Float64()
		trunk := colorOr(settings, "trunk", properties, [4]float32{0.2, 0.13, 0.08, 1})
		pieces := gardener.output.piece(plantMaterial, point[0], point[1])
		gardener.blob(pieces, point, ground, 0, height*0.55, 0.15, 0.15, trunk, 5)
		foliage := pickColor(settings["foliage"], random, [4]float32{0.1, 0.2, 0.06, 1})
		gardener.cluster(pieces, point, ground, height*0.4, height, height*0.3, foliage, random)
	case choice < values["trees"]+values["beds"]:
		pieces := gardener.output.piece(bloomsMaterial, point[0], point[1])
		for range 5 + random.IntN(5) {
			angle, reach := random.Float64()*2*math.Pi, random.Float64()*0.6
			spot := mapdata.Point{point[0] + reach*math.Cos(angle)/1000, point[1] + reach*math.Sin(angle)/1000}
			gardener.blob(pieces, spot, ground, 0, 0.2+0.25*random.Float64(), 0.12, 0.12, pickColor(settings["flowers"], random, [4]float32{0.7, 0.2, 0.2, 1}), 4)
		}
	default:
		radius := 0.35 + 0.45*random.Float64()
		gardener.cluster(gardener.output.piece(plantMaterial, point[0], point[1]), point, ground, -radius*0.2, radius*1.5, radius,
			pickColor(settings["foliage"], random, [4]float32{0.1, 0.2, 0.06, 1}), random)
	}
}

// cluster adds three to five overlapping blobs, a little apart and of slightly different shades:
// a shrub, or the crown of a tree.
func (gardener *gardener) cluster(piece *mesh.Piece, point mapdata.Point, ground, bottom, top, radius float64, color [4]float32, random *rand.Rand) {
	height := top - bottom
	for range 3 + random.IntN(3) {
		angle, reach := random.Float64()*2*math.Pi, radius*0.45*random.Float64()
		spot := mapdata.Point{point[0] + reach*math.Cos(angle)/1000, point[1] + reach*math.Sin(angle)/1000}
		size := radius * (0.55 + 0.35*random.Float64())
		low := bottom + height*0.25*random.Float64()
		shade := float32(0.88 + 0.24*random.Float64())
		tone := [4]float32{color[0] * shade, color[1] * shade, color[2] * shade, color[3]}
		gardener.blob(piece, spot, ground, low, low+height*(0.6+0.25*random.Float64()), size, size, tone, 7)
	}
}

// blob adds a closed, faceted shape around a vertical axis: from "bottom" to "top" metres over the
// ground, widest (radius) halfway; a few sides make it look like a shrub, a crown or a trunk.
func (gardener *gardener) blob(piece *mesh.Piece, point mapdata.Point, ground, bottom, top, radiusEast, radiusNorth float64, color [4]float32, sides int) {
	middle := (bottom + top) / 2
	centre := gardener.frame.Scene(point[0], point[1], ground+middle)
	lowest := gardener.frame.Scene(point[0], point[1], ground+bottom)
	highest := gardener.frame.Scene(point[0], point[1], ground+top)
	rim := make([]mesh.Vector, sides)
	for side := range sides {
		angle := 2 * math.Pi * float64(side) / float64(sides)
		rim[side] = gardener.frame.Scene(point[0]+radiusEast*math.Cos(angle)/1000, point[1]+radiusNorth*math.Sin(angle)/1000, ground+middle)
	}
	for side := range sides {
		next := rim[(side+1)%sides]
		piece.AddTriangle(rim[side], next, highest, rim[side].Add(next).Add(highest).Scale(1.0/3).Subtract(centre), color)
		piece.AddTriangle(rim[side], next, lowest, rim[side].Add(next).Add(lowest).Scale(1.0/3).Subtract(centre), color)
	}
}

func rangeOf(value any, low, high float64) (float64, float64) {
	if pair, isList := value.([]any); isList && len(pair) == 2 {
		first, firstIsNumber := toNumber(pair[0])
		second, secondIsNumber := toNumber(pair[1])
		if firstIsNumber && secondIsNumber {
			return first, second
		}
	}
	return low, high
}

func colorOr(settings style.Style, key string, properties map[string]any, fallback [4]float32) [4]float32 {
	color, err := settings.Color(key, properties, fallback)
	if err != nil {
		return fallback
	}
	return color
}

// pickColor takes a colour from a palette at random, a little lighter or darker each time.
func pickColor(palette any, random *rand.Rand, fallback [4]float32) [4]float32 {
	color := fallback
	if list, isList := palette.([]any); isList && len(list) > 0 {
		if text, isText := list[random.IntN(len(list))].(string); isText {
			if parsed, err := style.LinearColor(text); err == nil {
				color = parsed
			}
		}
	}
	shade := float32(0.85 + 0.3*random.Float64())
	return [4]float32{color[0] * shade, color[1] * shade, color[2] * shade, color[3]}
}

// pointInside draws a random point inside a polygon (outside its holes), or false after many tries.
func pointInside(polygon mapdata.Polygon, random *rand.Rand) (mapdata.Point, bool) {
	west, south, east, north := boundsOfRing(polygon.Exterior)
	for range 30 {
		point := mapdata.Point{west + (east-west)*random.Float64(), south + (north-south)*random.Float64()}
		if !ringContains(polygon.Exterior, point) {
			continue
		}
		inHole := false
		for _, hole := range polygon.Holes {
			inHole = inHole || ringContains(hole, point)
		}
		if !inHole {
			return point, true
		}
	}
	return mapdata.Point{}, false
}

func boundsOfRing(ring mapdata.Ring) (west, south, east, north float64) {
	west, south, east, north = math.Inf(1), math.Inf(1), math.Inf(-1), math.Inf(-1)
	for _, point := range ring {
		west, east = math.Min(west, point[0]), math.Max(east, point[0])
		south, north = math.Min(south, point[1]), math.Max(north, point[1])
	}
	return
}

// ringContains tells whether a point is inside a ring (even-odd rule).
func ringContains(ring mapdata.Ring, point mapdata.Point) bool {
	inside := false
	for index, previous := 0, len(ring)-1; index < len(ring); previous, index = index, index+1 {
		first, second := ring[index], ring[previous]
		if (first[1] > point[1]) != (second[1] > point[1]) &&
			point[0] < (second[0]-first[0])*(point[1]-first[1])/(second[1]-first[1])+first[0] {
			inside = !inside
		}
	}
	return inside
}

// footprintIndex finds whether a point is on, or near, a block.
type footprintIndex struct {
	cell  float64
	cells map[[2]int][]*block.Block
}

func newFootprintIndex(blocks []*block.Block) *footprintIndex {
	index := &footprintIndex{cell: 0.02, cells: map[[2]int][]*block.Block{}}
	for _, current := range blocks {
		west, south, east, north := boundsOfRing(current.Polygon.Exterior)
		for column := int(math.Floor(west / index.cell)); column <= int(math.Floor(east/index.cell)); column++ {
			for row := int(math.Floor(south / index.cell)); row <= int(math.Floor(north/index.cell)); row++ {
				index.cells[[2]int{column, row}] = append(index.cells[[2]int{column, row}], current)
			}
		}
	}
	return index
}

func (index *footprintIndex) near(point mapdata.Point, clear float64) bool {
	column, row := int(math.Floor(point[0]/index.cell)), int(math.Floor(point[1]/index.cell))
	for nearbyColumn := column - 1; nearbyColumn <= column+1; nearbyColumn++ {
		for nearbyRow := row - 1; nearbyRow <= row+1; nearbyRow++ {
			for _, current := range index.cells[[2]int{nearbyColumn, nearbyRow}] {
				if ringContains(current.Polygon.Exterior, point) || distanceToRing(current.Polygon.Exterior, point) < clear {
					return true
				}
			}
		}
	}
	return false
}

func distanceToRing(ring mapdata.Ring, point mapdata.Point) float64 {
	nearest := math.Inf(1)
	for index := range ring {
		nearest = math.Min(nearest, mapdata.Distance(point, mapdata.NearestOnSegment(ring[index], ring[(index+1)%len(ring)], point)))
	}
	return nearest
}

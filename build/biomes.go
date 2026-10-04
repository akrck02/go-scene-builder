package build

import (
	"math"
	"math/rand/v2"
	"runtime"
	"sync"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
)

// sowBiomes sows the vegetation of every biome over the cells of the biome layer: the "decorate"
// recipes (plants, scatter) of each biome in the style's terrain.biomes, never in the sea, on a
// block or on a ground feature of the map (places keep their own ground).
//
//	terrain:
//	  biomes:
//	    coniferas: {color: "#5b6a49", decorate: [{recipe: scatter, density: 0.0004, models: {abeto: 1}}]}
func sowBiomes(input Input, blocks []*block.Block, frame mesh.Frame, chunk float64) []*workerOutput {
	layer := input.Layers["biome"]
	if layer == nil || input.Relief == nil {
		return nil
	}
	settings := style.Style(input.Rules.Terrain).Section("biomes")
	decorations := map[int][]style.Style{}
	for category, label := range layer.Labels {
		entries, _ := settings.Section(label)["decorate"].([]any)
		for _, entry := range entries {
			decorations[category] = append(decorations[category], style.Style(asMap(entry)))
		}
	}
	if len(decorations) == 0 {
		return nil
	}
	occupied := newFootprintIndex(blocks)
	covered := newRingIndex(groundRings(input))
	covered.lines = linesWithWidth(input)
	workers := runtime.NumCPU()
	outputs := make([]*workerOutput, workers)
	var group sync.WaitGroup
	for worker := range workers {
		outputs[worker] = newWorkerOutput(chunk)
		group.Add(1)
		go func() {
			defer group.Done()
			gardener := &gardener{input: input, frame: frame, occupied: occupied, output: outputs[worker]}
			for row := worker; row < layer.Rows; row += workers {
				for column := range layer.Columns {
					category := int(math.Round(layer.At(row, column)))
					for index, settings := range decorations[category] {
						gardener.sowCell(layer, row, column, index, settings, covered)
					}
				}
			}
		}()
	}
	group.Wait()
	return outputs
}

// sowCell sows one recipe of a biome over one cell of the layer.
func (gardener *gardener) sowCell(layer *relief.Relief, row, column, recipe int, settings style.Style, covered *ringIndex) {
	cell := layer.Cell
	values, err := numbers(settings, nil, map[string]float64{"density": 0.0005, "trees": 0.5, "beds": 0, "clear": 2})
	if err != nil {
		return
	}
	random := rand.New(rand.NewPCG(uint64(row)*1_000_003+uint64(column), uint64(recipe)))
	count := int(cell*cell*1e6*values["density"] + random.Float64())
	if count == 0 {
		return
	}
	scatter := settings.Text("recipe", "") == "scatter"
	chosen, weights, _ := weightedModels(settings, gardener.input.Models)
	if scatter && len(chosen) == 0 {
		return
	}
	centreEast, centreNorth := layer.CellCentre(row, column)
	for range count {
		point := mapdata.Point{centreEast + (random.Float64()-0.5)*cell, centreNorth + (random.Float64()-0.5)*cell}
		if gardener.height(point[0], point[1]) <= 0 || covered.contains(point) || gardener.occupied.near(point, values["clear"]/1000) {
			continue
		}
		if scatter {
			gardener.place(point, pickWeighted(chosen, weights, random), settings, nil, random)
		} else {
			gardener.plant(point, settings, nil, random, values)
		}
	}
}

// groundRings are the outlines of the map's polygons (places keep their own ground: no forest on
// them).
func groundRings(input Input) []mapdata.Ring {
	var rings []mapdata.Ring
	for _, feature := range input.Map.Features {
		for _, polygon := range feature.Polygons {
			rings = append(rings, polygon.Exterior)
		}
	}
	return rings
}

// ringIndex finds whether a point is inside one of many rings, or on one of many wide lines.
type ringIndex struct {
	cell  float64
	cells map[[2]int][]mapdata.Ring
	lines *axisIndexWithWidths
}

func newRingIndex(rings []mapdata.Ring) *ringIndex {
	index := &ringIndex{cell: 0.2, cells: map[[2]int][]mapdata.Ring{}}
	for _, ring := range rings {
		west, south, east, north := boundsOfRing(ring)
		for column := int(math.Floor(west / index.cell)); column <= int(math.Floor(east/index.cell)); column++ {
			for row := int(math.Floor(south / index.cell)); row <= int(math.Floor(north/index.cell)); row++ {
				index.cells[[2]int{column, row}] = append(index.cells[[2]int{column, row}], ring)
			}
		}
	}
	return index
}

func (index *ringIndex) contains(point mapdata.Point) bool {
	if index.lines != nil && index.lines.covers(point) {
		return true
	}
	for _, ring := range index.cells[[2]int{int(math.Floor(point[0] / index.cell)), int(math.Floor(point[1] / index.cell))}] {
		if ringContains(ring, point) {
			return true
		}
	}
	return false
}

// linesWithWidth are the map's lines that the style gives a width (rivers, roads, walls), so no
// vegetation grows on them.
func linesWithWidth(input Input) *axisIndexWithWidths {
	index := &axisIndexWithWidths{cell: 0.1, cells: map[[2]int][]widthSegment{}}
	for _, feature := range input.Map.Features {
		if len(feature.Lines) == 0 {
			continue
		}
		resolved, matched := input.Rules.Resolve(feature.Kind, feature.Properties)
		if !matched {
			continue
		}
		shape := resolved.Section("shape")
		width, _ := shape.Number("width", feature.Properties, 0)
		if thickness, _ := shape.Number("thickness", feature.Properties, 0); thickness > width {
			width = thickness
		}
		if width <= 0 {
			continue
		}
		for _, line := range feature.Lines {
			for point := 1; point < len(line); point++ {
				index.add(widthSegment{line[point-1], line[point], (width/2 + 3) / 1000})
			}
		}
	}
	return index
}

type widthSegment struct {
	from, to mapdata.Point
	half     float64 // km
}

type axisIndexWithWidths struct {
	cell  float64
	cells map[[2]int][]widthSegment
}

func (index *axisIndexWithWidths) add(segment widthSegment) {
	west, east := math.Min(segment.from[0], segment.to[0])-segment.half, math.Max(segment.from[0], segment.to[0])+segment.half
	south, north := math.Min(segment.from[1], segment.to[1])-segment.half, math.Max(segment.from[1], segment.to[1])+segment.half
	for column := int(math.Floor(west / index.cell)); column <= int(math.Floor(east/index.cell)); column++ {
		for row := int(math.Floor(south / index.cell)); row <= int(math.Floor(north/index.cell)); row++ {
			index.cells[[2]int{column, row}] = append(index.cells[[2]int{column, row}], segment)
		}
	}
}

func (index *axisIndexWithWidths) covers(point mapdata.Point) bool {
	for _, segment := range index.cells[[2]int{int(math.Floor(point[0] / index.cell)), int(math.Floor(point[1] / index.cell))}] {
		if mapdata.Distance(point, mapdata.NearestOnSegment(segment.from, segment.to, point)) < segment.half {
			return true
		}
	}
	return false
}

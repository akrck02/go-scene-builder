package build

import (
	"math"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/recipes"
	"github.com/akrck02/go-scene-builder/style"
)

var upwards = mesh.Vector{0, 1, 0}

// ground drapes a feature's surfaces on the relief, "lift" metres above it (so they do not flicker
// with the terrain).
func (builder *featureBuilder) ground(feature *mapdata.Feature, resolved, shape style.Style) error {
	color, err := resolved.Color("color", feature.Properties, [4]float32{0.6, 0.6, 0.6, 1})
	if err != nil {
		return err
	}
	lift, err := shape.Number("lift", feature.Properties, 0.1)
	if err != nil {
		return err
	}
	polygons, err := builder.surfaces(feature, shape)
	if err != nil {
		return err
	}
	builder.plantingsOf(feature, resolved)
	for _, corners := range builder.triangles(polygons) {
		builder.drape(recipes.MaterialOf(groundMaterial, resolved), corners, color, func(point mapdata.Point) float64 { return builder.height(point[0], point[1]) + lift })
	}
	return nil
}

// water lays a flat sheet across a river: each point takes the height of the bed under the
// nearest point of the feature's axis (its lines) plus "above_bed" metres, so the water is level
// from bank to bank. Without lines, the sheet lies "above_bed" over the lowest corner of each polygon.
func (builder *featureBuilder) water(feature *mapdata.Feature, resolved, shape style.Style) error {
	color, err := resolved.Color("color", feature.Properties, [4]float32{0.15, 0.35, 0.5, 1})
	if err != nil {
		return err
	}
	aboveBed, err := shape.Number("above_bed", feature.Properties, 1)
	if err != nil {
		return err
	}
	polygons, err := builder.surfaces(feature, shape)
	if err != nil {
		return err
	}
	width, err := shape.Number("width", feature.Properties, 0)
	if err != nil {
		return err
	}
	axis := newAxisIndex(feature.Lines, math.Max(width/1000, 0.02))
	level := func(point mapdata.Point) float64 {
		bed := axis.nearest(point)
		return builder.height(bed[0], bed[1]) + aboveBed
	}
	if len(feature.Lines) == 0 {
		lowest := builder.lowestCorner(feature)
		level = func(mapdata.Point) float64 { return lowest + aboveBed }
	}
	for _, corners := range builder.triangles(polygons) {
		builder.drape(recipes.MaterialOf(waterMaterial, resolved), corners, color, level)
	}
	return nil
}

func (builder *featureBuilder) drape(material mesh.Material, corners triangle, color [4]float32, heightOf func(mapdata.Point) float64) {
	centre := centreOf(corners)
	piece := builder.output.piece(material, centre[0], centre[1])
	var scene [3]mesh.Vector
	for index, corner := range corners {
		scene[index] = builder.frame.Scene(corner[0], corner[1], heightOf(corner))
	}
	piece.AddTriangle(scene[0], scene[1], scene[2], upwards, color)
}

func (builder *featureBuilder) lowestCorner(feature *mapdata.Feature) float64 {
	lowest := math.Inf(1)
	for _, polygon := range feature.Polygons {
		for _, point := range polygon.Exterior {
			lowest = math.Min(lowest, builder.height(point[0], point[1]))
		}
	}
	if math.IsInf(lowest, 1) {
		return 0
	}
	return lowest
}

// axisIndex finds the nearest point of a river's axis quickly: its segments in a grid of cells.
type axisIndex struct {
	cell     float64
	segments [][2]mapdata.Point
	cells    map[[2]int][]int
}

func newAxisIndex(lines [][]mapdata.Point, cell float64) *axisIndex {
	index := &axisIndex{cell: cell, cells: map[[2]int][]int{}}
	for _, line := range lines {
		for point := 1; point < len(line); point++ {
			from, to := line[point-1], line[point]
			number := len(index.segments)
			index.segments = append(index.segments, [2]mapdata.Point{from, to})
			low, high := index.cellOf(mapdata.Point{math.Min(from[0], to[0]), math.Min(from[1], to[1])}),
				index.cellOf(mapdata.Point{math.Max(from[0], to[0]), math.Max(from[1], to[1])})
			for column := low[0]; column <= high[0]; column++ {
				for row := low[1]; row <= high[1]; row++ {
					index.cells[[2]int{column, row}] = append(index.cells[[2]int{column, row}], number)
				}
			}
		}
	}
	return index
}

func (index *axisIndex) cellOf(point mapdata.Point) [2]int {
	return [2]int{int(math.Floor(point[0] / index.cell)), int(math.Floor(point[1] / index.cell))}
}

// nearest looks in the rings of cells around a point, wider and wider, until it finds segments.
func (index *axisIndex) nearest(point mapdata.Point) mapdata.Point {
	centre := index.cellOf(point)
	best, bestSquare := point, math.Inf(1)
	for ring := 0; ring < 64 && math.IsInf(bestSquare, 1); ring++ {
		for column := centre[0] - ring - 1; column <= centre[0]+ring+1; column++ {
			for row := centre[1] - ring - 1; row <= centre[1]+ring+1; row++ {
				for _, number := range index.cells[[2]int{column, row}] {
					candidate := mapdata.NearestOnSegment(index.segments[number][0], index.segments[number][1], point)
					across, along := candidate[0]-point[0], candidate[1]-point[1]
					if square := across*across + along*along; square < bestSquare {
						best, bestSquare = candidate, square
					}
				}
			}
		}
	}
	return best
}

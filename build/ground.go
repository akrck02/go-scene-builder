package build

import (
	"math"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/style"
)

var upwards = mesh.Vector{0, 1, 0}

// ground drapes a feature's surfaces on the relief, "lift" metres above it (so they do not flicker
// with the terrain).
func (builder *featureBuilder) ground(feature *mapdata.Feature, resolved, shape style.Style) error {
	color, err := resolved.Color("color", [4]float32{0.6, 0.6, 0.6, 1})
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
	for _, corners := range builder.triangles(polygons) {
		builder.drape(groundMaterial, corners, color, func(point mapdata.Point) float64 { return builder.height(point[0], point[1]) + lift })
	}
	return nil
}

// water lays a flat sheet across a river: each point takes the height of the bed under the
// nearest point of the feature's axis (its lines) plus "above_bed" metres, so the water is level
// from bank to bank. Without lines, the sheet lies "above_bed" over the lowest corner of each polygon.
func (builder *featureBuilder) water(feature *mapdata.Feature, resolved, shape style.Style) error {
	color, err := resolved.Color("color", [4]float32{0.15, 0.35, 0.5, 1})
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
	level := func(point mapdata.Point) float64 {
		bed := nearestOnLines(feature.Lines, point)
		return builder.height(bed[0], bed[1]) + aboveBed
	}
	if len(feature.Lines) == 0 {
		lowest := builder.lowestCorner(feature)
		level = func(mapdata.Point) float64 { return lowest + aboveBed }
	}
	for _, corners := range builder.triangles(polygons) {
		builder.drape(waterMaterial, corners, color, level)
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

// nearestOnLines is the point of some line nearest to a point.
func nearestOnLines(lines [][]mapdata.Point, point mapdata.Point) mapdata.Point {
	best, bestDistance := point, math.Inf(1)
	for _, line := range lines {
		for index := 1; index < len(line); index++ {
			candidate := nearestOnSegment(line[index-1], line[index], point)
			if gap := distance(candidate, point); gap < bestDistance {
				best, bestDistance = candidate, gap
			}
		}
	}
	return best
}

func nearestOnSegment(from, to, point mapdata.Point) mapdata.Point {
	across, along := to[0]-from[0], to[1]-from[1]
	length := across*across + along*along
	if length == 0 {
		return from
	}
	share := math.Max(0, math.Min(1, ((point[0]-from[0])*across+(point[1]-from[1])*along)/length))
	return mapdata.Point{from[0] + share*across, from[1] + share*along}
}

package build

import (
	"math"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/style"
)

// volume raises each polygon of a feature as a prism: from "sink" metres below the lowest relief
// under its corners (so it does not float on a slope) to "height" metres above that lowest point,
// with walls of "walls.color" and a flat top of "roof.color" (stage 4 will put real roofs on it).
func (builder *featureBuilder) volume(feature *mapdata.Feature, resolved, shape style.Style) error {
	height, err := shape.Number("height", feature.Properties, 3)
	if err != nil {
		return err
	}
	sink, err := shape.Number("sink", feature.Properties, 1)
	if err != nil {
		return err
	}
	walls, err := resolved.Section("walls").Color("color", [4]float32{0.8, 0.78, 0.72, 1})
	if err != nil {
		return err
	}
	roof, err := resolved.Section("roof").Color("color", walls)
	if err != nil {
		return err
	}
	for _, polygon := range feature.Polygons {
		if !builder.coversAll(polygon.Exterior) {
			continue
		}
		base := builder.lowestOf(polygon.Exterior)
		builder.prism(polygon, base-sink, base+height, walls, roof)
	}
	return nil
}

// line raises each line of a feature as a wall of "thickness" and "height" metres, in pieces of at
// most "segment" metres, each sunk "sink" metres below the lowest relief under it.
func (builder *featureBuilder) line(feature *mapdata.Feature, resolved, shape style.Style) error {
	values := map[string]float64{"height": 3, "thickness": 1, "segment": 10, "sink": 1.5}
	for key, fallback := range values {
		value, err := shape.Number(key, feature.Properties, fallback)
		if err != nil {
			return err
		}
		values[key] = value
	}
	color, err := resolved.Color("color", [4]float32{0.7, 0.68, 0.64, 1})
	if err != nil {
		return err
	}
	for _, line := range feature.Lines {
		points := densified(line, values["segment"]/1000)
		for index := 1; index < len(points); index++ {
			outline := sideways(points[index-1], points[index], values["thickness"]/2000)
			if outline == nil || !builder.coversAll(outline) {
				continue
			}
			base := builder.lowestOf(mapdata.Ring{points[index-1], points[index]})
			builder.prism(mapdata.Polygon{Exterior: outline}, base-values["sink"], base+values["height"], color, color)
		}
	}
	return nil
}

// coversAll tells whether the relief covers every point (always, without a relief).
func (builder *featureBuilder) coversAll(points mapdata.Ring) bool {
	grid := builder.input.Relief
	for _, point := range points {
		if grid != nil && (point[0] < grid.West || point[0] > grid.East() || point[1] < grid.South() || point[1] > grid.North) {
			return false
		}
	}
	return true
}

func (builder *featureBuilder) lowestOf(points mapdata.Ring) float64 {
	lowest := math.Inf(1)
	for _, point := range points {
		lowest = math.Min(lowest, builder.height(point[0], point[1]))
	}
	return lowest
}

// prism adds walls around every ring of a polygon and a flat top, all in the chunk of its first corner.
func (builder *featureBuilder) prism(polygon mapdata.Polygon, bottom, top float64, walls, roof [4]float32) {
	piece := builder.output.piece(solidMaterial, polygon.Exterior[0][0], polygon.Exterior[0][1])
	for index, ring := range append([]mapdata.Ring{polygon.Exterior}, polygon.Holes...) {
		outwards := 1.0 // walls of the exterior face out; walls of holes face into the hole
		if index > 0 {
			outwards = -1
		}
		builder.wallsOf(piece, ring, bottom, top, outwards, walls)
	}
	geometry := builder.context.NewPolygon(ringsOf(polygon)).MakeValid()
	for part := range geometry.NumGeometries() {
		triangulation := geometry.Geometry(part).ConstrainedDelaunayTriangulation()
		for index := range triangulation.NumGeometries() {
			corners := triangulation.Geometry(index).ExteriorRing().CoordSeq().ToCoords()
			if len(corners) >= 3 {
				piece.AddTriangle(builder.frame.Scene(corners[0][0], corners[0][1], top), builder.frame.Scene(corners[1][0], corners[1][1], top),
					builder.frame.Scene(corners[2][0], corners[2][1], top), upwards, roof)
			}
		}
	}
}

// wallsOf adds a vertical quad for every side of a ring, facing away from the ring's inside
// (outwards = 1) or towards it (outwards = -1), whichever way the ring turns.
func (builder *featureBuilder) wallsOf(piece *mesh.Piece, ring mapdata.Ring, bottom, top, outwards float64, color [4]float32) {
	turn := 1.0
	if signedArea(ring) < 0 {
		turn = -1
	}
	for index := range ring {
		from, to := ring[index], ring[(index+1)%len(ring)]
		if from == to {
			continue
		}
		normal := mesh.SceneDirection((to[1]-from[1])*turn*outwards, -(to[0]-from[0])*turn*outwards, 0)
		piece.AddQuad(builder.frame.Scene(from[0], from[1], bottom), builder.frame.Scene(to[0], to[1], bottom),
			builder.frame.Scene(to[0], to[1], top), builder.frame.Scene(from[0], from[1], top), normal, color)
	}
}

// densified adds points along a line so no piece is longer than the given length.
func densified(line []mapdata.Point, longest float64) []mapdata.Point {
	result := []mapdata.Point{line[0]}
	for index := 1; index < len(line); index++ {
		from, to := line[index-1], line[index]
		steps := max(1, int(math.Ceil(distance(from, to)/longest)))
		for step := 1; step <= steps; step++ {
			share := float64(step) / float64(steps)
			result = append(result, mapdata.Point{from[0] + share*(to[0]-from[0]), from[1] + share*(to[1]-from[1])})
		}
	}
	return result
}

// sideways is the rectangle around a segment, half a thickness to each side, anticlockwise.
func sideways(from, to mapdata.Point, half float64) mapdata.Ring {
	length := distance(from, to)
	if length < 1e-6 {
		return nil
	}
	across := mapdata.Point{-(to[1] - from[1]) / length * half, (to[0] - from[0]) / length * half}
	return mapdata.Ring{
		{from[0] - across[0], from[1] - across[1]}, {to[0] - across[0], to[1] - across[1]},
		{to[0] + across[0], to[1] + across[1]}, {from[0] + across[0], from[1] + across[1]},
	}
}

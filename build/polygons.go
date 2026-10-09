package build

import (
	"math"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/style"
	"github.com/twpayne/go-geos"
)

// triangle is three map positions (km east and north).
type triangle [3]mapdata.Point

// surfaces are the polygons a feature covers: its polygons, and its lines widened by the style's
// "width" (metres). Points cover nothing.
func (builder *featureBuilder) surfaces(feature *mapdata.Feature, shape style.Style) ([]*geos.Geom, error) {
	var polygons []*geos.Geom
	for _, polygon := range feature.Polygons {
		polygons = append(polygons, builder.context.NewPolygon(ringsOf(polygon)).MakeValid())
	}
	if len(feature.Lines) > 0 {
		width, err := shape.Number("width", feature.Properties, 0)
		if err != nil {
			return nil, err
		}
		for _, line := range feature.Lines {
			if width > 0 {
				polygons = append(polygons, builder.context.NewLineString(coordinatesOf(line)).Buffer(width/2000, 4))
			}
		}
	}
	return builder.insideTheRelief(polygons), nil
}

// insideTheRelief cuts polygons to the relief's extent: outside it there is no ground to lie on.
func (builder *featureBuilder) insideTheRelief(polygons []*geos.Geom) []*geos.Geom {
	grid := builder.input.Relief
	if grid == nil {
		return polygons
	}
	extent := builder.context.NewPolygon([][][]float64{{{grid.West, grid.South()}, {grid.East(), grid.South()},
		{grid.East(), grid.North}, {grid.West, grid.North}, {grid.West, grid.South()}}})
	clipped := make([]*geos.Geom, 0, len(polygons))
	for _, polygon := range polygons {
		if part := polygon.Intersection(extent); !part.IsEmpty() {
			clipped = append(clipped, part)
		}
	}
	return clipped
}

// triangles splits polygons (with holes) into triangles that follow their outline exactly, each
// split again until no side is longer than largestSide, so draped triangles follow the relief.
// Large polygons are first cut into tiles, so that subdivision always gets down to largestSide.
func (builder *featureBuilder) triangles(polygons []*geos.Geom) []triangle {
	var result []triangle
	for _, geometry := range builder.tiled(polygons) {
		for part := range geometry.NumGeometries() {
			polygon := geometry.Geometry(part)
			if polygon.TypeID() != geos.TypeIDPolygon || polygon.IsEmpty() {
				continue
			}
			triangulation := polygon.ConstrainedDelaunayTriangulation()
			for index := range triangulation.NumGeometries() {
				corners := triangulation.Geometry(index).ExteriorRing().CoordSeq().ToCoords()
				if len(corners) < 3 {
					continue
				}
				whole := triangle{{corners[0][0], corners[0][1]}, {corners[1][0], corners[1][1]}, {corners[2][0], corners[2][1]}}
				result = appendSubdivided(result, whole, builder.largestSide(), 0)
			}
		}
	}
	return result
}

// tilesPerSide is how many largest sides fit across one tile: subdivision halves a side at most
// six times, so a tile's triangles always come down to largestSide.
const tilesPerSide = 16

// tiled cuts each polygon wider than a tile into the squares of a grid of tiles.
func (builder *featureBuilder) tiled(polygons []*geos.Geom) []*geos.Geom {
	tileSize := builder.largestSide() * tilesPerSide
	if math.IsInf(tileSize, 1) {
		return polygons
	}
	var result []*geos.Geom
	for _, polygon := range polygons {
		result = append(result, builder.tilesOf(polygon, tileSize)...)
	}
	return result
}

// tilesOf is the polygon itself when it fits in a tile, or its parts in each tile of the grid.
func (builder *featureBuilder) tilesOf(polygon *geos.Geom, tileSize float64) []*geos.Geom {
	west, south, east, north := envelopeOf(polygon)
	if east-west <= tileSize && north-south <= tileSize {
		return []*geos.Geom{polygon}
	}
	var parts []*geos.Geom
	// tiles are counted from the map's origin, so that neighbouring polygons share the same grid
	for column := math.Floor(west / tileSize); column*tileSize < east; column++ {
		for row := math.Floor(south / tileSize); row*tileSize < north; row++ {
			part := polygon.Intersection(builder.square(column*tileSize, row*tileSize, tileSize))
			if part.IsEmpty() {
				continue
			}
			parts = append(parts, part)
		}
	}
	return parts
}

// square is the square polygon with that south-west corner and side.
func (builder *featureBuilder) square(west, south, side float64) *geos.Geom {
	return builder.context.NewPolygon([][][]float64{{{west, south}, {west + side, south}, {west + side, south + side}, {west, south + side}, {west, south}}})
}

func envelopeOf(polygon *geos.Geom) (west, south, east, north float64) {
	bounds := polygon.Bounds()
	return bounds.MinX, bounds.MinY, bounds.MaxX, bounds.MaxY
}

// appendSubdivided splits a triangle into four until its longest side fits.
func appendSubdivided(result []triangle, whole triangle, largestSide float64, level int) []triangle {
	if level >= 6 || longestSide(whole) <= largestSide {
		return append(result, whole)
	}
	first, second, third := middle(whole[0], whole[1]), middle(whole[1], whole[2]), middle(whole[2], whole[0])
	for _, part := range []triangle{{whole[0], first, third}, {first, whole[1], second}, {third, second, whole[2]}, {first, second, third}} {
		result = appendSubdivided(result, part, largestSide, level+1)
	}
	return result
}

func longestSide(corners triangle) float64 {
	return math.Max(distance(corners[0], corners[1]), math.Max(distance(corners[1], corners[2]), distance(corners[2], corners[0])))
}

func distance(first, second mapdata.Point) float64 {
	return math.Hypot(second[0]-first[0], second[1]-first[1])
}

func middle(first, second mapdata.Point) mapdata.Point {
	return mapdata.Point{(first[0] + second[0]) / 2, (first[1] + second[1]) / 2}
}

func centreOf(corners triangle) mapdata.Point {
	return mapdata.Point{(corners[0][0] + corners[1][0] + corners[2][0]) / 3, (corners[0][1] + corners[1][1] + corners[2][1]) / 3}
}

func ringsOf(polygon mapdata.Polygon) [][][]float64 {
	rings := [][][]float64{closed(polygon.Exterior)}
	for _, hole := range polygon.Holes {
		rings = append(rings, closed(hole))
	}
	return rings
}

func closed(ring mapdata.Ring) [][]float64 {
	coordinates := coordinatesOf(ring)
	return append(coordinates, coordinates[0])
}

func coordinatesOf(points []mapdata.Point) [][]float64 {
	coordinates := make([][]float64, len(points))
	for index, point := range points {
		coordinates[index] = []float64{point[0], point[1]}
	}
	return coordinates
}

// signedArea is positive when a ring turns anticlockwise (east-north axes).
func signedArea(ring mapdata.Ring) float64 {
	area := 0.0
	for index := range ring {
		next := ring[(index+1)%len(ring)]
		area += ring[index][0]*next[1] - next[0]*ring[index][1]
	}
	return area / 2
}

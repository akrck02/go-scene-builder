package recipes

import (
	"math"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/style"
)

var upwards = mesh.Vector{0, 1, 0}

// flatRoof covers the footprint at the eaves.
func (decorator *Decorator) flatRoof(current *block.Block, piece *mesh.Piece, roof style.Style) error {
	color, err := roof.Color("color", current.Feature.Properties, [4]float32{0.5, 0.3, 0.2, 1})
	if err != nil {
		return err
	}
	rings := [][][]float64{closedCoordinates(current.Polygon.Exterior)}
	for _, hole := range current.Polygon.Holes {
		rings = append(rings, closedCoordinates(hole))
	}
	geometry := decorator.Context.NewPolygon(rings).MakeValid()
	for part := range geometry.NumGeometries() {
		triangulation := geometry.Geometry(part).ConstrainedDelaunayTriangulation()
		for index := range triangulation.NumGeometries() {
			corners := triangulation.Geometry(index).ExteriorRing().CoordSeq().ToCoords()
			if len(corners) >= 3 {
				piece.AddTriangle(decorator.at(corners[0], current.Eaves), decorator.at(corners[1], current.Eaves),
					decorator.at(corners[2], current.Eaves), upwards, color)
			}
		}
	}
	return nil
}

// slopedRoof puts a gable or hip roof over the smallest rectangle around the footprint: two slopes
// from the eaves (lowered by the overhang) up to the ridge, and two gable ends (wall colour) or
// two more slopes (hip).
func (decorator *Decorator) slopedRoof(current *block.Block, pieceFor PieceFor, piece, wallPiece *mesh.Piece, roof style.Style, walls [4]float32, hip bool) error {
	values := map[string]float64{"pitch": 38, "eaves": 0.5}
	for key, fallback := range values {
		value, err := roof.Number(key, current.Feature.Properties, fallback)
		if err != nil {
			return err
		}
		values[key] = value
	}
	color, err := roof.Color("color", current.Feature.Properties, [4]float32{0.5, 0.3, 0.2, 1})
	if err != nil {
		return err
	}
	corners := decorator.rectangle(current)
	if corners == nil {
		return decorator.flatRoof(current, piece, roof)
	}
	corners = alongRidge(corners, roof.Text("ridge", "long"), current.Facing)
	decorator.addSlopes(piece, wallPiece, corners, current.Eaves, values["pitch"], values["eaves"]/1000, hip, color, walls)
	return decorator.chimney(current, pieceFor, roof.Section("chimneys"), corners, values["pitch"])
}

var chimneyMaterial = mesh.Material{Name: "chimneys", Kind: "lit", HideBelow: facadeHideBelow}

// chimney raises, on a share of the houses, a stone chimney through one slope of the roof, between
// a fifth and four fifths along the ridge (chimneys: {share, width, above, color, material}).
func (decorator *Decorator) chimney(current *block.Block, pieceFor PieceFor, settings style.Style, corners mapdata.Ring, pitch float64) error {
	properties := current.Feature.Properties
	share, err := settings.Number("share", properties, 0)
	if err != nil || !chosen(properties, "chimney", share) {
		return err
	}
	width, err := settings.Number("width", properties, 0.6)
	if err != nil {
		return err
	}
	above, err := settings.Number("above", properties, 0.9)
	if err != nil {
		return err
	}
	color, err := settings.Color("color", properties, [4]float32{0.55, 0.5, 0.45, 1})
	if err != nil {
		return err
	}
	along, across := mapdata.Direction(corners[0], corners[1]), mapdata.Direction(corners[0], corners[3])
	halfWidth := mapdata.Distance(corners[0], corners[3]) * 1000 / 2
	length := mapdata.Distance(corners[0], corners[1]) * 1000
	where := 0.2 + 0.6*float64(style.Pick(properties, "chimney place", 1000))/1000
	fromRidge := halfWidth * 0.45 // down one slope
	ridgeHeight := current.Eaves + halfWidth*math.Tan(pitch*math.Pi/180)
	base := ridgeHeight - (fromRidge+width)*math.Tan(pitch*math.Pi/180)
	start := mapdata.Point{corners[0][0] + (along[0]*length*where+across[0]*(halfWidth-fromRidge-width))/1000,
		corners[0][1] + (along[1]*length*where+across[1]*(halfWidth-fromRidge-width))/1000}
	stack := wallFrame{frame: decorator.Frame, start: start, along: along, outward: across, ground: 0, length: width}
	piece := pieceFor(MaterialOf(chimneyMaterial, settings))
	stack.box(piece, 0, width, base, ridgeHeight+above, 0, width, color)
	stack.box(piece, -0.06, width+0.06, ridgeHeight+above, ridgeHeight+above+0.12, -0.06, width+0.06, color) // the cap
	return nil
}

// rectangle is the smallest rotated rectangle around the footprint, anticlockwise.
func (decorator *Decorator) rectangle(current *block.Block) mapdata.Ring {
	geometry := decorator.Context.NewPolygon([][][]float64{closedCoordinates(current.Polygon.Exterior)}).MinimumRotatedRectangle()
	if geometry.IsEmpty() {
		return nil
	}
	coordinates := geometry.ExteriorRing().CoordSeq().ToCoords()
	if len(coordinates) < 5 {
		return nil
	}
	ring := mapdata.Ring{}
	for _, coordinate := range coordinates[:4] {
		ring = append(ring, mapdata.Point{coordinate[0], coordinate[1]})
	}
	return ring.Anticlockwise()
}

// alongRidge turns the rectangle so the ridge runs from its first corner to its second.
func alongRidge(corners mapdata.Ring, ridge string, facing *block.Facing) mapdata.Ring {
	first, second := mapdata.Distance(corners[0], corners[1]), mapdata.Distance(corners[1], corners[2])
	turn := false
	switch {
	case ridge == "short":
		turn = first > second
	case (ridge == "along_street" || ridge == "across_street") && facing != nil:
		direction := mapdata.Direction(corners[0], corners[1])
		parallel := math.Abs(direction[0]*facing.Outward[0]+direction[1]*facing.Outward[1]) < 0.5 // first side runs along the street
		turn = parallel != (ridge == "along_street")
	default:
		turn = first < second
	}
	if turn {
		return mapdata.Ring{corners[1], corners[2], corners[3], corners[0]}
	}
	return corners
}

func (decorator *Decorator) addSlopes(piece, wallPiece *mesh.Piece, corners mapdata.Ring, eaves, pitch, overhang float64, hip bool, color, walls [4]float32) {
	along, across := mapdata.Direction(corners[0], corners[1]), mapdata.Direction(corners[0], corners[3])
	halfWidth := mapdata.Distance(corners[0], corners[3]) / 2
	halfLength := mapdata.Distance(corners[0], corners[1]) / 2
	slope := math.Tan(pitch * math.Pi / 180)
	shift := func(point mapdata.Point, alongSteps, acrossSteps float64) mapdata.Point {
		return mapdata.Point{point[0] + along[0]*alongSteps + across[0]*acrossSteps, point[1] + along[1]*alongSteps + across[1]*acrossSteps}
	}
	outer := mapdata.Ring{shift(corners[0], -overhang, -overhang), shift(corners[1], overhang, -overhang),
		shift(corners[2], overhang, overhang), shift(corners[3], -overhang, overhang)}
	ridgeHeight := eaves + halfWidth*1000*slope
	edgeHeight := eaves - overhang*1000*slope
	start, end := mapdata.Middle(outer[0], outer[3]), mapdata.Middle(outer[1], outer[2])
	if hip {
		inset := math.Min(halfWidth+overhang, halfLength+overhang-0.0001)
		start, end = shift(start, inset, 0), shift(end, -inset, 0)
	}
	low := func(point mapdata.Point) mesh.Vector { return decorator.at([]float64{point[0], point[1]}, edgeHeight) }
	high := func(point mapdata.Point) mesh.Vector { return decorator.at([]float64{point[0], point[1]}, ridgeHeight) }
	piece.AddQuad(low(outer[0]), low(outer[1]), high(end), high(start), upwards, color)
	piece.AddQuad(low(outer[2]), low(outer[3]), high(start), high(end), upwards, color)
	if hip {
		piece.AddTriangle(low(outer[1]), low(outer[2]), high(end), upwards, color)
		piece.AddTriangle(low(outer[3]), low(outer[0]), high(start), upwards, color)
		return
	}
	for _, end := range [][3]mapdata.Point{{corners[1], corners[2], along}, {corners[3], corners[0], {-along[0], -along[1]}}} {
		apex := mapdata.Middle(end[0], end[1])
		wallPiece.AddTriangle(decorator.at([]float64{end[0][0], end[0][1]}, eaves), decorator.at([]float64{end[1][0], end[1][1]}, eaves),
			decorator.at([]float64{apex[0], apex[1]}, ridgeHeight-0.02), mesh.SceneDirection(end[2][0], end[2][1], 0), walls)
	}
}

func (decorator *Decorator) at(coordinate []float64, height float64) mesh.Vector {
	return decorator.Frame.Scene(coordinate[0], coordinate[1], height)
}

func closedCoordinates(ring mapdata.Ring) [][]float64 {
	coordinates := make([][]float64, 0, len(ring)+1)
	for _, point := range ring {
		coordinates = append(coordinates, []float64{point[0], point[1]})
	}
	return append(coordinates, coordinates[0])
}

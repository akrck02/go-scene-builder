package recipes

import (
	"math"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/style"
)

// The stonework recipe dresses a house in stone where a mason would: a plinth along the foot of
// every wall that shows, and a column of dressed stones up the corners (sizes in metres):
//
//	stonework:
//	  plinth: {height: 0.8, out: 0.06}
//	  quoins: {width: 0.45, out: 0.04}
//	  color: ["#b9b0a0", "#a9a090"]
//	  material: {pattern: masonry, scale: 0.35}
//
// Window and door frames come with the facade (facade.frames).

// stoneMaterial hides from far away like the facade details: a plinth and corner stones are a few
// pixels then.
var stoneMaterial = mesh.Material{Name: "stonework", Kind: "lit", HideBelow: facadeHideBelow}

func (decorator *Decorator) stonework(current *block.Block, pieceFor PieceFor) error {
	settings := current.Style.Section("stonework")
	if len(settings) == 0 {
		return nil
	}
	properties := current.Feature.Properties
	color, err := settings.Color("color", properties, [4]float32{0.7, 0.67, 0.6, 1})
	if err != nil {
		return err
	}
	values, err := stoneworkNumbers(settings, properties)
	if err != nil {
		return err
	}
	piece := pieceFor(MaterialOf(stoneMaterial, settings))
	exterior := current.Polygon.Exterior
	for side := range exterior {
		if current.SharedUpTo(side) > current.Bottom {
			continue // a neighbour leans against it
		}
		from, to := current.Side(side)
		wall := wallFrame{frame: decorator.Frame, start: from, along: mapdata.Direction(from, to),
			outward: mapdata.Outward(from, to), ground: current.Ground, length: mapdata.Distance(from, to) * 1000}
		bottom := current.Bottom - current.Ground
		if values["plinth height"] > 0 {
			plinth(current, side, wall, piece, bottom, values, color)
		}
		if values["quoin width"] > 0 {
			decorator.quoins(wall, piece, values, current.Eaves-current.Ground, color)
		}
	}
	return nil
}

// plinth runs a stone base along the foot of a wall, from below the ground to "plinth height", and
// leaves the door and its steps free.
func plinth(current *block.Block, side int, wall wallFrame, piece *mesh.Piece, bottom float64, values map[string]float64, color [4]float32) {
	out, height := values["plinth out"], values["plinth height"]
	from, to := -out, wall.length+out
	if gap, found := doorGap(current, side, wall.length); found {
		wall.box(piece, from, gap.from, bottom, height, 0, out, color)
		from = gap.to
	}
	wall.box(piece, from, to, bottom, height, 0, out, color)
}

// stretch is a part of a wall, in metres along it.
type stretch struct{ from, to float64 }

// defaultDoorWidth is the facade's door width when the style does not set it (see facadeNumbers).
const defaultDoorWidth = 1.1

// doorGap is the stretch of the front that the plinth leaves free for the door and its steps: the
// door stands in the middle of the front (see facade). False on any other side.
func doorGap(current *block.Block, side int, length float64) (stretch, bool) {
	facade := current.Style.Section("facade")
	if current.Facing == nil || current.Facing.Side != side || len(facade) == 0 {
		return stretch{}, false
	}
	width, err := facade.Section("door").Number("width", current.Feature.Properties, defaultDoorWidth)
	if err != nil {
		width = defaultDoorWidth
	}
	halfGap := width/2 + stepsMargin
	return stretch{from: length/2 - halfGap, to: length/2 + halfGap}, true
}

func stoneworkNumbers(settings style.Style, properties map[string]any) (map[string]float64, error) {
	values := map[string]float64{}
	for _, entry := range []struct {
		name, section, key string
		fallback           float64
	}{
		{"plinth height", "plinth", "height", 0}, {"plinth out", "plinth", "out", 0.06},
		{"quoin width", "quoins", "width", 0}, {"quoin out", "quoins", "out", 0.04},
	} {
		value, err := settings.Section(entry.section).Number(entry.key, properties, entry.fallback)
		if err != nil {
			return nil, err
		}
		values[entry.name] = value
	}
	return values, nil
}

// quoins dress the start of a wall (its end is the start of the next one) with a column of corner
// stones from the plinth to the eaves: one box, whose stone pattern draws the courses (a stone per
// course would cost twelve triangles each, on every corner of every house).
func (decorator *Decorator) quoins(wall wallFrame, piece *mesh.Piece, values map[string]float64, height float64, color [4]float32) {
	bottom := math.Max(values["plinth height"], 0)
	if height-bottom < 0.5 {
		return
	}
	wall.box(piece, 0, values["quoin width"], bottom, height, 0, values["quoin out"], color)
}

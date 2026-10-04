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
			wall.box(piece, -values["plinth out"], wall.length+values["plinth out"], bottom, values["plinth height"], 0, values["plinth out"], color)
		}
		if values["quoin width"] > 0 {
			decorator.quoins(wall, piece, values, current.Eaves-current.Ground, color)
		}
	}
	return nil
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

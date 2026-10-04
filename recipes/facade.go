package recipes

import (
	"math"
	"math/rand/v2"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/style"
)

// The facade recipe dresses the side of a block that faces a street ("facade" in the style; all
// sizes in metres, colours or palettes, materials as everywhere):
//
//	facade:
//	  ground_floor: 3.5        # height of the ground floor; floor_height for the others
//	  floor_height: 3
//	  door: {width: 1.1, height: 2.3, color: [...], material: {...}}
//	  windows: {width: 0.9, height: 1.3, sill: 1, spacing: 2.6, color: "#2b3038", shutters: [...]}
//	  flower_boxes: {share: 0.6, color: "#6b4a2e", flowers: [...], glowing: {share: 0.3, color: "#d8e8ff"}}
//	  emblem: {size: 0.6, color: "#d9dee6"}       # a six-petal flower above the door
//	  lamp: {share: 0.3, color: "#ffb04a"}          # a glowing lamp beside the door
//
// Every choice (palette colours, which windows get flower boxes) is stable for each block.

// facadeMaterials are the materials of facade parts that do not set their own.
var (
	glassMaterial  = mesh.Material{Name: "glass", Kind: "lit", HideBelow: facadeHideBelow}
	detailMaterial = mesh.Material{Name: "details", Kind: "lit", HideBelow: facadeHideBelow}
	glowMaterial   = mesh.Material{Name: "glowing details", Kind: "glow", HideBelow: facadeHideBelow}
)

// wallFrame places things on a facade: along the side (metres from its start), up from the ground
// (metres) and out of the wall (metres).
type wallFrame struct {
	frame          mesh.Frame
	start          mapdata.Point
	along, outward mapdata.Point
	ground         float64
	length         float64 // metres
}

func (wall wallFrame) point(along, up, out float64) mesh.Vector {
	east := wall.start[0] + (wall.along[0]*along+wall.outward[0]*out)/1000
	north := wall.start[1] + (wall.along[1]*along+wall.outward[1]*out)/1000
	return wall.frame.Scene(east, north, wall.ground+up)
}

// box adds a box standing out of the wall: from along0 to along1, up0 to up1, out0 to out1.
func (wall wallFrame) box(piece *mesh.Piece, along0, along1, up0, up1, out0, out1 float64, color [4]float32) {
	outward := mesh.SceneDirection(wall.outward[0], wall.outward[1], 0)
	sideways := mesh.SceneDirection(wall.along[0], wall.along[1], 0)
	up, down := mesh.Vector{0, 1, 0}, mesh.Vector{0, -1, 0}
	corner := func(along, height, out float64) mesh.Vector { return wall.point(along, height, out) }
	piece.AddQuad(corner(along0, up0, out1), corner(along1, up0, out1), corner(along1, up1, out1), corner(along0, up1, out1), outward, color)
	piece.AddQuad(corner(along0, up1, out0), corner(along1, up1, out0), corner(along1, up1, out1), corner(along0, up1, out1), up, color)
	piece.AddQuad(corner(along0, up0, out0), corner(along1, up0, out0), corner(along1, up0, out1), corner(along0, up0, out1), down, color)
	piece.AddQuad(corner(along0, up0, out0), corner(along0, up0, out1), corner(along0, up1, out1), corner(along0, up1, out0), sideways.Scale(-1), color)
	piece.AddQuad(corner(along1, up0, out0), corner(along1, up0, out1), corner(along1, up1, out1), corner(along1, up1, out0), sideways, color)
}

// facadeOf is the wall frame of the side that faces a street, or false when there is none (or a
// neighbour covers it).
func (decorator *Decorator) facadeOf(current *block.Block) (wallFrame, bool) {
	if current.Facing == nil || !math.IsInf(current.SharedUpTo(current.Facing.Side), -1) {
		return wallFrame{}, false
	}
	from, to := current.Side(current.Facing.Side)
	return wallFrame{frame: decorator.Frame, start: from, along: mapdata.Direction(from, to), outward: current.Facing.Outward,
		ground: current.Ground, length: mapdata.Distance(from, to) * 1000}, true
}

// facade adds door, windows, shutters, flower boxes, emblem and lamp to the side facing a street.
func (decorator *Decorator) facade(current *block.Block, pieceFor PieceFor) error {
	settings := current.Style.Section("facade")
	wall, found := decorator.facadeOf(current)
	if len(settings) == 0 || !found || wall.length < 2 {
		return nil
	}
	properties := current.Feature.Properties
	random := rand.New(rand.NewPCG(style.Seed(properties, "facade"), uint64(current.Facing.Side)))
	values, err := facadeNumbers(settings, properties)
	if err != nil {
		return err
	}
	door := values["door width"] / 2
	doorAt := wall.length / 2
	floors := max(1, int(math.Round((current.Eaves-current.Ground-values["ground floor"])/values["floor height"]))+1)
	if err := decorator.door(wall, settings, properties, pieceFor, doorAt, door, values); err != nil {
		return err
	}
	for floor := range floors {
		base := 0.0
		if floor > 0 {
			base = values["ground floor"] + float64(floor-1)*values["floor height"]
		}
		if base+values["sill"]+values["window height"] > current.Eaves-current.Ground-0.3 {
			break
		}
		if err := decorator.windowRow(wall, settings, properties, pieceFor, random, floor, base, doorAt, door, values); err != nil {
			return err
		}
	}
	return nil
}

func facadeNumbers(settings style.Style, properties map[string]any) (map[string]float64, error) {
	values := map[string]float64{}
	for _, entry := range []struct {
		name, section, key string
		fallback           float64
	}{
		{"ground floor", "", "ground_floor", 3.5}, {"floor height", "", "floor_height", 3},
		{"door width", "door", "width", 1.1}, {"door height", "door", "height", 2.3},
		{"window width", "windows", "width", 0.9}, {"window height", "windows", "height", 1.3},
		{"sill", "windows", "sill", 1}, {"spacing", "windows", "spacing", 2.6},
		{"box share", "flower_boxes", "share", 0}, {"glowing share", "flower_boxes", "glowing", 0},
		{"emblem size", "emblem", "size", 0}, {"lamp share", "lamp", "share", 0},
	} {
		section := settings
		if entry.section != "" {
			section = settings.Section(entry.section)
		}
		if entry.name == "glowing share" {
			section = settings.Section("flower_boxes").Section("glowing")
			entry.key = "share"
		}
		value, err := section.Number(entry.key, properties, entry.fallback)
		if err != nil {
			return nil, err
		}
		values[entry.name] = value
	}
	return values, nil
}

// door adds the door, the emblem above it and, sometimes, a lamp beside it.
func (decorator *Decorator) door(wall wallFrame, settings style.Style, properties map[string]any, pieceFor PieceFor, at, half float64, values map[string]float64) error {
	section := settings.Section("door")
	color, err := section.Color("color", properties, [4]float32{0.25, 0.15, 0.08, 1})
	if err != nil {
		return err
	}
	wall.box(pieceFor(MaterialOf(detailMaterial, section)), at-half, at+half, 0, values["door height"], 0, 0.06, color)
	if size := values["emblem size"]; size > 0 {
		emblem := settings.Section("emblem")
		silver, err := emblem.Color("color", properties, [4]float32{0.7, 0.72, 0.76, 1})
		if err != nil {
			return err
		}
		decorator.emblem(wall, pieceFor(MaterialOf(detailMaterial, emblem)), at, values["door height"]+0.25+size/2, size/2, silver)
	}
	lamp := settings.Section("lamp")
	if values["lamp share"] > 0 && float64(style.Pick(properties, "lamp", 1000))/1000 < values["lamp share"] {
		light, err := lamp.Color("color", properties, [4]float32{1, 0.6, 0.2, 1})
		if err != nil {
			return err
		}
		side := at + half + 0.45
		wall.box(pieceFor(detailMaterial), side-0.04, side+0.04, 2.0, 2.4, 0, 0.12, [4]float32{0.08, 0.08, 0.08, 1})
		wall.box(pieceFor(MaterialOf(glowMaterial, lamp)), side-0.1, side+0.1, 2.05, 2.35, 0.12, 0.32, light)
	}
	return nil
}

// emblem adds a six-petal flower in relief on the wall (a town's badge, say): rounded petals
// around a raised centre.
func (decorator *Decorator) emblem(wall wallFrame, piece *mesh.Piece, along, up, radius float64, color [4]float32) {
	const petals, corners = 6, 48
	outward := mesh.SceneDirection(wall.outward[0], wall.outward[1], 0)
	centre := wall.point(along, up, 0.09)
	rim := make([]mesh.Vector, 0, corners)
	for corner := range corners {
		angle := 2 * math.Pi * float64(corner) / corners
		reach := radius * (0.45 + 0.55*math.Abs(math.Cos(petals/2*angle)))
		rim = append(rim, wall.point(along+reach*math.Cos(angle), up+reach*math.Sin(angle), 0.06))
	}
	for corner := range rim {
		piece.AddTriangle(centre, rim[corner], rim[(corner+1)%len(rim)], outward, color)
	}
}

// windowRow adds the windows of one floor: evenly spaced, none where the door is, each with
// shutters and, on upper floors, sometimes a flower box.
func (decorator *Decorator) windowRow(wall wallFrame, settings style.Style, properties map[string]any, pieceFor PieceFor, random *rand.Rand,
	floor int, base, doorAt, doorHalf float64, values map[string]float64) error {
	windows := settings.Section("windows")
	glass, err := windows.Color("color", properties, [4]float32{0.03, 0.04, 0.06, 1})
	if err != nil {
		return err
	}
	shutters, err := style.Style{"color": windows["shutters"]}.Color("color", properties, [4]float32{0.2, 0.3, 0.25, 1})
	if err != nil {
		return err
	}
	half := values["window width"] / 2
	count := max(1, int((wall.length-1)/values["spacing"]))
	sill, top := base+values["sill"], base+values["sill"]+values["window height"]
	for slot := range count {
		at := (float64(slot) + 0.5) * wall.length / float64(count)
		if floor == 0 && math.Abs(at-doorAt) < doorHalf+half+0.4 {
			continue
		}
		if at-half-half < 0.2 || at+half+half > wall.length-0.2 {
			continue
		}
		wall.box(pieceFor(glassMaterial), at-half, at+half, sill, top, 0, 0.03, glass)
		shutterPiece := pieceFor(MaterialOf(detailMaterial, windows))
		wall.box(shutterPiece, at-2*half, at-half-0.02, sill, top, 0, 0.05, shutters)
		wall.box(shutterPiece, at+half+0.02, at+2*half, sill, top, 0, 0.05, shutters)
		if floor > 0 && random.Float64() < values["box share"] {
			if err := decorator.flowerBox(wall, settings.Section("flower_boxes"), properties, pieceFor, random, at, half, sill, values); err != nil {
				return err
			}
		}
	}
	return nil
}

// flowerBox adds a box under a window with a row of flowers, glowing ones now and then.
func (decorator *Decorator) flowerBox(wall wallFrame, settings style.Style, properties map[string]any, pieceFor PieceFor, random *rand.Rand,
	at, half, sill float64, values map[string]float64) error {
	wood, err := settings.Color("color", properties, [4]float32{0.3, 0.2, 0.1, 1})
	if err != nil {
		return err
	}
	wall.box(pieceFor(MaterialOf(detailMaterial, settings)), at-half-0.1, at+half+0.1, sill-0.22, sill-0.02, 0, 0.28, wood)
	glowing := settings.Section("glowing")
	glows := random.Float64() < values["glowing share"]
	palette, _ := settings["flowers"].([]any)
	for along := at - half - 0.02; along < at+half; along += 0.16 {
		var color [4]float32
		piece := pieceFor(flowerMaterial)
		if glows {
			color, err = glowing.Color("color", properties, [4]float32{0.8, 0.9, 1, 1})
			piece = pieceFor(MaterialOf(glowMaterial, glowing))
		} else if len(palette) > 0 {
			color, err = style.LinearColor(asText(palette[random.IntN(len(palette))]))
		} else {
			color = [4]float32{0.8, 0.2, 0.3, 1}
		}
		if err != nil {
			return err
		}
		height := 0.14 + 0.12*random.Float64()
		wall.box(piece, along, along+0.11, sill-0.02, sill-0.02+height, 0.06+0.06*random.Float64(), 0.22, color)
	}
	return nil
}

var flowerMaterial = mesh.Material{Name: "flowers", Kind: "lit", HideBelow: facadeHideBelow}

// facadeHideBelow: facade details are not drawn when their square of the map looks smaller than
// this share of the screen (far away, they are a few pixels and cost much).
const facadeHideBelow = 0.12

func asText(value any) string {
	text, _ := value.(string)
	return text
}

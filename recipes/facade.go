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
//	  door: {width: 1.1, height: 2.3, round: 0.3, color: [...], material: {...}}   # round: share of round-topped doors
//	  windows: {width: 0.9, height: 1.3, sill: 1, spacing: 2.6, color: "#2b3038", shutters: [...]}
//	  flower_boxes: {share: 0.6, color: "#6b4a2e", flowers: [...], glowing: {share: 0.3, color: "#d8e8ff"}}
//	  emblem: {size: 0.6, color: "#d9dee6"}       # a six-petal flower above the door
//	  lamp: {share: 0.3, color: "#ffb04a"}          # a glowing lamp beside the door
//	  baskets: {share: 0.3, color: "#8a6a44", flowers: [...]}   # flower baskets hung beside the door
//	  ivy: {share: 0.2, color: ["#3f6b32", ...], height: [2, 6]} # ivy climbing the wall (takes the season)
//	  frames: {width: 0.14, out: 0.05, color: "#c9c0ae"}         # stone frames and sills round windows and doors
//	  awning: {share: 0.6, depth: 1.4, color: [...]}               # a cloth awning over the ground floor, with a fringe
//	  cornice: {height: 0.35, out: 0.3, color: "#d8cfbf"}          # a moulding under the eaves
//	  sides: {spacing: 4.5, flower_boxes: 0.2, frames: false}   # windows (no door) on the other open walls: sparser, plainer
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
// post adds an upright many-sided post standing out of the wall: its axis "out" metres from the
// wall, "along" it, from bottom to top; a basket, a pot, a column.
func (wall wallFrame) post(piece *mesh.Piece, along, out, radius, bottom, top float64, sides int, color [4]float32) {
	ring := func(up float64) []mesh.Vector {
		points := make([]mesh.Vector, sides)
		for side := range sides {
			angle := 2 * math.Pi * float64(side) / float64(sides)
			points[side] = wall.point(along+radius*math.Cos(angle), up, out+radius*math.Sin(angle))
		}
		return points
	}
	low, high := ring(bottom), ring(top)
	axisLow, axisHigh := wall.point(along, bottom, out), wall.point(along, top, out)
	for side := range sides {
		next := (side + 1) % sides
		middle := low[side].Add(low[next]).Scale(0.5).Subtract(axisLow)
		piece.AddQuad(low[side], low[next], high[next], high[side], middle, color)
		piece.AddTriangle(axisHigh, high[side], high[next], mesh.Vector{0, 1, 0}, color)
		piece.AddTriangle(axisLow, low[next], low[side], mesh.Vector{0, -1, 0}, color)
	}
}

// pane is a flat rectangle facing out of the wall, "out" metres in front of it.
func (wall wallFrame) pane(piece *mesh.Piece, along0, along1, up0, up1, out float64, color [4]float32) {
	piece.AddQuad(wall.point(along0, up0, out), wall.point(along1, up0, out), wall.point(along1, up1, out), wall.point(along0, up1, out),
		mesh.SceneDirection(wall.outward[0], wall.outward[1], 0), color)
}

func (wall wallFrame) box(piece *mesh.Piece, along0, along1, up0, up1, out0, out1 float64, color [4]float32) {
	outward := mesh.SceneDirection(wall.outward[0], wall.outward[1], 0)
	sideways := mesh.SceneDirection(wall.along[0], wall.along[1], 0)
	up, down := mesh.Vector{0, 1, 0}, mesh.Vector{0, -1, 0}
	corner := func(along, height, out float64) mesh.Vector { return wall.point(along, height, out) }
	piece.AddQuad(corner(along0, up0, out1), corner(along1, up0, out1), corner(along1, up1, out1), corner(along0, up1, out1), outward, color)
	piece.AddQuad(corner(along0, up1, out0), corner(along1, up1, out0), corner(along1, up1, out1), corner(along0, up1, out1), up, color)
	if up0 > 0.01 { // the underside of what stands on the ground never shows
		piece.AddQuad(corner(along0, up0, out0), corner(along1, up0, out0), corner(along1, up0, out1), corner(along0, up0, out1), down, color)
	}
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

// facade adds door, windows, shutters, flower boxes, emblem and lamp to the side facing a street,
// and, with "sides", windows to the other open walls.
func (decorator *Decorator) facade(current *block.Block, pieceFor PieceFor) error {
	settings := current.Style.Section("facade")
	if len(settings) == 0 {
		return nil
	}
	if err := decorator.sideWindows(current, settings, pieceFor); err != nil {
		return err
	}
	wall, found := decorator.facadeOf(current)
	if !found || wall.length < 2 {
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
	if err := decorator.door(wall, settings, properties, pieceFor, doorAt, door, values); err != nil {
		return err
	}
	if values["basket share"] > 0 && chosen(properties, "baskets", values["basket share"]) {
		if err := decorator.baskets(wall, settings.Section("baskets"), properties, pieceFor, random, doorAt, door); err != nil {
			return err
		}
	}
	if values["awning share"] > 0 && chosen(properties, "awning", values["awning share"]) {
		if err := decorator.awning(wall, settings.Section("awning"), properties, pieceFor, values); err != nil {
			return err
		}
	}
	if values["cornice height"] > 0 {
		if err := decorator.cornice(wall, settings.Section("cornice"), properties, pieceFor, current.Eaves-current.Ground, values); err != nil {
			return err
		}
	}
	if values["ivy share"] > 0 && chosen(properties, "ivy", values["ivy share"]) {
		if err := decorator.ivy(wall, settings.Section("ivy"), properties, pieceFor, random, current.Eaves-current.Ground); err != nil {
			return err
		}
	}
	for floor, base := range floorBases(current, values) {
		if err := decorator.windowRow(wall, settings, properties, pieceFor, random, floor, base, doorAt, door, values); err != nil {
			return err
		}
	}
	return nil
}

// sideWindows puts rows of windows, "sides spacing" apart, on every wall that is not the front and
// that no neighbour leans against.
func (decorator *Decorator) sideWindows(current *block.Block, settings style.Style, pieceFor PieceFor) error {
	sides := settings.Section("sides")
	if len(sides) == 0 {
		return nil
	}
	properties := current.Feature.Properties
	values, err := facadeNumbers(settings, properties)
	if err != nil {
		return err
	}
	if values["spacing"], err = sides.Number("spacing", properties, 2*values["spacing"]); err != nil {
		return err
	}
	if values["box share"], err = sides.Number("flower_boxes", properties, 0); err != nil {
		return err
	}
	if framed, _ := sides["frames"].(bool); !framed {
		values["frame width"] = 0
	}
	for side := range current.Polygon.Exterior {
		wall, open := decorator.openWall(current, side)
		if !open || wall.length < values["spacing"] {
			continue
		}
		random := rand.New(rand.NewPCG(style.Seed(properties, "sides"), uint64(side)))
		for floor, base := range floorBases(current, values) {
			if err := decorator.windowRow(wall, settings, properties, pieceFor, random, floor, base, math.Inf(-1), 0, values); err != nil {
				return err
			}
		}
	}
	return nil
}

// openWall is a side of the block that is not its front and that no neighbour covers.
func (decorator *Decorator) openWall(current *block.Block, side int) (wallFrame, bool) {
	if (current.Facing != nil && current.Facing.Side == side) || !math.IsInf(current.SharedUpTo(side), -1) {
		return wallFrame{}, false
	}
	from, to := current.Side(side)
	along := mapdata.Direction(from, to)
	outward := mapdata.Point{along[1], -along[0]} // the exterior is anticlockwise: outside is on the right
	return wallFrame{frame: decorator.Frame, start: from, along: along, outward: outward, ground: current.Ground,
		length: mapdata.Distance(from, to) * 1000}, true
}

// floorBases are the heights of the floors (metres above the ground) that have room for a window.
func floorBases(current *block.Block, values map[string]float64) []float64 {
	var bases []float64
	floors := max(1, int(math.Round((current.Eaves-current.Ground-values["ground floor"])/values["floor height"]))+1)
	for floor := range floors {
		base := 0.0
		if floor > 0 {
			base = values["ground floor"] + float64(floor-1)*values["floor height"]
		}
		if base+values["sill"]+values["window height"] > current.Eaves-current.Ground-0.3 {
			break
		}
		bases = append(bases, base)
	}
	return bases
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
		{"round share", "door", "round", 0}, {"basket share", "baskets", "share", 0}, {"ivy share", "ivy", "share", 0},
		{"frame width", "frames", "width", 0}, {"frame out", "frames", "out", 0.05},
		{"awning share", "awning", "share", 0}, {"awning depth", "awning", "depth", 1.4},
		{"cornice height", "cornice", "height", 0}, {"cornice out", "cornice", "out", 0.3},
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
	if err := decorator.frame(wall, settings, properties, pieceFor, at, half, 0, values["door height"], false, values); err != nil {
		return err
	}
	if chosen(properties, "round door", values["round share"]) {
		decorator.roundDoor(wall, pieceFor(MaterialOf(detailMaterial, section)), at, half, values["door height"], color)
	} else {
		wall.box(pieceFor(MaterialOf(detailMaterial, section)), at-half, at+half, 0, values["door height"], 0, 0.06, color)
	}
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
		wall.pane(pieceFor(glassMaterial), at-half, at+half, sill, top, 0.03, glass)
		if err := decorator.frame(wall, settings, properties, pieceFor, at, half, sill, top, true, values); err != nil {
			return err
		}
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

var (
	flowerMaterial = mesh.Material{Name: "flowers", Kind: "lit", HideBelow: facadeHideBelow}
	ivyMaterial    = mesh.Material{Name: "ivy", Kind: "lit", Foliage: true, HideBelow: facadeHideBelow}
)

// frame puts a stone frame round an opening: jambs, a lintel and, under a window, a sill that juts out.
func (decorator *Decorator) frame(wall wallFrame, settings style.Style, properties map[string]any, pieceFor PieceFor, at, half, bottom, top float64, sill bool, values map[string]float64) error {
	width, out := values["frame width"], values["frame out"]
	if width <= 0 {
		return nil
	}
	section := settings.Section("frames")
	color, err := section.Color("color", properties, [4]float32{0.78, 0.75, 0.68, 1})
	if err != nil {
		return err
	}
	piece := pieceFor(MaterialOf(detailMaterial, section))
	wall.box(piece, at-half-width, at-half, bottom, top+width, 0, out, color)
	wall.box(piece, at+half, at+half+width, bottom, top+width, 0, out, color)
	wall.box(piece, at-half-width, at+half+width, top, top+width, 0, out, color)
	if sill {
		wall.box(piece, at-half-width-0.05, at+half+width+0.05, bottom-0.08, bottom, 0, out+0.08, color)
	}
	return nil
}

// awning stretches a cloth over the ground floor, from wall to wall, with a fringe hanging at its front.
func (decorator *Decorator) awning(wall wallFrame, settings style.Style, properties map[string]any, pieceFor PieceFor, values map[string]float64) error {
	color, err := settings.Color("color", properties, [4]float32{0.7, 0.3, 0.25, 1})
	if err != nil {
		return err
	}
	piece := pieceFor(MaterialOf(awningMaterial, settings))
	top := math.Min(values["ground floor"]-0.25, values["door height"]+0.55)
	depth := values["awning depth"]
	wall.box(piece, 0.2, wall.length-0.2, top, top+0.06, 0, depth, color)
	wall.box(piece, 0.2, wall.length-0.2, top-0.3, top, depth-0.04, depth, color) // the fringe
	return nil
}

// cornice runs a moulding along the top of the wall, under the eaves.
func (decorator *Decorator) cornice(wall wallFrame, settings style.Style, properties map[string]any, pieceFor PieceFor, height float64, values map[string]float64) error {
	color, err := settings.Color("color", properties, [4]float32{0.85, 0.82, 0.75, 1})
	if err != nil {
		return err
	}
	piece := pieceFor(MaterialOf(detailMaterial, settings))
	band := values["cornice height"]
	wall.box(piece, -0.05, wall.length+0.05, height-band, height-band*0.45, 0, values["cornice out"]*0.6, color)
	wall.box(piece, -0.08, wall.length+0.08, height-band*0.45, height, 0, values["cornice out"], color)
	return nil
}

var awningMaterial = mesh.Material{Name: "awnings", Kind: "lit", DoubleSided: true, HideBelow: facadeHideBelow}

// chosen tells, stably for each block, whether it gets something that only a share of blocks get.
func chosen(properties map[string]any, what string, share float64) bool {
	return share > 0 && float64(style.Pick(properties, what, 1000))/1000 < share
}

// roundDoor adds a door whose top is a half circle: the straight part and a fan above it.
func (decorator *Decorator) roundDoor(wall wallFrame, piece *mesh.Piece, at, half, height float64, color [4]float32) {
	springing := math.Max(height-half, height*0.5)
	wall.box(piece, at-half, at+half, 0, springing, 0, 0.06, color)
	outward := mesh.SceneDirection(wall.outward[0], wall.outward[1], 0)
	centre := wall.point(at, springing, 0.06)
	const steps = 12
	for step := range steps {
		first, second := math.Pi*float64(step)/steps, math.Pi*float64(step+1)/steps
		piece.AddTriangle(centre, wall.point(at+half*math.Cos(first), springing+half*math.Sin(first), 0.06),
			wall.point(at+half*math.Cos(second), springing+half*math.Sin(second), 0.06), outward, color)
	}
}

// baskets hang a basket of flowers at each side of the door.
func (decorator *Decorator) baskets(wall wallFrame, settings style.Style, properties map[string]any, pieceFor PieceFor, random *rand.Rand, doorAt, doorHalf float64) error {
	wicker, err := settings.Color("color", properties, [4]float32{0.45, 0.32, 0.18, 1})
	if err != nil {
		return err
	}
	palette, _ := settings["flowers"].([]any)
	for _, side := range []float64{-1, 1} {
		at := doorAt + side*(doorHalf+0.75)
		if at < 0.5 || at > wall.length-0.5 {
			continue
		}
		wall.post(pieceFor(MaterialOf(detailMaterial, settings)), at, 0.3, 0.26, 1.45, 1.72, 10, wicker)
		wall.box(pieceFor(detailMaterial), at-0.02, at+0.02, 1.72, 2.15, 0.02, 0.06, wicker) // the hook
		for flower := range 9 {
			color := [4]float32{0.85, 0.3, 0.4, 1}
			if len(palette) > 0 {
				if color, err = style.LinearColor(asText(palette[random.IntN(len(palette))])); err != nil {
					return err
				}
			}
			angle, reach := 2*math.Pi*float64(flower)/9, 0.12+0.08*random.Float64()
			wall.post(pieceFor(flowerMaterial), at+reach*math.Cos(angle), 0.3+reach*math.Sin(angle), 0.05+0.02*random.Float64(),
				1.72, 1.8+0.12*random.Float64(), 6, color)
		}
	}
	return nil
}

// ivy climbs the wall from the ground: a few stems that wander up, covered in small leaves.
func (decorator *Decorator) ivy(wall wallFrame, settings style.Style, properties map[string]any, pieceFor PieceFor, random *rand.Rand, wallHeight float64) error {
	low, high := 2.0, 6.0
	if values, isList := settings["height"].([]any); isList && len(values) == 2 {
		low, _ = toFloat(values[0])
		high, _ = toFloat(values[1])
	}
	palette, _ := settings["color"].([]any)
	leafColor := func() ([4]float32, error) {
		if len(palette) == 0 {
			return [4]float32{0.2, 0.36, 0.15, 1}, nil
		}
		return style.LinearColor(asText(palette[random.IntN(len(palette))]))
	}
	piece := pieceFor(MaterialOf(ivyMaterial, settings))
	start := 0.3 + random.Float64()*math.Max(0.1, wall.length-0.6)
	for range 2 + random.IntN(2) {
		along, up := start+(random.Float64()-0.5)*1.2, 0.0
		top := math.Min(wallHeight-0.2, low+(high-low)*random.Float64())
		for up < top {
			along = math.Max(0.15, math.Min(wall.length-0.15, along+(random.Float64()-0.5)*0.35))
			color, err := leafColor()
			if err != nil {
				return err
			}
			size := 0.12 + 0.1*random.Float64()
			wall.box(piece, along-size, along+size, up, up+size*1.4, 0.02, 0.05+0.04*random.Float64(), color)
			up += size * 1.6
		}
	}
	return nil
}

// facadeHideBelow: facade details are not drawn when their square of the map looks smaller than
// this share of the screen (far away, they are a few pixels and cost much).
const facadeHideBelow = 0.12

func asText(value any) string {
	text, _ := value.(string)
	return text
}

func toFloat(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	}
	return 0, false
}

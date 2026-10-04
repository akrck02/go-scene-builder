package build

import (
	"math"
	"sort"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/recipes"
	"github.com/akrck02/go-scene-builder/style"
)

// line raises each line of a feature as a wall of "thickness" and "height" metres. By default it
// follows the ground in pieces of at most "segment" metres, each sunk "sink" metres below the
// lowest relief under it; with "follow: level" it is one level prism whose top is "height" metres
// over its highest end (a bridge deck from bank to bank); over a river dug into the relief its
// underside stays "clearance" metres over the water, it reaches out to the banks and stands on
// abutments where it does not touch the ground. Where lines of the features selected by
// "openings.where" cross it, a gap of "openings.width" metres is left, and the recipe gate_towers
// (in "decorate") raises a tower at each side of the gap, and closes it over an arch of
// "arch" metres if given (a gate in a tall wall). "battlements" put merlons along the top:
//
//	shape: {role: line, height: 30, thickness: 8, battlements: {height: 2, width: 2, gap: 1.5}}
//	openings: {where: {kind: street, class: road}, width: 6}
//	decorate: [{recipe: gate_towers, size: 5, height: 12, arch: 16, except_near: main_gate}]
//
// except_near names a kind of point feature: gates near one get neither towers nor arch (a gate
// built as a model of its own stands there).
func (builder *featureBuilder) line(feature *mapdata.Feature, resolved, shape style.Style) error {
	values, err := numbers(shape, feature.Properties, map[string]float64{"height": 3, "thickness": 1, "segment": 10, "sink": 1.5, "clearance": 1.5})
	if err != nil {
		return err
	}
	color, err := resolved.Color("color", feature.Properties, [4]float32{0.7, 0.68, 0.64, 1})
	if err != nil {
		return err
	}
	material := recipes.MaterialOf(solidMaterial, resolved)
	width, err := resolved.Section("openings").Number("width", feature.Properties, 6)
	if err != nil {
		return err
	}
	battlements, err := numbers(shape.Section("battlements"), feature.Properties, map[string]float64{"height": 0, "width": 2, "gap": 1.5})
	if err != nil {
		return err
	}
	for _, line := range feature.Lines {
		gaps := openingsAlong(line, builder.openings[feature], width/2000)
		if shape.Text("follow", "ground") == "level" {
			builder.levelLine(line, values, color, material)
		} else {
			for _, part := range withoutGaps(line, gaps) {
				builder.groundLine(part, values, battlements, color, material)
			}
		}
		if err := builder.gateTowers(feature, resolved, line, gaps, values, color, material); err != nil {
			return err
		}
	}
	return nil
}

func numbers(section style.Style, properties map[string]any, defaults map[string]float64) (map[string]float64, error) {
	values := map[string]float64{}
	for key, fallback := range defaults {
		value, err := section.Number(key, properties, fallback)
		if err != nil {
			return nil, err
		}
		values[key] = value
	}
	return values, nil
}

func (builder *featureBuilder) groundLine(line []mapdata.Point, values, battlements map[string]float64, color [4]float32, material mesh.Material) {
	points := densified(line, values["segment"]/1000)
	for index := 1; index < len(points); index++ {
		outline := sideways(points[index-1], points[index], values["thickness"]/2000)
		if outline == nil || !builder.coversAll(outline) {
			continue
		}
		base := builder.lowestOf(mapdata.Ring{points[index-1], points[index]})
		builder.prism(mapdata.Polygon{Exterior: outline}, base-values["sink"], base+values["height"], color, color, material)
		if battlements["height"] > 0 {
			builder.merlons(points[index-1], points[index], base+values["height"], values["thickness"], battlements, color, material)
		}
	}
}

// merlons raise the solid teeth of battlements along a piece of wall top, "gap" metres apart.
func (builder *featureBuilder) merlons(from, to mapdata.Point, top, thickness float64, battlements map[string]float64, color [4]float32, material mesh.Material) {
	length := mapdata.Distance(from, to)
	width, pitch := battlements["width"]/1000, (battlements["width"]+battlements["gap"])/1000
	for start := pitch / 2; start+width <= length; start += pitch {
		outline := sideways(pointAlong(from, to, start/length), pointAlong(from, to, (start+width)/length), thickness/2000)
		if outline != nil {
			builder.prism(mapdata.Polygon{Exterior: outline}, top, top+battlements["height"], color, color, material)
		}
	}
}

func (builder *featureBuilder) levelLine(line []mapdata.Point, values map[string]float64, color [4]float32, material mesh.Material) {
	if !builder.coversAll(mapdata.Ring(line)) {
		return
	}
	top := math.Max(builder.height(line[0][0], line[0][1]), builder.height(line[len(line)-1][0], line[len(line)-1][1])) + values["height"]
	bottom := top - values["height"] - values["sink"]
	if water, crosses := builder.waterAlong(line); crosses {
		top = math.Max(top, water+values["clearance"]+values["height"])
		bottom = math.Max(top-values["height"]-values["sink"], water+values["clearance"])
		line = builder.reachTheBanks(line, water+0.2)
		for _, end := range []mapdata.Point{line[0], line[len(line)-1]} {
			if ground := builder.height(end[0], end[1]); ground < bottom {
				builder.prism(mapdata.Polygon{Exterior: squareAround(end, values["thickness"]/2000)}, ground-values["sink"], top, color, color, material)
			}
		}
	}
	for index := 1; index < len(line); index++ {
		if outline := sideways(line[index-1], line[index], values["thickness"]/2000); outline != nil {
			builder.prism(mapdata.Polygon{Exterior: outline}, bottom, top, color, color, material)
		}
	}
}

// waterAlong is the highest water of the dug rivers under a line, and whether it crosses any.
func (builder *featureBuilder) waterAlong(line []mapdata.Point) (float64, bool) {
	if builder.channels == nil {
		return 0, false
	}
	highest, crosses := math.Inf(-1), false
	for _, point := range densified(line, builder.sampleSpacing()) {
		if water, under := builder.channels.waterUnder(point); under {
			highest, crosses = math.Max(highest, water), true
		}
	}
	return highest, crosses
}

// reachTheBanks lengthens a deck at both ends, along its direction, until it is over dry ground
// (higher than "dry", in metres), at most by twice the widest reach of the channels.
func (builder *featureBuilder) reachTheBanks(line []mapdata.Point, dry float64) []mapdata.Point {
	step, farthest := builder.sampleSpacing(), 2*builder.channels.cell
	extend := func(end, inner mapdata.Point) mapdata.Point {
		length := mapdata.Distance(end, inner)
		if length == 0 {
			return end
		}
		east, north := (end[0]-inner[0])/length, (end[1]-inner[1])/length
		for travelled := 0.0; travelled < farthest; travelled += step {
			point := mapdata.Point{end[0] + east*travelled, end[1] + north*travelled}
			if builder.height(point[0], point[1]) >= dry {
				return point
			}
		}
		return mapdata.Point{end[0] + east*farthest, end[1] + north*farthest}
	}
	extended := append([]mapdata.Point{}, line...)
	extended[0] = extend(line[0], line[1])
	extended[len(line)-1] = extend(line[len(line)-1], line[len(line)-2])
	return extended
}

// sampleSpacing is how finely lines are walked to meet the water: a quarter of a relief cell.
func (builder *featureBuilder) sampleSpacing() float64 {
	if builder.input.Relief == nil {
		return liveSampleSpacing / 2
	}
	return builder.input.Relief.Cell / 4
}

// squareAround is a square of a half side (km) around a point.
func squareAround(centre mapdata.Point, half float64) mapdata.Ring {
	return mapdata.Ring{{centre[0] - half, centre[1] - half}, {centre[0] + half, centre[1] - half},
		{centre[0] + half, centre[1] + half}, {centre[0] - half, centre[1] + half}}
}

// gap is a stretch of a line, from and to as distances along it (km).
type gap struct{ from, to float64 }

// openingsAlong turns the openings that lie on a line into stretches of half a width to each side.
func openingsAlong(line []mapdata.Point, openings []mapdata.Point, halfWidth float64) []gap {
	var gaps []gap
	for _, opening := range openings {
		travelled := 0.0
		for index := 1; index < len(line); index++ {
			from, to := line[index-1], line[index]
			nearest := mapdata.NearestOnSegment(from, to, opening)
			if mapdata.Distance(nearest, opening) < 1e-6 {
				along := travelled + mapdata.Distance(from, nearest)
				gaps = append(gaps, gap{along - halfWidth, along + halfWidth})
				break
			}
			travelled += mapdata.Distance(from, to)
		}
	}
	return gaps
}

// withoutGaps cuts the gaps out of a line and returns the parts left.
func withoutGaps(line []mapdata.Point, gaps []gap) [][]mapdata.Point {
	if len(gaps) == 0 {
		return [][]mapdata.Point{line}
	}
	inGap := func(along float64) bool {
		for _, current := range gaps {
			if along > current.from && along < current.to {
				return true
			}
		}
		return false
	}
	var parts [][]mapdata.Point
	var current []mapdata.Point
	travelled := 0.0
	for index := 1; index < len(line); index++ {
		from, to := line[index-1], line[index]
		length := mapdata.Distance(from, to)
		cuts := []float64{0, length}
		for _, stretch := range gaps {
			for _, edge := range []float64{stretch.from, stretch.to} {
				if edge > travelled && edge < travelled+length {
					cuts = append(cuts, edge-travelled)
				}
			}
		}
		sort.Float64s(cuts)
		for cut := 1; cut < len(cuts); cut++ {
			start, end := pointAlong(from, to, cuts[cut-1]/length), pointAlong(from, to, cuts[cut]/length)
			if inGap(travelled + (cuts[cut-1]+cuts[cut])/2) {
				if len(current) >= 2 {
					parts = append(parts, current)
				}
				current = nil
				continue
			}
			if len(current) == 0 {
				current = append(current, start)
			}
			current = append(current, end)
		}
		travelled += length
	}
	if len(current) >= 2 {
		parts = append(parts, current)
	}
	return parts
}

func pointAlong(from, to mapdata.Point, share float64) mapdata.Point {
	return mapdata.Point{from[0] + share*(to[0]-from[0]), from[1] + share*(to[1]-from[1])}
}

// gateTowers raises, for the gate_towers recipe, a square tower on the line at each side of every gap.
func (builder *featureBuilder) gateTowers(feature *mapdata.Feature, resolved style.Style, line []mapdata.Point, gaps []gap, wall map[string]float64, color [4]float32, material mesh.Material) error {
	decorations, _ := resolved["decorate"].([]any)
	for _, entry := range decorations {
		settings := style.Style(asMap(entry))
		if settings.Text("recipe", "") != "gate_towers" {
			continue
		}
		values, err := numbers(settings, feature.Properties, map[string]float64{"size": 5, "height": 12, "sink": 1.5, "arch": 0})
		if err != nil {
			return err
		}
		towerColor, err := settings.Color("color", feature.Properties, color)
		if err != nil {
			return err
		}
		half := values["size"] / 2000
		landmarks := builder.pointsOfKind(settings.Text("except_near", ""))
		for _, stretch := range gaps {
			if centre, _ := pointAndDirection(line, (stretch.from+stretch.to)/2); nearAny(centre, landmarks, 4*half+(stretch.to-stretch.from)) {
				continue // a gate built as a model of its own stands here
			}
			for _, along := range []float64{stretch.from - half, stretch.to + half} {
				centre, direction := pointAndDirection(line, along)
				builder.tower(centre, direction, half, values, towerColor, material)
			}
			if values["arch"] > 0 {
				builder.archOver(line, stretch, wall, values["arch"], color, material)
			}
		}
	}
	return nil
}

func (builder *featureBuilder) tower(centre, direction mapdata.Point, half float64, values map[string]float64, color [4]float32, material mesh.Material) {
	outline := sideways(mapdata.Point{centre[0] - direction[0]*half, centre[1] - direction[1]*half},
		mapdata.Point{centre[0] + direction[0]*half, centre[1] + direction[1]*half}, half)
	if outline == nil || !builder.coversAll(outline) {
		return
	}
	base := builder.lowestOf(outline)
	builder.prism(mapdata.Polygon{Exterior: outline}, base-values["sink"], base+values["height"], color, color, material)
}

// pointsOfKind are the points of the map's features of a kind ("" = none).
func (builder *featureBuilder) pointsOfKind(kind string) []mapdata.Point {
	if kind == "" {
		return nil
	}
	var points []mapdata.Point
	for _, feature := range builder.input.Map.Features {
		if feature.Kind == kind {
			points = append(points, feature.Points...)
		}
	}
	return points
}

func nearAny(point mapdata.Point, others []mapdata.Point, distance float64) bool {
	for _, other := range others {
		if mapdata.Distance(point, other) <= distance {
			return true
		}
	}
	return false
}

// archOver closes a gap in a wall above "arch" metres from the ground, up to the wall's top.
func (builder *featureBuilder) archOver(line []mapdata.Point, stretch gap, wall map[string]float64, arch float64, color [4]float32, material mesh.Material) {
	start, _ := pointAndDirection(line, stretch.from)
	end, _ := pointAndDirection(line, stretch.to)
	outline := sideways(start, end, wall["thickness"]/2000)
	if outline == nil || !builder.coversAll(outline) || arch >= wall["height"] {
		return
	}
	ground := builder.lowestOf(mapdata.Ring{start, end})
	builder.prism(mapdata.Polygon{Exterior: outline}, ground+arch, ground+wall["height"], color, color, material)
}

// pointAndDirection is the point at a distance along a line and the line's direction there.
func pointAndDirection(line []mapdata.Point, along float64) (mapdata.Point, mapdata.Point) {
	travelled := 0.0
	for index := 1; index < len(line); index++ {
		from, to := line[index-1], line[index]
		length := mapdata.Distance(from, to)
		if along <= travelled+length || index == len(line)-1 {
			share := math.Max(0, math.Min(1, (along-travelled)/math.Max(length, 1e-12)))
			return pointAlong(from, to, share), mapdata.Direction(from, to)
		}
		travelled += length
	}
	return line[0], mapdata.Point{1, 0}
}

func asMap(value any) map[string]any {
	result, _ := value.(map[string]any)
	return result
}

// point turns each point of a feature into a block: a square or round ("form") footprint of
// "size" metres across, "height" metres high, with the walls and roof of its style.
func (builder *featureBuilder) point(feature *mapdata.Feature, resolved, shape style.Style) error {
	values, err := numbers(shape, feature.Properties, map[string]float64{"size": 5, "height": 10, "sink": 1})
	if err != nil {
		return err
	}
	half := values["size"] / 2000
	for _, centre := range feature.Points {
		outline := mapdata.Ring{}
		sides := 4
		if shape.Text("form", "square") == "round" {
			sides = 16
		}
		for corner := range sides {
			angle := 2*math.Pi*float64(corner)/float64(sides) + math.Pi/float64(sides)
			radius := half
			if sides == 4 {
				radius = half * math.Sqrt2
			}
			outline = append(outline, mapdata.Point{centre[0] + radius*math.Cos(angle), centre[1] + radius*math.Sin(angle)})
		}
		if !builder.coversAll(outline) {
			continue
		}
		base := builder.lowestOf(outline)
		builder.output.blocks = append(builder.output.blocks, &block.Block{Feature: feature, Style: resolved,
			Polygon: mapdata.Polygon{Exterior: outline}, Ground: base, Bottom: base - values["sink"], Eaves: base + values["height"]})
	}
	return nil
}

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
// over its highest end (a bridge deck from bank to bank). Where lines of the features selected by
// "openings.where" cross it, a gap of "openings.width" metres is left, and the recipe gate_towers
// (in "decorate") raises a tower at each side of the gap:
//
//	openings: {where: {kind: street, class: road}, width: 6}
//	decorate: [{recipe: gate_towers, size: 5, height: 12}]
func (builder *featureBuilder) line(feature *mapdata.Feature, resolved, shape style.Style) error {
	values, err := numbers(shape, feature.Properties, map[string]float64{"height": 3, "thickness": 1, "segment": 10, "sink": 1.5})
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
	for _, line := range feature.Lines {
		gaps := openingsAlong(line, builder.openings[feature], width/2000)
		if shape.Text("follow", "ground") == "level" {
			builder.levelLine(line, values, color, material)
		} else {
			for _, part := range withoutGaps(line, gaps) {
				builder.groundLine(part, values, color, material)
			}
		}
		if err := builder.gateTowers(feature, resolved, line, gaps, color, material); err != nil {
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

func (builder *featureBuilder) groundLine(line []mapdata.Point, values map[string]float64, color [4]float32, material mesh.Material) {
	points := densified(line, values["segment"]/1000)
	for index := 1; index < len(points); index++ {
		outline := sideways(points[index-1], points[index], values["thickness"]/2000)
		if outline == nil || !builder.coversAll(outline) {
			continue
		}
		base := builder.lowestOf(mapdata.Ring{points[index-1], points[index]})
		builder.prism(mapdata.Polygon{Exterior: outline}, base-values["sink"], base+values["height"], color, color, material)
	}
}

func (builder *featureBuilder) levelLine(line []mapdata.Point, values map[string]float64, color [4]float32, material mesh.Material) {
	if !builder.coversAll(mapdata.Ring(line)) {
		return
	}
	top := math.Max(builder.height(line[0][0], line[0][1]), builder.height(line[len(line)-1][0], line[len(line)-1][1])) + values["height"]
	for index := 1; index < len(line); index++ {
		if outline := sideways(line[index-1], line[index], values["thickness"]/2000); outline != nil {
			builder.prism(mapdata.Polygon{Exterior: outline}, top-values["height"]-values["sink"], top, color, color, material)
		}
	}
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
func (builder *featureBuilder) gateTowers(feature *mapdata.Feature, resolved style.Style, line []mapdata.Point, gaps []gap, color [4]float32, material mesh.Material) error {
	decorations, _ := resolved["decorate"].([]any)
	for _, entry := range decorations {
		settings := style.Style(asMap(entry))
		if settings.Text("recipe", "") != "gate_towers" {
			continue
		}
		values, err := numbers(settings, feature.Properties, map[string]float64{"size": 5, "height": 12, "sink": 1.5})
		if err != nil {
			return err
		}
		towerColor, err := settings.Color("color", feature.Properties, color)
		if err != nil {
			return err
		}
		half := values["size"] / 2000
		for _, stretch := range gaps {
			for _, along := range []float64{stretch.from - half, stretch.to + half} {
				centre, direction := pointAndDirection(line, along)
				builder.tower(centre, direction, half, values, towerColor, material)
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

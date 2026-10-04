package build

import (
	"math"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/models"
	"github.com/akrck02/go-scene-builder/style"
)

// Decorations follow rules instead of falling anywhere. Every copy of a model or plant keeps off:
//   - houses and other blocks ("clear" metres around them, 1.5 by default);
//   - every line that has a width (streets, walls, rivers, bridges), "keep_off_lines" metres beyond
//     its edge (0.8 by default; -1 allows them on lines);
//   - models placed on their own (a fountain, a gate), by their size;
//   - the copies already placed in the same feature, each taking the circle it covers plus
//     "spacing" metres (0.3 by default).
//
// "edge: 4" keeps the copies within 4 m of the edge of their feature (stalls round a square).

// keepOut holds what decorations stay away from, for the whole map.
type keepOut struct {
	lines  *axisIndexWithWidths // with the true half widths
	models *discIndex           // models placed on their own
}

func newKeepOut(input Input) *keepOut {
	keep := &keepOut{lines: &axisIndexWithWidths{cell: 0.05, cells: map[[2]int][]widthSegment{}}, models: newDiscIndex()}
	for _, feature := range input.Map.Features {
		resolved, matched := input.Rules.Resolve(feature.Kind, feature.Properties)
		if !matched {
			continue
		}
		shape := resolved.Section("shape")
		if shape.Text("role", "") == "model" {
			keep.addModel(feature, shape, input.Models)
			continue
		}
		if len(feature.Lines) == 0 {
			continue
		}
		width, _ := shape.Number("width", feature.Properties, 0)
		if thickness, _ := shape.Number("thickness", feature.Properties, 0); thickness > width {
			width = thickness
		}
		if width <= 0 {
			continue
		}
		for _, line := range feature.Lines {
			for point := 1; point < len(line); point++ {
				keep.lines.add(widthSegment{line[point-1], line[point], width / 2000})
			}
		}
	}
	return keep
}

func (keep *keepOut) addModel(feature *mapdata.Feature, shape style.Style, library *models.Library) {
	model := library.Model(shape.Text("model", ""))
	if model == nil {
		return
	}
	scale, _ := shape.Number("scale", feature.Properties, 1)
	for _, point := range feature.Points {
		keep.models.add(point, radiusOf(model)*scale)
	}
}

// onLine tells whether a point lies on a line with a width, or within margin (km) of its edge.
func (keep *keepOut) onLine(point mapdata.Point, margin float64) bool {
	reach := margin + 0.05 // the widest half width the cells were filled with
	column, row := int(math.Floor(point[0]/keep.lines.cell)), int(math.Floor(point[1]/keep.lines.cell))
	for nearbyColumn := column - 1; nearbyColumn <= column+1; nearbyColumn++ {
		for nearbyRow := row - 1; nearbyRow <= row+1; nearbyRow++ {
			for _, segment := range keep.lines.cells[[2]int{nearbyColumn, nearbyRow}] {
				distance := mapdata.Distance(point, mapdata.NearestOnSegment(segment.from, segment.to, point))
				if distance < segment.half+margin && distance < reach+segment.half {
					return true
				}
			}
		}
	}
	return false
}

// radiusOf is how far (km) a model reaches from its origin across the ground.
func radiusOf(model *models.Model) float64 {
	radius := 0.0
	for _, piece := range model.Pieces {
		radius = math.Max(radius, horizontalReach(piece))
	}
	return radius
}

func horizontalReach(piece *mesh.Piece) float64 {
	reach := 0.0
	for index := 0; index+2 < len(piece.Positions); index += 3 {
		reach = math.Max(reach, math.Hypot(float64(piece.Positions[index]), float64(piece.Positions[index+2])))
	}
	return reach
}

// discIndex finds circles (centre and radius, km) that overlap a new one.
type discIndex struct {
	cell    float64
	cells   map[[2]int][]disc
	largest float64
}

type disc struct {
	centre mapdata.Point
	radius float64
}

func newDiscIndex() *discIndex {
	return &discIndex{cell: 0.01, cells: map[[2]int][]disc{}}
}

func (index *discIndex) add(centre mapdata.Point, radius float64) {
	key := [2]int{int(math.Floor(centre[0] / index.cell)), int(math.Floor(centre[1] / index.cell))}
	index.cells[key] = append(index.cells[key], disc{centre, radius})
	index.largest = math.Max(index.largest, radius)
}

// overlaps tells whether a circle overlaps any circle of the index.
func (index *discIndex) overlaps(centre mapdata.Point, radius float64) bool {
	rings := int(math.Ceil((radius + index.largest) / index.cell))
	column, row := int(math.Floor(centre[0]/index.cell)), int(math.Floor(centre[1]/index.cell))
	for nearbyColumn := column - rings; nearbyColumn <= column+rings; nearbyColumn++ {
		for nearbyRow := row - rings; nearbyRow <= row+rings; nearbyRow++ {
			for _, other := range index.cells[[2]int{nearbyColumn, nearbyRow}] {
				if mapdata.Distance(centre, other.centre) < radius+other.radius {
					return true
				}
			}
		}
	}
	return false
}

// placementRules are one decoration's rules, in km.
type placementRules struct {
	clear, lineMargin, spacing, edge float64
}

func rulesOf(settings style.Style, properties map[string]any) placementRules {
	values, _ := numbers(settings, properties, map[string]float64{"clear": 1.5, "keep_off_lines": 0.8, "spacing": 0.3, "edge": 0})
	return placementRules{clear: values["clear"] / 1000, lineMargin: values["keep_off_lines"] / 1000,
		spacing: values["spacing"] / 1000, edge: values["edge"] / 1000}
}

// allowed tells whether a copy of a radius (km) may stand at a point of a feature's polygon, and
// reserves its place if so.
func (gardener *gardener) allowed(point mapdata.Point, radius float64, rules placementRules, polygon mapdata.Polygon, taken *discIndex) bool {
	if taken == nil {
		return true // the forests of a biome layer keep their own rules
	}
	if gardener.occupied.near(point, rules.clear+radius) {
		return false
	}
	if rules.lineMargin >= 0 && gardener.keep.onLine(point, rules.lineMargin+radius) {
		return false
	}
	if gardener.keep.models.overlaps(point, radius+rules.spacing) {
		return false
	}
	if rules.edge > 0 && distanceToRing(polygon.Exterior, point) > rules.edge {
		return false
	}
	if !ringContains(polygon.Exterior, point) || taken.overlaps(point, radius+rules.spacing/2) {
		return false
	}
	taken.add(point, radius+rules.spacing/2)
	return true
}

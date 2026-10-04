package relations

import (
	"math"

	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/style"
)

// Openings are, for every line feature that has them, the points where other lines cut it (a
// road crossing a city wall: a gate).
type Openings map[*mapdata.Feature][]mapdata.Point

// FindOpenings looks, for every feature whose selector (from cutters; nil = none) is set, for the
// points where lines of the features that match the selector cross its lines.
func FindOpenings(features []*mapdata.Feature, cutters func(*mapdata.Feature) map[string]any) Openings {
	index := newSegmentIndex(0.05)
	for number, feature := range features {
		for _, line := range feature.Lines {
			for point := 1; point < len(line); point++ {
				index.add(segment{from: line[point-1], to: line[point], owner: number})
			}
		}
	}
	openings := Openings{}
	for number, feature := range features {
		when := cutters(feature)
		if when == nil {
			continue
		}
		for _, line := range feature.Lines {
			for point := 1; point < len(line); point++ {
				from, to := line[point-1], line[point]
				reach := mapdata.Distance(from, to)/2 + 0.001
				index.near(mapdata.Middle(from, to), reach, func(other segment) {
					cutter := features[other.owner]
					if other.owner == number || !style.Selects(when, cutter.Kind, cutter.Properties) {
						return
					}
					if crossing, crosses := intersection(from, to, other.from, other.to); crosses && !contains(openings[feature], crossing) {
						openings[feature] = append(openings[feature], crossing)
					}
				})
			}
		}
	}
	return openings
}

// intersection is where two segments cross, if they do.
func intersection(first, second, third, fourth mapdata.Point) (mapdata.Point, bool) {
	across := (second[0]-first[0])*(fourth[1]-third[1]) - (second[1]-first[1])*(fourth[0]-third[0])
	if math.Abs(across) < 1e-15 {
		return mapdata.Point{}, false
	}
	share := ((third[0]-first[0])*(fourth[1]-third[1]) - (third[1]-first[1])*(fourth[0]-third[0])) / across
	otherShare := ((third[0]-first[0])*(second[1]-first[1]) - (third[1]-first[1])*(second[0]-first[0])) / across
	if share < 0 || share > 1 || otherShare < 0 || otherShare > 1 {
		return mapdata.Point{}, false
	}
	return mapdata.Point{first[0] + share*(second[0]-first[0]), first[1] + share*(second[1]-first[1])}, true
}

func contains(points []mapdata.Point, point mapdata.Point) bool {
	for _, existing := range points {
		if mapdata.Distance(existing, point) < 1e-6 {
			return true
		}
	}
	return false
}

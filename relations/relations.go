// Package relations works out what a block alone does not know (stage 2 of the scene builder):
// which of its sides looks onto a street ("facing") and which of its walls a neighbour leans
// against ("rows"). The style file sets what counts as a street and the tolerances:
//
//	relations:
//	  facing: {when: {kind: street}, reach: 20}   # metres from a side to the street's axis
//	  rows: {tolerance: 0.4}                       # metres between two walls that touch
package relations

import (
	"math"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/style"
)

// Compute fills the Facing and Shared fields of every block.
func Compute(blocks []*block.Block, features []*mapdata.Feature, settings map[string]any) {
	facingSettings := style.Style(settings).Section("facing")
	if when, _ := facingSettings["when"].(map[string]any); when != nil {
		reach, _ := facingSettings.Number("reach", nil, 20)
		findFacing(blocks, streetIndex(features, when), reach/1000)
	}
	tolerance, _ := style.Style(settings).Section("rows").Number("tolerance", nil, 0.4)
	findShared(blocks, tolerance/1000)
}

func streetIndex(features []*mapdata.Feature, when map[string]any) *segmentIndex {
	index := newSegmentIndex(0.05)
	for number, feature := range features {
		if !style.Selects(when, feature.Kind, feature.Properties) {
			continue
		}
		for _, line := range feature.Lines {
			for point := 1; point < len(line); point++ {
				index.add(segment{from: line[point-1], to: line[point], owner: number})
			}
		}
	}
	return index
}

// findFacing gives every block the side whose middle is nearest to a street, if any is within reach.
func findFacing(blocks []*block.Block, streets *segmentIndex, reach float64) {
	for _, current := range blocks {
		best, bestDistance := -1, reach
		for side := range current.Polygon.Exterior {
			from, to := current.Side(side)
			middle := mapdata.Middle(from, to)
			streets.near(middle, reach, func(street segment) {
				if gap := mapdata.Distance(middle, mapdata.NearestOnSegment(street.from, street.to, middle)); gap < bestDistance {
					best, bestDistance = side, gap
				}
			})
		}
		if best >= 0 {
			from, to := current.Side(best)
			current.Facing = &block.Facing{Side: best, Outward: mapdata.Outward(from, to)}
		}
	}
}

// findShared marks the sides of a block that lie against a side of another block: opposite
// directions, both ends within tolerance of the other side's line and within its length.
func findShared(blocks []*block.Block, tolerance float64) {
	sides := newSegmentIndex(0.02)
	for number, current := range blocks {
		current.Shared = make([]bool, len(current.Polygon.Exterior))
		for side := range current.Polygon.Exterior {
			from, to := current.Side(side)
			sides.add(segment{from: from, to: to, owner: number, side: side})
		}
	}
	for number, current := range blocks {
		for side := range current.Polygon.Exterior {
			from, to := current.Side(side)
			sides.near(mapdata.Middle(from, to), tolerance, func(other segment) {
				if other.owner != number && against(from, to, other.from, other.to, tolerance) {
					current.Shared[side] = true
				}
			})
		}
	}
}

// against tells whether a side lies along another one, facing it, and within its length.
func against(from, to, otherFrom, otherTo mapdata.Point, tolerance float64) bool {
	direction, otherDirection := mapdata.Direction(from, to), mapdata.Direction(otherFrom, otherTo)
	if direction[0]*otherDirection[0]+direction[1]*otherDirection[1] > -0.98 {
		return false
	}
	for _, end := range []mapdata.Point{from, to} {
		if mapdata.Distance(end, mapdata.NearestOnSegment(otherFrom, otherTo, end)) > tolerance {
			return false
		}
	}
	return math.Abs(mapdata.Distance(from, to)) > tolerance
}

package recipes

import (
	"math"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
)

// walls adds a vertical quad from the bottom to the eaves for every side of the block; where a
// neighbour leans against a side, only the part above the neighbour's eaves (if any) is drawn. The
// walls of courtyards face into them.
func (decorator *Decorator) walls(current *block.Block, piece *mesh.Piece, color [4]float32) {
	exterior := current.Polygon.Exterior
	for side := range exterior {
		bottom := math.Max(current.Bottom, current.SharedUpTo(side))
		if bottom >= current.Eaves {
			continue
		}
		from, to := current.Side(side)
		decorator.wall(piece, from, to, bottom, current.Eaves, mapdata.Outward(from, to), color)
	}
	for _, hole := range current.Polygon.Holes {
		ring := hole.Anticlockwise()
		for index := range ring {
			from, to := ring[index], ring[(index+1)%len(ring)]
			outward := mapdata.Outward(from, to)
			decorator.wall(piece, from, to, current.Bottom, current.Eaves, mapdata.Point{-outward[0], -outward[1]}, color)
		}
	}
}

// onRectangle is the block standing on a rectangle instead of its footprint (no courtyards). A side
// of the rectangle is covered as high as the footprint sides along it (parallel, within 0.5 m).
func onRectangle(current *block.Block, rectangle mapdata.Ring) *block.Block {
	moved := *current
	moved.Polygon = mapdata.Polygon{Exterior: rectangle}
	moved.Covered = make([]float64, len(rectangle))
	if current.Facing != nil {
		moved.Facing = facingOnRectangle(rectangle, current.Facing.Outward)
	}
	for side := range rectangle {
		from, to := rectangle[side], rectangle[(side+1)%len(rectangle)]
		direction := mapdata.Direction(from, to)
		moved.Covered[side] = math.Inf(-1)
		for original := range current.Polygon.Exterior {
			start, end := current.Side(original)
			along := mapdata.Direction(start, end)
			middle := mapdata.Middle(start, end)
			if direction[0]*along[0]+direction[1]*along[1] > 0.98 && mapdata.Distance(middle, mapdata.NearestOnSegment(from, to, middle)) < 0.0005 {
				moved.Covered[side] = math.Max(moved.Covered[side], current.SharedUpTo(original))
			}
		}
	}
	return &moved
}

// facingOnRectangle is the side of the rectangle that faces the same way as the footprint's.
func facingOnRectangle(rectangle mapdata.Ring, outward mapdata.Point) *block.Facing {
	best, bestDot := 0, -2.0
	for side := range rectangle {
		normal := mapdata.Outward(rectangle[side], rectangle[(side+1)%len(rectangle)])
		if dot := normal[0]*outward[0] + normal[1]*outward[1]; dot > bestDot {
			best, bestDot = side, dot
		}
	}
	return &block.Facing{Side: best, Outward: mapdata.Outward(rectangle[best], rectangle[(best+1)%len(rectangle)])}
}

func (decorator *Decorator) wall(piece *mesh.Piece, from, to mapdata.Point, bottom, top float64, outward mapdata.Point, color [4]float32) {
	if from == to {
		return
	}
	frame := decorator.Frame
	piece.AddQuad(frame.Scene(from[0], from[1], bottom), frame.Scene(to[0], to[1], bottom), frame.Scene(to[0], to[1], top),
		frame.Scene(from[0], from[1], top), mesh.SceneDirection(outward[0], outward[1], 0), color)
}

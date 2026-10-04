package recipes

import (
	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
)

// walls adds a vertical quad from the bottom to the eaves for every side of the block, except the
// sides a neighbour leans against; the walls of courtyards face into them.
func (decorator *Decorator) walls(current *block.Block, piece *mesh.Piece, color [4]float32) {
	exterior := current.Polygon.Exterior
	for side := range exterior {
		if side < len(current.Shared) && current.Shared[side] {
			continue
		}
		from, to := current.Side(side)
		decorator.wall(piece, from, to, current.Bottom, current.Eaves, mapdata.Outward(from, to), color)
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

func (decorator *Decorator) wall(piece *mesh.Piece, from, to mapdata.Point, bottom, top float64, outward mapdata.Point, color [4]float32) {
	if from == to {
		return
	}
	frame := decorator.Frame
	piece.AddQuad(frame.Scene(from[0], from[1], bottom), frame.Scene(to[0], to[1], bottom), frame.Scene(to[0], to[1], top),
		frame.Scene(from[0], from[1], top), mesh.SceneDirection(outward[0], outward[1], 0), color)
}

// Package block holds the basic blocks the builder decorates: volumes with their footprint, the
// height of their base and eaves, their style and their relations with the rest of the map.
package block

import (
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/style"
)

// Block is one volume of the map (a polygon of a feature with the volume role).
type Block struct {
	Feature *mapdata.Feature
	Style   style.Style
	Polygon mapdata.Polygon // exterior anticlockwise
	Bottom  float64         // metres: where the walls start (a little below the ground)
	Eaves   float64         // metres: where the walls end and the roof starts
	Facing  *Facing         // the side towards a street; nil when no street is near
	Shared  []bool          // for every side of the exterior, whether a neighbour has a wall against it
}

// Facing is the side of a block that looks onto a street.
type Facing struct {
	Side    int           // index of the side: from Exterior[Side] to Exterior[Side+1]
	Outward mapdata.Point // unit direction from the block towards the street
}

// Side returns the two ends of a side of the exterior.
func (block *Block) Side(index int) (mapdata.Point, mapdata.Point) {
	exterior := block.Polygon.Exterior
	return exterior[index], exterior[(index+1)%len(exterior)]
}

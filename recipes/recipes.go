// Package recipes decorates blocks (stage 4 of the scene builder): every recipe takes a block, its
// style and its relations and adds geometry to a piece. Recipes so far: walls (without the walls a
// neighbour leans against) and roofs: flat, gable and hip.
//
//	walls: {color: "#e9dfc9"}
//	roof: {recipe: gable, pitch: 38, ridge: long, eaves: 0.5, color: "#a4553a"}
//
// ridge: long or short (the side of the footprint the ridge follows), along_street or
// across_street (relative to the side that faces a street; long when no street is near).
package recipes

import (
	"fmt"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/twpayne/go-geos"
)

// Decorator adds the geometry of blocks to pieces, with its own GEOS context.
type Decorator struct {
	Frame   mesh.Frame
	Context *geos.Context
}

// Decorate adds the walls and the roof of a block to a piece.
func (decorator *Decorator) Decorate(current *block.Block, piece *mesh.Piece) error {
	walls, err := current.Style.Section("walls").Color("color", [4]float32{0.8, 0.78, 0.72, 1})
	if err != nil {
		return err
	}
	decorator.walls(current, piece, walls)
	roof := current.Style.Section("roof")
	switch recipe := roof.Text("recipe", "flat"); recipe {
	case "flat":
		return decorator.flatRoof(current, piece, roof)
	case "gable", "hip":
		return decorator.slopedRoof(current, piece, roof, walls, recipe == "hip")
	default:
		return fmt.Errorf("unknown roof recipe %q", recipe)
	}
}

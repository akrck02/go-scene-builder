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

// PieceFor returns the piece where geometry of a material goes.
type PieceFor func(material mesh.Material) *mesh.Piece

// Decorate adds the walls and the roof of a block, each to the piece of its material.
func (decorator *Decorator) Decorate(current *block.Block, pieceFor PieceFor) error {
	wallSection := current.Style.Section("walls")
	walls, err := wallSection.Color("color", current.Feature.Properties, [4]float32{0.8, 0.78, 0.72, 1})
	if err != nil {
		return err
	}
	wallPiece := pieceFor(MaterialOf(WallMaterial, wallSection))
	roof := current.Style.Section("roof")
	roofPiece := pieceFor(MaterialOf(RoofMaterial, roof))
	switch recipe := roof.Text("recipe", "flat"); recipe {
	case "flat":
		decorator.walls(current, wallPiece, walls)
		if err := decorator.facade(current, pieceFor); err != nil {
			return err
		}
		return decorator.flatRoof(current, roofPiece, roof)
	case "gable", "hip":
		// a sloped roof covers the smallest rectangle around the footprint; the walls follow that
		// rectangle too, or gaps would show between them and the gable ends
		if rectangle := decorator.rectangle(current); rectangle != nil {
			current = onRectangle(current, rectangle)
		}
		decorator.walls(current, wallPiece, walls)
		if err := decorator.facade(current, pieceFor); err != nil {
			return err
		}
		return decorator.slopedRoof(current, roofPiece, wallPiece, roof, walls, recipe == "hip")
	default:
		return fmt.Errorf("unknown roof recipe %q", recipe)
	}
}

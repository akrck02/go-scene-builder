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
		if err := decorator.stonework(current, pieceFor); err != nil {
			return err
		}
		if err := decorator.facade(current, pieceFor); err != nil {
			return err
		}
		return decorator.flatRoof(current, roofPiece, roof)
	case "gable", "hip":
		// a gable or hip roof needs a rectangle: a footprint that nearly is one is squared (walls and
		// roof on the rectangle); any other keeps its walls on the footprint, so they meet the
		// neighbours' exactly, under a roof sloping from every side (or a flat one if not convex)
		rectangle := decorator.rectangle(current)
		if rectangle == nil || !nearlyRectangle(current.Polygon.Exterior, rectangle) {
			decorator.walls(current, wallPiece, walls)
			if err := decorator.stonework(current, pieceFor); err != nil {
				return err
			}
			if err := decorator.facade(current, pieceFor); err != nil {
				return err
			}
			if !convex(current.Polygon.Exterior) || len(current.Polygon.Holes) > 0 {
				return decorator.flatRoof(current, roofPiece, roof)
			}
			return decorator.tentRoof(current, roofPiece, roof)
		}
		current = onRectangle(current, rectangle)
		decorator.walls(current, wallPiece, walls)
		if err := decorator.stonework(current, pieceFor); err != nil {
			return err
		}
		if err := decorator.facade(current, pieceFor); err != nil {
			return err
		}
		return decorator.slopedRoof(current, pieceFor, roofPiece, wallPiece, roof, walls, recipe == "hip")
	default:
		return fmt.Errorf("unknown roof recipe %q", recipe)
	}
}

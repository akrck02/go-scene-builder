package build

import (
	"fmt"
	"math"

	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/recipes"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
)

// terrainChunk is how many cells a terrain piece covers on each side.
const terrainChunk = 64

var terrainMaterial = mesh.Material{Name: "terrain", Kind: "lit"}

// terrainPieces turns the relief into walkable pieces: a vertex on every cell centre, two
// triangles per square, smooth normals, and the colour of the style's "terrain" section:
//
//	terrain: {color: "#6b7f47", bands: [{above: 350, color: "#9e9e99", blend: 30}], material: {pattern: grass, scale: 2}}
//
// where each band takes over above a height, blending over "blend" metres.
func terrainPieces(grid *relief.Relief, settings map[string]any, frame mesh.Frame) []*mesh.Piece {
	colorOf := terrainColors(style.Style(settings))
	var pieces []*mesh.Piece
	for firstRow := 0; firstRow < grid.Rows-1; firstRow += terrainChunk {
		for firstColumn := 0; firstColumn < grid.Columns-1; firstColumn += terrainChunk {
			lastRow, lastColumn := min(firstRow+terrainChunk, grid.Rows-1), min(firstColumn+terrainChunk, grid.Columns-1)
			piece := &mesh.Piece{Name: fmt.Sprintf("terrain %d,%d", firstRow/terrainChunk, firstColumn/terrainChunk),
				Material: recipes.MaterialOf(terrainMaterial, style.Style(settings)), Ground: true}
			for row := firstRow; row <= lastRow; row++ {
				for column := firstColumn; column <= lastColumn; column++ {
					east, north := grid.CellCentre(row, column)
					height := grid.At(row, column)
					piece.AddVertex(frame.Scene(east, north, height), terrainNormal(grid, row, column), colorOf(height))
				}
			}
			addGridIndices(piece, lastRow-firstRow+1, lastColumn-firstColumn+1)
			pieces = append(pieces, piece)
		}
	}
	return pieces
}

// addGridIndices adds two triangles per square of a grid of vertices, wound to face up.
func addGridIndices(piece *mesh.Piece, rows, columns int) {
	for row := 0; row < rows-1; row++ {
		for column := 0; column < columns-1; column++ {
			corner := uint32(row*columns + column)
			below, right := corner+uint32(columns), corner+1
			piece.Indices = append(piece.Indices, corner, below, right, right, below, below+1)
		}
	}
}

// terrainNormal is the normal of the relief at a cell centre, in scene coordinates.
func terrainNormal(grid *relief.Relief, row, column int) mesh.Vector {
	west, east := grid.At(row, max(0, column-1)), grid.At(row, min(grid.Columns-1, column+1))
	north, south := grid.At(max(0, row-1), column), grid.At(min(grid.Rows-1, row+1), column)
	spacing := grid.Cell * 1000 * 2
	eastward, northward := (east-west)/spacing, (north-south)/spacing
	return mesh.SceneDirection(-eastward, -northward, 1).Normalized()
}

// terrainColors returns the colour of the ground at a height.
func terrainColors(settings style.Style) func(height float64) [4]float32 {
	base, err := settings.Color("color", nil, [4]float32{0.17, 0.23, 0.07, 1})
	if err != nil {
		base = [4]float32{0.17, 0.23, 0.07, 1}
	}
	type band struct {
		above, blend float64
		color        [4]float32
	}
	var bands []band
	list, _ := settings["bands"].([]any)
	for _, entry := range list {
		values, _ := entry.(map[string]any)
		section := style.Style(values)
		above, _ := section.Number("above", nil, 0)
		blend, _ := section.Number("blend", nil, 0)
		color, err := section.Color("color", nil, base)
		if err == nil {
			bands = append(bands, band{above, blend, color})
		}
	}
	return func(height float64) [4]float32 {
		color := base
		for _, current := range bands {
			share := 1.0
			if current.blend > 0 {
				share = math.Max(0, math.Min(1, (height-current.above+current.blend/2)/current.blend))
			} else if height < current.above {
				share = 0
			}
			for channel := range 3 {
				color[channel] += float32(share) * (current.color[channel] - color[channel])
			}
		}
		return color
	}
}

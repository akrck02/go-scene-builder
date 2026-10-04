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
func terrainPieces(grid, biomes *relief.Relief, settings map[string]any, frame mesh.Frame) []*mesh.Piece {
	colorOf := terrainColors(style.Style(settings), biomes)
	material := recipes.MaterialOf(terrainMaterial, style.Style(settings))
	var pieces []*mesh.Piece
	for firstRow := 0; firstRow < grid.Rows-1; firstRow += terrainChunk {
		for firstColumn := 0; firstColumn < grid.Columns-1; firstColumn += terrainChunk {
			chunk := terrainChunkArea{firstRow, min(firstRow+terrainChunk, grid.Rows-1), firstColumn, min(firstColumn+terrainChunk, grid.Columns-1)}
			piece := chunk.piece(grid, frame, colorOf, 1)
			piece.Name = fmt.Sprintf("terrain %d,%d", firstRow/terrainChunk, firstColumn/terrainChunk)
			piece.Material, piece.Ground = material, true
			for _, level := range terrainLevels {
				if version := chunk.piece(grid, frame, colorOf, level.step); version != nil {
					version.Material = material
					piece.Details = append(piece.Details, mesh.Detail{Piece: version, ScreenSize: level.screenSize})
				}
			}
			pieces = append(pieces, piece)
		}
	}
	return pieces
}

// terrainLevels are the simpler versions of the terrain: every 2nd, 4th and 8th vertex, drawn when
// a chunk looks smaller than a share of the screen.
var terrainLevels = []struct {
	step       int
	screenSize float64
}{{2, 0.45}, {4, 0.22}, {8, 0.11}}

// terrainChunkArea is the cells a terrain piece covers.
type terrainChunkArea struct {
	firstRow, lastRow, firstColumn, lastColumn int
}

// samples are the rows (or columns) of a chunk at a step, always with its last one.
func samples(first, last, step int) []int {
	var result []int
	for index := first; index < last; index += step {
		result = append(result, index)
	}
	return append(result, last)
}

// piece builds a chunk at a step (1 = every vertex), with skirts under its edges so neighbouring
// chunks drawn at other steps leave no cracks; nil when the step is too coarse for the chunk.
func (chunk terrainChunkArea) piece(grid *relief.Relief, frame mesh.Frame, colorOf func(height, east, north float64) [4]float32, step int) *mesh.Piece {
	rows, columns := samples(chunk.firstRow, chunk.lastRow, step), samples(chunk.firstColumn, chunk.lastColumn, step)
	if step > 1 && (len(rows) < 3 || len(columns) < 3) {
		return nil
	}
	piece := &mesh.Piece{}
	for _, row := range rows {
		for _, column := range columns {
			east, north := grid.CellCentre(row, column)
			height := grid.At(row, column)
			piece.AddVertex(frame.Scene(east, north, height), terrainNormal(grid, row, column), colorOf(height, east, north))
		}
	}
	addGridIndices(piece, len(rows), len(columns))
	addSkirts(piece, len(rows), len(columns), grid.Cell*1000*float64(step))
	return piece
}

// addSkirts hangs a strip of "depth" metres under every edge of a grid of vertices, seen from both
// sides.
func addSkirts(piece *mesh.Piece, rows, columns int, depth float64) {
	edges := [][]int{}
	top, bottom, left, right := []int{}, []int{}, []int{}, []int{}
	for column := range columns {
		top, bottom = append(top, column), append(bottom, (rows-1)*columns+column)
	}
	for row := range rows {
		left, right = append(left, row*columns), append(right, row*columns+columns-1)
	}
	edges = append(edges, top, bottom, left, right)
	for _, edge := range edges {
		lowered := make([]uint32, len(edge))
		for index, vertex := range edge {
			position := mesh.Vector{float64(piece.Positions[3*vertex]), float64(piece.Positions[3*vertex+1]) - depth/1000, float64(piece.Positions[3*vertex+2])}
			normal := mesh.Vector{float64(piece.Normals[3*vertex]), float64(piece.Normals[3*vertex+1]), float64(piece.Normals[3*vertex+2])}
			color := [4]float32(piece.Colors[4*vertex : 4*vertex+4])
			lowered[index] = piece.AddVertex(position, normal, color)
		}
		for index := 1; index < len(edge); index++ {
			first, second := uint32(edge[index-1]), uint32(edge[index])
			piece.Indices = append(piece.Indices, first, lowered[index-1], second, second, lowered[index-1], lowered[index])
			piece.Indices = append(piece.Indices, first, second, lowered[index-1], second, lowered[index], lowered[index-1])
		}
	}
}

// biomeColors returns the colour of the biomes around a place, averaged over 3 × 3 samples half a
// layer cell apart (so borders blend), or false where no biome with a colour is near.
func biomeColors(settings style.Style, biomes *relief.Relief) func(east, north float64) ([4]float32, bool) {
	colors := map[int][4]float32{}
	if biomes != nil {
		for category, label := range biomes.Labels {
			if color, err := settings.Section(label).Color("color", nil, [4]float32{}); err == nil && color[3] > 0 {
				colors[category] = color
			}
		}
	}
	return func(east, north float64) ([4]float32, bool) {
		if len(colors) == 0 {
			return [4]float32{}, false
		}
		var sum [4]float32
		count := float32(0)
		for rowOffset := -1; rowOffset <= 1; rowOffset++ {
			for columnOffset := -1; columnOffset <= 1; columnOffset++ {
				category, inside := biomes.Category(east+float64(columnOffset)*biomes.Cell/2, north+float64(rowOffset)*biomes.Cell/2)
				if color, known := colors[category]; inside && known {
					for channel := range 4 {
						sum[channel] += color[channel]
					}
					count++
				}
			}
		}
		if count == 0 {
			return [4]float32{}, false
		}
		return [4]float32{sum[0] / count, sum[1] / count, sum[2] / count, sum[3] / count}, true
	}
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

// terrainColors returns the colour of the ground at a height and place: the biome's colour (from
// the biome layer and terrain.biomes, blended over the neighbouring cells), or the base colour,
// then the height bands over it.
func terrainColors(settings style.Style, biomes *relief.Relief) func(height, east, north float64) [4]float32 {
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
	biomeColor := biomeColors(settings.Section("biomes"), biomes)
	return func(height, east, north float64) [4]float32 {
		color := base
		if blended, found := biomeColor(east, north); found {
			color = blended
		}
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

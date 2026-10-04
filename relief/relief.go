// Package relief reads a regular grid of heights: a header (relieve.json) and the values in metres
// as little-endian float32, row by row from the north (relieve.f32, next to the header).
//
//	{"columns": 380, "rows": 380, "cell": 0.0125, "west": 46.0, "north": 27.1, "units": "km"}
//
// west and north are the edges of the grid; each value is the height at the centre of its cell.
package relief

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strings"
)

// Relief is a grid of heights in metres over kilometres east and north.
type Relief struct {
	Columns int     `json:"columns"`
	Rows    int     `json:"rows"`
	Cell    float64 `json:"cell"`
	West    float64 `json:"west"`
	North   float64 `json:"north"`
	Units   string  `json:"units"`
	Heights []float32 `json:"-"`
}

// Load reads the header and its values file (the header path with .f32 instead of .json).
func Load(headerPath string) (*Relief, error) {
	content, err := os.ReadFile(headerPath)
	if err != nil {
		return nil, err
	}
	var grid Relief
	if err := json.Unmarshal(content, &grid); err != nil {
		return nil, fmt.Errorf("relief: %w", err)
	}
	if err := grid.convertUnits(); err != nil {
		return nil, err
	}
	values, err := os.ReadFile(strings.TrimSuffix(headerPath, ".json") + ".f32")
	if err != nil {
		return nil, err
	}
	if len(values) != 4*grid.Columns*grid.Rows {
		return nil, fmt.Errorf("relief: %d bytes of heights for %d × %d cells", len(values), grid.Columns, grid.Rows)
	}
	grid.Heights = make([]float32, grid.Columns*grid.Rows)
	for index := range grid.Heights {
		grid.Heights[index] = math.Float32frombits(binary.LittleEndian.Uint32(values[4*index:]))
	}
	return &grid, nil
}

func (grid *Relief) convertUnits() error {
	switch grid.Units {
	case "", "km":
	case "m":
		grid.Cell, grid.West, grid.North = grid.Cell/1000, grid.West/1000, grid.North/1000
	default:
		return fmt.Errorf("relief: unknown units %q", grid.Units)
	}
	if grid.Columns < 2 || grid.Rows < 2 || grid.Cell <= 0 {
		return fmt.Errorf("relief: needs at least 2 × 2 cells of positive size")
	}
	return nil
}

// East and South are the other two edges of the grid.
func (grid *Relief) East() float64  { return grid.West + float64(grid.Columns)*grid.Cell }
func (grid *Relief) South() float64 { return grid.North - float64(grid.Rows)*grid.Cell }

// Centre is the middle of the grid.
func (grid *Relief) Centre() (east, north float64) {
	return (grid.West + grid.East()) / 2, (grid.North + grid.South()) / 2
}

// At is the height of a cell centre.
func (grid *Relief) At(row, column int) float64 {
	return float64(grid.Heights[row*grid.Columns+column])
}

// CellCentre is the position of a cell centre.
func (grid *Relief) CellCentre(row, column int) (east, north float64) {
	return grid.West + (float64(column)+0.5)*grid.Cell, grid.North - (float64(row)+0.5)*grid.Cell
}

// Height is the height in metres at a point, interpolated over the same two triangles per cell
// square that the terrain mesh uses (so what is draped lies on the drawn ground). A nil relief is
// flat at 0.
func (grid *Relief) Height(east, north float64) float64 {
	if grid == nil {
		return 0
	}
	column := clamp((east-grid.West)/grid.Cell-0.5, 0, float64(grid.Columns)-1.001)
	row := clamp((grid.North-north)/grid.Cell-0.5, 0, float64(grid.Rows)-1.001)
	whole, wholeRow := int(column), int(row)
	across, down := column-float64(whole), row-float64(wholeRow)
	topLeft, topRight := grid.At(wholeRow, whole), grid.At(wholeRow, whole+1)
	bottomLeft, bottomRight := grid.At(wholeRow+1, whole), grid.At(wholeRow+1, whole+1)
	if across+down <= 1 {
		return topLeft + across*(topRight-topLeft) + down*(bottomLeft-topLeft)
	}
	return bottomRight + (1-across)*(bottomLeft-bottomRight) + (1-down)*(topRight-bottomRight)
}

func clamp(value, low, high float64) float64 {
	return math.Max(low, math.Min(high, value))
}

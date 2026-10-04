package relations

import (
	"math"

	"github.com/akrck02/go-scene-builder/mapdata"
)

// segment is a straight piece of a line or of a block's side.
type segment struct {
	from, to mapdata.Point
	owner    int // block or line it belongs to
	side     int // side of the block (for blocks)
}

// segmentIndex finds segments near a point: a grid of cells, each with the segments whose box touches it.
type segmentIndex struct {
	cell  float64
	cells map[[2]int][]segment
}

func newSegmentIndex(cell float64) *segmentIndex {
	return &segmentIndex{cell: cell, cells: map[[2]int][]segment{}}
}

func (index *segmentIndex) cellOf(point mapdata.Point) [2]int {
	return [2]int{int(math.Floor(point[0] / index.cell)), int(math.Floor(point[1] / index.cell))}
}

func (index *segmentIndex) add(piece segment) {
	low := index.cellOf(mapdata.Point{math.Min(piece.from[0], piece.to[0]), math.Min(piece.from[1], piece.to[1])})
	high := index.cellOf(mapdata.Point{math.Max(piece.from[0], piece.to[0]), math.Max(piece.from[1], piece.to[1])})
	for column := low[0]; column <= high[0]; column++ {
		for row := low[1]; row <= high[1]; row++ {
			key := [2]int{column, row}
			index.cells[key] = append(index.cells[key], piece)
		}
	}
}

// near calls visit for every segment in the cells within reach of a point (a segment may be visited
// more than once when it spans several cells).
func (index *segmentIndex) near(point mapdata.Point, reach float64, visit func(segment)) {
	low := index.cellOf(mapdata.Point{point[0] - reach, point[1] - reach})
	high := index.cellOf(mapdata.Point{point[0] + reach, point[1] + reach})
	for column := low[0]; column <= high[0]; column++ {
		for row := low[1]; row <= high[1]; row++ {
			for _, piece := range index.cells[[2]int{column, row}] {
				visit(piece)
			}
		}
	}
}

package recipes

import (
	"math"
	"testing"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/style"
	"github.com/twpayne/go-geos"
)

func houseWithRoof(recipe string, shared []bool) (*mesh.Piece, error) {
	ring := mapdata.Ring{{0, 0}, {0.006, 0}, {0.006, 0.01}, {0, 0.01}}
	current := &block.Block{Feature: &mapdata.Feature{}, Polygon: mapdata.Polygon{Exterior: ring}, Bottom: 0, Eaves: 6, Shared: shared,
		Style: style.Style{"roof": map[string]any{"recipe": recipe, "pitch": 45.0, "eaves": 0.0, "ridge": "long"}}}
	piece := &mesh.Piece{}
	decorator := &Decorator{Context: geos.NewContext()}
	return piece, decorator.Decorate(current, piece)
}

func highest(piece *mesh.Piece) float64 {
	top := 0.0
	for index := 1; index < len(piece.Positions); index += 3 {
		top = math.Max(top, float64(piece.Positions[index]))
	}
	return top
}

func TestGableRoof(test *testing.T) {
	piece, err := houseWithRoof("gable", nil)
	if err != nil {
		test.Fatal(err)
	}
	// 4 walls (8) + 2 slopes (4) + 2 gable ends (2)
	if piece.TriangleCount() != 14 {
		test.Errorf("%d triangles, want 14", piece.TriangleCount())
	}
	// 6 m wide house, 45°: the ridge is 3 m over the eaves at 6 m
	if top := highest(piece); math.Abs(top-0.009) > 0.0001 {
		test.Errorf("ridge at %v km, want 0.009", top)
	}
}

func TestHipAndFlatRoofsAndSharedWalls(test *testing.T) {
	hip, err := houseWithRoof("hip", nil)
	if err != nil || hip.TriangleCount() != 8+4+2 {
		test.Errorf("hip: %d triangles, %v", hip.TriangleCount(), err)
	}
	flat, err := houseWithRoof("flat", []bool{false, true, false, true})
	if err != nil || flat.TriangleCount() != 4+2 {
		test.Errorf("flat with two shared walls: %d triangles, want 6, %v", flat.TriangleCount(), err)
	}
	if _, err := houseWithRoof("dome", nil); err == nil {
		test.Error("an unknown recipe should be reported")
	}
}

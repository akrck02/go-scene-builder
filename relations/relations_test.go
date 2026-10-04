package relations

import (
	"math"
	"testing"

	"github.com/akrck02/go-scene-builder/block"
	"github.com/akrck02/go-scene-builder/mapdata"
)

func house(west float64) *block.Block {
	ring := mapdata.Ring{{west, 0}, {west + 0.006, 0}, {west + 0.006, 0.01}, {west, 0.01}}
	return &block.Block{Feature: &mapdata.Feature{}, Polygon: mapdata.Polygon{Exterior: ring}, Eaves: 6 + west*1000}
}

func TestFacingAndRows(test *testing.T) {
	street := &mapdata.Feature{Kind: "street", Lines: [][]mapdata.Point{{{-0.05, -0.004}, {0.05, -0.004}}}}
	left, right, alone := house(0), house(0.006), house(0.03)
	settings := map[string]any{"facing": map[string]any{"when": map[string]any{"kind": "street"}, "reach": 20.0}}
	Compute([]*block.Block{left, right, alone}, []*mapdata.Feature{street}, settings)
	for name, current := range map[string]*block.Block{"left": left, "right": right, "alone": alone} {
		if current.Facing == nil || current.Facing.Side != 0 || current.Facing.Outward != (mapdata.Point{0, -1}) {
			test.Errorf("%s should face the street to the south: %+v", name, current.Facing)
		}
	}
	if left.SharedUpTo(1) != right.Eaves || right.SharedUpTo(3) != left.Eaves {
		test.Errorf("the houses share a wall: left %v, right %v", left.Covered, right.Covered)
	}
	for side := range alone.Polygon.Exterior {
		if !math.IsInf(alone.SharedUpTo(side), -1) {
			test.Errorf("the lone house shares side %d", side)
		}
	}
}

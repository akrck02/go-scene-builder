package gltf

import (
	"encoding/binary"
	"encoding/json"
	"math"
	"testing"

	"github.com/akrck02/go-scene-builder/mesh"
)

func TestQuantisedPositionsComeBack(test *testing.T) {
	piece := &mesh.Piece{Name: "test", Material: mesh.Material{Name: "solid"}}
	corners := []mesh.Vector{{-0.3, 0.01, 0.2}, {0.25, 0.4, -0.1}, {0.1, 0.02, 0.3}}
	piece.AddTriangle(corners[0], corners[1], corners[2], mesh.Vector{0, 1, 0}, [4]float32{1, 0, 0, 1})
	content, err := Encode([]*mesh.Piece{piece}, nil)
	if err != nil {
		test.Fatal(err)
	}
	if string(content[:4]) != "glTF" || binary.LittleEndian.Uint32(content[8:]) != uint32(len(content)) {
		test.Fatal("not a glb container")
	}
	length := binary.LittleEndian.Uint32(content[12:])
	var parsed document
	if err := json.Unmarshal(content[20:20+length], &parsed); err != nil {
		test.Fatal(err)
	}
	data := content[20+length+8:]
	node := parsed.Nodes[0]
	view := parsed.BufferViews[parsed.Accessors[parsed.Meshes[0].Primitives[0].Attributes["POSITION"]].BufferView]
	for vertex := range 3 {
		for axis := range 3 {
			offset := view.ByteOffset + vertex*view.ByteStride + 2*axis
			whole := float64(int16(binary.LittleEndian.Uint16(data[offset:])))
			back := whole*node.Scale[axis] + node.Translation[axis]
			if want := float64(piece.Positions[3*vertex+axis]); math.Abs(back-want) > 1e-4 {
				test.Errorf("vertex %d axis %d: %v back, %v written", vertex, axis, back, want)
			}
		}
	}
}

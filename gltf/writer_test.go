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
	content, err := Encode([]*mesh.Piece{piece}, nil, nil)
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

func TestInstancesAreWrittenOnce(t *testing.T) {
	model := &mesh.Piece{Name: "flower", Material: mesh.Material{Name: "flowers"}}
	model.AddTriangle(mesh.Vector{0, 0, 0}, mesh.Vector{0.001, 0, 0}, mesh.Vector{0, 0.001, 0}, mesh.Vector{0, 0, 1}, [4]float32{1, 1, 1, 1})
	copies := &mesh.Instances{Name: "flowers", Model: []*mesh.Piece{model}}
	for index := range 100 {
		copies.Add(mesh.Vector{float64(index) * 0.01, 0, 0}, 0.5, 1, [4]float32{1, 1, 1, 1})
	}
	content, err := Encode(nil, []*mesh.Instances{copies}, nil)
	if err != nil {
		t.Fatal(err)
	}
	length := binary.LittleEndian.Uint32(content[12:])
	var parsed map[string]any
	if err := json.Unmarshal(content[20:20+length], &parsed); err != nil {
		t.Fatal(err)
	}
	nodes := parsed["nodes"].([]any)
	if len(nodes) != 1 || nodes[0].(map[string]any)["extensions"] == nil {
		t.Fatalf("one instanced node expected: %v", nodes)
	}
	if used := parsed["extensionsUsed"].([]any); len(used) != 2 {
		t.Errorf("instancing should be declared: %v", used)
	}
}

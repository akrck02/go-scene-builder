// Package gltf writes pieces as a binary glTF 2.0 file (.glb) with the conventions go-renderer
// reads: positions and normals as float32, colours as normalised bytes, a node per piece, "ground"
// in the extras of walkable pieces, the material "kind" and "depthBias" in the material extras, and
// the environment (sun, fog, water, vertical scale) in the scene extras.
package gltf

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"

	"github.com/akrck02/go-scene-builder/mesh"
)

const (
	componentUnsignedByte = 5121
	componentUnsignedInt  = 5125
	componentFloat        = 5126
	targetVertices        = 34962
	targetIndices         = 34963
)

type document struct {
	Asset       map[string]string `json:"asset"`
	Scene       int               `json:"scene"`
	Scenes      []sceneEntry      `json:"scenes"`
	Nodes       []nodeEntry       `json:"nodes"`
	Meshes      []meshEntry       `json:"meshes"`
	Materials   []materialEntry   `json:"materials"`
	Accessors   []accessorEntry   `json:"accessors"`
	BufferViews []bufferViewEntry `json:"bufferViews"`
	Buffers     []bufferEntry     `json:"buffers"`
}

type sceneEntry struct {
	Nodes  []int          `json:"nodes"`
	Extras map[string]any `json:"extras,omitempty"`
}

type nodeEntry struct {
	Name   string         `json:"name"`
	Mesh   int            `json:"mesh"`
	Extras map[string]any `json:"extras,omitempty"`
}

type meshEntry struct {
	Name       string           `json:"name"`
	Primitives []primitiveEntry `json:"primitives"`
	Extras     map[string]any   `json:"extras,omitempty"`
}

type primitiveEntry struct {
	Attributes map[string]int `json:"attributes"`
	Indices    int            `json:"indices"`
	Material   int            `json:"material"`
}

type materialEntry struct {
	Name        string         `json:"name"`
	Metallic    map[string]any `json:"pbrMetallicRoughness"`
	DoubleSided bool           `json:"doubleSided"`
	AlphaMode   string         `json:"alphaMode"`
	Extras      map[string]any `json:"extras"`
}

type accessorEntry struct {
	BufferView    int       `json:"bufferView"`
	ComponentType int       `json:"componentType"`
	Normalized    bool      `json:"normalized,omitempty"`
	Count         int       `json:"count"`
	Type          string    `json:"type"`
	Minimum       []float32 `json:"min,omitempty"`
	Maximum       []float32 `json:"max,omitempty"`
}

type bufferViewEntry struct {
	Buffer     int `json:"buffer"`
	ByteOffset int `json:"byteOffset"`
	ByteLength int `json:"byteLength"`
	Target     int `json:"target,omitempty"`
}

type bufferEntry struct {
	ByteLength int `json:"byteLength"`
}

// writer gathers the document and its binary data.
type writer struct {
	document  document
	binary    bytes.Buffer
	materials map[string]int
}

// Write saves the pieces as a .glb, with the given scene extras (for example {"environment": …}).
func Write(path string, pieces []*mesh.Piece, sceneExtras map[string]any) (int, error) {
	content, err := Encode(pieces, sceneExtras)
	if err != nil {
		return 0, err
	}
	return len(content), os.WriteFile(path, content, 0o644)
}

// Encode returns the .glb bytes of the pieces.
func Encode(pieces []*mesh.Piece, sceneExtras map[string]any) ([]byte, error) {
	output := &writer{materials: map[string]int{}}
	output.document.Asset = map[string]string{"version": "2.0", "generator": "go-scene-builder"}
	output.document.Scenes = []sceneEntry{{Nodes: []int{}, Extras: sceneExtras}}
	for _, piece := range pieces {
		if piece.TriangleCount() > 0 {
			output.addPiece(piece)
		}
	}
	output.document.Buffers = []bufferEntry{{ByteLength: output.binary.Len()}}
	text, err := json.Marshal(output.document)
	if err != nil {
		return nil, err
	}
	return container(text, output.binary.Bytes()), nil
}

func (output *writer) addPiece(piece *mesh.Piece) {
	attributes := map[string]int{
		"POSITION": output.floatAccessor(piece.Positions, "VEC3", true),
		"NORMAL":   output.floatAccessor(piece.Normals, "VEC3", false),
		"COLOR_0":  output.colorAccessor(piece.Colors),
	}
	primitive := primitiveEntry{Attributes: attributes, Indices: output.indexAccessor(piece.Indices), Material: output.material(piece.Material)}
	var extras map[string]any
	if piece.Ground {
		extras = map[string]any{"ground": true}
	}
	output.document.Meshes = append(output.document.Meshes, meshEntry{Name: piece.Name, Primitives: []primitiveEntry{primitive}, Extras: extras})
	output.document.Nodes = append(output.document.Nodes, nodeEntry{Name: piece.Name, Mesh: len(output.document.Meshes) - 1, Extras: extras})
	output.document.Scenes[0].Nodes = append(output.document.Scenes[0].Nodes, len(output.document.Nodes)-1)
}

func (output *writer) material(material mesh.Material) int {
	if index, known := output.materials[material.Name]; known {
		return index
	}
	kind := material.Kind
	if kind == "" {
		kind = "lit"
	}
	extras := map[string]any{"kind": kind}
	if material.DepthBias > 0 {
		extras["depthBias"] = material.DepthBias
	}
	output.document.Materials = append(output.document.Materials, materialEntry{
		Name:        material.Name,
		Metallic:    map[string]any{"baseColorFactor": []float32{1, 1, 1, 1}, "metallicFactor": 0, "roughnessFactor": 0.9},
		DoubleSided: material.DoubleSided, AlphaMode: "OPAQUE", Extras: extras})
	output.materials[material.Name] = len(output.document.Materials) - 1
	return output.materials[material.Name]
}

// view appends data to the binary buffer (aligned to 4 bytes) and returns its buffer view.
func (output *writer) view(data []byte, target int) int {
	for output.binary.Len()%4 != 0 {
		output.binary.WriteByte(0)
	}
	offset := output.binary.Len()
	output.binary.Write(data)
	output.document.BufferViews = append(output.document.BufferViews, bufferViewEntry{ByteOffset: offset, ByteLength: len(data), Target: target})
	return len(output.document.BufferViews) - 1
}

func (output *writer) accessor(entry accessorEntry) int {
	output.document.Accessors = append(output.document.Accessors, entry)
	return len(output.document.Accessors) - 1
}

func (output *writer) floatAccessor(values []float32, kind string, withBounds bool) int {
	data := make([]byte, 4*len(values))
	for index, value := range values {
		binary.LittleEndian.PutUint32(data[4*index:], math.Float32bits(value))
	}
	entry := accessorEntry{BufferView: output.view(data, targetVertices), ComponentType: componentFloat, Count: len(values) / 3, Type: kind}
	if withBounds {
		entry.Minimum, entry.Maximum = boundsOf(values)
	}
	return output.accessor(entry)
}

func (output *writer) colorAccessor(values []float32) int {
	data := make([]byte, len(values))
	for index, value := range values {
		data[index] = byte(math.Round(math.Max(0, math.Min(1, float64(value))) * 255))
	}
	view := output.view(data, targetVertices)
	return output.accessor(accessorEntry{BufferView: view, ComponentType: componentUnsignedByte, Normalized: true, Count: len(values) / 4, Type: "VEC4"})
}

func (output *writer) indexAccessor(indices []uint32) int {
	data := make([]byte, 4*len(indices))
	for index, value := range indices {
		binary.LittleEndian.PutUint32(data[4*index:], value)
	}
	view := output.view(data, targetIndices)
	return output.accessor(accessorEntry{BufferView: view, ComponentType: componentUnsignedInt, Count: len(indices), Type: "SCALAR"})
}

func boundsOf(positions []float32) ([]float32, []float32) {
	minimum := []float32{math.MaxFloat32, math.MaxFloat32, math.MaxFloat32}
	maximum := []float32{-math.MaxFloat32, -math.MaxFloat32, -math.MaxFloat32}
	for index, value := range positions {
		axis := index % 3
		minimum[axis] = min(minimum[axis], value)
		maximum[axis] = max(maximum[axis], value)
	}
	return minimum, maximum
}

// container wraps the JSON and binary chunks in the GLB header.
func container(text, data []byte) []byte {
	for len(text)%4 != 0 {
		text = append(text, ' ')
	}
	for len(data)%4 != 0 {
		data = append(data, 0)
	}
	var result bytes.Buffer
	total := 12 + 8 + len(text) + 8 + len(data)
	for _, value := range []uint32{0x46546C67, 2, uint32(total), uint32(len(text)), 0x4E4F534A} {
		binary.Write(&result, binary.LittleEndian, value)
	}
	result.Write(text)
	for _, value := range []uint32{uint32(len(data)), 0x004E4942} {
		binary.Write(&result, binary.LittleEndian, value)
	}
	result.Write(data)
	return result.Bytes()
}

package models

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/akrck02/go-scene-builder/mesh"
)

// readGLB reads the meshes of a binary glTF file (a model made elsewhere, in metres, y up) into
// pieces: one per material, with the material's base colour times the vertex colours, every node's
// transform applied.
func readGLB(path string) ([]*mesh.Piece, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	document, data, err := splitGLB(content)
	if err != nil {
		return nil, err
	}
	reader := &glbReader{document: document, data: data, pieces: map[int]*mesh.Piece{}}
	scene := document.Scene
	if scene >= len(document.Scenes) {
		return nil, fmt.Errorf("no scene")
	}
	for _, node := range document.Scenes[scene].Nodes {
		if err := reader.node(node, identity()); err != nil {
			return nil, err
		}
	}
	pieces := make([]*mesh.Piece, 0, len(reader.pieces))
	for material := -1; material < len(document.Materials); material++ {
		if piece := reader.pieces[material]; piece != nil {
			pieces = append(pieces, piece)
		}
	}
	return pieces, nil
}

type glbDocument struct {
	Scene  int `json:"scene"`
	Scenes []struct {
		Nodes []int `json:"nodes"`
	} `json:"scenes"`
	Nodes []struct {
		Mesh        *int      `json:"mesh"`
		Children    []int     `json:"children"`
		Matrix      []float64 `json:"matrix"`
		Translation []float64 `json:"translation"`
		Rotation    []float64 `json:"rotation"`
		Scale       []float64 `json:"scale"`
	} `json:"nodes"`
	Meshes []struct {
		Primitives []struct {
			Attributes map[string]int `json:"attributes"`
			Indices    *int           `json:"indices"`
			Material   *int           `json:"material"`
		} `json:"primitives"`
	} `json:"meshes"`
	Materials []struct {
		Name     string `json:"name"`
		Metallic struct {
			BaseColor []float64 `json:"baseColorFactor"`
		} `json:"pbrMetallicRoughness"`
		DoubleSided bool `json:"doubleSided"`
	} `json:"materials"`
	Accessors []struct {
		BufferView    int    `json:"bufferView"`
		ByteOffset    int    `json:"byteOffset"`
		ComponentType int    `json:"componentType"`
		Normalized    bool   `json:"normalized"`
		Count         int    `json:"count"`
		Type          string `json:"type"`
	} `json:"accessors"`
	BufferViews []struct {
		ByteOffset int `json:"byteOffset"`
		ByteLength int `json:"byteLength"`
		ByteStride int `json:"byteStride"`
	} `json:"bufferViews"`
}

func splitGLB(content []byte) (*glbDocument, []byte, error) {
	if len(content) < 20 || string(content[:4]) != "glTF" {
		return nil, nil, fmt.Errorf("not a binary glTF file")
	}
	length := int(binary.LittleEndian.Uint32(content[12:]))
	var document glbDocument
	if err := json.Unmarshal(content[20:20+length], &document); err != nil {
		return nil, nil, err
	}
	var data []byte
	if rest := content[20+length:]; len(rest) >= 8 {
		data = rest[8 : 8+int(binary.LittleEndian.Uint32(rest))]
	}
	return &document, data, nil
}

type glbReader struct {
	document *glbDocument
	data     []byte
	pieces   map[int]*mesh.Piece
}

type matrix [16]float64 // column-major, as in glTF

func identity() matrix { return matrix{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1} }

func (first matrix) times(second matrix) matrix {
	var result matrix
	for column := range 4 {
		for row := range 4 {
			for index := range 4 {
				result[column*4+row] += first[index*4+row] * second[column*4+index]
			}
		}
	}
	return result
}

func (transform matrix) apply(x, y, z, w float64) [3]float64 {
	return [3]float64{
		transform[0]*x + transform[4]*y + transform[8]*z + transform[12]*w,
		transform[1]*x + transform[5]*y + transform[9]*z + transform[13]*w,
		transform[2]*x + transform[6]*y + transform[10]*z + transform[14]*w,
	}
}

func (reader *glbReader) node(index int, parent matrix) error {
	node := reader.document.Nodes[index]
	local := identity()
	if len(node.Matrix) == 16 {
		copy(local[:], node.Matrix)
	} else {
		local = translationRotationScale(node.Translation, node.Rotation, node.Scale)
	}
	world := parent.times(local)
	if node.Mesh != nil {
		if err := reader.mesh(*node.Mesh, world); err != nil {
			return err
		}
	}
	for _, child := range node.Children {
		if err := reader.node(child, world); err != nil {
			return err
		}
	}
	return nil
}

func translationRotationScale(translation, rotation, scale []float64) matrix {
	x, y, z, w := 0.0, 0.0, 0.0, 1.0
	if len(rotation) == 4 {
		x, y, z, w = rotation[0], rotation[1], rotation[2], rotation[3]
	}
	scaleX, scaleY, scaleZ := 1.0, 1.0, 1.0
	if len(scale) == 3 {
		scaleX, scaleY, scaleZ = scale[0], scale[1], scale[2]
	}
	result := matrix{
		(1 - 2*(y*y+z*z)) * scaleX, 2 * (x*y + z*w) * scaleX, 2 * (x*z - y*w) * scaleX, 0,
		2 * (x*y - z*w) * scaleY, (1 - 2*(x*x+z*z)) * scaleY, 2 * (y*z + x*w) * scaleY, 0,
		2 * (x*z + y*w) * scaleZ, 2 * (y*z - x*w) * scaleZ, (1 - 2*(x*x+y*y)) * scaleZ, 0,
		0, 0, 0, 1,
	}
	if len(translation) == 3 {
		result[12], result[13], result[14] = translation[0], translation[1], translation[2]
	}
	return result
}

func (reader *glbReader) mesh(index int, transform matrix) error {
	for _, primitive := range reader.document.Meshes[index].Primitives {
		positions, err := reader.accessor(primitive.Attributes["POSITION"])
		if err != nil {
			return err
		}
		var normals, colors [][]float64
		if accessor, present := primitive.Attributes["NORMAL"]; present {
			if normals, err = reader.accessor(accessor); err != nil {
				return err
			}
		}
		if accessor, present := primitive.Attributes["COLOR_0"]; present {
			if colors, err = reader.accessor(accessor); err != nil {
				return err
			}
		}
		indices, err := reader.indices(primitive.Indices, len(positions))
		if err != nil {
			return err
		}
		material := -1
		if primitive.Material != nil {
			material = *primitive.Material
		}
		reader.add(reader.pieceFor(material), transform, positions, normals, colors, indices, reader.baseColor(material))
	}
	return nil
}

func (reader *glbReader) pieceFor(material int) *mesh.Piece {
	if piece := reader.pieces[material]; piece != nil {
		return piece
	}
	name, doubleSided := "model", false
	if material >= 0 {
		name, doubleSided = reader.document.Materials[material].Name, reader.document.Materials[material].DoubleSided
	}
	piece := &mesh.Piece{Name: name, Material: mesh.Material{Name: "model " + name, Kind: "lit", DoubleSided: doubleSided}}
	reader.pieces[material] = piece
	return piece
}

func (reader *glbReader) baseColor(material int) [4]float64 {
	if material < 0 || len(reader.document.Materials[material].Metallic.BaseColor) != 4 {
		return [4]float64{1, 1, 1, 1}
	}
	color := reader.document.Materials[material].Metallic.BaseColor
	return [4]float64{color[0], color[1], color[2], color[3]}
}

// add appends a primitive's triangles to a piece, in kilometres (the file is in metres).
func (reader *glbReader) add(piece *mesh.Piece, transform matrix, positions, normals, colors [][]float64, indices []int, base [4]float64) {
	normalTransform := transform
	normalTransform[12], normalTransform[13], normalTransform[14] = 0, 0, 0
	offset := uint32(piece.VertexCount())
	for vertex, position := range positions {
		placed := transform.apply(position[0], position[1], position[2], 1)
		normal := mesh.Vector{0, 1, 0}
		if vertex < len(normals) {
			turned := normalTransform.apply(normals[vertex][0], normals[vertex][1], normals[vertex][2], 0)
			normal = mesh.Vector(turned).Normalized()
		}
		color := [4]float32{float32(base[0]), float32(base[1]), float32(base[2]), float32(base[3])}
		if vertex < len(colors) {
			for channel := range min(4, len(colors[vertex])) {
				color[channel] *= float32(colors[vertex][channel])
			}
		}
		piece.AddVertex(mesh.Vector{placed[0] / 1000, placed[1] / 1000, placed[2] / 1000}, normal, color)
	}
	for _, index := range indices {
		piece.Indices = append(piece.Indices, offset+uint32(index))
	}
}

var componentCounts = map[string]int{"SCALAR": 1, "VEC2": 2, "VEC3": 3, "VEC4": 4}

// accessor reads an accessor's elements as numbers (normalised integers become 0–1 or −1–1).
func (reader *glbReader) accessor(index int) ([][]float64, error) {
	if index >= len(reader.document.Accessors) {
		return nil, fmt.Errorf("no accessor %d", index)
	}
	accessor := reader.document.Accessors[index]
	view := reader.document.BufferViews[accessor.BufferView]
	components := componentCounts[accessor.Type]
	size := map[int]int{5120: 1, 5121: 1, 5122: 2, 5123: 2, 5125: 4, 5126: 4}[accessor.ComponentType]
	if components == 0 || size == 0 {
		return nil, fmt.Errorf("accessor %d: unsupported type", index)
	}
	stride := view.ByteStride
	if stride == 0 {
		stride = components * size
	}
	start := view.ByteOffset + accessor.ByteOffset
	elements := make([][]float64, accessor.Count)
	for element := range accessor.Count {
		values := make([]float64, components)
		for component := range components {
			at := start + element*stride + component*size
			if at+size > len(reader.data) {
				return nil, fmt.Errorf("accessor %d runs past the data", index)
			}
			values[component] = readComponent(reader.data[at:], accessor.ComponentType, accessor.Normalized)
		}
		elements[element] = values
	}
	return elements, nil
}

func readComponent(data []byte, componentType int, normalized bool) float64 {
	switch componentType {
	case 5126:
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(data)))
	case 5121:
		return scaled(float64(data[0]), 255, normalized)
	case 5123:
		return scaled(float64(binary.LittleEndian.Uint16(data)), 65535, normalized)
	case 5120:
		return math.Max(scaled(float64(int8(data[0])), 127, normalized), -1)
	case 5122:
		return math.Max(scaled(float64(int16(binary.LittleEndian.Uint16(data))), 32767, normalized), -1)
	case 5125:
		return float64(binary.LittleEndian.Uint32(data))
	}
	return 0
}

func scaled(value, maximum float64, normalized bool) float64 {
	if normalized {
		return value / maximum
	}
	return value
}

func (reader *glbReader) indices(accessor *int, vertices int) ([]int, error) {
	if accessor == nil {
		indices := make([]int, vertices)
		for index := range indices {
			indices[index] = index
		}
		return indices, nil
	}
	elements, err := reader.accessor(*accessor)
	if err != nil {
		return nil, err
	}
	indices := make([]int, len(elements))
	for index, element := range elements {
		indices[index] = int(element[0])
	}
	return indices, nil
}

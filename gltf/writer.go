// Package gltf writes pieces as a binary glTF 2.0 file (.glb) with the conventions go-renderer
// reads, quantised to take less room (KHR_mesh_quantization): positions as 16-bit integers that
// the node scales and moves back, normals and colours as normalised bytes, 16-bit indices when
// they fit. A node per piece, "ground"
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
	componentByte          = 5120
	componentUnsignedByte  = 5121
	componentShort         = 5122
	componentUnsignedShort = 5123
	componentUnsignedInt   = 5125
	componentFloat         = 5126
	largestShort           = 32767
	targetVertices         = 34962
	targetIndices          = 34963
)

type document struct {
	Asset       map[string]string `json:"asset"`
	Used        []string          `json:"extensionsUsed"`
	Required    []string          `json:"extensionsRequired"`
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
	Name        string         `json:"name"`
	Mesh        int            `json:"mesh"`
	Translation []float64      `json:"translation,omitempty"`
	Scale       []float64      `json:"scale,omitempty"`
	Extensions  map[string]any `json:"extensions,omitempty"`
	Extras      map[string]any `json:"extras,omitempty"`
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
	ByteStride int `json:"byteStride,omitempty"`
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

// Write saves the pieces and instanced models as a .glb, with the given scene extras (for example
// {"environment": …}).
func Write(path string, pieces []*mesh.Piece, instances []*mesh.Instances, sceneExtras map[string]any) (int, error) {
	content, err := Encode(pieces, instances, sceneExtras)
	if err != nil {
		return 0, err
	}
	return len(content), os.WriteFile(path, content, 0o644)
}

// Encode returns the .glb bytes of the pieces and instanced models.
func Encode(pieces []*mesh.Piece, instances []*mesh.Instances, sceneExtras map[string]any) ([]byte, error) {
	output := &writer{materials: map[string]int{}}
	output.document.Asset = map[string]string{"version": "2.0", "generator": "go-scene-builder"}
	output.document.Used = []string{"KHR_mesh_quantization"}
	output.document.Required = []string{"KHR_mesh_quantization"}
	output.document.Scenes = []sceneEntry{{Nodes: []int{}, Extras: sceneExtras}}
	for _, piece := range pieces {
		if piece.TriangleCount() > 0 {
			output.addPiece(piece)
		}
	}
	for _, copies := range instances {
		if copies.Count() > 0 {
			output.addInstances(copies)
		}
	}
	if len(instances) > 0 {
		output.document.Used = append(output.document.Used, "EXT_mesh_gpu_instancing")
	}
	output.document.Buffers = []bufferEntry{{ByteLength: output.binary.Len()}}
	text, err := json.Marshal(output.document)
	if err != nil {
		return nil, err
	}
	return container(text, output.binary.Bytes()), nil
}

func (output *writer) addPiece(piece *mesh.Piece) {
	// the simpler versions share the node, so they share its quantisation
	all := append([]float32(nil), piece.Positions...)
	for _, detail := range piece.Details {
		if detail.Piece != nil {
			all = append(all, detail.Piece.Positions...)
		}
	}
	centre, scale := quantisation(all)
	quantised := func(version *mesh.Piece) map[string]int {
		return map[string]int{"POSITION": output.positionAccessor(version.Positions, centre, scale),
			"NORMAL": output.normalAccessor(version.Normals), "COLOR_0": output.colorAccessor(version.Colors)}
	}
	var extras map[string]any
	if piece.Ground {
		extras = map[string]any{"ground": true}
	}
	index := output.addMesh(piece, quantised(piece), extras)
	output.addDetails(index, piece, quantised)
	output.document.Nodes = append(output.document.Nodes, nodeEntry{Name: piece.Name, Mesh: index,
		Translation: centre[:], Scale: []float64{scale, scale, scale}, Extras: extras})
	output.document.Scenes[0].Nodes = append(output.document.Scenes[0].Nodes, len(output.document.Nodes)-1)
}

// addMesh writes a mesh of one primitive and returns its index.
func (output *writer) addMesh(piece *mesh.Piece, attributes map[string]int, extras map[string]any) int {
	primitive := primitiveEntry{Attributes: attributes, Indices: output.indexAccessor(piece.Indices), Material: output.material(piece.Material)}
	output.document.Meshes = append(output.document.Meshes, meshEntry{Name: piece.Name, Primitives: []primitiveEntry{primitive}, Extras: extras})
	return len(output.document.Meshes) - 1
}

// addDetails writes the simpler versions of a piece (an empty version as one degenerate triangle)
// and lists them in the main mesh's extras.
func (output *writer) addDetails(main int, piece *mesh.Piece, attributes func(*mesh.Piece) map[string]int) {
	if len(piece.Details) == 0 {
		return
	}
	var levels []any
	for _, detail := range piece.Details {
		version := detail.Piece
		if version == nil {
			version = nothingLike(piece)
		}
		index := output.addMesh(version, attributes(version), nil)
		levels = append(levels, map[string]any{"mesh": index, "screenSize": detail.ScreenSize})
	}
	entry := &output.document.Meshes[main]
	if entry.Extras == nil {
		entry.Extras = map[string]any{}
	}
	entry.Extras["detail"] = levels
}

// nothingLike is a version of a piece that draws nothing: one triangle with no area.
func nothingLike(piece *mesh.Piece) *mesh.Piece {
	empty := &mesh.Piece{Name: piece.Name + " (hidden)", Material: piece.Material}
	corner := mesh.Vector{float64(piece.Positions[0]), float64(piece.Positions[1]), float64(piece.Positions[2])}
	for range 3 {
		empty.Indices = append(empty.Indices, empty.AddVertex(corner, mesh.Vector{0, 1, 0}, [4]float32{}))
	}
	return empty
}

// addInstances writes a model once (a mesh per piece, positions as floats: the instance transforms
// must not be scaled by a node) and a node per piece that repeats it (EXT_mesh_gpu_instancing).
func (output *writer) addInstances(copies *mesh.Instances) {
	count := copies.Count()
	rotations := make([]float32, 0, 4*count)
	scales := make([]float32, 0, 3*count)
	for index := range count {
		half := float64(copies.Turns[index]) / 2
		rotations = append(rotations, 0, float32(math.Sin(half)), 0, float32(math.Cos(half)))
		scale := copies.Scales[index]
		scales = append(scales, scale, scale, scale)
	}
	attributes := map[string]any{
		"TRANSLATION": output.floatAccessor(copies.Positions, "VEC3", false),
		"ROTATION":    output.floatAccessor(rotations, "VEC4", false),
		"SCALE":       output.floatAccessor(scales, "VEC3", false),
		"_COLOR":      output.floatAccessor(copies.Tints, "VEC4", false),
	}
	floats := func(version *mesh.Piece) map[string]int {
		return map[string]int{"POSITION": output.floatAccessor(version.Positions, "VEC3", true),
			"NORMAL": output.normalAccessor(version.Normals), "COLOR_0": output.colorAccessor(version.Colors)}
	}
	for _, piece := range copies.Model {
		if piece.TriangleCount() == 0 {
			continue
		}
		index := output.addMesh(piece, floats(piece), nil)
		output.addDetails(index, piece, floats)
		output.document.Nodes = append(output.document.Nodes, nodeEntry{Name: copies.Name, Mesh: index,
			Extensions: map[string]any{"EXT_mesh_gpu_instancing": map[string]any{"attributes": attributes}}})
		output.document.Scenes[0].Nodes = append(output.document.Scenes[0].Nodes, len(output.document.Nodes)-1)
	}
}

// floatAccessor stores float32 values; with bounds for positions.
func (output *writer) floatAccessor(values []float32, kind string, withBounds bool) int {
	data := make([]byte, 0, 4*len(values))
	for _, value := range values {
		data = binary.LittleEndian.AppendUint32(data, math.Float32bits(value))
	}
	components := map[string]int{"VEC3": 3, "VEC4": 4}[kind]
	entry := accessorEntry{BufferView: output.view(data, 0), ComponentType: componentFloat, Count: len(values) / components, Type: kind}
	if withBounds {
		entry.Minimum, entry.Maximum = boundsOf(values)
	}
	return output.accessor(entry)
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
	if material.Foliage {
		extras["foliage"] = true
	}
	if material.Pattern != "" {
		extras["pattern"] = material.Pattern
		extras["patternScale"] = material.PatternScale / 1000 // metres to the scene's kilometres
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
	return output.stridedView(data, target, 0)
}

func (output *writer) stridedView(data []byte, target, stride int) int {
	for output.binary.Len()%4 != 0 {
		output.binary.WriteByte(0)
	}
	offset := output.binary.Len()
	output.binary.Write(data)
	output.document.BufferViews = append(output.document.BufferViews, bufferViewEntry{ByteOffset: offset, ByteLength: len(data), ByteStride: stride, Target: target})
	return len(output.document.BufferViews) - 1
}

func (output *writer) accessor(entry accessorEntry) int {
	output.document.Accessors = append(output.document.Accessors, entry)
	return len(output.document.Accessors) - 1
}

// quantisation is the centre and the scale that turn a piece's positions into 16-bit integers.
func quantisation(positions []float32) ([3]float64, float64) {
	minimum, maximum := boundsOf(positions)
	var centre [3]float64
	reach := 0.0
	for axis := range 3 {
		centre[axis] = (float64(minimum[axis]) + float64(maximum[axis])) / 2
		reach = math.Max(reach, (float64(maximum[axis])-float64(minimum[axis]))/2)
	}
	if reach == 0 {
		reach = 1
	}
	return centre, reach / largestShort
}

func (output *writer) positionAccessor(positions []float32, centre [3]float64, scale float64) int {
	data := make([]byte, 0, 8*len(positions)/3)
	minimum := []float32{largestShort, largestShort, largestShort}
	maximum := []float32{-largestShort, -largestShort, -largestShort}
	for index, value := range positions {
		axis := index % 3
		whole := int16(math.Round((float64(value) - centre[axis]) / scale))
		minimum[axis], maximum[axis] = min(minimum[axis], float32(whole)), max(maximum[axis], float32(whole))
		data = binary.LittleEndian.AppendUint16(data, uint16(whole))
		if axis == 2 {
			data = append(data, 0, 0) // vertex attributes are aligned to 4 bytes
		}
	}
	view := output.stridedView(data, targetVertices, 8)
	return output.accessor(accessorEntry{BufferView: view, ComponentType: componentShort, Count: len(positions) / 3, Type: "VEC3",
		Minimum: minimum, Maximum: maximum})
}

func (output *writer) normalAccessor(normals []float32) int {
	data := make([]byte, 0, 4*len(normals)/3)
	for index, value := range normals {
		data = append(data, byte(int8(math.Round(math.Max(-1, math.Min(1, float64(value)))*127))))
		if index%3 == 2 {
			data = append(data, 0)
		}
	}
	view := output.stridedView(data, targetVertices, 4)
	return output.accessor(accessorEntry{BufferView: view, ComponentType: componentByte, Normalized: true, Count: len(normals) / 3, Type: "VEC3"})
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
	largest := uint32(0)
	for _, value := range indices {
		largest = max(largest, value)
	}
	if largest <= math.MaxUint16 {
		data := make([]byte, 0, 2*len(indices))
		for _, value := range indices {
			data = binary.LittleEndian.AppendUint16(data, uint16(value))
		}
		view := output.view(data, targetIndices)
		return output.accessor(accessorEntry{BufferView: view, ComponentType: componentUnsignedShort, Count: len(indices), Type: "SCALAR"})
	}
	data := make([]byte, 0, 4*len(indices))
	for _, value := range indices {
		data = binary.LittleEndian.AppendUint32(data, value)
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

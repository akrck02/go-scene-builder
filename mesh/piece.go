// Package mesh holds what the builder produces: pieces of geometry (positions, normals, colours
// and indices) with a material, in scene coordinates: kilometres, x to the east, y up, z to the
// south, centred on an origin.
package mesh

import "math"

// Vector is a point or direction in scene coordinates.
type Vector [3]float64

func (vector Vector) Subtract(other Vector) Vector {
	return Vector{vector[0] - other[0], vector[1] - other[1], vector[2] - other[2]}
}

func (vector Vector) Add(other Vector) Vector {
	return Vector{vector[0] + other[0], vector[1] + other[1], vector[2] + other[2]}
}

func (vector Vector) Scale(factor float64) Vector {
	return Vector{vector[0] * factor, vector[1] * factor, vector[2] * factor}
}

func (vector Vector) Dot(other Vector) float64 {
	return vector[0]*other[0] + vector[1]*other[1] + vector[2]*other[2]
}

func (vector Vector) Cross(other Vector) Vector {
	return Vector{vector[1]*other[2] - vector[2]*other[1], vector[2]*other[0] - vector[0]*other[2], vector[0]*other[1] - vector[1]*other[0]}
}

func (vector Vector) Normalized() Vector {
	length := math.Sqrt(vector.Dot(vector))
	if length == 0 {
		return Vector{0, 1, 0}
	}
	return vector.Scale(1 / length)
}

// Frame turns map positions (km east and north, height in metres) into scene coordinates.
type Frame struct {
	OriginEast, OriginNorth float64
}

// Scene is a map position in scene coordinates.
func (frame Frame) Scene(east, north, heightMetres float64) Vector {
	return Vector{east - frame.OriginEast, heightMetres / 1000, frame.OriginNorth - north}
}

// SceneDirection is a horizontal map direction (east, north) in scene coordinates.
func SceneDirection(east, north, up float64) Vector {
	return Vector{east, up, -north}
}

// Material is how a piece is drawn.
type Material struct {
	Name         string
	Kind         string // "lit", "glow" (shines in the dark) or "water", as the renderer understands them
	DoubleSided  bool
	DepthBias    float64 // the renderer draws it a little nearer, so flat ground pieces do not flicker
	Pattern      string  // procedural surface pattern: cobbles, masonry, plaster, shingles, planks, grass
	PatternScale float64 // metres per pattern cell (a stone, a course of blocks, a row of shingles)
	Foliage      bool    // leaves: takes the colour of the season
	HideBelow    float64 // the piece is not drawn when it looks smaller than this share of the screen; 0 = always
}

// Piece is a mesh with one material.
type Piece struct {
	Name      string
	Material  Material
	Ground    bool // walkable
	Positions []float32
	Normals   []float32
	Colors    []float32 // linear RGBA
	Indices   []uint32
	Details   []Detail // simpler versions for when the piece looks small, finer first
}

// Detail is a simpler version of a piece, drawn when the piece looks smaller than ScreenSize (a
// share of the screen height); a nil Piece draws nothing.
type Detail struct {
	Piece      *Piece
	ScreenSize float64
}

// VertexCount is the number of vertices of the piece.
func (piece *Piece) VertexCount() int { return len(piece.Positions) / 3 }

// TriangleCount is the number of triangles of the piece.
func (piece *Piece) TriangleCount() int { return len(piece.Indices) / 3 }

// AddVertex appends a vertex and returns its index.
func (piece *Piece) AddVertex(position, normal Vector, color [4]float32) uint32 {
	index := uint32(piece.VertexCount())
	piece.Positions = append(piece.Positions, float32(position[0]), float32(position[1]), float32(position[2]))
	piece.Normals = append(piece.Normals, float32(normal[0]), float32(normal[1]), float32(normal[2]))
	piece.Colors = append(piece.Colors, color[:]...)
	return index
}

// AddTriangle appends a flat triangle facing towards outward (any vector on the side it faces).
func (piece *Piece) AddTriangle(first, second, third, outward Vector, color [4]float32) {
	normal := second.Subtract(first).Cross(third.Subtract(first))
	if normal.Dot(outward) < 0 {
		second, third = third, second
		normal = normal.Scale(-1)
	}
	normal = normal.Normalized()
	for _, corner := range []Vector{first, second, third} {
		piece.Indices = append(piece.Indices, piece.AddVertex(corner, normal, color))
	}
}

// AddQuad appends a flat convex quadrilateral (corners in order) facing towards outward: four
// vertices shared by its two triangles.
func (piece *Piece) AddQuad(first, second, third, fourth, outward Vector, color [4]float32) {
	normal := second.Subtract(first).Cross(third.Subtract(first))
	if normal.Dot(normal) < 1e-30 {
		normal = third.Subtract(first).Cross(fourth.Subtract(first))
	}
	if normal.Dot(normal) < 1e-30 {
		return // no area
	}
	if normal.Dot(outward) < 0 {
		second, fourth = fourth, second
		normal = normal.Scale(-1)
	}
	normal = normal.Normalized()
	a, b := piece.AddVertex(first, normal, color), piece.AddVertex(second, normal, color)
	c, d := piece.AddVertex(third, normal, color), piece.AddVertex(fourth, normal, color)
	piece.Indices = append(piece.Indices, a, b, c, a, c, d)
}

// Append adds another piece's geometry to this one.
func (piece *Piece) Append(other *Piece) {
	offset := uint32(piece.VertexCount())
	piece.Positions = append(piece.Positions, other.Positions...)
	piece.Normals = append(piece.Normals, other.Normals...)
	piece.Colors = append(piece.Colors, other.Colors...)
	for _, index := range other.Indices {
		piece.Indices = append(piece.Indices, index+offset)
	}
}

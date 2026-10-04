package models

import (
	"math"
	"math/rand/v2"

	"github.com/akrck02/go-scene-builder/mesh"
)

var upwards = mesh.Vector{0, 1, 0}

// addBox adds a box standing on its placement: size [x, y, z] in metres.
func addBox(piece *mesh.Piece, place placement, size [3]float64, color [4]float32) {
	halfX, halfZ, height := size[0]/2, size[2]/2, size[1]
	corner := func(x, y, z float64) mesh.Vector { return place.point(x, y, z) }
	centre := place.point(0, height/2, 0)
	faces := [][4]mesh.Vector{
		{corner(-halfX, 0, halfZ), corner(halfX, 0, halfZ), corner(halfX, height, halfZ), corner(-halfX, height, halfZ)},
		{corner(halfX, 0, -halfZ), corner(-halfX, 0, -halfZ), corner(-halfX, height, -halfZ), corner(halfX, height, -halfZ)},
		{corner(halfX, 0, halfZ), corner(halfX, 0, -halfZ), corner(halfX, height, -halfZ), corner(halfX, height, halfZ)},
		{corner(-halfX, 0, -halfZ), corner(-halfX, 0, halfZ), corner(-halfX, height, halfZ), corner(-halfX, height, -halfZ)},
		{corner(-halfX, height, halfZ), corner(halfX, height, halfZ), corner(halfX, height, -halfZ), corner(-halfX, height, -halfZ)},
		{corner(-halfX, 0, -halfZ), corner(halfX, 0, -halfZ), corner(halfX, 0, halfZ), corner(-halfX, 0, halfZ)},
	}
	for _, face := range faces {
		middle := face[0].Add(face[1]).Add(face[2]).Add(face[3]).Scale(0.25)
		piece.AddQuad(face[0], face[1], face[2], face[3], middle.Subtract(centre), color)
	}
}

// addCylinder adds a cylinder (or a cone, with top 0) standing on its placement.
func addCylinder(piece *mesh.Piece, place placement, bottom, top, height float64, sides int, color [4]float32) {
	sides = max(3, sides)
	ring := func(radius, y float64) []mesh.Vector {
		points := make([]mesh.Vector, sides)
		for side := range sides {
			angle := 2 * math.Pi * float64(side) / float64(sides)
			points[side] = place.point(radius*math.Cos(angle), y, radius*math.Sin(angle))
		}
		return points
	}
	low, high := ring(bottom, 0), ring(top, height)
	axisLow, axisHigh := place.point(0, 0, 0), place.point(0, height, 0)
	for side := range sides {
		next := (side + 1) % sides
		middle := low[side].Add(low[next]).Scale(0.5).Subtract(axisLow)
		if top > 0 {
			piece.AddQuad(low[side], low[next], high[next], high[side], middle, color)
			piece.AddTriangle(axisHigh, high[side], high[next], place.direction(0, 1, 0), color)
		} else {
			piece.AddTriangle(low[side], low[next], axisHigh, middle, color)
		}
		piece.AddTriangle(axisLow, low[next], low[side], place.direction(0, -1, 0), color)
	}
}

// blobShape is an ellipsoid standing on its placement: its size in metres, how finely it is cut, how
// far it bends out of round (a share of its size) and whether it is shaded smooth or in facets.
type blobShape struct {
	size         [3]float64
	sides, rings int
	wobble       float64
	seed         uint64
	smooth       bool
}

// addBlob adds an ellipsoid (a tree crown, a bush, a flower bud).
func addBlob(piece *mesh.Piece, place placement, shape blobShape, color [4]float32) {
	sides, rings := max(4, shape.sides), max(2, shape.rings)
	bend := newBend(shape.seed)
	grid := make([][]mesh.Vector, rings+1)
	for ring := range rings + 1 {
		grid[ring] = make([]mesh.Vector, sides)
		latitude := math.Pi * (float64(ring)/float64(rings) - 0.5)
		for side := range sides {
			longitude := 2 * math.Pi * float64(side) / float64(sides)
			direction := [3]float64{math.Cos(latitude) * math.Cos(longitude), math.Sin(latitude), math.Cos(latitude) * math.Sin(longitude)}
			radius := 1 + shape.wobble*bend.at(direction)
			grid[ring][side] = place.point(shape.size[0]/2*radius*direction[0], shape.size[1]/2*(1+radius*direction[1]),
				shape.size[2]/2*radius*direction[2])
		}
	}
	centre := place.point(0, shape.size[1]/2, 0)
	if shape.smooth {
		addSmoothGrid(piece, grid, centre, color)
		return
	}
	for ring := range rings {
		for side := range sides {
			next := (side + 1) % sides
			first, second, third, fourth := grid[ring][side], grid[ring][next], grid[ring+1][next], grid[ring+1][side]
			outward := first.Add(second).Add(third).Add(fourth).Scale(0.25).Subtract(centre)
			switch ring {
			case 0:
				piece.AddTriangle(first, third, fourth, outward, color)
			case rings - 1:
				piece.AddTriangle(first, second, third, outward, color)
			default:
				piece.AddQuad(first, second, third, fourth, outward, color)
			}
		}
	}
}

// addSmoothGrid adds a closed surface given as rings of points from the bottom pole to the top one,
// with a normal at every point from its neighbours, so it is shaded round.
func addSmoothGrid(piece *mesh.Piece, grid [][]mesh.Vector, centre mesh.Vector, color [4]float32) {
	rings, sides := len(grid)-1, len(grid[0])
	indices := make([][]uint32, rings+1)
	for ring := range rings + 1 {
		indices[ring] = make([]uint32, sides)
		for side := range sides {
			indices[ring][side] = piece.AddVertex(grid[ring][side], smoothNormal(grid, ring, side, centre), color)
		}
	}
	for ring := range rings {
		for side := range sides {
			next := (side + 1) % sides
			if ring > 0 {
				addFacing(piece, indices[ring][side], indices[ring][next], indices[ring+1][next], centre)
			}
			if ring < rings-1 {
				addFacing(piece, indices[ring][side], indices[ring+1][next], indices[ring+1][side], centre)
			}
		}
	}
}

// smoothNormal is the normal at a point of a grid of rings, across its neighbours; at the poles,
// straight down or up.
func smoothNormal(grid [][]mesh.Vector, ring, side int, centre mesh.Vector) mesh.Vector {
	rings, sides := len(grid)-1, len(grid[0])
	var normal mesh.Vector
	switch ring {
	case 0:
		normal = mesh.Vector{0, -1, 0}
	case rings:
		normal = upwards
	default:
		around := grid[ring][(side+1)%sides].Subtract(grid[ring][(side+sides-1)%sides])
		along := grid[ring+1][side].Subtract(grid[ring-1][side])
		normal = around.Cross(along)
	}
	if normal.Dot(grid[ring][side].Subtract(centre)) < 0 {
		normal = normal.Scale(-1)
	}
	return normal.Normalized()
}

// addFacing adds a triangle of existing vertices, turned to face away from a centre.
func addFacing(piece *mesh.Piece, first, second, third uint32, centre mesh.Vector) {
	position := func(index uint32) mesh.Vector {
		return mesh.Vector{float64(piece.Positions[3*index]), float64(piece.Positions[3*index+1]), float64(piece.Positions[3*index+2])}
	}
	a, b, c := position(first), position(second), position(third)
	normal := b.Subtract(a).Cross(c.Subtract(a))
	if normal.Dot(a.Add(b).Add(c).Scale(1.0/3).Subtract(centre)) < 0 {
		second, third = third, second
	}
	piece.Indices = append(piece.Indices, first, second, third)
}

// bend is a smooth, repeatable bending of a sphere: a few waves in random directions.
type bend struct {
	directions          [4][3]float64
	frequencies, phases [4]float64
}

func newBend(seed uint64) bend {
	random := rand.New(rand.NewPCG(seed, 5))
	var result bend
	for wave := range result.directions {
		z, angle := 2*random.Float64()-1, 2*math.Pi*random.Float64()
		across := math.Sqrt(1 - z*z)
		result.directions[wave] = [3]float64{across * math.Cos(angle), across * math.Sin(angle), z}
		result.frequencies[wave] = 1.5 + 2.5*random.Float64()
		result.phases[wave] = 2 * math.Pi * random.Float64()
	}
	return result
}

// at is the bending in a direction, about between -1 and 1.
func (current bend) at(direction [3]float64) float64 {
	total := 0.0
	for wave, towards := range current.directions {
		along := direction[0]*towards[0] + direction[1]*towards[1] + direction[2]*towards[2]
		total += math.Sin(current.frequencies[wave]*along + current.phases[wave])
	}
	return total / 2
}

// addPetals adds a ring of flat petals around the placement, tilted up from flat; both faces show.
func addPetals(piece *mesh.Piece, place placement, count int, length, width, tilt float64, color [4]float32) {
	count = max(1, count)
	for petal := range count {
		angle := 2 * math.Pi * float64(petal) / float64(count)
		along := [2]float64{math.Cos(angle), math.Sin(angle)}
		across := [2]float64{-along[1], along[0]}
		reach := func(distance, height float64) (float64, float64, float64) {
			return along[0] * distance, height, along[1] * distance
		}
		baseX, baseY, baseZ := reach(0, 0)
		tipX, tipY, tipZ := reach(length*math.Cos(tilt), length*math.Sin(tilt))
		middleX, middleY, middleZ := reach(length*0.5*math.Cos(tilt), length*0.5*math.Sin(tilt))
		base, tip := place.point(baseX, baseY, baseZ), place.point(tipX, tipY, tipZ)
		left := place.point(middleX+across[0]*width/2, middleY, middleZ+across[1]*width/2)
		right := place.point(middleX-across[0]*width/2, middleY, middleZ-across[1]*width/2)
		normal := mesh.Vector{-along[0] * math.Sin(tilt), math.Cos(tilt), -along[1] * math.Sin(tilt)}
		piece.AddQuad(base, right, tip, left, normal, color)
		piece.AddQuad(base, left, tip, right, normal.Scale(-1), color)
	}
}

// ogiveOutline is the outline of a pointed (equilateral) arch standing on the ground, from the
// bottom left round to the bottom right: straight sides of "straight" metres, then two arcs of radius
// "width" that meet at the top; each arc is cut in "steps" pieces.
func ogiveOutline(width, straight float64, steps int) [][2]float64 {
	half := width / 2
	outline := [][2]float64{{-half, 0}, {-half, straight}}
	for step := 1; step < steps; step++ { // left arc: centre on the right side, from 180° to 120°
		angle := math.Pi - (math.Pi/3)*float64(step)/float64(steps)
		outline = append(outline, [2]float64{half + width*math.Cos(angle), straight + width*math.Sin(angle)})
	}
	outline = append(outline, [2]float64{0, straight + width*math.Sqrt(3)/2})
	for step := steps - 1; step >= 1; step-- { // right arc, mirrored
		angle := math.Pi - (math.Pi/3)*float64(step)/float64(steps)
		outline = append(outline, [2]float64{-(half + width*math.Cos(angle)), straight + width*math.Sin(angle)})
	}
	return append(outline, [2]float64{half, straight}, [2]float64{half, 0})
}

// addOgive adds a pointed arch "depth" metres thick (z). Solid (a door leaf, a window) when border is
// 0; otherwise a frame "border" metres wide around an opening of the same shape.
func addOgive(piece *mesh.Piece, place placement, width, straight, depth, border float64, steps int, color [4]float32) {
	steps = max(2, steps)
	outer := ogiveOutline(width, straight, steps)
	front, back := depth/2, -depth/2
	at := func(point [2]float64, z float64) mesh.Vector { return place.point(point[0], point[1], z) }
	centre := place.point(0, straight/2, 0)
	sides := func(outline [][2]float64, outwards float64) {
		for index := 0; index+1 < len(outline); index++ {
			first, second := outline[index], outline[index+1]
			middle := at([2]float64{(first[0] + second[0]) / 2, (first[1] + second[1]) / 2}, 0)
			piece.AddQuad(at(first, front), at(second, front), at(second, back), at(first, back), middle.Subtract(centre).Scale(outwards), color)
		}
	}
	if border <= 0 {
		middle := [2]float64{0, straight / 2}
		for index := 0; index+1 < len(outer); index++ {
			piece.AddTriangle(at(middle, front), at(outer[index], front), at(outer[index+1], front), place.direction(0, 0, 1), color)
			piece.AddTriangle(at(middle, back), at(outer[index+1], back), at(outer[index], back), place.direction(0, 0, -1), color)
		}
		sides(outer, 1)
		piece.AddQuad(at(outer[len(outer)-1], front), at(outer[0], front), at(outer[0], back), at(outer[len(outer)-1], back), place.direction(0, -1, 0), color)
		return
	}
	inner := ogiveOutline(width-2*border, straight, steps)
	for index := 0; index+1 < len(outer); index++ {
		piece.AddQuad(at(outer[index], front), at(outer[index+1], front), at(inner[index+1], front), at(inner[index], front), place.direction(0, 0, 1), color)
		piece.AddQuad(at(outer[index], back), at(inner[index], back), at(inner[index+1], back), at(outer[index+1], back), place.direction(0, 0, -1), color)
	}
	sides(outer, 1)
	sides(inner, -1)
	for _, jamb := range [][2][2]float64{{outer[0], inner[0]}, {inner[len(inner)-1], outer[len(outer)-1]}} {
		piece.AddQuad(at(jamb[0], front), at(jamb[1], front), at(jamb[1], back), at(jamb[0], back), place.direction(0, -1, 0), color)
	}
}

package models

import (
	"math"

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
			piece.AddTriangle(axisHigh, high[side], high[next], upwards, color)
		} else {
			piece.AddTriangle(low[side], low[next], axisHigh, middle, color)
		}
		piece.AddTriangle(axisLow, low[next], low[side], upwards.Scale(-1), color)
	}
}

// addBlob adds a faceted ellipsoid (size [x, y, z] in metres) standing on its placement.
func addBlob(piece *mesh.Piece, place placement, size [3]float64, sides int, color [4]float32) {
	sides = max(4, sides)
	const rings = 4
	point := func(ring, side int) mesh.Vector {
		latitude := math.Pi * (float64(ring)/rings - 0.5)
		longitude := 2 * math.Pi * float64(side) / float64(sides)
		return place.point(size[0]/2*math.Cos(latitude)*math.Cos(longitude), size[1]/2*(1+math.Sin(latitude)),
			size[2]/2*math.Cos(latitude)*math.Sin(longitude))
	}
	centre := place.point(0, size[1]/2, 0)
	for ring := range rings {
		for side := range sides {
			next := (side + 1) % sides
			first, second, third, fourth := point(ring, side), point(ring, next), point(ring+1, next), point(ring+1, side)
			outward := first.Add(second).Add(third).Add(fourth).Scale(0.25).Subtract(centre)
			if ring == 0 {
				piece.AddTriangle(first, third, fourth, outward, color)
			} else if ring == rings-1 {
				piece.AddTriangle(first, second, third, outward, color)
			} else {
				piece.AddQuad(first, second, third, fourth, outward, color)
			}
		}
	}
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

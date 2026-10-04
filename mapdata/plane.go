package mapdata

import "math"

// Distance is the distance between two points.
func Distance(first, second Point) float64 {
	return math.Hypot(second[0]-first[0], second[1]-first[1])
}

// Middle is the point halfway between two points.
func Middle(first, second Point) Point {
	return Point{(first[0] + second[0]) / 2, (first[1] + second[1]) / 2}
}

// Direction is the unit vector from one point to another (zero if they coincide).
func Direction(from, to Point) Point {
	length := Distance(from, to)
	if length == 0 {
		return Point{}
	}
	return Point{(to[0] - from[0]) / length, (to[1] - from[1]) / length}
}

// NearestOnSegment is the point of a segment nearest to a point.
func NearestOnSegment(from, to, point Point) Point {
	across, along := to[0]-from[0], to[1]-from[1]
	length := across*across + along*along
	if length == 0 {
		return from
	}
	share := math.Max(0, math.Min(1, ((point[0]-from[0])*across+(point[1]-from[1])*along)/length))
	return Point{from[0] + share*across, from[1] + share*along}
}

// SignedArea is positive when a ring turns anticlockwise.
func (ring Ring) SignedArea() float64 {
	area := 0.0
	for index := range ring {
		next := ring[(index+1)%len(ring)]
		area += ring[index][0]*next[1] - next[0]*ring[index][1]
	}
	return area / 2
}

// Anticlockwise returns the ring turning anticlockwise (reversed if needed).
func (ring Ring) Anticlockwise() Ring {
	if ring.SignedArea() >= 0 {
		return ring
	}
	reversed := make(Ring, len(ring))
	for index, point := range ring {
		reversed[len(ring)-1-index] = point
	}
	return reversed
}

// Outward is the unit normal pointing out of an anticlockwise ring along its side from → to.
func Outward(from, to Point) Point {
	direction := Direction(from, to)
	return Point{direction[1], -direction[0]}
}

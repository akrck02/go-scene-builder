package mapdata

import (
	"encoding/json"
	"fmt"
)

// addGeometry splits a GeoJSON geometry into the feature's points, lines and polygons.
func (convert conversion) addGeometry(feature *Feature, geometry geometryFile) error {
	switch geometry.Type {
	case "Point":
		return convert.addPoints(feature, geometry.Coordinates, false)
	case "MultiPoint":
		return convert.addPoints(feature, geometry.Coordinates, true)
	case "LineString":
		return convert.addLines(feature, geometry.Coordinates, false)
	case "MultiLineString":
		return convert.addLines(feature, geometry.Coordinates, true)
	case "Polygon":
		return convert.addPolygons(feature, geometry.Coordinates, false)
	case "MultiPolygon":
		return convert.addPolygons(feature, geometry.Coordinates, true)
	case "GeometryCollection":
		for _, part := range geometry.Geometries {
			if err := convert.addGeometry(feature, part); err != nil {
				return err
			}
		}
		return nil
	}
	return fmt.Errorf("unknown geometry %q", geometry.Type)
}

func (convert conversion) addPoints(feature *Feature, raw json.RawMessage, many bool) error {
	var points [][]float64
	if many {
		if err := json.Unmarshal(raw, &points); err != nil {
			return err
		}
	} else {
		var point []float64
		if err := json.Unmarshal(raw, &point); err != nil {
			return err
		}
		points = [][]float64{point}
	}
	for _, point := range points {
		if len(point) >= 2 {
			feature.Points = append(feature.Points, convert.point(point))
		}
	}
	return nil
}

func (convert conversion) addLines(feature *Feature, raw json.RawMessage, many bool) error {
	var lines [][][]float64
	if many {
		if err := json.Unmarshal(raw, &lines); err != nil {
			return err
		}
	} else {
		var line [][]float64
		if err := json.Unmarshal(raw, &line); err != nil {
			return err
		}
		lines = [][][]float64{line}
	}
	for _, line := range lines {
		if converted := convert.line(line); len(converted) >= 2 {
			feature.Lines = append(feature.Lines, converted)
		}
	}
	return nil
}

func (convert conversion) addPolygons(feature *Feature, raw json.RawMessage, many bool) error {
	var polygons [][][][]float64
	if many {
		if err := json.Unmarshal(raw, &polygons); err != nil {
			return err
		}
	} else {
		var polygon [][][]float64
		if err := json.Unmarshal(raw, &polygon); err != nil {
			return err
		}
		polygons = [][][][]float64{polygon}
	}
	for _, rings := range polygons {
		if polygon, valid := convert.polygon(rings); valid {
			feature.Polygons = append(feature.Polygons, polygon)
		}
	}
	return nil
}

func (convert conversion) line(coordinates [][]float64) []Point {
	points := make([]Point, 0, len(coordinates))
	for _, coordinate := range coordinates {
		if len(coordinate) >= 2 {
			points = append(points, convert.point(coordinate))
		}
	}
	return points
}

// polygon reads the rings of a polygon, dropping the repeated last point of each ring.
func (convert conversion) polygon(rings [][][]float64) (Polygon, bool) {
	var polygon Polygon
	for index, coordinates := range rings {
		ring := Ring(convert.line(coordinates))
		if len(ring) > 1 && ring[0] == ring[len(ring)-1] {
			ring = ring[:len(ring)-1]
		}
		if len(ring) < 3 {
			if index == 0 {
				return polygon, false
			}
			continue
		}
		if index == 0 {
			polygon.Exterior = ring
		} else {
			polygon.Holes = append(polygon.Holes, ring)
		}
	}
	return polygon, true
}

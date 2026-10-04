// Package mapdata reads a 2D map: a GeoJSON FeatureCollection whose features carry an "id" and a
// "kind" in their properties. Coordinates are turned into kilometres, east and north, whatever the
// file declares in its collection properties ("units": "km" | "m"; "axes": "east-north" |
// "east-south"; "origin": the point the scene is centred on, in the same units and axes).
package mapdata

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

// Point is a position in kilometres: east, north.
type Point [2]float64

// Ring is a closed line, without repeating the first point.
type Ring []Point

// Polygon is an exterior ring with optional holes.
type Polygon struct {
	Exterior Ring
	Holes    []Ring
}

// Feature is one shape of the map, split into its points, lines and polygons.
type Feature struct {
	Identifier string
	Kind       string
	Properties map[string]any
	Points     []Point
	Lines      [][]Point
	Polygons   []Polygon
}

// Map is the whole map, in kilometres east and north.
type Map struct {
	Features []*Feature
	Origin   *Point   // where the scene is centred; nil = let the builder choose
	Warnings []string // features without id or kind, geometries that were skipped
}

type collectionFile struct {
	Properties map[string]any `json:"properties"`
	Features   []featureFile  `json:"features"`
}

type featureFile struct {
	Properties map[string]any `json:"properties"`
	Geometry   *geometryFile  `json:"geometry"`
}

type geometryFile struct {
	Type        string          `json:"type"`
	Coordinates json.RawMessage `json:"coordinates"`
	Geometries  []geometryFile  `json:"geometries"`
}

// Load reads a map from a GeoJSON file.
func Load(path string) (*Map, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(content)
}

// Parse reads a map from GeoJSON text.
func Parse(content []byte) (*Map, error) {
	var collection collectionFile
	if err := json.Unmarshal(content, &collection); err != nil {
		return nil, fmt.Errorf("map: %w", err)
	}
	conversion, err := conversionOf(collection.Properties)
	if err != nil {
		return nil, err
	}
	result := &Map{Origin: conversion.origin(collection.Properties)}
	for index, file := range collection.Features {
		feature := result.featureFrom(index, file)
		if file.Geometry != nil {
			if err := conversion.addGeometry(feature, *file.Geometry); err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("feature %s: %v", feature.Identifier, err))
			}
		}
		result.Features = append(result.Features, feature)
	}
	return result, nil
}

func (result *Map) featureFrom(index int, file featureFile) *Feature {
	properties := file.Properties
	if properties == nil {
		properties = map[string]any{}
	}
	feature := &Feature{Identifier: identifierOf(properties["id"]), Properties: properties}
	feature.Kind, _ = properties["kind"].(string)
	if feature.Identifier == "" {
		feature.Identifier = "feature-" + strconv.Itoa(index)
		result.Warnings = append(result.Warnings, fmt.Sprintf("feature %d has no id", index))
	}
	if feature.Kind == "" {
		result.Warnings = append(result.Warnings, fmt.Sprintf("feature %s has no kind", feature.Identifier))
	}
	return feature
}

func identifierOf(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	}
	return ""
}

// conversion turns file coordinates into kilometres east and north.
type conversion struct {
	scale      float64
	northSign  float64
}

func conversionOf(properties map[string]any) (conversion, error) {
	result := conversion{scale: 1, northSign: 1}
	switch units, _ := properties["units"].(string); units {
	case "", "km":
	case "m":
		result.scale = 0.001
	default:
		return result, fmt.Errorf("map: unknown units %q", units)
	}
	switch axes, _ := properties["axes"].(string); axes {
	case "", "east-north":
	case "east-south":
		result.northSign = -1
	default:
		return result, fmt.Errorf("map: unknown axes %q", axes)
	}
	return result, nil
}

func (convert conversion) point(coordinates []float64) Point {
	return Point{coordinates[0] * convert.scale, coordinates[1] * convert.scale * convert.northSign}
}

func (convert conversion) origin(properties map[string]any) *Point {
	values, _ := properties["origin"].([]any)
	if len(values) != 2 {
		return nil
	}
	east, eastIsNumber := values[0].(float64)
	north, northIsNumber := values[1].(float64)
	if !eastIsNumber || !northIsNumber {
		return nil
	}
	point := convert.point([]float64{east, north})
	return &point
}

package mapdata

import "testing"

func TestParseConvertsUnitsAndAxes(test *testing.T) {
	content := []byte(`{"type": "FeatureCollection", "properties": {"units": "m", "axes": "east-south", "origin": [1000, -2000]},
	 "features": [
	  {"type": "Feature", "properties": {"id": 7, "kind": "building", "floors": 2},
	   "geometry": {"type": "Polygon", "coordinates": [[[0,0],[1000,0],[1000,1000],[0,0]]]}},
	  {"type": "Feature", "properties": {"kind": "street"},
	   "geometry": {"type": "MultiLineString", "coordinates": [[[0,0],[500,0]], [[0,0],[0,500]]]}}]}`)
	parsed, err := Parse(content)
	if err != nil {
		test.Fatal(err)
	}
	building := parsed.Features[0]
	if building.Identifier != "7" || building.Kind != "building" || len(building.Polygons) != 1 {
		test.Fatalf("building read as %+v", building)
	}
	if ring := building.Polygons[0].Exterior; len(ring) != 3 || ring[2] != (Point{1, -1}) {
		test.Errorf("ring should be in km, north up, without the repeated point: %v", ring)
	}
	if *parsed.Origin != (Point{1, 2}) {
		test.Errorf("origin %v", *parsed.Origin)
	}
	if street := parsed.Features[1]; len(street.Lines) != 2 || street.Identifier != "feature-1" {
		test.Errorf("street read as %+v", street)
	}
	if len(parsed.Warnings) != 1 {
		test.Errorf("one warning expected (street without id): %v", parsed.Warnings)
	}
}

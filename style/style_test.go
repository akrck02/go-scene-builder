package style

import (
	"math"
	"testing"
)

const example = `
units: m
rules:
  - when: {kind: building}
    shape: {role: volume, height: "3.5 + (floors - 1) * 3"}
    roof: {color: "#a4553a", pitch: 38}
  - when: {kind: building, use: [shop, workshop]}
    roof: {color: "#6f5a4a"}
  - when: {kind: building, floors: {min: 3}}
    roof: {pitch: 30}
`

func TestCascade(test *testing.T) {
	rules, err := Parse([]byte(example))
	if err != nil {
		test.Fatal(err)
	}
	resolved, matched := rules.Resolve("building", map[string]any{"use": "workshop", "floors": 3.0})
	if !matched {
		test.Fatal("no rule matched")
	}
	roof := resolved.Section("roof")
	if roof.Text("color", "") != "#6f5a4a" {
		test.Errorf("later rule should replace the colour, got %v", roof["color"])
	}
	if pitch, _ := roof.Number("pitch", nil, 0); pitch != 30 {
		test.Errorf("range rule should set the pitch to 30, got %v", pitch)
	}
	if height, err := resolved.Section("shape").Number("height", map[string]any{"floors": 3.0}, 0); err != nil || height != 9.5 {
		test.Errorf("height = %v, %v; want 9.5", height, err)
	}
	if _, matched := rules.Resolve("street", nil); matched {
		test.Error("a street should match no rule")
	}
}

func TestExpressions(test *testing.T) {
	cases := map[string]float64{"2 + 3 * 4": 14, "(2 + 3) * 4": 20, "-width / 2": -1.5, "width*1000": 3000}
	for expression, want := range cases {
		got, err := Evaluate(expression, map[string]any{"width": 3.0})
		if err != nil || math.Abs(got-want) > 1e-9 {
			test.Errorf("%s = %v, %v; want %v", expression, got, err, want)
		}
	}
	for _, wrong := range []string{"2 +", "missing * 2", "(1", "1 / 0"} {
		if _, err := Evaluate(wrong, map[string]any{}); err == nil {
			test.Errorf("%q should fail", wrong)
		}
	}
}

func TestLinearColor(test *testing.T) {
	color, err := LinearColor("#ffffff")
	if err != nil || color != [4]float32{1, 1, 1, 1} {
		test.Errorf("white = %v, %v", color, err)
	}
	if _, err := LinearColor("red"); err == nil {
		test.Error("a name is not a colour")
	}
}

func TestColorFromAProperty(test *testing.T) {
	section := Style{"color": "@color"}
	color, err := section.Color("color", map[string]any{"color": "#ffffff"}, [4]float32{})
	if err != nil || color != [4]float32{1, 1, 1, 1} {
		test.Errorf("colour from the property: %v, %v", color, err)
	}
	if color, _ := section.Color("color", nil, [4]float32{0, 0, 0, 1}); color != [4]float32{0, 0, 0, 1} {
		test.Errorf("without the property, the fallback: %v", color)
	}
}

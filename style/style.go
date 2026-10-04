package style

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Style is the merged values of the rules that match one feature.
type Style map[string]any

// Section returns a nested section (empty if missing).
func (style Style) Section(name string) Style {
	section, _ := style[name].(map[string]any)
	return Style(section)
}

// Text returns a text value.
func (style Style) Text(key, fallback string) string {
	if text, isText := style[key].(string); isText {
		return text
	}
	return fallback
}

// Number returns a numeric value: a number, or an expression over the feature's properties.
func (style Style) Number(key string, properties map[string]any, fallback float64) (float64, error) {
	value, present := style[key]
	if !present {
		return fallback, nil
	}
	if number, isNumber := numberOf(value); isNumber {
		return number, nil
	}
	if expression, isText := value.(string); isText {
		return Evaluate(expression, properties)
	}
	return 0, fmt.Errorf("%s: not a number: %v", key, value)
}

// Color returns a colour written as "#rrggbb" (sRGB), in the linear RGBA that glTF uses.
func (style Style) Color(key string, fallback [4]float32) ([4]float32, error) {
	text, isText := style[key].(string)
	if !isText {
		return fallback, nil
	}
	return LinearColor(text)
}

// LinearColor turns "#rrggbb" or "#rrggbbaa" (sRGB) into linear RGBA.
func LinearColor(text string) ([4]float32, error) {
	hex := strings.TrimPrefix(text, "#")
	if len(hex) != 6 && len(hex) != 8 {
		return [4]float32{}, fmt.Errorf("colour %q is not #rrggbb", text)
	}
	color := [4]float32{0, 0, 0, 1}
	for channel := 0; channel < len(hex)/2; channel++ {
		value, err := strconv.ParseUint(hex[2*channel:2*channel+2], 16, 8)
		if err != nil {
			return color, fmt.Errorf("colour %q: %w", text, err)
		}
		if channel < 3 {
			color[channel] = float32(srgbToLinear(float64(value) / 255))
		} else {
			color[channel] = float32(value) / 255
		}
	}
	return color, nil
}

func srgbToLinear(value float64) float64 {
	if value <= 0.04045 {
		return value / 12.92
	}
	return math.Pow((value+0.055)/1.055, 2.4)
}

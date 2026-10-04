package build

import "math"

// Environment is the scene extras the renderer reads: the sun from the style's "environment.sun"
// ([azimuth, elevation] in degrees, azimuth clockwise from north), fog from the size of the
// scene, a sea only when "environment.water_level" (metres) is given, and true heights.
func (result *Result) Environment(settings map[string]any, extentKilometres float64) map[string]any {
	azimuth, elevation := 235.0, 38.0
	if sun, isList := settings["sun"].([]any); isList && len(sun) == 2 {
		azimuth, _ = toNumber(sun[0])
		elevation, _ = toNumber(sun[1])
	}
	azimuth, elevation = azimuth*math.Pi/180, elevation*math.Pi/180
	environment := map[string]any{
		"sunDirection":  []float64{math.Sin(azimuth) * math.Cos(elevation), math.Sin(elevation), -math.Cos(azimuth) * math.Cos(elevation)},
		"fog":           map[string]any{"near": extentKilometres * 1.1, "far": extentKilometres * 3},
		"verticalScale": 1,
	}
	if level, present := toNumber(settings["water_level"]); present {
		environment["water"] = map[string]any{"level": level / 1000, "size": extentKilometres * 14}
	}
	return environment
}

func toNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	}
	return 0, false
}

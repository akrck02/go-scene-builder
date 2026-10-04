package style

import "fmt"

// selects tells whether every condition of a selector holds for a feature. Conditions:
//
//	kind: building                 the feature's kind
//	use: workshop                  a property equal to a value
//	use: [shop, workshop]          a property equal to one of the values
//	floors: {min: 2, max: 3}       a numeric property within a range
func selects(when map[string]any, kind string, properties map[string]any) bool {
	for key, condition := range when {
		var value any = kind
		if key != "kind" {
			var present bool
			value, present = properties[key]
			if !present {
				return false
			}
		}
		if !holds(condition, value) {
			return false
		}
	}
	return true
}

func holds(condition, value any) bool {
	switch typed := condition.(type) {
	case []any:
		for _, option := range typed {
			if equal(option, value) {
				return true
			}
		}
		return false
	case map[string]any:
		return inRange(typed, value)
	}
	return equal(condition, value)
}

func inRange(bounds map[string]any, value any) bool {
	number, isNumber := numberOf(value)
	if !isNumber {
		return false
	}
	if minimum, present := numberOf(bounds["min"]); present && number < minimum {
		return false
	}
	if maximum, present := numberOf(bounds["max"]); present && number > maximum {
		return false
	}
	return true
}

// equal compares a value written in the rules with a feature's: numbers by value whatever their
// type (YAML gives int, JSON float64), the rest by their text.
func equal(written, actual any) bool {
	first, firstIsNumber := numberOf(written)
	second, secondIsNumber := numberOf(actual)
	if firstIsNumber && secondIsNumber {
		return first == second
	}
	return fmt.Sprint(written) == fmt.Sprint(actual)
}

func numberOf(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	}
	return 0, false
}

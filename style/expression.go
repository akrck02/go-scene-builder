package style

import (
	"fmt"
	"strconv"
	"unicode"
)

// Evaluate computes a simple arithmetic expression over a feature's numeric properties:
// numbers, property names, + - * /, parentheses and a leading minus. Nothing else, so the rules
// stay readable.
func Evaluate(expression string, properties map[string]any) (float64, error) {
	parser := &expressionParser{text: []rune(expression), properties: properties}
	value, err := parser.sum()
	if err != nil {
		return 0, fmt.Errorf("%q: %w", expression, err)
	}
	parser.skipSpaces()
	if parser.position < len(parser.text) {
		return 0, fmt.Errorf("%q: unexpected %q", expression, string(parser.text[parser.position:]))
	}
	return value, nil
}

type expressionParser struct {
	text       []rune
	position   int
	properties map[string]any
}

func (parser *expressionParser) skipSpaces() {
	for parser.position < len(parser.text) && unicode.IsSpace(parser.text[parser.position]) {
		parser.position++
	}
}

// next returns the next symbol without consuming it (0 at the end).
func (parser *expressionParser) next() rune {
	parser.skipSpaces()
	if parser.position >= len(parser.text) {
		return 0
	}
	return parser.text[parser.position]
}

func (parser *expressionParser) sum() (float64, error) {
	value, err := parser.product()
	for err == nil {
		operator := parser.next()
		if operator != '+' && operator != '-' {
			return value, nil
		}
		parser.position++
		var right float64
		if right, err = parser.product(); operator == '+' {
			value += right
		} else {
			value -= right
		}
	}
	return 0, err
}

func (parser *expressionParser) product() (float64, error) {
	value, err := parser.factor()
	for err == nil {
		operator := parser.next()
		if operator != '*' && operator != '/' {
			return value, nil
		}
		parser.position++
		var right float64
		if right, err = parser.factor(); operator == '*' {
			value *= right
		} else if right == 0 {
			return 0, fmt.Errorf("division by zero")
		} else {
			value /= right
		}
	}
	return 0, err
}

func (parser *expressionParser) factor() (float64, error) {
	switch symbol := parser.next(); {
	case symbol == '-':
		parser.position++
		value, err := parser.factor()
		return -value, err
	case symbol == '(':
		parser.position++
		value, err := parser.sum()
		if err == nil && parser.next() != ')' {
			return 0, fmt.Errorf("missing )")
		}
		parser.position++
		return value, err
	case unicode.IsDigit(symbol) || symbol == '.':
		return parser.number()
	case unicode.IsLetter(symbol) || symbol == '_':
		return parser.property()
	case symbol == 0:
		return 0, fmt.Errorf("unexpected end")
	default:
		return 0, fmt.Errorf("unexpected %q", string(symbol))
	}
}

func (parser *expressionParser) number() (float64, error) {
	start := parser.position
	for parser.position < len(parser.text) && (unicode.IsDigit(parser.text[parser.position]) || parser.text[parser.position] == '.') {
		parser.position++
	}
	return strconv.ParseFloat(string(parser.text[start:parser.position]), 64)
}

func (parser *expressionParser) property() (float64, error) {
	start := parser.position
	for parser.position < len(parser.text) && (unicode.IsLetter(parser.text[parser.position]) || unicode.IsDigit(parser.text[parser.position]) || parser.text[parser.position] == '_') {
		parser.position++
	}
	name := string(parser.text[start:parser.position])
	value, isNumber := numberOf(parser.properties[name])
	if !isNumber {
		return 0, fmt.Errorf("property %q is not a number", name)
	}
	return value, nil
}

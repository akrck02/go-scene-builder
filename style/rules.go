// Package style reads the author's style rules: a YAML list of rules, each with a selector
// ("when") and the values it sets. For every feature the rules are applied in order, like a style
// sheet: each rule that matches adds or replaces values over the earlier ones, the last one wins.
//
//	units: m
//	environment: {sun: [235, 38]}
//	terrain: {color: "#6b7f47", bands: [{above: 350, color: "#9e9e99", blend: 30}]}
//	relations: {facing: {when: {kind: street}, reach: 20}, rows: {tolerance: 0.4}}
//	rules:
//	  - when: {kind: building}
//	    shape: {role: volume, height: "3.5 + (floors - 1) * 3"}
//	  - when: {kind: building, use: [shop, workshop], floors: {min: 2}}
//	    roof: {color: "#8e4631"}
package style

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Rules is a whole style file.
type Rules struct {
	Units       string         `yaml:"units"`
	Environment map[string]any `yaml:"environment"`
	Terrain     map[string]any `yaml:"terrain"`
	Relations   map[string]any `yaml:"relations"`
	Rules       []Rule         `yaml:"rules"`
}

// Rule is a selector and the values it sets.
type Rule struct {
	When   map[string]any
	Values map[string]any
}

// UnmarshalYAML splits a rule into its selector and its values.
func (rule *Rule) UnmarshalYAML(node *yaml.Node) error {
	var all map[string]any
	if err := node.Decode(&all); err != nil {
		return err
	}
	rule.When, _ = all["when"].(map[string]any)
	delete(all, "when")
	rule.Values = all
	return nil
}

// Load reads a style file.
func Load(path string) (*Rules, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(content)
}

// Parse reads style rules from YAML text.
func Parse(content []byte) (*Rules, error) {
	var rules Rules
	if err := yaml.Unmarshal(content, &rules); err != nil {
		return nil, fmt.Errorf("style: %w", err)
	}
	if rules.Units != "" && rules.Units != "m" {
		return nil, fmt.Errorf("style: units must be m (metres), not %q", rules.Units)
	}
	return &rules, nil
}

// Resolve is the style of a feature: the values of every matching rule, merged in order.
// matched is false when no rule matched.
func (rules *Rules) Resolve(kind string, properties map[string]any) (resolved Style, matched bool) {
	resolved = Style{}
	for _, rule := range rules.Rules {
		if selects(rule.When, kind, properties) {
			merge(resolved, rule.Values)
			matched = true
		}
	}
	return resolved, matched
}

// merge copies values into a style, merging nested sections instead of replacing them.
func merge(into map[string]any, values map[string]any) {
	for key, value := range values {
		section, isSection := value.(map[string]any)
		existing, hasSection := into[key].(map[string]any)
		if isSection && hasSection {
			merge(existing, section)
			continue
		}
		if isSection {
			copied := map[string]any{}
			merge(copied, section)
			into[key] = copied
			continue
		}
		into[key] = value
	}
}

package recipes

import (
	"fmt"

	"github.com/akrck02/go-scene-builder/mesh"
	"github.com/akrck02/go-scene-builder/style"
)

// Base materials of the recipes; the style's "material" sections change their kind and pattern.
var (
	WallMaterial = mesh.Material{Name: "walls", Kind: "lit"}
	RoofMaterial = mesh.Material{Name: "roofs", Kind: "lit"}
)

// MaterialOf is a base material with the "material" settings of a style section:
//
//	material: {pattern: cobbles, scale: 0.35}   # a procedural pattern, metres per cell
//	material: {kind: glow}                      # shines with its own colour in the dark
func MaterialOf(base mesh.Material, section style.Style) mesh.Material {
	settings := section.Section("material")
	if len(settings) == 0 {
		return base
	}
	material := base
	material.Kind = settings.Text("kind", base.Kind)
	material.Pattern = settings.Text("pattern", "")
	material.PatternScale, _ = settings.Number("scale", nil, 1)
	material.Name = fmt.Sprintf("%s %s %s %g", base.Name, material.Kind, material.Pattern, material.PatternScale)
	return material
}

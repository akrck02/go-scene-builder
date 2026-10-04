package models

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/akrck02/go-scene-builder/gltf"
	"github.com/akrck02/go-scene-builder/mesh"
)

const library = `
models:
  lily:
    parts:
      - {shape: cylinder, radius: 0.01, height: 0.35, color: "#4f6b35"}
      - {shape: petals, count: 6, length: 0.07, width: 0.035, tilt: 35, at: [0, 0.35, 0], color: "#7da7e0", material: {kind: glow}}
  banco:
    parts:
      - {shape: box, size: [1.6, 0.08, 0.4], at: [0, 0.42, 0], color: "#6b4a2e"}
      - {shape: blob, size: [0.2, 0.42, 0.2], at: [-0.6, 0, 0]}
      - {shape: cone, radius: 0.1, height: 0.3, at: [0.6, 0, 0]}
`

func TestModelsFromParts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.yaml")
	os.WriteFile(path, []byte(library), 0o644)
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	lily := loaded.Model("lily")
	if lily == nil || len(lily.Pieces) != 2 {
		t.Fatalf("the lily has a lit stem and glowing petals: %+v", lily)
	}
	if lily.Pieces[1].Material.Kind != "glow" {
		t.Errorf("petals should glow: %+v", lily.Pieces[1].Material)
	}
	top := 0.0
	for _, piece := range lily.Pieces {
		for index := 1; index < len(piece.Positions); index += 3 {
			top = math.Max(top, float64(piece.Positions[index]))
		}
	}
	// stem 0.35 m + petals rising 0.07 × sin 35°
	if want := (0.35 + 0.07*math.Sin(35*math.Pi/180)) / 1000; math.Abs(top-want) > 1e-6 {
		t.Errorf("the lily is %v km tall, want %v", top, want)
	}
	if loaded.Model("banco") == nil || loaded.Model("nothing") != nil {
		t.Error("the bench is in the library, nothing else")
	}
}

func TestModelsFromGLB(t *testing.T) {
	piece := &mesh.Piece{Name: "test", Material: mesh.Material{Name: "solid", Kind: "lit"}}
	piece.AddTriangle(mesh.Vector{0, 0, 0}, mesh.Vector{1, 0, 0}, mesh.Vector{0, 2, 0}, mesh.Vector{0, 0, 1}, [4]float32{1, 0, 0, 1})
	content, err := gltf.Encode([]*mesh.Piece{piece}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	folder := t.TempDir()
	os.WriteFile(filepath.Join(folder, "model.glb"), content, 0o644)
	os.WriteFile(filepath.Join(folder, "models.yaml"), []byte("models:\n  imported: {file: model.glb}\n"), 0o644)
	loaded, err := Load(filepath.Join(folder, "models.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	imported := loaded.Model("imported")
	if imported == nil || len(imported.Pieces) != 1 || imported.Pieces[0].TriangleCount() != 1 {
		t.Fatalf("one triangle expected: %+v", imported)
	}
	// the file is read in metres: 2 units up become 0.002 km
	if top := imported.Pieces[0].Positions[7]; math.Abs(float64(top)-0.002) > 1e-6 {
		t.Errorf("top vertex at %v, want 0.002", top)
	}
}

func TestModelLevels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.yaml")
	os.WriteFile(path, []byte(`
models:
  tree:
    parts:
      - {shape: cylinder, radius: 0.3, height: 4, color: "#5a4030"}
      - {shape: blob, size: [5, 5, 5], at: [0, 3, 0], color: "#56713a", material: {foliage: true}}
    levels:
      - {below: 0.05, parts: [{shape: blob, size: [5, 7, 5], color: "#56713a", material: {foliage: true}}]}
      - {below: 0.01}
`), 0o644)
	loaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	trunk, crown := loaded.Model("tree").Pieces[0], loaded.Model("tree").Pieces[1]
	if len(trunk.Details) != 2 || trunk.Details[0].Piece != nil {
		t.Errorf("far away the trunk disappears: %+v", trunk.Details)
	}
	if len(crown.Details) != 2 || crown.Details[0].Piece == nil || crown.Details[1].Piece != nil || !crown.Material.Foliage {
		t.Errorf("the crown becomes one blob, then nothing: %+v", crown.Details)
	}
}

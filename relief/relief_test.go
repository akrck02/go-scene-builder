package relief

import (
	"encoding/binary"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAndHeight(test *testing.T) {
	folder := test.TempDir()
	header := filepath.Join(folder, "relieve.json")
	os.WriteFile(header, []byte(`{"columns": 2, "rows": 2, "cell": 10, "west": 0, "north": 20, "units": "m"}`), 0o644)
	values := make([]byte, 16)
	for index, height := range []float32{0, 10, 20, 30} {
		binary.LittleEndian.PutUint32(values[4*index:], math.Float32bits(height))
	}
	os.WriteFile(filepath.Join(folder, "relieve.f32"), values, 0o644)
	grid, err := Load(header)
	if err != nil {
		test.Fatal(err)
	}
	if grid.Cell != 0.01 || grid.East() != 0.02 || grid.South() != 0 {
		test.Errorf("grid in km: %+v", grid)
	}
	east, north := grid.CellCentre(0, 0)
	if got := grid.Height(east, north); math.Abs(got) > 1e-9 {
		test.Errorf("height at the first centre = %v", got)
	}
	if got := grid.Height(0.01, 0.01); math.Abs(got-15) > 1e-6 {
		test.Errorf("height in the middle = %v, want 15", got)
	}
	var flat *Relief
	if flat.Height(5, 5) != 0 {
		test.Error("a nil relief is flat")
	}
}

// Command build-scene turns a 2D map (GeoJSON with id and kind), a relief and style rules into a
// 3D scene (.glb) that go-renderer's viewer opens.
//
//	build-scene -map city.geojson -relief relieve.json -style style.yaml -out city.glb
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime/pprof"
	"sort"
	"strings"
	"time"

	"github.com/akrck02/go-scene-builder/build"
	"github.com/akrck02/go-scene-builder/gltf"
	"github.com/akrck02/go-scene-builder/mapdata"
	"github.com/akrck02/go-scene-builder/models"
	"github.com/akrck02/go-scene-builder/relief"
	"github.com/akrck02/go-scene-builder/style"
)

func main() {
	mapPath := flag.String("map", "", "map: GeoJSON whose features have an id and a kind")
	reliefPath := flag.String("relief", "", "relief header (relieve.json, with relieve.f32 next to it); empty = flat")
	stylePath := flag.String("style", "", "style rules (YAML)")
	output := flag.String("out", "scene.glb", "scene to write")
	title := flag.String("title", "", "title of the scene")
	showWarnings := flag.Int("warnings", 10, "how many warnings to print")
	profile := flag.String("profile", "", "write a CPU profile to this file (go tool pprof)")
	var layers layerFlags
	flag.Var(&layers, "layer", "a grid of categories, name=path (biome=biomes.json colours the terrain and sows its vegetation); can repeat")
	flag.Parse()
	if *profile != "" {
		stop, err := startProfile(*profile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer stop()
	}
	if *mapPath == "" || *stylePath == "" {
		fmt.Fprintln(os.Stderr, "usage: build-scene -map map.geojson [-relief relieve.json] -style style.yaml [-out scene.glb]")
		os.Exit(2)
	}
	if err := run(*mapPath, *reliefPath, *stylePath, *output, *title, *showWarnings, layers); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func startProfile(path string) (func(), error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	if err := pprof.StartCPUProfile(file); err != nil {
		return nil, err
	}
	return func() { pprof.StopCPUProfile(); file.Close() }, nil
}

func run(mapPath, reliefPath, stylePath, output, title string, showWarnings int, layers layerFlags) error {
	clock := newClock()
	input, err := readInput(mapPath, reliefPath, stylePath)
	if err != nil {
		return err
	}
	input.Layers = map[string]*relief.Relief{}
	for name, path := range layers {
		if input.Layers[name], err = relief.Load(path); err != nil {
			return err
		}
	}
	clock.lap("read", fmt.Sprintf("%d features", len(input.Map.Features)))
	result := build.Build(input)
	clock.lap("build", describe(result))
	extent := 2.0
	if input.Relief != nil {
		extent = input.Relief.East() - input.Relief.West
	}
	extras := map[string]any{"environment": result.Environment(input.Rules.Environment, extent)}
	if title != "" {
		extras["title"] = title
	}
	size, err := gltf.Write(output, result.Pieces, result.Instances, extras)
	if err != nil {
		return err
	}
	clock.lap("write", fmt.Sprintf("%s, %.1f MB", output, float64(size)/1e6))
	clock.total()
	printWarnings(append(input.Map.Warnings, result.Warnings...), showWarnings)
	return nil
}

func readInput(mapPath, reliefPath, stylePath string) (build.Input, error) {
	var input build.Input
	var err error
	if input.Map, err = mapdata.Load(mapPath); err != nil {
		return input, err
	}
	if input.Rules, err = style.Load(stylePath); err != nil {
		return input, err
	}
	if input.Models, err = models.Load(input.Rules.ModelFiles()...); err != nil {
		return input, err
	}
	if reliefPath != "" {
		input.Relief, err = relief.Load(reliefPath)
	}
	return input, err
}

func describe(result *build.Result) string {
	triangles := 0
	for _, piece := range result.Pieces {
		triangles += piece.TriangleCount()
	}
	roles := make([]string, 0, len(result.Roles))
	for role, count := range result.Roles {
		roles = append(roles, fmt.Sprintf("%s %d", role, count))
	}
	sort.Strings(roles)
	copies := 0
	for _, instances := range result.Instances {
		copies += instances.Count()
	}
	return fmt.Sprintf("%d pieces, %d triangles; %v; %d blocks, %d facing a street, %d shared walls; %d model copies",
		len(result.Pieces), triangles, roles, result.Blocks, result.Facing, result.Shared, copies)
}

func printWarnings(warnings []string, limit int) {
	if len(warnings) == 0 {
		return
	}
	fmt.Printf("%d warnings\n", len(warnings))
	for _, warning := range warnings[:min(limit, len(warnings))] {
		fmt.Println("  " + warning)
	}
}

// clock prints how long every stage took.
type clock struct{ started, last time.Time }

func newClock() *clock {
	now := time.Now()
	return &clock{now, now}
}

func (timer *clock) lap(stage, detail string) {
	now := time.Now()
	fmt.Printf("%-6s %6.2f s  %s\n", stage, now.Sub(timer.last).Seconds(), detail)
	timer.last = now
}

func (timer *clock) total() {
	fmt.Printf("%-6s %6.2f s\n", "total", time.Since(timer.started).Seconds())
}

// layerFlags collects -layer name=path flags.
type layerFlags map[string]string

func (layers *layerFlags) String() string { return fmt.Sprint(map[string]string(*layers)) }

func (layers *layerFlags) Set(value string) error {
	name, path, found := strings.Cut(value, "=")
	if !found {
		return fmt.Errorf("expected name=path, got %q", value)
	}
	if *layers == nil {
		*layers = layerFlags{}
	}
	(*layers)[name] = path
	return nil
}

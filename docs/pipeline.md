# The pipeline

A map says *where* things are: this polygon is a house, this line is a street. It does not say
what a house looks like. go-scene-builder adds that, following **style rules** written by you, so
the same map can become a stone town or a timber village just by changing the rules. It knows
nothing about any particular world: the map and the rules do.

```mermaid
flowchart LR
    map[Map<br/>GeoJSON] --> one
    relief[Relief<br/>heights] --> one
    style[Style rules<br/>YAML] --> three
    one[1 · Basic blocks<br/>terrain, ground,<br/>water, volumes, lines] --> two[2 · Relations<br/>what faces a street,<br/>shared walls, gaps] --> three[3 · Style rules<br/>in cascade] --> four[4 · Decoration<br/>roofs, facades,<br/>plants, models] --> out[Pieces<br/>.glb or a live scene]
```

1. **Basic blocks.** The relief becomes terrain; ground features (streets, gardens, squares) are
   draped over it; water lies level; houses become volumes; walls and bridges become lines that
   follow the ground; points become small blocks or models.
2. **Relations.** What a block cannot know alone: which side of a house looks onto a street, which
   walls neighbours share (so they are not drawn twice), where a road crosses a wall and opens a
   gate.
3. **Style rules.** A cascade, like a style sheet: general rules first, specific ones after, the
   last that matches wins.
4. **Decoration.** Recipes add roofs, facades with doors and windows, towers at gates, plants in
   gardens and models such as lamps and benches.

## Run it

GEOS must be installed (`pkg-config geos`).

```bash
go run ./cmd/build-scene -map map.geojson -relief relieve.json -style style.yaml -out scene.glb
go run ./cmd/build-scene -map map.geojson -relief relieve.json -layer biome=biomes.json -style style.yaml -out scene.glb
```

It prints how long each stage took and the first warnings (features no rule styles, wrong values).
`-profile cpu.out` writes a CPU profile for `go tool pprof`. Then open the scene:
`go run ./cmd/visor scene.glb` in go-renderer.

## What goes in

**The map** is a GeoJSON `FeatureCollection`. Every feature has `properties.id` (stable between
versions) and `properties.kind` (a free word: the rules give it a meaning); any other property can
be read by the rules (`floors`, `use`, `width`…). The collection's properties say how to read the
coordinates: `units` (`km` or `m`), `axes` (`east-north` or `east-south`) and `origin` (the point
the scene is centred on).

**The relief** is `relieve.json`, `{"columns", "rows", "cell", "west", "north", "units"}` with the
edges of the grid, and next to it `relieve.f32`: heights in metres as little-endian float32, row by
row from the north, each at the centre of its cell. Without a relief the ground is flat.

**Layers** are grids of categories in the same format with `labels` (the name of every value), such
as the biomes a climate simulator writes: `-layer biome=biomes.json`. They colour the terrain and
sow vegetation ([decoration](decoration.md#biomes)).

## What comes out

A binary glTF 2.0 file, quantised to take less room. Scene coordinates are in kilometres, x east,
y up, z south, centred on the origin. Pieces are grouped by material in squares of `chunk` km
(0.5 by default) so the viewer only draws those in sight; terrain pieces are marked walkable
(`ground`). The scene's extras carry the sun, fog, an optional sea and the vertical scale, which
[go-renderer reads](https://github.com/akrck02/go-renderer/blob/feature/escenas-3d/docs/gltf-extras.md).

## As a library

go-simulator uses the builder to draw growing towns live with the same style rules. `build.Build`
takes the same inputs as the command plus `Heights` (any source of ground heights, for a scene
whose terrain exists already), `Detail` (the longest side of draped triangles without a relief) and
`Context` (features that take part in the relations without being built, such as the streets
around a house being rebuilt).

## Status

Done: reading maps, reliefs and rules; the basic blocks; the relations between blocks; recipes for
walls, roofs (flat, gable, hip), gate towers, facades (door, windows, shutters, flower boxes, an
emblem, lamps) and plants; materials with procedural patterns and glowing materials; colour
palettes; models drawn as instances; biomes; levels of detail; rivers and bridges; the `.glb`
writer; `build-scene`; and use as a library from go-simulator. Next: balconies and more
decorations.

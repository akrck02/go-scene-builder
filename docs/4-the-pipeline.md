# 🗺️ The pipeline

A map says *where* things are: this polygon is a house, this line is a street. It does not say
what a house looks like. go-scene-builder adds that, following **style rules** written by you, so
the same map can become a stone town or a timber village just by changing the rules. It knows
nothing about any particular world: the map and the rules do.

<p align="center">
  <img alt="map.geojson, relief.json and style.yaml, with optional layers, go into build-scene: 1 basic blocks, 2 relations, 3 style rules, 4 decoration; out comes scene.glb, which go-renderer's viewer opens" src="images/how-it-works.svg" width="880">
</p>

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

<table>
<tr>
<th width="50%">Stages 1 to 3 only: basic blocks</th>
<th width="50%">With stage 4: decoration</th>
</tr>
<tr>
<td><img alt="The example village's square and streets as plain boxes on draped ground, with no roofs, windows or trees" src="images/blocks.png"></td>
<td><img alt="The same square and streets with gable roofs, chimneys, windows, shutters, lamps and a bell tower" src="images/style-plaster.png"></td>
</tr>
<tr>
<td>Houses are volumes of the right height, streets and the square are draped on the relief.</td>
<td>Roofs, facades on the side that faces a street, a tower and lamps on the square.</td>
</tr>
</table>

## Run it

GEOS must be installed (`pkg-config geos`).

```bash
go run ./cmd/build-scene -map map.geojson -relief relieve.json -style style.yaml -out scene.glb
go run ./cmd/build-scene -map map.geojson -relief relieve.json -layer biome=biomes.json -style style.yaml -out scene.glb
```

It prints how long each stage took and the first warnings (features no rule styles, wrong values).
`-profile cpu.out` writes a CPU profile for `go tool pprof`. Then open the scene:
`go run ./cmd/visor scene.glb` in go-renderer.

The [example island](3-the-example-island.md) is a complete set of inputs to try it with.

## What goes in

<table>
<tr>
<th width="50%">The map</th>
<th width="50%">The relief</th>
</tr>
<tr>
<td>
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="images/map-dark.png">
  <img alt="The example island's map drawn flat: a legend of kinds and geometries, a coast, a beach, houses along two streets, a square, gardens, roads, a river with a bridge and a forest" src="images/map-light.png">
</picture>
</td>
<td>
<picture>
  <source media="(prefers-color-scheme: dark)" srcset="images/relief-dark.png">
  <img alt="The example island's relief, shaded, with a line every 20 metres up to a hill of about 150 metres" src="images/relief-light.png">
</picture>
</td>
</tr>
<tr>
<td>Every feature has a <code>kind</code>; the rules give it a meaning.</td>
<td>160 × 120 cells of 10 metres; under 0 metres is the sea floor.</td>
</tr>
</table>

**The map** is a GeoJSON `FeatureCollection`. Every feature has `properties.id` (stable between
versions) and `properties.kind` (a free word: the rules give it a meaning); any other property can
be read by the rules (`floors`, `use`, `width`…). The collection's properties say how to read the
coordinates: `units` (`km` or `m`), `axes` (`east-north` or `east-south`) and `origin` (the point
the scene is centred on).

```json
{"type": "FeatureCollection",
 "properties": {"units": "m", "axes": "east-north", "origin": [0, 0]},
 "features": [
  {"type": "Feature", "properties": {"id": "main-street", "kind": "street", "width": 8},
   "geometry": {"type": "LineString", "coordinates": [[-430, -220], [-260, -210], [-200, -210]]}}
 ]}
```

**The relief** is `relieve.json`, `{"columns", "rows", "cell", "west", "north", "units"}` with the
edges of the grid, and next to it `relieve.f32`: heights in metres as little-endian float32, row by
row from the north, each at the centre of its cell. Without a relief the ground is flat. The header
can have any name: the values are read from the same path with `.f32` instead of `.json` (the
example island's are `relief.json` and `relief.f32`).

**Layers** are grids of categories in the same format with `labels` (the name of every value), such
as the biomes a climate simulator writes: `-layer biome=biomes.json`. They colour the terrain and
sow vegetation ([decoration](6-decoration.md#biomes)).

## What comes out

<p align="center">
  <img alt="The example island built and seen from above the sea: the terrain, the beach, the village, the river and the forest on the hill" src="images/island.png" width="720">
</p>

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

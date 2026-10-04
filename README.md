# go-scene-builder

Turns any **2D map** (GeoJSON whose features carry an `id` and a `kind`) and a **relief** (a grid
of heights) into a **3D scene** (`.glb`) that [go-renderer](../go-renderer)'s viewer opens,
following **style rules** written by the author. It knows nothing about any particular world: the
map and the rules do.

```
map + relief → 1 basic blocks → 2 relations → 3 style rules → 4 decoration → pieces → .glb / live scene
```

Done so far (phases 1 to 4): reading maps, reliefs and rules; the basic blocks (terrain, draped
ground, level water, volumes, lines, points); the relations between blocks (the side that faces a
street, the walls neighbours share, the gaps where lines cross); recipes for walls, roofs (flat,
gable, hip) and gate towers; a quantised `.glb` writer; `build-scene`; and use as a library from
go-simulator, which draws live simulations with it (`scene_style` in its rules files). Next:
windows, chimneys and trees (phase 5), and levels of detail for large terrains.

As a library, `build.Build` also takes `Heights` (any source of ground heights, for a scene whose
terrain exists already), `Detail` (the longest side of draped triangles without a relief) and
`Context` (features that take part in the relations without being built).

## Run

GEOS must be installed (`pkg-config geos`).

```bash
go run ./cmd/build-scene -map map.geojson -relief relieve.json -style style.yaml -out scene.glb
```

It prints how long each stage took and the first warnings (features no rule styles, wrong values).
`-profile cpu.out` writes a CPU profile for `go tool pprof`.

## Inputs

**Map**: a GeoJSON `FeatureCollection`. Every feature has `properties.id` (stable between
versions) and `properties.kind` (free vocabulary: the rules give it a meaning); any other property
can be read by the rules. The collection properties say how to read coordinates: `units` (`km` or
`m`), `axes` (`east-north` or `east-south`) and `origin` (the point the scene is centred on).

**Relief**: `relieve.json`, `{"columns", "rows", "cell", "west", "north", "units"}` with the
edges of the grid, and next to it `relieve.f32`, heights in metres as little-endian float32, row by
row from the north, each at the centre of its cell. Without a relief the ground is flat.

**Style rules**: YAML, applied in cascade (every matching rule adds or replaces values; the last
one wins). Selectors match the kind, properties equal to a value or one of a list, and numeric
ranges. Values are numbers, `#rrggbb` colours (or `"@name"`: the colour in the feature's property `name`)
or arithmetic over the feature's numeric properties.

```yaml
units: m
chunk: 0.5                             # km: pieces are grouped in squares this big (larger for larger maps)
environment: {sun: [235, 38]}          # azimuth and elevation in degrees; water_level adds a sea;
                                       # vertical_scale exaggerates the relief seen from above
terrain: {color: "#6b7f47", bands: [{above: 350, color: "#9e9e99", blend: 30}]}
relations:
  facing: {when: {kind: street}, reach: 20}   # which side of a block looks onto a street
  rows: {tolerance: 0.4}                       # walls closer than this are shared (not drawn)
rules:
  - when: {kind: street}
    shape: {role: ground, width: "width * 1000", lift: 0.12}
    color: "#e6dcc4"
  - when: {kind: river}
    shape: {role: water, width: "width * 1000", above_bed: 1.8}
  - when: {kind: building}
    shape: {role: volume, height: "3.5 + (floors - 1) * 3"}
    walls: {color: "#e9dfc9"}
    roof: {recipe: gable, pitch: 38, ridge: long, eaves: 0.5, color: "#a4553a"}
  - when: {kind: building, use: [shop, workshop], floors: {min: 3}}
    roof: {color: "#8e4631"}
  - when: {kind: wall}
    shape: {role: line, height: 9, thickness: 3}
    openings: {where: {kind: street, class: road}, width: 6}   # a gap where a road crosses it
    decorate: [{recipe: gate_towers, size: 6, height: 13}]    # and a tower at each side of the gap
  - when: {kind: bridge}
    shape: {role: line, follow: level, thickness: "width * 1000", height: 0.4, sink: 0.8}
  - when: {kind: tower}
    shape: {role: point, size: 8, height: 15, form: round}
```

Roles: `ground` (polygons, or lines widened by `width`, draped `lift` metres over the relief),
`water` (level sheet `above_bed` metres over the bed under the feature's own line), `volume`
(a block: walls from `sink` metres under the lowest ground under its corners up to `height`, where
the roof starts), `line` (prisms of
`thickness` and `height` in pieces of `segment` metres following the ground, or with `follow: level`
one level prism whose top is `height` over its highest end, like a bridge deck), `point` (a square
or round block of `size` metres, decorated like volumes) and `none`.

Volumes are decorated by recipes: `walls` (colour; where a neighbour leans against a wall only the
part above its eaves is drawn) and `roof`: `recipe`
`flat`, `gable` or `hip`, with `pitch` (degrees), `eaves` (overhang, metres), `color` and `ridge`:
`long` or `short` (the side of the footprint's rectangle the ridge follows), `along_street` or
`across_street` (relative to the side that faces a street). Under a sloped roof the walls stand on
the footprint's smallest rectangle, like the roof.

## Output

Scene coordinates in kilometres, x east, y up, z south, centred on the origin. Pieces are grouped
by material in 0.5 km squares so the viewer draws only those in sight; terrain pieces are marked
walkable (`ground`). The scene extras carry the sun, fog, an optional sea and `verticalScale: 1`.

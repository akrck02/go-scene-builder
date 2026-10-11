# go-scene-builder

Turns any **2D map** (GeoJSON whose features carry an `id` and a `kind`) and a **relief** (a grid
of heights) into a **3D scene** (`.glb`) that [go-renderer](https://github.com/akrck02/go-renderer)'s viewer opens,
following **style rules** written by the author. It knows nothing about any particular world: the
map and the rules do.

```
map + relief → 1 basic blocks → 2 relations → 3 style rules → 4 decoration → pieces → .glb / live scene
```

```bash
go run ./cmd/build-scene -map map.geojson -relief relieve.json -style style.yaml -out scene.glb
```

Needs GEOS (`pkg-config geos`).

## Documentation

Everything is in [docs/](docs/README.md): [the pipeline](docs/pipeline.md),
[style rules](docs/style.md) and [decoration](docs/decoration.md).

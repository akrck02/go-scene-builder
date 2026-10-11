# go-scene-builder

> **Note:** this project was written with an AI assistant and is pending thorough review. See the [AI disclosure](#ai-disclosure).

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

## AI disclosure

I write most of my code by hand, so I want to be clear about this project: **it was written by an
AI assistant (Claude, by Anthropic) under my direction.** I decided what to build and how it should
behave; the code, the tests and the documentation were written by the AI. Every commit made this
way carries a `Co-Authored-By: Claude` line (22 of 22 commits here).

**Status: pending thorough review.** The code compiles and its tests pass, but I have not yet
reviewed it line by line. Treat it as a working draft, not as code I vouch for.

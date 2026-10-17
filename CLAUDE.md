# go-scene-builder

Turns a 2D map (GeoJSON with `id` and `kind`) and a relief into a 3D scene (`.glb`) for
go-renderer, following style rules written by the author. Start with `docs/4-the-pipeline.md`; style rules
in `docs/5-style-rules.md`.

## Commands

```bash
go vet ./...
go test ./...
go run ./cmd/build-scene -map map.geojson -relief relief.json -style style.yaml -out scene.glb
```

## Rules

- **Generic only.** This project knows nothing about any particular world, game or story. Never put
  names, places, dates, numbers or settings from a user's own project in code, comments, tests,
  examples, docs or commit messages. Examples use made-up places (like the example island in the docs)
  and round, neutral numbers.
- **Readable code.** Full-word names (no acronyms, no one-letter variables), small functions that do
  one thing, early returns instead of nested branches, comments that say *why*. Match the code
  around you.
- **Tests with every change.** `go vet ./...` and `go test ./...` must pass before committing.
- **Docs live in `docs/`**, written for people: explanatory, friendly, with 2D images where they
  help. Keep the README short and point to `docs/`. Update the docs in the same commit as the
  behaviour they describe.
- **AI disclosure.** Commits written with an AI assistant end with a `Co-Authored-By: Claude …`
  line, and the *AI disclosure* section of the README stays true. The
  code is pending human review: say so, don't overclaim.
- **Git.** Commit only when asked; never push, force-push, rewrite history or delete branches or
  the repository without the author's explicit permission. Do not change the global git config.

## Builder rules

- The builder knows nothing about a world: what things look like comes from the map and the style
  rules, never from code.

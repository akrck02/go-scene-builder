# go-scene-builder documentation

go-scene-builder turns a flat map into a 3D scene you can walk through. You give it a map (what is
where), a relief (how high the ground is) and style rules (what things look like), and it writes a
`.glb` file that [go-renderer](https://github.com/akrck02/go-renderer)'s viewer opens.

| If you want to… | Read |
|---|---|
| understand the four stages and what goes in and out | [The pipeline](pipeline.md) |
| write style rules: shapes, materials, walls, roofs | [Style rules](style.md) |
| dress the scene: facades, plants, models, biomes, rivers, bridges, gates | [Decoration](decoration.md) |

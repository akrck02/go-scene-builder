# 📚 go-scene-builder documentation

go-scene-builder turns a flat map into a 3D scene you can walk through. You give it a map (what is
where), a relief (how high the ground is) and style rules (what things look like), and it writes a
`.glb` file that [go-renderer](https://github.com/akrck02/go-renderer)'s viewer opens.

| | Page | What is in it |
|---|---|---|
| 🏴‍☠️ | [The quest](quest.md) | Build a village step by step, from one house to a walled town, in 6 short levels. |
| 🗺️ | [The pipeline](pipeline.md) | The four stages, what goes in (map, relief, layers) and out (`.glb`), running it, and use as a library. |
| 🎨 | [Style rules](style.md) | Writing rules in cascade: selectors and values, shapes, materials, walls and roofs, houses on slopes, terrain. |
| 🌳 | [Decoration](decoration.md) | Dressing the scene: facades, plants, models, biomes, rivers and bridges, tall walls and gates, levels of detail. |
| 🏝️ | [The example island](example.md) | A small made-up island in `examples/`: its files, how to build and view it, how the pictures were made. |

New here? Start with the [project README](../README.md), then build the [example island](example.md).

## 🚧 Not done yet

- Balconies and more decorations.

# Atlases rendered from Kenney's 3D kits

The `kits-z*.png` pages and their `kits-z*.json` manifests are rendered by `tools/render-sprites/render.py`
(Blender 5.2, headless) from [Kenney](https://kenney.nl)'s CC0 3D kits, fetched by `make kits` into the gitignored `tools/kits/`.
Only the rendered pages, the manifests and each kit's `License.txt` are committed.
`make sprites` regenerates them; the page budget is eight 2048 px pages per zoom level.

| Kit | Page | Used for |
| --- | --- | --- |
| City Kit Commercial | https://kenney.nl/assets/city-kit-commercial | session buildings by context fill; the city hall and library |
| City Kit Roads | https://kenney.nl/assets/city-kit-roads | avenues, bends, crossings, ends; lamp posts |
| City Kit Industrial | https://kenney.nl/assets/city-kit-industrial | the power plant and its stack; the towers (water towers); shipping containers, in reserve |
| City Kit Suburban | https://kenney.nl/assets/city-kit-suburban | two of the park and street trees; the planters on the plaza's edge |
| Nature Kit | https://kenney.nl/assets/nature-kit | six more tree species, the plaza's bushes, and the grass, weeds and saplings on an overgrown lot, scaled to the Suburban trees' height and repainted in their greens by the render script |
| Car Kit | https://kenney.nl/assets/car-kit | the cars on avenues with traffic (scaled to a third of a cell) |
| Space Kit | https://kenney.nl/assets/space-kit | the rover at the door of a working session |
| Train Kit | https://kenney.nl/assets/train-kit | the ledger: a diesel and container wagons per model on the freight loop (scaled to a quarter) |
| Watercraft Kit | https://kenney.nl/assets/watercraft-kit | the tugs that bring a new session up the river and take a demolished one away (scaled to a quarter) |
| (modelled in the script) | — | the drone that stands for a subagent in flight; the plaza's fountain, in three spray frames |

Every piece is cut at four headings (0, 90, 180, 270 degrees) under one sun,
seen by an orthographic camera 30 degrees above the ground and turned 45 degrees,
so a one-unit tile projects as the map's 2:1 diamond, 132 px wide and 66 tall at zoom 1.
Each sprite's manifest entry gives its page, its rectangle and the pixel where the piece's ground origin lands.

Kenney's terms, from [kenney.nl/support](https://kenney.nl/support): CC0, free for any use, attribution welcome but not required.
Thank you, Kenney; [support the studio](https://kenney.nl/donate) if the work helps you too.

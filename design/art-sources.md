## Where the art comes from

Aria's decision, 2026-10-18, recorded by item 123 in the [meta repository's roadmap](https://github.com/Botropolis-City/meta/blob/main/roadmap/123-no-art-from-image-models.md).
Read this before adding, changing or generating any picture the project draws, ships or publishes.

### The three sources

Every picture comes from one of three places, and from nowhere else.

1. **A human artist**, credited, and paid when the work is commissioned.
2. **An openly licensed asset pack**, with its licence followed, its licence file committed beside what was taken from it, and its author thanked.
   Today that is Kenney's CC0 kits, in `pkg/assets/kits/`, and Inter under the SIL Open Font License, in `pkg/assets/fonts/`, with Go Mono, by Bigelow & Holmes under the Go project's BSD licence, embedded from `golang.org/x/image` and its licence committed in `pkg/assets/fonts/gomono/`.
   The thanks are in the README.
3. **Geometry built by hand in code**: primitives placed, sized and coloured in a Blender script (`tools/render-sprites/`) or shapes drawn in Go (`pkg/render/`), the way an artist models.
   Claude writing that code is allowed,
   because what it writes is geometry, not a picture.

### What is ruled out

**Any model whose output is a picture, a texture or a mesh**: image generators, AI texture tools, image-to-3D and text-to-3D, and AI upscalers.

Those models are trained on artists' work and can reproduce it without anyone knowing.
The rule exists to keep that out of the city.
Aria, in her words: *"so we don't rely on any models trained on art they may inadvertently use other's art."*

Photographs and other pictures may be used as references to build from, as an artist uses them,
and are not committed.
The photographs of the monument sign that bug 45 was built from are the example.

### What it covers

Everything the project shows anyone: the atlases and tile packs, the shapes drawn in Go, the logo (`packaging/botropolis.svg`, twelve hand-placed polygons), the icons, the website at botropolis.city, and the demo frames.
Frames rendered from the app are the app's own art,
so they fall under the rule already.

### What guards it

`tools/check-asset-sources`, run by `make lint` and by `testing/acceptance/asset_sources_test.go`, holds every image under `pkg/assets/` to a traceable source.
Each must either sit in a directory with a non-empty licence file,
or be an atlas page in `pkg/assets/kits/` that a manifest there names, which `tools/render-sprites` writes and `make sprites-check` reproduces.
An image that is neither fails, by name.

That catches an asset with no traceable source.
It cannot tell whether a picture came from an image model, and nothing can,
so for that this document is the guard.

### Where it stood when written

The audit of 2026-10-18 found the rule already held.
All 155 atlas sprites are either from Kenney's kits or modelled from boxes and cylinders in `tools/render-sprites/buildings.py` and `render.py`; the in-house ones are the 95 under `botropolis/`.
The 16 px packs were Kenney's, and went with the top-down map (item 183).
Monument signs, scaffolding, badges, the river and the wires are vector shapes drawn in Go.

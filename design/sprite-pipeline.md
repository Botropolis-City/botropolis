## Sprite pipeline

Roadmap phase 9, 2026-09-18; Aria compared the first kit district with `r64-fit.png` and chose the kits.
`make kits` runs `tools/fetch-kits`, which follows each kenney.nl asset page's download link into the gitignored `tools/kits/<slug>/`;
the kits themselves are never committed, only what is rendered from them and their CC0 licence files under `pkg/assets/kits/<slug>/`.
`tools/render-sprites/render.py` is the headless Blender script (Blender 5.2 from `extra`, a developer dependency only).
Its `scene` mode renders one composed district for judging the look (`make kit-district`);
its `atlas` mode cuts every piece the map uses at four headings and two zoom levels, alpha-cropped with the pixel where the piece's ground origin lands,
shelf-packed onto 2048 px pages under a budget of eight per zoom, with a JSON manifest (`make sprites`; `tools/shrink-pngs` re-encodes the pages).
One sun with soft cast shadows; an orthographic camera 30° above the ground and turned 45° plus the heading, so a one-unit tile projects as the map's 2:1 diamond, 132 px wide and 66 tall at zoom 1 (atan(1/2) is the diamond's edge angle on screen, not the camera's tilt; the first atlases were cut at it and every tile came out a tenth too short).
Kits not modelled at one unit per cell are scaled on import (the Car Kit to 0.12); the drone and the plaza's fountain are modelled in the script from primitives in the kits' palette
(no kit on disk has a fountain, and the fountain's three spray frames are cut as three pieces the city cycles).
The Nature Kit's trees are brought onto the city's terms there too: each is scaled to a height in the Suburban trees' range and its named materials repainted in their greens,
because the one-palette rule is about colour and the pipeline is where colour is decided.
Every piece is anchored where it meets the ground, derived in Blender from its own mesh: the centre of its footprint in x and y, and the ground plane in z —
or the foot of the piece when it never reaches the ground, as the drone does not.
A model's own origin is not trusted for this, because a kit is free to put it anywhere and one kit does:
the Space Kit models its rover two tiles east and one and a half south of its origin, which is how workers came to stand in the avenue.
z is clamped at the ground rather than taken as the lowest point, because the Nature Kit sets its trees and bushes slightly into the earth on purpose.

That anchor is a fact about a mesh standing in for a fact about a picture, which is a species of inference worth naming because it has already gone wrong once elsewhere.
Bug 23 argued from `chimney-large` being a hollow shell — and it is, its 120 vertices put the lowest twelve on a ring of radius 0.5 with no filled disc — to a conclusion about why the piece reads as floating.
The inference failed because an isometric camera never sees an underside, so no property of the underside can explain anything the camera shows.
The anchors are the same shape of claim and are sound for a reason that has to be stated rather than assumed: they were validated against the rendered sprites, not against the meshes they came from.
Bug 20 measured how far every one of the 67 pieces moved when the derivation replaced the old anchors, and read the result off the frames — which is what caught the Space Kit's rover sitting two and a half tiles from its origin.
A derived anchor that had never been checked against a drawn sprite would be exactly as trustworthy as the hollow-shell premise was.

The render is reproducible to within a handful of pixels, and `make sprites-check` is built around that number rather than around a hope.
It re-cuts every atlas and runs `tools/atlas-diff.py`, which holds the manifests to a byte — every number in them is a decision the pipeline made —
and compares the pages as decoded pixels, passing a page while fewer than 400 of its 16,777,216 bytes differ.

The noise floor, measured on no-op renders in September 2026: seven of the nine pages come back byte-identical,
and the other two move 42 and 47 bytes — ten or so pixels, each by one or two of 255.
An independent run moved 142 bytes on one page.
Eevee at 32 TAA samples under software GL is very nearly, not exactly, reproducible, so a byte comparison fails on pixels nothing in the city can see,
and a gate that cries wolf is a gate nobody reads.
The tolerance is roughly three times the worst run observed; a change that moves real geometry moves whole sprites, which is tens of thousands of bytes, not hundreds.

The atlases therefore do not churn in git, so they stay committed: no git-lfs, no build-time render in the package.
`tools/shrink-pngs` passes `-define png:exclude-chunk=date` because ImageMagick otherwise stamps the wall clock into every page.
`tools/atlas-diff.py --self-test` checks its own PNG decoder against all five filters before it judges anything, and the Makefile runs it first.

`pkg/assets/kits.go` loads the atlases; `pkg/render/kits.go` draws a piece with its origin on a world point as the camera's heading sees it, picking the atlas cut at or below the zoom;
`pkg/render/recipes.go` says which piece stands for what:
a session's building is one of the city's own kinds, modelled in `tools/render-sprites/buildings.py`: one per district type and storey count, one to five storeys by how heavy the session's work has been on the user's own history (item 109);
the Industrial shipping containers are held in reserve for item 86; avenues are Roads pieces picked and turned by each cell's joins;
the plant is an Industrial hall with a stack, the hall and library the two Commercial pieces left, the MCP towers water towers; trees are the Suburban kit's; cars the Car Kit's;
each subagent in flight is a small drone working on a building's faces and roof. The Space Kit rover that stood at a working session's door was retired in item 89, when the building took over showing its own work.
The camera carries a heading (`r` turns it a quarter about the window's middle) and the projection, corners, fit bounds, minimap and back-to-front order all go through it;
the wheel eases the zoom to its ladder target unless motion is reduced.
The fit view shows the sprites; only far below it does the flat map view take over.
The 2D isometric packs of 2026-09-17 are gone,
and so are the 16 px packs, which went with the top-down drawing path (item 183).

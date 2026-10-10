# 74. Night glow draws as a grid of discs again, in the demo frames

**Open. Filed 2026-10-10 (r376), from looking at the first `make demo-media` output while building the website. Nobody's call — a defect, to be diagnosed.**

Item 37 capped each storey's glow at half a storey so a lit building stops wearing a column of merged discs, and it was checked at fit and at zoom.
The demo frames show the same shape at fit: working buildings at night carry a grid of translucent beige discs, about three across and a storey apart, sitting over the facade and rising above the roof line.
The discs are beige rather than the white of item 37's saturated column, so this may be a second mechanism rather than item 37 coming back.

Where to look, all in `dist/demo/` after `make demo-media`:

| shot | where |
| --- | --- |
| `hamlet/hero.png` | the two working buildings at the top of the city, around (860–1110, 140–330) |
| `layers/night.png` | tidepool's and orchard's blocks, around (460–820, 260–520), and the tower at (1310, 330) |
| `hamlet/drift.mp4` | every frame after the zoom; worst at the closest point, 5–9 s |

The daytime frames of the same scenarios have no discs, so whatever draws them is night-only.

**Why it matters now:** the hamlet is the best-looking night scenario and the obvious second hero for botropolis.city, and these frames are the ones a visitor sees first.
The website uses the metropolis by day until this is fixed.

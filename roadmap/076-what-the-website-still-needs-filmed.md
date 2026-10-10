# 76. What the website still needs filmed

**Open. Filed 2026-10-10 (r376), when botropolis.city was rebuilt around the demo media. Nobody's call — each is a scenario or a filming option, and each replaces a stand-in the site already ships.**

The site lives in its own repository, `Botropolis-City/botropolis.city`, checked out at `~/workspaces/github/botropolis.city`.
It reads `dist/demo/manifest.json` through `make media-import` there, so anything below reaches the site with `make media-import media-upload` and a push.

1. **A hero loop.** The title screen is `metropolis/hero.png` with a CSS drift, because every clip carries the interface and none loops.
   Wanted: the metropolis with the interface hidden, 12–16 s, a slow drift or turn whose last frame is its first, 3 MB or less. A 4:5 crop for phones is a bonus.
2. **A still per view.** The nine-view switcher is frames of `layers/views.mp4` at hard-coded seconds (`tools/import-media.ts` in the site repo), so refilming that clip can move them.
   Wanted: one still per view (attention, spend, pressure, staleness, servers, traffic, models, fan-out, health), plus night and a plain day, all from the `layers` camera.
3. **A social card.** `og:image` is `metropolis/hero.png` cropped to 1200×630 by the site build.
   Wanted: a 1200×630 shot framed for it, interface hidden.
4. **Posters from a chosen frame.** `timelapse/day.png` caught the drawn pointer mid-hover over the power plant, because a clip's poster is whatever frame it lands on.
   Wanted: a `poster_at` on a shot.
5. **Anchors, optionally.** The site's hotspot map is placed by hand in percentages (`src/data/glance.ts`), and moves out of true whenever the metropolis is refilmed.
   Wanted: the manifest carrying the screen position of named objects in each still — the plant, the towers, the storage yard, the fountain, each district — so the hotspots follow the frame.

The night stills are held back by item 74, not by anything here.

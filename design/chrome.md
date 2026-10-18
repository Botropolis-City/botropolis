## Chrome

Started 2026-09-18 with the roadmap's phase 7.
`pkg/ui` is the chrome's layout model: theme tokens (one palette with one accent and the six state tones, an 8 px grid, one radius, a 1 px hairline, four type sizes at 12/14/16/20)
and pure layouts such as the resource strip, each a function of the theme, the data and a text-measuring callback, unit-tested without a window.
`pkg/render` draws them.
Text is Inter (variable TTF, OFL, embedded from `pkg/assets/fonts`) through `text/v2`;
the bitmap font is gone.
The frame is laid out in device pixels (`LayoutF` times the display's scale factor),
so nothing is upscaled after the fact;
`render_scale` overrides the factor, which is how the acceptance screenshots get a reproducible 1× and 2×.
The daemon links none of this: `pkg/ui`, `pkg/assets` and Ebitengine stay out of `botropolisd`'s dependency graph.
States are listed by urgency everywhere — needs-you, working, unattended, resting, then the resource numbers —
from `state.Order` alone;
the strip, the window title, the bar and the TUI rows walk it,
and a zero count drops out without moving the others.

Phase 7 finished the same day.
`pkg/ui` holds the strip, the plate (every in-world label sits on one), the card, the footer with its key row, the minimap's box,
the help overlay, the settings panel and its model, and a button row for the cards to come;
each is a pure layout with its own tests.
Keys: `?` help, `h` hide the UI, `p` save a frame to `~/Pictures` from the game's own buffer, `0` resets the view and `f` is Skyline (the whole city if it reads at the zoom-out floor, otherwise the floor centred on it), the wheel stops at Skyline and one notch further, after the wheel has rested, opens the City Plan (item 106), which `m` (`city_plan_key`) toggles from anywhere, Esc closes back to where the camera was, and a click on a badge leaves by flying into its district, arrows pan, `+`/`-` zoom, `s` settings.
The settings panel edits `liveliness` (still, calm, steady or lively; `reduced_motion` forces still), `render_scale`, `shuttered_days`, `overgrown_days`, `terminal` and `district_type` (how a new district's type is chosen: random, guess, or one type for all; item 109) live where it can
and writes the one key back through `config.Save`, which never spells out defaults.
The acceptance test shoots the sample city at 1× and 2× with no daemon and checks the frame doubles,
the first strip dot is a state tone at both scales, and the bars are dark with ground between them.

## Info views

The number keys pick one of six lenses — Attention, Work, Cost, Capacity, Health and Connections (`city.Lenses`) — and each lens is a question the map can be asked (item 88).
A lens folds the views that answer it, opens on the first, and steps to the next when its key is pressed again, so the ten views are still what draws the map; a lens chooses among them.
Before item 88 the views were the choice themselves, nine of them on nine keys, and some of what follows was written then.

A view is subtractive: it shows one network and takes the rest away.
The city behind it recedes — `colorm.ChangeHSV(0, 0.18, 0.75)` drains most of its saturation and a little of its value — and only the objects the view is about keep their colour.

Draining colour needs `colorm`, not `ebiten.ColorScale`.
A `ColorScale` multiplies each channel independently, so it can only ever darken;
it cannot move a colour toward grey, because that means mixing channels.
The first attempt at receding the city used one and looked like nothing had happened at all.
Worth knowing before reaching for the cheaper-looking API.

Colour is validated, not asserted.
`tools/validate-palette.py` is the dataviz reference's own validator, vendored so the palettes can be checked here rather than by hand:
OKLCH lightness band and chroma floor, OKLab delta E under a Machado-Oliveira-Fernandes severity-1.0 simulation, and WCAG contrast against the surface.
`pkg/ui`'s palette tests apply the same metrics in Go, so the tool and the tests cannot drift.
The metric matters more than it looks: the first version of those tests measured euclidean distance in sRGB, which is not perceptually uniform, and passed a palette containing a pair full-colour readers could not separate.

The legend is a row of its own above the key row, drawn for as long as the view is up.
A status line that fades takes the view's meaning with it, so the legend is not one:
a ramp view draws the ramp with both ends written out, a categorical view a swatch per category.
`Scene.Legend` returns a structure rather than a sentence because the chrome draws colour, not text.
When no view is up it has no height, so entering and leaving a view never moves the map.
The view itself is remembered in `layout.json` by name rather than by number, so reordering the views cannot silently change what a saved layout means.

While a view is up the strip gives its state tones back and keeps only the counts.
That is the same correctness requirement, not a flourish: the tones have to be off the screen for the view's palette to be legal at all.

There used to be two projections, and the views had to mean the same thing in both.
The top-down map had no sprites to push through a colour matrix, only flat fills, so it drained the fill colour with `ui.Receded` instead.
Item 106 retired it: `--projection top` still parses, warns, and opens the isometric map.
`ui.Receded` outlived it, because the isometric map has flat fills of its own — the driveway pads, and the streets when no kit is loaded — and they have to recede with the sprites around them.
`colorm.ChangeHSV` does not work in HSV despite its name — YCbCr, hue rotated in the CbCr plane, luma scaled by value and chroma by saturation×value — so `ui.Receded` is written as that same transform and held against the matrix by a test over eight colours.
Writing the honest HSV version instead lands two units away on a mid grey: invisible, and enough to make a pad and the sprite beside it recede by different functions.

Anything that hands the sprite path a tint of its own is invisible to all of this.
The recede happens where the tint is nil, so a sprite that always supplies a `ColorScale` — the water towers, which carry their server's call count — never receded at all, and stood over a city that had stepped back.
It is a class of bug rather than one bug: the 16 px top-down projection had the same shape and was never checked before it was retired.

Two things the validator established that the eye did not.

The first is the cap.
A legend is an adjacent-pairs surface — in bars and lines only neighbours touch — and the reference's validated eight-hue palette carries six that way.
A map is an all-pairs surface, because any two districts can sit side by side, and the same palette fails at four.
Three pass on the dark ground.
So a categorical view colours the top three kinds and folds everything else into one "other", painted a neutral rather than a fourth hue:
"other" is the absence of a category, and giving it a colour would claim the sessions inside it had something in common.
Thirteen MCP servers were never going to be thirteen colours; Servers is legible because colour carries the top three and the contextual highlight carries the rest.

The networks — the wires from the plant, the beams from the towers, the cars on the roads, the freight loop, the drones at work on the buildings — each sit behind the view that asks about them.
It is the view that decides, not the lens, so stepping through a lens's views can change which networks are up:

| lens | view | networks |
| --- | --- | --- |
| Attention | `attention` | traffic (the cars), cranes (the drones), freight (the train) |
| Work | `groups` | — |
| Cost | `spend` | wires, freight (the train) |
| Cost | `models` | — |
| Capacity | `pressure` | — |
| Capacity | `fan-out` | cranes (the drones) |
| Health | `health` | wires |
| Health | `staleness` | — |
| Connections | `servers` | beams |
| Connections | `traffic` | traffic (the cars), cranes (the drones) |

Attention keeps the cars, the drones and the train because each of those answers for itself on hover.
Health keeps the wires for what is *not* there: a session the daemon has seen no hook events from has no wire, and that missing wire says the numbers beside that building are files-only guesses.
Which view draws which network is a property of `pkg/city`, so it is testable without a window, and `pkg/city/network_doc_test.go` holds this table against it (item 178).

The carrier is tinted, not only what it runs to: a wire by the rate it is carrying, a beam by its calls, a road by its traffic.
Each already held that number, so this reads what was there rather than adding anything.

The movers are gated before the sorted list rather than at the draw, so a view that does not want cars does not place them either — nothing is created and nothing is depth-sorted.
That is the same decision as the view itself, one level down: the cheapest drawing is the drawing not done.

Taking the networks out of Attention is worth **26% of a frame** on a live city — 3.31 ms to 2.45 ms, seven paired samples, every pair in the same direction.
It is worth nothing at all on the sample fixture, which has no wires, no roads, two beams and one freight carriage.
That is the same fact twice: the size of the win is the size of your networks, so the number belongs beside the city it was taken on.

The `detail` setting is the other lever, and a blunter one: `plain` drops the trees, the lamps and the planting, which is the largest single thing in a frame by count and the only part of the map whose absence costs no information.
It is worth 19% of an Attention frame on the live city, on top of the networks.
One setting, not a checkbox per kind — three boxes would be three ways to ask the same question.

Contextual highlight is the other half of how a view stays legible, and the half that does not need colour at all.
Put the pointer on a water tower and the sessions that call that server stay lit while the rest of the city fades.
That is what makes Servers readable with three colours and thirteen servers: colour carries the top three, and the pointer carries the rest.
The lit set fades its complement rather than receding it, using the same treatment as search, so "not what you asked about" always looks the same.
It stays down when there is nothing to tie — a session that calls no servers ties nothing, and fading the map to say so is worse than saying nothing.

The second is that the view has to be subtractive for the palette to be legal at all.
The state tones already spend amber, blue, teal, violet, slate, red and green.
Of the 56 ways to pick three of the reference's eight dark slots, 15 clear the all-pairs gates, and every one of them lands within delta E 2.2 of some state tone under deuteranopia.
The sequential ramp is no better placed: it passes within 11.1 of the error red for a full-colour reader and within 0.3 of the waiting teal under deuteranopia.
Seven tones and a view palette do not both fit in the space.
That makes "the chrome recedes with the city" a correctness requirement rather than a polish item — the tones are kept off the screen rather than out of the palette.


**The river carries three unrelated meanings, which makes it the most loaded single medium on the map.**
A tug means a session arrived or left. A liner, a cargo ship or a sailing boat means a share of a usage limit. A white speedboat means you pressed the refresh key and figures were fetched.
Three meanings on one waterway is past what colour alone can carry, so each is held apart on more than one axis at once: the gauge boats **hold station** while the other two **move**; the courier is **low and slender** where the tugs are **tall and blocky**, measured rather than judged — 1.25 against 2.24 in model height; and each answers the hover with a different kind of sentence.

Each also passes the test above, but the third passes it differently and that is worth saying.
A tug and a gauge boat appear unbidden, so they have to explain themselves to someone who did not ask for them.
**The courier appears because a key was pressed one frame earlier**, with a status line already naming what it is doing — it is feedback for a deliberate action, not ambient motion, and the action is most of its explanation.
Its card still teaches the key, because Aria had not realised the refresh existed at all: a thing that only appears when you already know the trick is no use for learning the trick.

**The honest limit.** All three are hoverable in principle and hard to hover in practice, because two of them are moving and the courier lives five seconds. Pointing at a moving object is a real cost that the "it can answer for itself" test does not measure, and the river is where that cost has accumulated. A fourth meaning should not be added here.

**The base view draws only the networks that explain themselves.**
The rule is subtractive — a view says something by taking things away, and the base view is quiet so that the quiet means something.
Traffic was the first exception, added 2026-09-26, and the test for an exception is whether the thing can answer for itself without the view to explain it.
A car can: its hover card names the two projects and what passes between them.
The drones joined it on that test (item 89), and so did the train, whose card names its model, its tokens and its cost (item 78).
A wire cannot, so the wires stay on the views that are about spend and health.
The exception is not "cars are nice"; it is that an object carrying its own explanation is not the unexplained motion the rule guards against.

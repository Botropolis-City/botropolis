## Two things that must agree

The most productive bug in this project is not a bug. It is a shape, and it has turned up seven times in three phases, in disguises that looked nothing like each other until they were laid side by side.

**Somewhere, two things must say the same thing, and nothing makes them.**
They agree on the day they are written, because whoever wrote them held both in mind at once. Then one of them changes.

| | the two things | how it showed | what closed it |
| --- | --- | --- | --- |
| bug 23 | the stack's ground point, written once to draw with and once to sort by | a tower drawn in front of the slab that should hide it | `plantStackAt` — one expression, two callers |
| bug 34 | the atlas manifest and what `go:embed` ships | 922 KB of a page nothing could reach, in every binary | `render.py` deletes pages the manifest does not name |
| bug 33 | the mesh and the sprite | a true sentence about an open rim, used to explain a picture that never shows undersides | measure the object the claim is about |
| phase 20 | the footer's verb and what a click does | "click attach" while a click selected | `labels_test.go` — the one that cannot derive |
| phase 20 | the plan's planting and the city's landmarks | planters growing out of the power plant | `plan.Plots` — the plan reserves, both read the reservation |
| phase 20 | the layout drawn and the layout hit-tested | *nothing yet* | hit-test what was drawn |

**Four of the seven were closed by derivation**: make one side compute from the other so they cannot differ, rather than writing both and hoping.
That is the first thing to reach for, and it is usually smaller than the duplication it replaces.

**One could not be.** A verb in a key row cannot be computed from what the code does without reflection, so it is tested instead — the test asserts the promise against the behaviour. That is the fallback, not the default, and it is weaker: a test covers the pairs someone thought of.

**One was caught before it bit**, which is the only reason it is in the table with an empty column. A click hit-tested a layout recomputed from `scene.Size()` while the draw used `screen.Bounds()`. They agreed, because `LayoutF` feeds both. Agreement that holds *today* is the signature — it reads as a coincidence rather than a fact, and that feeling is the thing to act on.

**What to ask, before the eighth of this shape.** Where two things must say the same thing: can one be made to derive from the other? If not, what test ties them? And if neither, write down that they are coupled and why, where the person about to change one will see it — which is what `TestDrained`'s comment does for the palette, and what the press/release comment does for the card.

**The seventh was caught by its own guard, which is what this section is for.**
The river's width and the usage boats' hull sizes are two things that must agree: the water has to be wide enough for three hulls abreast.
Bug 52 changed what the width was *for* — it had been derived from an across-river reading that the fix deleted — so the old derivation stopped holding anything up while still looking like a reason.
Rather than restate the hull sizes beside the river, `TestRiverFitsThreeHullsAbreast` re-measures them from the atlas every run and fails if the width stops clearing them.
It failed on the two-cell river with the exact overlap worked out by hand beforehand, which is the first time this shape was caught by a test rather than by Aria looking at a frame.
A stale derivation is worth as much attention as a stale number: when a constant's reason changes, the comment explaining it becomes the thing that is wrong.

**The honest gap.** `labels_test.go` ties the help's verbs, the view key's nine questions and Enter's promise. It does not tie the five mover cards or the legend's aggregate, and those are the likeliest to drift, because a card's sentence is assembled from several fields and any one of them can change meaning while the sentence stays the same. The class is not closed.

Not every wrong claim in this project has been this shape, and the next section is one that is not — a single number, correctly measured, that meant nothing because of the set it was compared against.

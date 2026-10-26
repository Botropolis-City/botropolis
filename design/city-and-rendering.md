## The city

| On the map | Stands for | Hover shows |
| --- | --- | --- |
| District | a project (cwd root, worktrees folded in; a temporary directory stands in the district its transcript ties it to — a team, a lead, subagent visits, messages, Claude Code’s scratchpad or files it edited — and otherwise in one *Temporary* district, item 272) | active hours, sessions, tokens, PRs |
| Building | a session | title, branch, model, state, age |
| District type: residential, shopping, office or industrial | chosen once per district by `district_type` (random from its path, a guess from its top-level file names, or one type for all), kept in the layout and changed from a building's card (item 109) | what kind of buildings it grows |
| Lit / blinds down / boarded up / overgrown | live / resting / shuttered after `shuttered_days` / overgrown after `overgrown_days` | the rung, and how long it has been quiet |
| Building height (one to five storeys), and its kind within the district's type | the session's heft over its whole life, on the user's own history cut into fifths (item 109) | tokens in context, last compaction |
| Worker at the bench | the main thread | current tool and file |
| Cranes on the roof | subagents and workflows in flight | count, names, tokens |
| Power plant at the centre | the API | tokens today by model, cache hit ratio |
| Power lines to a building (poles, sagging wires, sparks) | token flow, and that the daemon has the session's hook events: no wire means it has seen none and is reading files | tokens/min; cache-read vs. fresh drawn differently |
| Freight loop round the city, a train per model | the ledger: that model's spend over the last day, a wagon per unit (the unit grows to keep the longest train to six); the engine is the model's context-window class, up to 200k, up to 1M, over 1M or unknown (item 147) | model, spend, the context window and where it was read, the wagon unit |
| A tug on the river | a session arriving (from the north, docking beside its district before its building rises) or leaving (downriver, once it is gone from the map) | the session, its project, its container colour |
| Radio towers at the edge | MCP servers | sessions attached, calls today |
| Beam tower → building | a session using that server | calls this session |
| A road between two districts (autotiled, laid only where it runs) | a connection: a session that read or edited files in the other project, messaged a teammate working there, or had a subagent work there | both directions' traffic, which files, which sessions |
| A car on a road | one session on that connection working right now | the two projects |
| Library | skills | top skills invoked |
| City hall | `stats-cache.json` rollups | daily activity, model mix |
| Flag on a building | a PR | number, state |
| Smoke | an API error | the error |
| Night | loops and scheduled wakeups running unattended | — |
| Resource strip along the top | city-wide tallies: sessions by state, tokens/h, ~cost and cache hit over 24 h, subagents, MCP calls, PRs, errors | — |
| Ground: grass, the wood on the parks, the belt and the corridors | nothing — drawn lower in contrast than anything that means something, and answers no hover | — |

Dropped from bot-crossing: the ship, arrival and departure walks, idle pottering,
size-by-transcript, and the desktop-app "unread" flag.

## Rendering

2D top-down with pre-rendered, y-sorted sprites — the Factorio approach.
Ebitengine, so the whole project stays in Go.
First pass is flat coloured shapes with text; sprites (Kenney CC0 city and isometric packs to start) come once the model is right.

## View

The map is isometric (Kenney's isometric packs).
One wheel notch out past Skyline, it becomes the City Plan: a flat map of district badges at their places, the groups' lines and the usage readings in a legend (`pkg/city/cityplan.go`, item 106).
The plan has its own zoom, `PlanZoomSteps`, magnifying the fitted sheet about the pointer, and its pan is held so the sheet always covers the window; badges of districts off the window are left off, and whether badges are compact is decided by those in view (item 282).
Past 30% coverage every badge goes compact, a name and a status dot, and `LayoutPlanBadges` places them in order, each on its own spot if free and otherwise on the nearest free one, so none overlap; districts needing you are placed first, and the layout is cached because the plan is laid out every frame.
The `projection` setting is gone (item 149), so the map always opens isometric,
and `pkg/render` no longer has a top-down drawing path: `render.NewGame` turns any scene it is handed isometric, because `TopDown` is `Projection`'s zero value (item 183).
The top-down projection survives only inside `pkg/city`, as the identity, which most of its tests run in.
Isometric 2:1 is an affine projection, so `pkg/city` keeps rectangles and only the renderer sees diamonds.
Below a detail zoom the map view draws flat state-coloured blocks, the way Factorio's chart replaces sprites with map colours;
above it, buildings are stacked from the pack's ground floors, storeys and roofs.
Stopped sessions sat in a yard inside each district, then in one storage district along the south; since item 86 a stopped session keeps its own lot in its district and climbs the ladder there, and a district stays while it has any session.
A stopped building's windows go dark and, once it is shuttered, two planks are nailed across each pane: `tools/render-sprites` finds every window pane in each rendered building sprite, where the glass is the kit's one clearly blue surface, and writes its corners into the atlas manifest, so the planks sit on the windows the sprite shows from every heading (`pkg/render/facade.go`).
Our own kinds' windows are found by their lit glass, by a colour rule the manifest carries so the map's tests tell glass the same way; each pane is a patch of that glass split at the building's near corner, at the projection's slope, and pushed out until it covers every pixel of it, and planks are held to the building's own pixels row by row (item 166).
At night a lived-in building shows those same panes lit: the sprite's own pixels inside each pane, drawn again from the atlas page over the dark at day brightness, every pane in one `DrawTriangles` call; while it rises, only the panes of its finished storeys, each pane's storey measured from the ground of its own face, because a storey is a sloped band across each face rather than a row of the sprite (`pkg/render/windows.go`, item 166).
Nothing is cut or kept for it: a cut layer of each sprite's glass, the first version, added about 20 MB to the start-up heap's peak that `TestStartUpPeak` guards (item 70).
The lights are laid in a screen-sized layer as each building is drawn, in depth order, and every sprite drawn after cuts its own shape out of it, so a building in front hides the windows behind it; the layer goes over the night wash once the scene is drawn.
The lot ages on the ground, in a pass under every building (`pkg/render/lot.go`): its paving darkens with each rung, cracks once shuttered, and once overgrown grows grass in the cracks, tufts at the base of the walls and in the yard, and now and then a sapling, all seeded by the session's id and drawn whatever the detail setting, because they carry a datum.
When an overgrown session wakes, its growth clears in scatter order over `city.MowFor`, or fades over `city.MowFadeFor` under reduced motion.
A packed-up session leaves its district for the depot, a block the plan lays along the south when there is anything in it, from its west edge and as wide as its longest row: a row of shipping-container crates per project in its container colour, oldest first (`pkg/city/depot.go`). It stays, empty, until the truck carrying its last crate has left it.
Its district goes when nothing else is in it, and its slot stays in `layout.json`, so unpacking brings it back to its old lot.
The daemon is the only writer of what is packed: the map asks it with the `pack` op.
Labels are fixed-size and sit on the floor or above the kerb, never over what they name.

## City plan

Phase 8 of the roadmap, 2026-09-18.
`pkg/plan` is a pure function from the live districts' sizes in cells, and the tower count to where everything goes:
a plaza at slot (0,0), rings of slots around it whose columns and rows are sized to the largest block in them so the avenues run straight,
a district's slot remembered in `layout.json` while it stays live and replaced from the innermost ring when another took it,
unused slots as park blocks, a two-cell park belt, a telecom ridge along the north, the river down the east edge.
The avenue grid is in the plan as corridors, so routing and slots never depend on what is connected;
the fountain is the plaza's centre cell.
The trees that mean nothing — on the parks, the belt and the corridors — are planted as ground (item 85): clustered where a smooth noise rises over a threshold, and spaced by Poisson-disk sampling at least `plan.TreeSpacing` apart, every choice seeded from position so a plan always grows the same wood.
They give way where a road is laid, are drawn a little translucent and darker than anything that can be hovered, and, like the parks they stand on, answer no hover.
`pkg/city` converts cells to pixels, routes each connection along the corridors from kerb to kerb (a connection between neighbours runs the shared corridor),
and lays road only on the corridor cells a route crosses, each cell joined to the road beside it.
A corridor with no road on it is grass, so a road on the map always means a connection.
The lamps at crossings and the trees along the avenues stand only beside laid road.
A connection is one road per pair of districts, whichever way it was crossed, and lasts while any session that crossed it is on the map, live or resting.
When the last one goes, its road grows over in one 1.5 s fade, a 0.3 s cross-fade under reduced motion; a connection that comes back meanwhile ends the fade and lays the road again.
The pair's traffic counts stay on the road's card; the cars on it are one per session on the connection that is working, so a road nobody is working on keeps its tarmac and has no cars (item 85).
A district can be dragged to another slot: a press on its floor or name plate picks it up, a ghost of its block follows the pointer over the outline of the slot it would land on, and the drop writes only remembered slots to `layout.json`.
Dropping on a slot another district holds swaps the two; the ring just outside the city takes a drop and grows the city by a ring; the plaza and anything further out refuse it, outlined in the error colour.
A press anywhere else pans, and so does any drag with Shift or Alt held.
A right or middle drag pans from wherever it starts and never picks anything up (item 279); the help offers it first, because i3 and other window managers take Alt+drag to move windows.
`pkg/city` also puts the plant, hall and library on the plaza.
The lake, the hashed river and the per-cell hashed grass, dirt and trees are gone.
The plan stands on an island (`City.Island()`, item 85): its coast lies about two cells past the plan's edge and wanders by a smooth noise seeded only by position, so the same plan always has the same coast and a bigger city a bigger island.
A beach lies on the land side of the coast and shallows on the sea's, and a faint far shore stands some twenty-four cells out.
The river runs through the island and out to the sea at both ends.
Its drawn banks wander outward from its band by a smooth noise and never inward, so the band the gauges hold station in is always water, and each mouth flares across the coast while its blue fades into the shallows', so it meets the sea with no edge; tugs come in from the sea at its north mouth and leave by the south, while the usage gauges and the refresh courier keep to the river's band inside the plan.
`pkg/city` says what ground lies at any point, and `pkg/render` paints that once per plan into a texture in world space and draws it through the camera's own affine projection, so the coast is soft and costs one draw a frame.
The fit frames the plan, and the camera clamps so the island or its shallows stay under the middle of the window.

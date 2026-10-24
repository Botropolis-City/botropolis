# Demo media

Screenshots and videos of the city are filmed from demo sessions, never from a real `~/.claude`.
Twice before, frames of a real city published things they should not have: item 72 is the record.
This directory is the way around that.

## How it fits together

| path | holds |
| --- | --- |
| `corpus/` | recorded transcripts, one folder per toy project, as Claude Code wrote them |
| `scenarios/` | one city each: which sessions stand where, in what state, at what time of day, and the shots to make of it |
| `Dockerfile` | the sealed containers the demo is filmed and recorded in |

`botropolis-demo stage` builds a disposable home from the corpus for one scenario.
It moves every transcript so it ends as long ago as the scenario says,
because the power plant and the trains only see the last day or week, and a resting lot ages by how long it has been quiet.
It writes a session record for each live session, pointed at a detached `sleep`,
because the city decides a session is live by whether its pid answers.

`botropolis-demo film` stages each scenario afresh for every shot, runs the city headless against it, and encodes clips.
It writes `dist/demo/manifest.json` listing every file with its title, description, size and duration.

## Filming

```sh
make demo-media
```

That builds the binaries and the renderer image, then films every scenario inside a container.
The container has no network and a read-only root, and it mounts only the binaries, the corpus, the scenarios and the output directory.
`HOME`, the XDG directories and `TZ` all point inside it.
Nothing the city does can reach the real home, including the reads that would otherwise fall back to it.

`DEMO_CORPUS`, `DEMO_SCENARIOS` and `DEMO_OUT` point it somewhere else.

On a terminal it draws one progress line, redrawn in place, with the shot it is on, the frame it has reached, the time so far and an ETA;
piped to a file it writes that line every ten seconds instead.
The city's and ffmpeg's own output goes to `dist/demo/film.log`.
A clip is slow to film: software GL renders 1080p at a few frames a second, so a sixteen-second clip takes six or seven minutes.
`make demo-record` reports the same way, counting prompts.

A run only films the shots that changed.
Each shot is keyed on itself, its scenario, the corpus and both binaries; a shot with the same key as last time, whose files are all still there, is reused from `dist/demo/.film-cache`.
The key leaves out the shot's `title` and `description` and the scenario's `description`, because they draw nothing:
a reused shot takes its caption from the scenario as it reads now, so a caption change reaches `manifest.json` with no filming.
Editing one clip refilms that clip.
Changing the code refilms everything, since the city may draw differently.
`DEMO_FRESH=1 make demo-media` films everything regardless.

The key names the code by a hash of its source, not of the built binaries, so a cache filmed on one machine holds on another building the same commit.
To carry it over, copy `dist/demo` including `.film-cache/`:

```sh
rsync -avh --progress --exclude film.log \
  <this-host>:<checkout>/dist/demo/ <other-checkout>/dist/demo/
```

Then `make demo-media` there films only what that commit changed.
Test files are not in the hash, so editing a test refilms nothing.

`make demo-adopt` files every cached shot under the current key without filming, for when the key's recipe changes and the pictures cannot have.
Adopted shots take their captions from the scenario too.
It refuses, naming the shot, if anything has nothing cached, rather than start filming.

## Recording

Recording runs the real `claude` CLI against the toy projects, so it spends tokens on your account.
Make a long-lived token once with `claude setup-token`, then give it to the recorder in whichever way suits:

```sh
CLAUDE_CODE_OAUTH_TOKEN=... make demo-record
echo 'CLAUDE_CODE_OAUTH_TOKEN=...' > .env && chmod 600 .env && make demo-record
secret-run --env CLAUDE_CODE_OAUTH_TOKEN=<ref> -- make demo-record
```

`.env` is gitignored, and a token in the environment wins over one in `.env`.
The token reaches the container by name, never on a command line, and only the `demo-record` recipe sees it.

`make demo-plan` prints every session still to record and the most it could cost if every prompt hit its cap.
Sessions already in the corpus are skipped, so a run that stops can be started again.

## A scenario

```yaml
name: hamlet
description: One person, two projects, late evening.
clock: "22:30"          # the light follows this; 14:00 when unset
mcp: [tracker, weather] # radio towers
sessions:
  - ref: tidepool/6f1d  # <project>/<session id or a prefix of it>
    state: working      # working, needs-you, unattended, resting
    ago: 2m             # how long before now its last line sits
    started: 3h         # how long before now it was opened; its recording's own span when unset
  - project: beacon     # an empty plot needs no recording
    state: empty
groups: []              # the groups the city should find, each as its members' titles
shots:
  - name: overview
    title: A quiet hamlet
    description: Two projects, one session working.
  - name: tour
    keys: [equal, equal, tab]
    record: {seconds: 12, fps: 30}
    formats: [mp4, webm, gif]
```

A shot also takes `window`, `hover`, `detail`, `signage`, `scale` and its own `clock`.
Unknown keys are refused, and so is a scenario that places no sessions.

## Start times and groups

A recording runs for minutes, so staged as recorded every session looks opened just now.
The city groups sessions started within a minute of each other,
and the metropolis once showed one group across twelve districts (item 223).
Give each live session a `started:` that reads like a real day, spaced more than fifteen minutes from the others unless they belong together.
`started:` moves only the recording's opening back, so the newest line stays at `ago`.

`groups:` lists every group the scenario means to show, its teams and any group it means the city to infer.
`testing/integration/demo/scenarios_test.go` stages every scenario and fails when the city finds a group not listed, or misses one that is.

## What a recording cannot cheaply produce

A placement can add to its recording, each as a synthetic line stamped at the recording's last moment:

```yaml
  - ref: tidepool/fix-leap-day
    state: working
    title: Leap-day fix          # the sign over the building
    prs: [open, merged]          # roof flags, in these states
    errors: 2                    # smoke
    team: harbor                 # a camp
    agent: lead
  - ref: tidepool/fix-leap-day   # with a project too: a clone,
    project: driftwood           # a session of its own in another district
    state: needs-you
```

## Usage on the river

`usage:` floats the plan-usage boats, and `usage_changes:` moves them during a clip:

```yaml
usage: {session: 12, weekly: 40, models: {Opus: 20}}
usage_changes:
  - {at: 10s, session: 35, weekly: 43}
```

## Growing a building

`replay:` stages only a session's opening prompt and reveals the rest of its recording over the span, at its own pacing,
with its usage scaled so its context window fills to the share asked for:

```yaml
  - ref: tidepool/fix-table-timezone
    state: working
    arrive: 2s                       # a tug brings it in
    replay: {over: 18s, context: 74%}
    leave: 27s                       # then it rests on its lot
```

## Choreography

A clip's `script:` drives a pointer that is drawn into the frames, in seconds of video:

```yaml
    script:
      - {at: 0.5, point_at: "Race between list and checkout", over: 1.5}   # a session by title
      - {at: 2.0, wheel: 3, over: 2}                                       # zoom toward the pointer
      - {at: 4.5, click: true}                                             # raises its card
      - {at: 7.0, key: I}                                                  # any key, by name
      - {at: 9.0, pan_to: "district:kiln", over: 2}                       # glide the camera there
      - {at: 11.5, hold: ArrowLeft, for: 0.5}                              # or nudge it with an arrow
      - {at: 12.0, point_at: plant, over: 1}                               # or "district:<name>"
```

`pan_to` is the way to move about: it ends with its target in the middle of the window, where an arrow held at high zoom can carry the camera off the city.
`wheel` turns whole notches, spread over the span; the camera eases between them.
The scenario test resolves every `point_at`, every `pan_to` and every key name against the staged city, so a typo fails `make test` rather than a film run.

## Timelines

A clip with `timeline: true` plays the scenario's arrivals, departures and changes while it records:

```yaml
  - ref: kiln/ramp-rates
    state: working
    arrive: 3s                   # a tug brings it in
  - ref: lanternfish/snapshots
    state: working
    leave: 9s                    # its process ends; it rests on its lot
  - ref: tidepool/fix-leap-day
    state: working
    prs: [open]
    changes:
      - at: 6s
        prs: [merged]            # the same PR, merged: a celebration
      - at: 10s
        state: needs-you
```

The timeline keeps to video time, not wall time: an event at 6s happens once the frame six seconds into the clip is written.
The city re-reads the home every two seconds, so allow that much before a change shows.
A still, and a clip without `timeline: true`, shows the city as it stands at the start, without the sessions still to arrive.

`botropolis-demo stage --play` plays a timeline against a live home in real time, to watch in a window:

```sh
bin/botropolis-demo stage --scenario demo/scenarios/workday.yaml --home /tmp/city --play &
XDG_STATE_HOME=/tmp/city-state botropolis city --home /tmp/city --socket /nonexistent/b.sock
bin/botropolis-demo unstage --home /tmp/city
```

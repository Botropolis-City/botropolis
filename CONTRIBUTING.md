# Contributing

This file is how the repository works.
[README.md](README.md) is what Botropolis is and how to install it;
[DESIGN.md](DESIGN.md) is what exists and the rules the work is held to;
[ROADMAP.md](https://github.com/Botropolis-City/meta/blob/main/ROADMAP.md), in the Botropolis-City meta repository, is what is open.

## Layout

```
cmd/                  one directory per binary
  botropolis/           the CLI and the city
  botropolisd/          the daemon
  botropolis-hook/      the hook forwarder
pkg/                  everything else, one package per concern
  claude/               reads ~/.claude; never writes to it
  state/                the read model the daemon serves
  city/                 the model of the city: what each object means
  plan/                 where things stand, deterministically
  render/               Ebitengine, the atlases, the draw order
  ui/                   pure layout structs, unit-tested, drawn by render
  cli/                  one file per cobra command
  app/                  the fx graph wiring commands together
testing/
  integration/          across packages
  acceptance/           the built binaries, as a user runs them
  helpers/              shared setup, and the generated fixture
tools/                 scripts; every one takes --help
packaging/             what the .deb, .rpm and Arch packages install
design/ rules/ roadmap/  detail behind the three index files
```

## The gate

```
make build      # three binaries into bin/
make test       # unit, integration, acceptance
make lint       # docs-check, version-check, go vet, golangci-lint
make format     # gofmt
```

`make test lint` must be green before a pull request.
Work in this order and you will not be surprised at the end:
write, build, test, fix, lint, format, build again, commit.

## Tests

Nested `t.Run` reading as a sentence, and one assertion per test:

```go
t.Run("when a session has no assistant records", func(t *testing.T) {
    t.Run("and the transcript is read", func(t *testing.T) {
        t.Run("it should report zero messages", func(t *testing.T) {
            assert.Zero(t, transcript.Usage.Messages)
        })
    })
})
```

Tests are idempotent, clean up after themselves, and never depend on machine state.
Unit tests sit beside the code;
anything crossing packages goes in `testing/`.

**A green test is not a guard until it has been made to fail.**
Before trusting a new test, change the code it covers so that it *should* fail, and check that it does, with the plausible wrong implementation rather than an absurd one.
[rules/a-green-test-is-not-a-guard.md](rules/a-green-test-is-not-a-guard.md) has the five times this repository shipped a test that could not fail.

## Fixtures

Integration and acceptance tests run against **`testing/helpers/fixtures/synthetic/`, a small Claude home written by hand and committed**.
It holds each shape the readers must handle: a conversation, one waiting on a tool, one with no reply, a bridge stub, a session with subagents, a road between two repositories, and a transcript holding only a title.
Nothing in it is sampled from a real home, so the tests give the same answer on every machine, CI included.
When Claude Code writes a new shape, add it here by hand, with a line in `manifest.json` saying what it is.

Every integration and acceptance package runs with `HOME`, the XDG directories and `CLAUDE_CONFIG_DIR` pointed at an empty temp dir (`helpers.RunIsolated`), and asserts that it is (`helpers.AssertIsolated`).
So a test that reaches for the real home fails on any machine instead of passing on yours.
A new package under `testing/` needs both, in a `TestMain` and a `TestHomeIsolation`.

`make fixtures` still samples a scrubbed slice of your own `~/.claude`, into `testing/helpers/fixtures/sample/`, for finding new shapes and for `make gif`:

```
make fixtures            # writes testing/helpers/fixtures/sample/
```

`tools/make-fixtures.py` scrubs as it copies.
Prompts, tool output, titles and account identifiers become deterministic placeholders, and project paths, branches, PR repositories and MCP server names are renamed to stable aliases.
Ids, timestamps and token counts are kept verbatim.
A sampled fixture is gitignored and no test reads it.
If you are ever about to commit one, something has gone wrong: copy the shape into the synthetic home by hand instead.

## Makefiles and scripts

Targets use `::`, are silent by default, and announce themselves with `$(LOG)`:

```make
docs-check ::
	$(LOG) "Checking the documentation structure"
	@tools/check-docs
```

`$()` for internal commands, `${}` for variables.
Scripts are bash, start `#!/usr/bin/env bash`, and every one takes `--help`.

## Markdown

One sentence per line, and a line break at each complete idea in a compound sentence.
This is not a column limit — lines are as long as the idea is.
It exists so a diff shows which sentence changed.

When you edit one sentence, change only that sentence's lines.
Do not reflow the paragraph around it, and do not reformat a file in the same commit as a change to what it says.

`make lint` runs `tools/check-docs`, which fails if a relative link does not resolve from its own file's directory or if a document is linked from nowhere.
The roadmap's own checks — status lines, the open table, numbering — live with the roadmap in the meta repository.

## Cutting a release

The version lives in `VERSION`, one line, and everything else reads it.

1. Edit `VERSION`.
2. Add the matching section to `CHANGELOG.md` — [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), newest first, with the comparison links at the bottom updated.
3. `make version-check` — it fails, naming files, if anything disagrees.
4. Commit and push to `main`, and let CI build the packages.
5. Run the **release** workflow. It reads `VERSION`, downloads what CI built for that commit, takes the release notes from the changelog, and creates the tag and the release.

Nothing is rebuilt at release time, so what ships is what was tested.
The workflow takes a `dry-run` input that resolves and validates everything without creating anything.

## Pull requests

Say what changed and why.
If a number appears in the description, say how it was measured — this repository has a long history of numbers that were right about the wrong thing, and [rules/a-plausible-wrong-number.md](rules/a-plausible-wrong-number.md) is the list.

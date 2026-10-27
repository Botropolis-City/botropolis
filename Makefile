GREEN := "\033[0;32m"
NC := "\033[0;0m"

LOG := @sh -c '\
       printf ${GREEN}; \
       echo -e "\n> $$1\n"; \
       printf ${NC}' VALUE

# Captured before .env is read, so a token in the environment (from secret-run,
# say) wins over one in the file. See demo-record.
DEMO_TOKEN_FROM_ENV := $(CLAUDE_CODE_OAUTH_TOKEN)
-include .env

# Item 103: Go and golangci-lint are pinned in the repository, not taken from
# whatever the package manager last installed. GOTOOLCHAIN=auto would let a
# newer local Go win over go.mod's toolchain line, so make names it exactly,
# and Go downloads that toolchain into the module cache if it is missing. The
# linter lives in its own module because golangci-lint's dependencies would
# otherwise upgrade the app's own (24 of them, at v2.15.0).
GO_MOD ?= go.mod
GO_TOOLCHAIN := $(shell awk '$$1 == "toolchain" { print $$2 }' ${GO_MOD})
export GOTOOLCHAIN := ${GO_TOOLCHAIN}
GOLANGCI_LINT := go tool -modfile=tools/lint/go.mod golangci-lint
# Item 186: golangci-lint takes a lock in TMPDIR for the whole machine and,
# without this flag, gives up on it after five seconds, so a lint failed
# whenever another session was linting. With it, a lint waits its turn. The
# format probe passes --allow-parallel-runners instead (item 133).
GOLANGCI_LINT_FLAGS ?= --allow-serial-runners
GOLANGCI_LINT_PIN := $(shell awk '$$1 == "github.com/golangci/golangci-lint/v2" { print $$2 }' tools/lint/go.mod)

MODULE := botropolis.city/botropolis
BINARIES := botropolis botropolisd botropolis-hook
VERSION ?= $(shell cat VERSION)
LDFLAGS := -X ${MODULE}/pkg/version.Version=${VERSION}
FIXTURE ?= sample

# Ebitengine reaches GL and X11 through dlopen rather than linking them, so
# nothing here needs cgo -- verified by building with it off and rendering a
# frame. That is what makes GOARCH=arm64 a plain `go build` with no cross
# toolchain, and it is why the release matrix is cheap.
export CGO_ENABLED = 0
GOARCH ?= $(shell go env GOARCH)

targets ::
	@awk -F'::?[[:space:]]*' '/^[a-zA-Z0-9][^$#\/\t=]*::?([^=]|$$)/ { \
		gsub(/^[[:space:]]+|[[:space:]]+$$/, "", $$1); \
		gsub(/^[[:space:]]+|[[:space:]]+$$/, "", $$2); \
		split($$1,A,/ /); \
		target=A[1]; \
		deps=$$2; \
		printf "%s", target; \
		if (deps) printf " → %s", deps; \
		print "" \
	}' $(MAKEFILE_LIST)

build ::
	$(LOG) "Building ${BINARIES} (${VERSION})"
	@for bin in ${BINARIES}; do \
		go build -trimpath -ldflags "${LDFLAGS}" -o bin/$$bin ./cmd/$$bin || exit 1; \
	done

# The city's frame tooling -- --screenshot, --record, --keys, --headless and
# the rest -- is only in builds with the devtools tag (item 167), so release
# builds neither show nor promise it. build-dev builds the same binaries with
# it, for filming, frames and measuring; every packaging target depends on
# build, which builds them again without it.
DEV_TAGS := -tags devtools

build-dev ::
	$(LOG) "Building ${BINARIES} (${VERSION}) with the devtools tag"
	@for bin in ${BINARIES}; do \
		go build ${DEV_TAGS} -trimpath -ldflags "${LDFLAGS}" -o bin/$$bin ./cmd/$$bin || exit 1; \
	done

# The demo stager is a development tool for filming the city, so it is built
# beside the binaries rather than among them: nothing packages it.
demo-tool ::
	$(LOG) "Building botropolis-demo"
	@go build -trimpath -ldflags "${LDFLAGS}" -o bin/botropolis-demo ./cmd/botropolis-demo

# Demo media is filmed inside a container with no network, a read-only root,
# and only the binaries, the corpus and the scenarios mounted, so the real
# home cannot be reached by anything the city does -- including the reads
# that fall back to the real home directory. See demo/README.md.
DEMO_IMAGE ?= botropolis-demo
DEMO_CORPUS ?= demo/corpus
DEMO_SCENARIOS ?= demo/scenarios
DEMO_OUT ?= dist/demo
# Shots unchanged since the last run are reused; DEMO_FRESH=1 films them all.
DEMO_FRESH ?=

# The tags are machine-wide, so another checkout's build can move one between
# this build and its run: a recording once ran in an image two days older,
# without tmux (item 105). Each build writes its image's ID into this
# checkout's bin/, and every run here uses that ID. The tags stay for people.
DEMO_RENDERER_ID := bin/.demo-renderer.iid
DEMO_RECORDER_ID := bin/.demo-recorder.iid

demo-image ::
	$(LOG) "Building the demo images"
	@mkdir -p bin
	@docker build -q -f demo/Dockerfile --target renderer -t ${DEMO_IMAGE}-renderer --iidfile ${DEMO_RENDERER_ID} demo >/dev/null
	@docker build -q -f demo/Dockerfile --target recorder -t ${DEMO_IMAGE}-recorder --iidfile ${DEMO_RECORDER_ID} demo >/dev/null

# Recording spends real tokens on the account in CLAUDE_CODE_OAUTH_TOKEN, so
# it is never a dependency of anything. The token comes from the environment,
# or from CLAUDE_CODE_OAUTH_TOKEN=... in .env (gitignored), and the environment
# wins. Either way it reaches the container by name, never on a command line:
#
#   CLAUDE_CODE_OAUTH_TOKEN=... make demo-record        (or put it in .env)
#   secret-run --env CLAUDE_CODE_OAUTH_TOKEN=<ref> -- make demo-record
#
# The demo's token is the 1Password item DEMO_TOKEN_REF names (item 105):
#
#   secret-run --env "CLAUDE_CODE_OAUTH_TOKEN=${DEMO_TOKEN_REF}" -- make demo-record
#
# It is exported to this one recipe only, not to every target .env reaches.
#
# Sessions the corpus already holds are skipped, so a run can be resumed.
# `make demo-plan` prints what would be recorded and the most it could cost.
DEMO_ONLY ?=
DEMO_TOKEN_REF ?= op:Botropolis/Botropolis Demo Recorder Token/credential

demo-plan :: demo-tool
	@bin/botropolis-demo record --dry-run --scripts demo/scripts --corpus ${DEMO_CORPUS} $(if ${DEMO_ONLY},--only ${DEMO_ONLY})

demo-record :: export CLAUDE_CODE_OAUTH_TOKEN := $(or ${DEMO_TOKEN_FROM_ENV},${CLAUDE_CODE_OAUTH_TOKEN})
demo-record :: demo-tool demo-image
	$(LOG) "Recording demo sessions into ${DEMO_CORPUS}"
	@test -n "$${CLAUDE_CODE_OAUTH_TOKEN}" || { echo "CLAUDE_CODE_OAUTH_TOKEN is not set: export it, put it in .env, or use secret-run" >&2; exit 1; }
	@mkdir -p ${DEMO_CORPUS}
	@echo "Recording in image $$(cat ${DEMO_RECORDER_ID})"
	@docker run --rm $$([ -t 2 ] && echo -t) -e CLAUDE_CODE_OAUTH_TOKEN \
		--user 1000:1000 \
		-v $(CURDIR)/bin:/opt/botropolis:ro \
		-v $(CURDIR)/demo:/demo:ro \
		-v $(abspath ${DEMO_CORPUS}):/corpus \
		$$(cat ${DEMO_RECORDER_ID}) \
		/opt/botropolis/botropolis-demo record --image $$(cat ${DEMO_RECORDER_ID}) $(if ${DEMO_ONLY},--only ${DEMO_ONLY})

# The shots are keyed on a hash of the source rather than of the binaries, so
# a cache filmed here is good on another machine building the same commit.
# Tests are left out: changing one cannot change a frame.
DEMO_SOURCE = $(shell { find cmd pkg -type f ! -name '*_test.go' -print0 | LC_ALL=C sort -z | xargs -0 sha256sum; sha256sum go.mod go.sum VERSION; } | sha256sum | cut -c1-64)

DEMO_FILM = docker run --rm $$([ -t 2 ] && echo -t) --network none --read-only --tmpfs /tmp:exec \
		--user $$(id -u):$$(id -g) -e HOME=/tmp \
		-v $(CURDIR)/bin:/opt/botropolis:ro \
		-v $(abspath ${DEMO_CORPUS}):/corpus:ro \
		-v $(abspath ${DEMO_SCENARIOS}):/scenarios:ro \
		-v $(abspath ${DEMO_OUT}):/out \
		$$(cat ${DEMO_RENDERER_ID}) \
		/opt/botropolis/botropolis-demo film --corpus /corpus --out /out --botropolis /opt/botropolis/botropolis \
		--source ${DEMO_SOURCE} $(if ${DEMO_FRESH},--fresh) /scenarios

demo-media :: build-dev demo-tool demo-image
	$(LOG) "Filming ${DEMO_SCENARIOS} from ${DEMO_CORPUS} into ${DEMO_OUT}"
	@echo "Filming in image $$(cat ${DEMO_RENDERER_ID})"
	@mkdir -p ${DEMO_OUT}
	@${DEMO_FILM}

# Item 236: frames that compare two builds are filmed holding the landing
# lock, so no landing's gate runs beside them. A worktree session cannot run
# flock around the film itself, so make takes the lock, as for make land.
# make demo-media-locked DEMO_SCENARIOS=<dir> DEMO_OUT=<dir>
DEMO_MEDIA_GATE ?= demo-media

demo-media-locked ::
	@${MAKE} --no-print-directory land-queue-tool
	@${LAND_QUEUED} --name demo-media-locked -- ${MAKE} --no-print-directory ${DEMO_MEDIA_GATE} DEMO_SCENARIOS='${DEMO_SCENARIOS}' DEMO_OUT='${DEMO_OUT}'

# Files every cached shot under the current key without filming anything:
# for after the key's own recipe changes, when the pictures cannot have. A
# shot with nothing cached stops it, rather than starting an hour of filming.
demo-adopt :: build-dev demo-tool demo-image
	$(LOG) "Adopting the shots cached in ${DEMO_OUT} under the current keys"
	@${DEMO_FILM} --adopt

test :: test-unit test-integration test-acceptance

# Item 131: go test's default ten-minute timeout panics the acceptance suite
# when several item sessions test at once; at a load average of about 63 it
# took twelve minutes. Item 186: at about 150, pkg/render's unit tests hit it
# too, so every suite takes the same explicit timeout. CI sets a tighter
# ACCEPTANCE_TIMEOUT, since a CI run takes about four and a half minutes and a
# hang should fail it quickly.
TEST_TIMEOUT ?= 40m
ACCEPTANCE_TIMEOUT ?= ${TEST_TIMEOUT}

# Item 263: 0.3.1 failed to build for anyone who had set district_type,
# because a unit test read the builder's own config. So the unit suite always
# runs in a copy of a home whose config, and BOTROPOLIS_ variables, set every
# key away from its default; testing/helpers checks the file covers every key.
HOSTILE_HOME := testing/helpers/fixtures/hostile-home

test-unit ::
	$(LOG) "Running unit tests under a hostile config"
	@set -e; \
	export GOCACHE="$$(go env GOCACHE)" GOMODCACHE="$$(go env GOMODCACHE)" GOPATH="$$(go env GOPATH)" GOENV="$$(go env GOENV)"; \
	home="$$(mktemp -d)"; \
	trap 'rm -rf -- "$$home"' EXIT; \
	cp -R ${HOSTILE_HOME}/. "$$home"; \
	export HOME="$$home" XDG_CONFIG_HOME="$$home/.config"; \
	export BOTROPOLIS_CONFIG="$$XDG_CONFIG_HOME/botropolis/config.toml"; \
	eval "$$(awk -F' = ' '/^[a-z_]+ = / { gsub(/"/, "", $$2); print "export BOTROPOLIS_" toupper($$1) "='\''" $$2 "'\''" }' "$$BOTROPOLIS_CONFIG")"; \
	go test -timeout ${TEST_TIMEOUT} ./cmd/... ./pkg/... ./tools/acceptance-targets/...; \
	go test -timeout ${TEST_TIMEOUT} ${DEV_TAGS} ./pkg/cli/...

test-integration ::
	$(LOG) "Running integration tests"
	@go test -timeout ${TEST_TIMEOUT} ./testing/helpers/... ./testing/integration/...

# Item 226: test-acceptance runs each acceptance test in a process of its
# own, so each has the isolated home TestMain gives a run, ACCEPTANCE_PARALLEL
# at a time: by default half the cores and at most six, since fourteen at once
# is what took the load to 178 (item 186). The binary is built once and named
# to every process. The tests that measure CPU or memory call
# performanceTier(t) and run only in test-performance, on a quiet machine:
# the performance manager runs it on demand and before each release, and the
# release workflow runs it before the tag.
ACCEPTANCE_PARALLEL ?=
ACCEPTANCE_BINARY := .cache/acceptance/botropolis

test-acceptance ::
	$(LOG) "Running acceptance tests"
	@${MAKE} --no-print-directory test-acceptance-run RUN=

# The performance tier holds the landing lock, so no landing's gate runs
# beside what it measures.
# RUN='^TestName$' narrows it to tests within the tier, and refuses a
# pattern that matches none of them.
PERFORMANCE_GATE ?= test-performance-run

test-performance ::
	$(LOG) "Running the acceptance tests that measure CPU and memory"
	@${MAKE} --no-print-directory land-queue-tool
	@${LAND_QUEUED} --name test-performance -- ${MAKE} --no-print-directory ${PERFORMANCE_GATE} RUN='${RUN}'

test-performance-run ::
	@pattern=$$(go run ./tools/acceptance-targets tier performance -run '${RUN}') && \
		BOTROPOLIS_PERFORMANCE=1 go test -count=1 -v -timeout ${ACCEPTANCE_TIMEOUT} -run "$$pattern" ./testing/acceptance/

# Item 128: 40 city starts while the input devices are busy and the main
# thread is signalled, each of which must exit 0, and item 230: 40 building
# sheets drawn by the demo tool the same way. Needs an openable
# /dev/input/event*, and says so when there is none. SOAK_GAMEPAD_RUN narrows
# it to one case, and it logs every run as it finishes.
SOAK_GAMEPAD_RUN ?= TestStartUnderSignals
SOAK_GAMEPAD_TIMEOUT ?= 60m

soak-gamepad ::
	$(LOG) "Soaking the city's start under signals"
	@BOTROPOLIS_SOAK=1 go test -count=1 -v -timeout ${SOAK_GAMEPAD_TIMEOUT} -run '${SOAK_GAMEPAD_RUN}' ./testing/acceptance/

# Item 125: 120 screenshot runs at load, each of which must exit 0. Minutes
# long, so not part of test-acceptance.
soak-teardown ::
	$(LOG) "Soaking the city's teardown"
	@BOTROPOLIS_SOAK=1 go test -count=1 -timeout 40m -run TestTeardownSoak ./testing/acceptance/

# Targeted acceptance runs, items 117 and 226. test-acceptance-map runs each
# test on its own under coverage and records the packages it executed, one map
# per revision under ${XDG_CACHE_HOME:-~/.cache}/botropolis/acceptance-map,
# shared by every checkout and worktree. test-acceptance-changed runs only the
# tests a change since the nearest map can reach, a change inside
# testing/acceptance through its own call graph, and runs everything, saving a
# new map, saying why, whenever main's last CI run was not green, the map is
# more than 30 commits back, or the change cannot be traced. make land's gate
# runs it; CI runs the whole suite on every push.
test-acceptance-map ::
	$(LOG) "Mapping the acceptance tests to the packages they run"
	@go run ./tools/acceptance-targets map

test-acceptance-changed ::
	$(LOG) "Running the acceptance tests a change can reach"
	@go build -tags devtools -o ${ACCEPTANCE_BINARY} ./cmd/botropolis
	@BOTROPOLIS_ACCEPTANCE_BINARY=${CURDIR}/${ACCEPTANCE_BINARY} go run ./tools/acceptance-targets changed -timeout ${ACCEPTANCE_TIMEOUT} $(if ${ACCEPTANCE_PARALLEL},-p ${ACCEPTANCE_PARALLEL})

# What make land runs: everything make test does, but only the acceptance
# tests a change can reach.
test-landing :: test-unit test-integration test-acceptance-changed

# Wall time for the bar's tasks, item 91. It reports and never gates: the
# acceptance tests gate on steps, ticks and work instead. CI runs it once per
# release, not per push, and attaches bench.txt to the release (item 104).
BENCH_TIME ?= 3x
BENCH_COUNT ?= 6

bench ::
	$(LOG) "Benchmarking the bar's tasks"
	@go test ./testing/acceptance -run '^$$' -bench . -benchtime ${BENCH_TIME} -count ${BENCH_COUNT}

lint :: go-version-check lint-version-check docs-check version-check workflows-check demo-check darwin-check asset-sources-check antialias-check
	$(LOG) "Linting"
	@go vet ./...
	@go vet ${DEV_TAGS} ./...
	@${MAKE} --no-print-directory lint-golangci

lint-golangci ::
	@${GOLANGCI_LINT} run ${GOLANGCI_LINT_FLAGS} ./...

# Item 186: item sessions land one at a time. Each runs make land, one plain
# command, which takes a lock for the whole machine and, holding it, rebases
# onto the remote's main, runs the gate and pushes. Nothing is piped, so the
# gate's exit code alone decides the push. Every process under the lock holds
# it, so a session killed with its process group releases it at once.
LAND_LOCK ?= /tmp/botropolis-land.lock
LAND_REMOTE ?= github
LAND_BRANCH ?= main
LAND_GATE ?= lint test-landing

# Item 295: flock hands a released lock to any waiter, so a landing could wait
# behind later arrivals. Everything that takes the lock queues for it through
# tools/land-queue and is served in arrival order, with the flock kept beneath.
# The queue sits beside the lock, so a test that moves the lock moves it too.
LAND_QUEUE ?= $(basename ${LAND_LOCK}).queue
LAND_QUEUE_TOOL := .cache/land-queue/land-queue
LAND_QUEUED = ${LAND_QUEUE_TOOL} run --lock ${LAND_LOCK} --queue ${LAND_QUEUE}

land-queue-tool ::
	@go build -o ${LAND_QUEUE_TOOL} ./tools/land-queue

# Who holds the landing lock and who waits for it, from which worktree.
land-queue :: land-queue-tool
	@${LAND_QUEUE_TOOL} list --lock ${LAND_LOCK} --queue ${LAND_QUEUE}

land :: land-queue-tool
	@${LAND_QUEUED} --name land -- ${MAKE} --no-print-directory land-under-lock

# Item 226: acceptance tests run beside a landing's gate slow it and fail it,
# so a session runs them holding the same lock. A worktree session cannot run
# flock around go test itself, since the worktree guard cannot see what flock
# runs, so make takes the lock, as it does for make land.
# make test-acceptance-locked RUN='^TestName$'
LOCKED_GATE ?= test-acceptance-run

test-acceptance-locked ::
	@test -n "${RUN}" || { echo "make test-acceptance-locked needs the tests to run: RUN='^TestName\$$'"; exit 2; }
	@${MAKE} --no-print-directory land-queue-tool
	@${LAND_QUEUED} --name test-acceptance-locked -- ${MAKE} --no-print-directory ${LOCKED_GATE} RUN='${RUN}'

test-acceptance-run ::
	@go build -tags devtools -o ${ACCEPTANCE_BINARY} ./cmd/botropolis
	@BOTROPOLIS_ACCEPTANCE_BINARY=${CURDIR}/${ACCEPTANCE_BINARY} go run ./tools/acceptance-targets run -timeout ${ACCEPTANCE_TIMEOUT} -run '${RUN}' $(if ${ACCEPTANCE_PARALLEL},-p ${ACCEPTANCE_PARALLEL})

land-under-lock ::
	$(LOG) "Landing on ${LAND_REMOTE}/${LAND_BRANCH}, holding ${LAND_LOCK}"
	@git fetch ${LAND_REMOTE}
	@tools/ci-status
	@git rebase ${LAND_REMOTE}/${LAND_BRANCH}
	@${MAKE} --no-print-directory ${LAND_GATE}
	@git push ${LAND_REMOTE} HEAD:${LAND_BRANCH}

go-version-check ::
	$(LOG) "Checking Go is the toolchain go.mod pins"
	@tools/check-pin go "${GO_TOOLCHAIN}" -- go env GOVERSION

lint-version-check ::
	$(LOG) "Checking golangci-lint is the version tools/lint/go.mod pins"
	@tools/check-pin golangci-lint "${GOLANGCI_LINT_PIN}" -- ${GOLANGCI_LINT} version

# Everything under demo/ is published in effect, so it is checked for anything
# from a real machine, and every corpus file must be on a reviewed list.
demo-check ::
	$(LOG) "Checking the demo corpus came from no real machine"
	@tools/check-demo-corpus

# Nobody here has a Mac (item 80), so this is what keeps a Linux-only call
# from landing unnoticed; the macOS CI job is what runs the darwin paths.
darwin-check ::
	$(LOG) "Vetting the module for macOS"
	@GOOS=darwin GOARCH=arm64 go vet ./...

docs-check ::
	$(LOG) "Checking the documentation structure"
	@tools/check-docs

# Every image under pkg/assets needs a traceable source: a licence file beside
# it, or an atlas page a manifest names (item 123, design/art-sources.md).
asset-sources-check ::
	$(LOG) "Checking every image under pkg/assets has a source"
	@tools/check-asset-sources

# No shape is drawn with an anti-aliased vector call, which costs stencil
# passes and heap every frame; draw through pkg/shapes (item 191).
antialias-check ::
	$(LOG) "Checking nothing draws with anti-aliased vector calls"
	@tools/check-antialias

version-check ::
	$(LOG) "Checking every file that names a version agrees"
	@tools/check-version

workflows-check ::
	$(LOG) "Checking the workflows parse"
	@tools/check-workflows

packages-check ::
	$(LOG) "Checking the three packages carry the same files"
	@tools/check-packages

# Deliberately not part of `lint`: it reaches the network to ask what the remote
# advertises, and it is a gate for making the repository public rather than a
# per-commit check.
publishable ::
	$(LOG) "Checking nothing unpublishable is in the repository"
	@tools/check-publishable

# nfpm builds all three package formats from packaging/nfpm.yaml. It is not in
# Arch's repositories -- `yay -S nfpm-bin` -- and CI installs it on the runner.
nfpm ::
	@command -v nfpm >/dev/null || { echo "nfpm is not installed: yay -S nfpm-bin"; exit 1; }

package-deb :: build completions man nfpm
	$(LOG) "Packaging botropolis ${VERSION} as a deb"
	@mkdir -p dist
	@VERSION=${VERSION} GOARCH=${GOARCH} nfpm package --config packaging/nfpm.yaml --packager deb --target dist/

package-rpm :: build completions man nfpm
	$(LOG) "Packaging botropolis ${VERSION} as an rpm"
	@mkdir -p dist
	@VERSION=${VERSION} GOARCH=${GOARCH} nfpm package --config packaging/nfpm.yaml --packager rpm --target dist/

package-archlinux :: build completions man nfpm
	$(LOG) "Packaging botropolis ${VERSION} for Arch"
	@mkdir -p dist
	@VERSION=${VERSION} GOARCH=${GOARCH} nfpm package --config packaging/nfpm.yaml --packager archlinux --target dist/

package :: package-deb package-rpm package-archlinux

# The units, desktop entry, icon and shell shim travel with the binaries, so a
# package built from this tarball installs the same set as one built from source.
# Without them botropolis-bin could only ship three executables.
# Generated by the binary's own `completion` command (cobra), for the shells
# shell-init supports. The scripts are the same on every architecture, so they
# are made with the host's toolchain: GOARCH is unset because a cross build
# passes it on the command line, and make exports that into every recipe.
completions ::
	$(LOG) "Generating shell completions into bin/completions/"
	@mkdir -p bin/completions
	@env -u GOARCH -u GOOS go run ./cmd/botropolis completion bash > bin/completions/botropolis.bash
	@env -u GOARCH -u GOOS go run ./cmd/botropolis completion zsh > bin/completions/_botropolis
	@env -u GOARCH -u GOOS go run ./cmd/botropolis completion fish > bin/completions/botropolis.fish

# The manual pages, from the command trees and the config keys, gzipped, with
# the date of the commit built so a package builds the same twice (item 167).
# Made with the host's toolchain, as the completions are.
MAN_DATE ?= $(shell git log -1 --no-show-signature --format=%cs 2>/dev/null || date -u +%F)

man ::
	$(LOG) "Writing the manual pages into bin/man/"
	@rm -rf bin/man
	@env -u GOARCH -u GOOS go run -ldflags "${LDFLAGS}" ./tools/man -out bin/man -date ${MAN_DATE}

tarball :: build completions man
	$(LOG) "Tarring botropolis ${VERSION} with its licences and packaging"
	@mkdir -p dist
	@rm -rf dist/botropolis-${VERSION}
	@mkdir -p dist/botropolis-${VERSION}
	@cp ${BINARIES:%=bin/%} dist/botropolis-${VERSION}/
	@cp LICENSE README.md CHANGELOG.md dist/botropolis-${VERSION}/
	@cp pkg/assets/fonts/inter/LICENSE.txt dist/botropolis-${VERSION}/inter-OFL.txt
	@cp pkg/assets/fonts/gomono/LICENSE dist/botropolis-${VERSION}/gomono-BSD.txt
	@cp pkg/assets/kits/nature-kit/License.txt dist/botropolis-${VERSION}/kenney-CC0.txt
	@cp pkg/assets/kits/README.md dist/botropolis-${VERSION}/kenney-kits.md
	@cp -r packaging dist/botropolis-${VERSION}/packaging
	@cp -r bin/completions dist/botropolis-${VERSION}/completions
	@cp -r bin/man dist/botropolis-${VERSION}/man
	@rm -f dist/botropolis-${VERSION}/packaging/nfpm.yaml
	@tar -C dist -czf dist/botropolis-${VERSION}-linux-${GOARCH}.tar.gz botropolis-${VERSION}
	@rm -rf dist/botropolis-${VERSION}

# botropolis-* misses the deb, which is botropolis_0.1.0_amd64.deb by Debian
# convention. Everything in dist/ is checksummed, and the count is asserted so a
# naming convention nobody expected cannot quietly drop a file again.
checksums ::
	$(LOG) "Writing SHA256SUMS"
	@cd dist >/dev/null && rm -f SHA256SUMS && sha256sum $$(ls | grep -v '^SHA256SUMS$$') > SHA256SUMS
	@cd dist >/dev/null && n=$$(ls | grep -cv '^SHA256SUMS$$') && m=$$(wc -l < SHA256SUMS) && \
		[ "$$n" = "$$m" ] || { echo "SHA256SUMS covers $$m of $$n files in dist/"; exit 1; }

format ::
	$(LOG) "Formatting"
	@gofmt -l -w .

clean ::
	$(LOG) "Cleaning"
	@rm -rf bin dist coverage.out
	@go clean -testcache

analyze ::
	$(LOG) "Analyzing ~/.claude history"
	@python3 tools/analyze-history.py

# Recorded from the scrubbed fixture, not from the machine it is run on. The
# committed GIF used to be of the author's own city, so it carried project names
# on the district plates, MCP vendor names on the towers and a real daily spend
# across the strip -- published, in a repository whose history had just been
# rewritten to remove exactly that. Pointing this at ~/.claude is opt-in now.
gif :: build-dev fixtures
	$(LOG) "Recording docs/botropolis.gif headlessly from the ${FIXTURE} fixture (24 s)"
	@rm -rf dist/frames && mkdir -p dist/frames
	@bin/botropolis city --headless --record dist/frames --seconds 24 \
		--home testing/helpers/fixtures/${FIXTURE}/home --socket /nonexistent/botropolis.sock \
		--keys n,n,equal,equal,tab,r,r,b,b,x,escape,t,escape,slash,m,a,r,q,enter,f
	@ffmpeg -loglevel error -y -framerate 10 -i dist/frames/frame-%05d.png -vf "fps=8,scale=880:-1:flags=lanczos,split[s0][s1];[s0]palettegen=max_colors=160[p];[s1][p]paletteuse=dither=bayer:bayer_scale=4" docs/botropolis.gif
	@ls -la docs/botropolis.gif | awk '{print $$5 " bytes"}'

kits ::
	$(LOG) "Fetching the Kenney kits into tools/kits"
	@tools/fetch-kits

sprites :: kits
	$(LOG) "Cutting the kit atlases into pkg/assets/kits (tools/render-sprites)"
	@blender -b --python tools/render-sprites/render.py -- atlas --out pkg/assets/kits 2>&1 | grep -E "WROTE|BUDGET|Traceback|Error" || true
	@tools/shrink-pngs pkg/assets/kits/kits-z*.png

# Cheap: re-encodes the nine shipped pages and compares, no Blender. This
# is the guard for item 63 — render.py writing the pages and shrink-pngs
# compressing them were two steps with nothing checking that both ran,
# and nine pages shipped unshrunk for fifty revisions because of it.
atlas-shrunk ::
	$(LOG) "Checking every shipped atlas page is at full compression"
	@tools/shrink-pngs --check pkg/assets/kits/kits-z*.png

sprites-check :: sprites atlas-shrunk
	$(LOG) "Checking the re-render against the committed atlases"
	@python3 tools/render-sprites/pack.py --self-test
	@python3 tools/atlas-diff.py --self-test
	@python3 tools/atlas-diff.py

kit-district :: kits
	$(LOG) "Rendering one district from the Kenney kits (tools/render-sprites)"
	@blender -b --python tools/render-sprites/render.py -- scene --out docs/screenshots/kit-district.png 2>&1 | grep -E "WROTE|Traceback|Error" || true

fixtures ::
	$(LOG) "Generating fixture ${FIXTURE} from ~/.claude"
	@python3 tools/make-fixtures.py --name ${FIXTURE}

# The APT and RPM repositories at pkg.botropolis.city. CI publishes them
# from the release workflow; these build and test the same thing locally.
# The signing key is read from 1Password by reference and reaches the
# container as an environment variable, never as a file.
PKG_SIGNING_KEY_REF ?= op:Botropolis/Botropolis Packages Signing Key/botropolis-packages-secret.asc
PKG_RELEASES ?= 3

package-repos-image ::
	$(LOG) "Building the package-repository image"
	@docker build -q -t botropolis-pkgrepo packaging/repo >/dev/null

package-repos-in ::
	$(LOG) "Downloading the last ${PKG_RELEASES} releases' .deb and .rpm into .pkgrepo-in/"
	@rm -rf .pkgrepo-in && mkdir -p .pkgrepo-in
	@for tag in $$(gh release list -R Botropolis-City/botropolis --limit ${PKG_RELEASES} --json tagName -q '.[].tagName'); do \
		gh release download "$$tag" -R Botropolis-City/botropolis --dir .pkgrepo-in \
			--pattern 'botropolis_*_*.deb' --pattern 'botropolis-*-*.rpm' || exit 1; \
	done
	@ls .pkgrepo-in

package-repos :: package-repos-image
	$(LOG) "Building signed repositories into .pkgrepo/"
	@secret-run --env "PKG_SIGNING_KEY=${PKG_SIGNING_KEY_REF}" -- \
		docker run --rm -e PKG_SIGNING_KEY \
			-v "$$PWD":/work -u "$$(id -u):$$(id -g)" botropolis-pkgrepo \
			tools/build-package-repos .pkgrepo-in .pkgrepo

brew-tap ::
	$(LOG) "Building the Homebrew tap for ${VERSION} into .brewtap/brew"
	@tools/build-brew-tap ${VERSION} .brewtap/brew

brew-tap-test ::
	$(LOG) "Installing from .brewtap/ in a clean Homebrew container"
	@testing/acceptance/packages/brew-test.sh --tap .brewtap/brew ${VERSION}

package-repos-test ::
	$(LOG) "Installing from .pkgrepo/ in clean Debian and Fedora containers"
	@testing/acceptance/packages/install-test.sh --repo .pkgrepo ${VERSION}

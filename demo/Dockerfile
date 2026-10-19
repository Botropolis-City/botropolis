# The demo is filmed in containers so that nothing it touches can be the
# real home: no ~/.claude, no ~/.claude.json, no XDG state, no network.
#
#   renderer  runs botropolis headless and encodes the clips.
#   recorder  runs Claude Code against the toy projects to make the corpus.
#
# Binaries are built on the host (CGO_ENABLED=0, see the Makefile) and
# mounted in, so the image never needs rebuilding for a code change.

FROM debian:trixie-slim AS renderer
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      xvfb \
      libgl1 libegl1 libgl1-mesa-dri \
      libx11-6 libxcursor1 libxi6 libxinerama1 libxrandr2 \
      ffmpeg tzdata \
 && rm -rf /var/lib/apt/lists/*
ENV LIBGL_ALWAYS_SOFTWARE=1
WORKDIR /work

# The recorder runs the real claude CLI against the toy projects. It is
# the only container with network access, because it has to reach the
# API; it still sees only the demo directory and the corpus it writes.
# Credentials arrive as CLAUDE_CODE_OAUTH_TOKEN, forwarded by name from
# the environment `secret-run` builds, never on a command line.
FROM debian:trixie-slim AS recorder
ARG CLAUDE_CODE_VERSION=2.1.286
RUN apt-get update \
 && apt-get install -y --no-install-recommends \
      ca-certificates git make jq ripgrep procps \
      golang-go python3 python3-pytest \
      nodejs npm \
 && npm install -g --no-fund --no-audit @anthropic-ai/claude-code@${CLAUDE_CODE_VERSION} \
 && npm cache clean --force \
 && rm -rf /var/lib/apt/lists/*
RUN useradd --create-home --uid 1000 --shell /bin/bash demo
USER demo
ENV HOME=/home/demo \
    DISABLE_AUTOUPDATER=1 \
    CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 \
    GOFLAGS=-mod=mod \
    GOTOOLCHAIN=local
WORKDIR /home/demo

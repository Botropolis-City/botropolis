#!/usr/bin/env bash
set -uo pipefail

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--tap DIR | --base URL] [--native] VERSION
       $(basename "$0") --help

Taps the Botropolis Homebrew tap exactly as the install instructions say,
installs botropolis from it, runs the formula's test, and checks it reports
VERSION.

  --tap DIR    serve a tap built by tools/build-brew-tap (default: .brewtap),
               the way the bucket does: as static files
  --base URL   use a published tap instead, such as https://pkg.botropolis.city
  --native     use this machine's brew rather than a Linux container; this is
               how the macOS CI job runs it. It also starts the daemon with
               brew services, which is launchd on a Mac, and requires
               botropolis doctor to reach it: launchd hands the daemon a
               different environment from the shell the client runs in

Environment:
  BREW_IMAGE   the container (default: homebrew/brew:4.6.20)
USAGE
}

tap=.brewtap
base=
native=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h | --help) usage; exit 0 ;;
    --tap) tap=$2; shift 2 ;;
    --base) base=$2; tap=; shift 2 ;;
    --native) native=1; shift ;;
    *) break ;;
  esac
done
[[ $# -eq 1 ]] || { usage >&2; exit 2; }
version=$1
image=${BREW_IMAGE:-homebrew/brew:4.6.20}

install='
  brew tap botropolis-city/botropolis "$BASE/brew"
  brew install botropolis-city/botropolis/botropolis
  brew test botropolis-city/botropolis/botropolis
  p=$(brew --prefix)
  for f in "$p/etc/bash_completion.d/botropolis" "$p/share/zsh/site-functions/_botropolis" "$p/share/fish/vendor_completions.d/botropolis.fish"; do
    test -s "$f" || { echo "no completion at $f"; exit 1; }
  done
  for f in "$p/share/man/man1/botropolis.1.gz" "$p/share/man/man5/botropolis-config.5.gz"; do
    test -s "$f" || { echo "no manual page at $f"; exit 1; }
  done
  botropolis --version'

if ((native)); then
  if [[ -n $tap ]]; then
    port=8765
    (cd "$tap/.." && exec python3 -m http.server "$port" --bind 127.0.0.1) >/dev/null 2>&1 &
    server=$!
    trap 'kill "$server" 2>/dev/null; brew untap botropolis-city/botropolis >/dev/null 2>&1' EXIT
    base=http://127.0.0.1:$port
    for _ in $(seq 30); do curl -fsS "$base/brew/HEAD" >/dev/null 2>&1 && break; sleep 1; done
  fi
  got=$(BASE=$base bash -euc "$install" 2>&1 | tail -1)
  if [[ $got == "botropolis $version" ]]; then
    trap 'kill "${server:-}" 2>/dev/null; brew services stop botropolis >/dev/null 2>&1; brew untap botropolis-city/botropolis >/dev/null 2>&1' EXIT
    brew services start botropolis >/dev/null 2>&1
    daemon=
    for _ in $(seq 30); do
      daemon=$(botropolis doctor 2>&1 | awk '$1 == "daemon"')
      [[ $daemon == *" ok "* ]] && break
      sleep 1
    done
    if [[ $daemon != *" ok "* ]]; then
      printf '  \033[0;31m✗\033[0m brew services started botropolisd but botropolis doctor says: %s\n' "$daemon"
      tail -20 "$(brew --prefix)/var/log/botropolisd.log" 2>/dev/null | sed 's/^/    /'
      exit 1
    fi
    printf '  \033[0;32m✓\033[0m brew services runs botropolisd under launchd, and the client reaches it: %s\n' "$(echo "$daemon" | tr -s ' ')"
  fi
else
  net="brewtest-$$"
  cleanup() {
    docker rm -f "tap-$$" >/dev/null 2>&1
    docker network rm "$net" >/dev/null 2>&1
  }
  trap cleanup EXIT
  docker network create "$net" >/dev/null
  if [[ -n $tap ]]; then
    docker run -d --rm --name "tap-$$" --network "$net" --network-alias pkg \
      -v "$(realpath "$tap")":/srv/brew:ro -w /srv python:3-alpine python -m http.server 8000 >/dev/null
    base=http://pkg:8000
    for _ in $(seq 30); do
      docker run --rm --network "$net" curlimages/curl -fsS "$base/brew/HEAD" >/dev/null 2>&1 && break
      sleep 1
    done
  fi
  got=$(docker run --rm --network "$net" -e BASE="$base" -e HOMEBREW_NO_ANALYTICS=1 -e HOMEBREW_NO_ENV_HINTS=1 \
    "$image" bash -euc "$install" 2>&1 | tail -1)
fi

where=$( ((native)) && echo "$(uname -s) $(uname -m)" || echo "$image")
if [[ $got == "botropolis $version" ]]; then
  printf '  \033[0;32m✓\033[0m brew on %s installs botropolis %s from %s\n' "$where" "$version" "${base}/brew"
else
  printf '  \033[0;31m✗\033[0m brew on %s: %s\n' "$where" "$got"
  exit 1
fi

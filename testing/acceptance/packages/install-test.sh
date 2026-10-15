#!/usr/bin/env bash
set -uo pipefail

usage() {
  cat <<USAGE
Usage: $(basename "$0") [--repo DIR | --base URL] VERSION
       $(basename "$0") --help

Adds the Botropolis package repository to a clean Debian and a clean Fedora
container exactly as the install instructions say, installs botropolis, and
checks it reports VERSION. Signature checks stay on throughout: an unsigned
or wrongly signed repository fails.

  --repo DIR   serve a repository built by tools/build-package-repos (default:
               .pkgrepo). It is served from a copy whose botropolis.sources and
               botropolis.repo point at the local server; those two files are not
               signed, so everything that is stays exactly what will be published.
  --base URL   use a published repository instead, such as
               https://pkg.botropolis.city

Environment:
  DISTROS   which to test (default: "debian fedora")
USAGE
}

repo=.pkgrepo
base=
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h | --help) usage; exit 0 ;;
    --repo) repo=$2; shift 2 ;;
    --base) base=$2; repo=; shift 2 ;;
    *) break ;;
  esac
done
[[ $# -eq 1 ]] || { usage >&2; exit 2; }
version=$1

net="pkgtest-$$"
cleanup() {
  docker rm -f "pkg-$$" >/dev/null 2>&1
  docker network rm "$net" >/dev/null 2>&1
}
trap cleanup EXIT
docker network create "$net" >/dev/null

if [[ -n $repo ]]; then
  base=http://pkg:8000
  served=$(mktemp -d)
  cp -r "$repo"/. "$served"/
  sed -i -E "s#https?://[^/ ]+#${base}#" "$served/botropolis.sources" "$served/botropolis.repo"
  trap 'cleanup; rm -rf "$served"' EXIT
  docker run -d --rm --name "pkg-$$" --network "$net" --network-alias pkg \
    -v "$served":/srv:ro -w /srv python:3-alpine python -m http.server 8000 >/dev/null
  for _ in $(seq 30); do
    docker run --rm --network "$net" curlimages/curl -fsS "$base/botropolis.asc" >/dev/null 2>&1 && break
    sleep 1
  done
fi

debian() {
  docker run --rm --network "$net" debian:trixie bash -euc "
    apt-get update -qq && apt-get install -y -qq curl ca-certificates >/dev/null
    install -d /etc/apt/keyrings
    curl -fsSLo /etc/apt/keyrings/botropolis.asc $base/botropolis.asc
    curl -fsSLo /etc/apt/sources.list.d/botropolis.sources $base/botropolis.sources
    apt-get update -qq
    apt-get install -y -qq botropolis >/dev/null
    for f in /usr/share/bash-completion/completions/botropolis /usr/share/zsh/site-functions/_botropolis /usr/share/fish/vendor_completions.d/botropolis.fish; do test -s \$f || { echo \"no completion at \$f\"; exit 1; }; done
    botropolis --version"
}

fedora() {
  docker run --rm --network "$net" fedora:latest bash -euc "
    curl -fsSLo /etc/yum.repos.d/botropolis.repo $base/botropolis.repo
    dnf install -y -q botropolis >/dev/null
    rpm -q --qf '%{SIGPGP:pgpsig}%{RSAHEADER:pgpsig}\n' botropolis | grep -q 'Key ID'
    for f in /usr/share/bash-completion/completions/botropolis /usr/share/zsh/site-functions/_botropolis /usr/share/fish/vendor_completions.d/botropolis.fish; do test -s \$f || { echo \"no completion at \$f\"; exit 1; }; done
    botropolis --version"
}

failed=0
for distro in ${DISTROS:-debian fedora}; do
  got=$("$distro" 2>&1 | tail -1)
  if [[ $got == "botropolis $version" ]]; then
    printf '  \033[0;32m✓\033[0m %s installs botropolis %s from %s\n' "$distro" "$version" "$base"
  else
    printf '  \033[0;31m✗\033[0m %s: %s\n' "$distro" "$got"
    failed=1
  fi
done
exit $failed

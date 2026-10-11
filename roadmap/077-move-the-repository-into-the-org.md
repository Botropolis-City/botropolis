# 77. Move the repository into the Botropolis-City org, under a vanity import path

**Done 2026-10-10 (`98cda52`, r380; released as v0.1.5 at `a1586dd`, r385). Filed 2026-10-10 (r376). Aria moved the repository; the module path followed.**

The GitHub org is `Botropolis-City`, because `botropolis` was taken while this project was paused.
The website and its infrastructure are already there; this repository is still `auroq/botropolis`.

GitHub redirects the old URLs after a transfer, but the name is written into the module path in `go.mod` and every import, `MODULE` in the `Makefile`, the homepage in `packaging/nfpm.yaml`, both systemd units, `SECURITY.md`, the issue template, `CHANGELOG.md`'s link references, the AUR `PKGBUILD`s, and the website's links and install commands.

**The suggestion: make the module path `botropolis.city/botropolis` rather than `github.com/Botropolis-City/botropolis`.**
The site serves the `<meta name="go-import">` that makes it resolve, so the module path stops depending on which org holds the repository.
Given how the org name was settled, that independence is the point: the next move costs a redirect, not a rewrite of every import.

Doing it in the same change as the transfer means the import paths are rewritten once.
Item 72's checks (`make publishable`) should run after it, since the transfer changes what the remote advertises.

## Closed

Aria transferred the repository to `Botropolis-City` on 2026-10-10, and `98cda52` followed it:

- The module path is `botropolis.city/botropolis`, as recommended, across `go.mod`, every import and `MODULE` in the `Makefile`. Three files had a project import in the standard-library group, which sorted to the top once the path began with a `b`; each was moved to the project's group.
- Links to the repository moved: the issue template, `SECURITY.md`, `CHANGELOG.md`'s references and the README. `packaging/nfpm.yaml`'s homepage and both systemd units' `Documentation=` point at `https://botropolis.city`.
- The website serves `go-import` and `go-source` at `/botropolis` and each command under it (site repo `18a7c20`), with a test per path that also runs against production after every deploy.
- Checked with the real tool: `GOPROXY=direct go list -m botropolis.city/botropolis@v0.1.5` resolves through the site to the new repository.
- Both AUR packages point at the new URL, at 0.1.5. The source package had derived the `-X` version stamp's path from `url`, which the move would have broken silently; it now names the module itself, and the built package reports `botropolis 0.1.5`.

v0.1.4 still declares the old module path, so `@v0.1.4` cannot be fetched under the new one; `@latest` is v0.1.5 and works.
Item 72's `make publishable` ran against the new remote, without the token list (see item 72's closing).


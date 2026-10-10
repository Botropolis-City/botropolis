# 77. Move the repository into the Botropolis-City org, under a vanity import path

**Open. Filed 2026-10-10 (r376). Aria's call — when, and whether before item 72's visibility flip.**

The GitHub org is `Botropolis-City`, because `botropolis` was taken while this project was paused.
The website and its infrastructure are already there; this repository is still `auroq/botropolis`.

GitHub redirects the old URLs after a transfer, but the name is written into the module path in `go.mod` and every import, `MODULE` in the `Makefile`, the homepage in `packaging/nfpm.yaml`, both systemd units, `SECURITY.md`, the issue template, `CHANGELOG.md`'s link references, the AUR `PKGBUILD`s, and the website's links and install commands.

**The suggestion: make the module path `botropolis.city/botropolis` rather than `github.com/Botropolis-City/botropolis`.**
The site serves the `<meta name="go-import">` that makes it resolve, so the module path stops depending on which org holds the repository.
Given how the org name was settled, that independence is the point: the next move costs a redirect, not a rewrite of every import.

Doing it in the same change as the transfer means the import paths are rewritten once.
Item 72's checks (`make publishable`) should run after it, since the transfer changes what the remote advertises.

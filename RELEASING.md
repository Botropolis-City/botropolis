# Releasing

The steps a release does not ship without, in order.
What each piece of the release machinery does, and why, is in the Releasing section of [CLAUDE.md](CLAUDE.md); this is the list to work down.
Paths outside this repository are in the Botropolis-City meta repository, the directory above this one when it is checked out there.

1. **Edit `VERSION`, and add the matching `CHANGELOG.md` section.**
2. **`make version-check`**, which fails, naming files, when `VERSION`, the changelog and the built binaries disagree.
3. **Push to `main` with `make land`, and let CI finish.**
   CI builds the tarball, `.deb`, `.rpm`, Arch package and `SHA256SUMS` that the release will publish, so the commit it built is the one under test from here on.
4. **Soak that build overnight, and do not ship it over budget** (item 164).
   From the meta repository, `make perf-soak PERF_BUILD=<dir>`, where `<dir>` holds that commit's `botropolis` and `botropolisd`.
   It runs eight hours, so start it when nobody will use the machine until morning, never while someone works at it (Aria, 2026-10-25).
   It exits 0 within the growth budget of 1 MiB an hour, 1 over it, and 2 when the run is not a measurement, such as a screen that blanked; only 0 lets the release go on.
   Write its numbers up as a report in `performance/reports/`, following `performance/templates/report.md`, and file a numbered item for anything that grows.
   The method and its traps are run 8 in `performance/method.md`.
5. **Re-film the demo and website frames, once for the release** (Aria's ruling: once per release, not per item).
   `make demo-media` films the scenarios here, and `make media-import` in botropolis.city brings the clips onto the website.
   Aria judges the frames before they go out.
6. **Run the release workflow with `dry_run` first, then for real.**
   It reads `VERSION`, refuses a version already tagged, downloads the artifact CI built in step 3, and tags that commit; the package repositories and the Homebrew tap publish from it.

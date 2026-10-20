# Data files from earlier versions

Each file here is a state file as an earlier Botropolis wrote it, named `<kind>-<version>.<ext>`.
1.0.0 promises that every later 1.x reads each one without losing what it holds (item 149),
and `TestSurface` in `../../surface_test.go` loads every file here and checks that it did.

Each holds the same thing: the session `aaaaaaaa-0000-0000-0000-000000000149` at `/w/lib`,
in slot 4 of its district where the file has slots, and one message with an input of 10 where it has usage.

**Never edit or delete a file here.**
Each line of `../surface/data.txt` is one of these files,
so deleting one is a removal that `make version-check` refuses without a *Removed* changelog entry and a major release.
When a format gets a new version, add a file for it.

| file | written by |
| --- | --- |
| `layout-1.json` | every build up to 0.2.2, before `readableFrom` |
| `ledger-1.json` | every build up to 0.2.2, when the ledger was a bare map |
| `ledger-2.json` | 0.2.3 on |
| `stats-1.db` | item 110's builds (schema at `d00cd8a`) |
| `stats-2.db` | item 116's first step (`6919f76`) |
| `stats-3.db` | item 116's second step (`6fd298a`) on |

The `stats-*.db` files are built from the SQL in `../data-sources/` by `build-stats.sh`:
the version 1 schema taken from `d00cd8a`, then each step the store applies.

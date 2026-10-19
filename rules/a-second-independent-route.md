## Verification means a second independent route, and the two must be able to disagree out loud

Every remedy in the week that produced the other rules in this directory was the same shape, arrived at four times without either session noticing until the end.

| the thing to be trusted | the second route |
| --- | --- |
| a number | derive it again a different way — count the complement, the total, the same set grouped another way |
| a link graph | follow references the other direction; outward asks whether a link resolves, inward asks whether a file is pointed at |
| a check | a second caller, so the checks survive losing either one |
| a bound | an independent source, so the fault cannot move the reference it is measured against |

That is the first half and it is the well-known half.
The second half is the one that cost every round: **a second route is worthless while its answer is indistinguishable from the first's.**

In each case the failure was not that the two routes disagreed and nobody looked.
It was that the output was the same either way, so there was nothing to look at.

- `VmRSS` agreeing across two rigs read exactly like a measurement confirmed, when the samples were in different states.
- A link check that tried the bare path before the relative one printed `0 broken` for a link that did not resolve.
- An empty documentation tree printed `0 links resolve, 0 items carry a status, open table matches` and exited 0.
- A bound taken from the working tree printed `1-66, none missing` after two files were deleted.
- `except: pass` printed a healthy summary when the independent source was unreachable.
- `go test` printed `ok (cached)` while the script it calls failed on the same tree, because the cache keyed on files the test binary opened and the subprocess's reads were invisible to it.
- `go test` printed `ok (cached)` for the acceptance tests after the binary they drive had changed, because the test binary did not link that binary's code; a blank import then passed the probe meant to catch it, only because the probe added a line and shifted every later line number (item 117).

Seven greens, none of them lying about anything it actually asserted, and not one distinguishable from a green that meant what it appeared to mean.

**So the test of a verification is not "did I check it twice" but "would the second check print something different if it were the one that was wrong".**
If it would not, there is one route wearing two coats.
The practical form: make the degraded case say it is degraded — the note on an unreachable `git`, the row that names which source a bound came from, `THIS IS NOT A MEASUREMENT` on a zero-frame sample — and prefer a check that names what it found over one that reports a count of nothing.

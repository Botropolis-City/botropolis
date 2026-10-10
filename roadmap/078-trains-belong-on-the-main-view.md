# 78. Trains belong on the main view, as the cars do

**Open. Filed 2026-10-10 (r377) at Aria's request: the trains that run round the outside should always be visible, like the cars. Nobody's call — she has asked for it, and the change is small.**

Bug 48 put the cars on the main view by adding `NetworkTraffic` to `ViewAttention` in `shown` (`pkg/city/network.go`).
The freight loop never got the same treatment: `NetworkFreight` is drawn only by `ViewSpend`, so on the view the city opens in, the trains are missing.
The track is not gated — `isoRails` draws it unconditionally (`pkg/render/iso.go`) — so the main view shows a railway round every district with nothing running on it.
The demo frames show exactly that: every metropolis, workshop and workday still has the loop and no trains.

**The change:** `ViewAttention: {NetworkTraffic, NetworkFreight}`.
The gate in `freight()` already reads `shown`, so nothing else moves.

What has to change with it:
- `pkg/city/network_test.go` asserts that the attention view shows no network but traffic; it has to name freight as the second exception, or it fails as it should.
- The comment over `trafficCars` and `freight` in `pkg/render/iso.go` says "Attention pays for neither", which has been wrong since bug 48 and would be wrong twice.
- Bug 48's reasoning carries over and should be recorded the same way: the info-view model is subtractive on purpose, and a train on the base view is motion the view does not explain. It is safe for the same reason the car is — a train's card names its model, its tokens, the unit a wagon stands for and the cost.
- A day with no tokens has no trains at all (`trains()` returns nothing for an empty breakdown), so "always visible" means "whenever there is a day's spend to carry". If an idle city should still run a train, that is a separate question: an empty train would be a train that means nothing.
- Cost: the carriages join the depth-sorted list on the main view, which is the view the CPU bar (item 69) is measured on.

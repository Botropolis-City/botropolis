# 75. A road's label is drawn under the tower beside it

**Open. Filed 2026-10-10 (r376), from the demo frames. Nobody's call — a defect.**

The traffic label `1 file` on the avenue between thistledown and copperleaf is drawn before thistledown's tower, so the tower covers its right half.
Labels are chrome and should draw over the city, or move off whatever stands on them, the way the tower signage in [later.md](later.md) does.

Seen in `metropolis/overview.png`, `metropolis/plain.png` and `metropolis/sidebar.png` (around (1185, 487) at 1920×1080), and in `metropolis/flyover.mp4` whenever that avenue is in shot.
The flat projection (`workshop/flat.png`) has a road label too, at (743, 388), and it is not covered there, so the fault is in the isometric draw order rather than in placing the label.

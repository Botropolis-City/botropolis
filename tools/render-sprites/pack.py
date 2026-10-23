#!/usr/bin/env python3
"""Pack sprites onto atlas pages so the pages cost the least texture.

Ebitengine rounds every texture up to a power of two a side, so a page
costs PAGE times the power of two at or above its height, and what a
level costs is the sum (item 162). Shelved in the order they were cut,
z2's sprites took five full pages and a part, 88 MiB; filled one page at
a time, best short side first, they take four and a 2048x1024 one, 72.

pack() fills a page with every remaining sprite that fits, MaxRects with
the best short-side fit, before opening the next, and tries a few
orderings, keeping the cheapest. It is deterministic: the same sizes
always land in the same places, so a re-cut diffs cleanly.

Standard library only, so render.py under Blender and tools/repack-atlas
both use it, and --self-test runs anywhere.
"""

import sys

PAGE = 2048
PAD = 2

ORDERS = (
    ("height", lambda wh: (-wh[1], -wh[0])),
    ("area", lambda wh: (-wh[0] * wh[1], -wh[1])),
    ("longest side", lambda wh: (-max(wh), -min(wh))),
    ("width", lambda wh: (-wh[0], -wh[1])),
)


def side(n):
    """A texture side as Ebitengine allocates it."""
    s = 16
    while s < n:
        s <<= 1
    return s


def fill(sizes, order, page=PAGE, pad=PAD):
    """Place sizes, indexed, in that order: each page takes every one that
    fits before the next opens. Returns {index: (page, x, y)} and each
    page's height, down to its lowest sprite's bottom plus the pad."""
    room = page - pad
    remaining = list(order)
    placed, heights = {}, []
    while remaining:
        free = [(0, 0, room, room)]
        used, left = 0, []
        for i in remaining:
            w, h = sizes[i][0] + pad, sizes[i][1] + pad
            if w > room or h > room:
                raise ValueError(f"a sprite of {sizes[i][0]}x{sizes[i][1]} does not fit a {page} page")
            best = None
            for fx, fy, fw, fh in free:
                if w <= fw and h <= fh:
                    score = (min(fw - w, fh - h), max(fw - w, fh - h), fy, fx)
                    if best is None or score < best[0]:
                        best = (score, fx, fy)
            if best is None:
                left.append(i)
                continue
            _, x, y = best
            placed[i] = (len(heights), x + pad, y + pad)
            used = max(used, y + h + pad)
            free = split(free, x, y, w, h)
        if len(left) == len(remaining):
            raise ValueError("no sprite fits an empty page")
        heights.append(used)
        remaining = left
    return placed, heights


def split(free, x, y, w, h):
    """The free rectangles left once (x, y, w, h) is taken."""
    out = []
    for fx, fy, fw, fh in free:
        if x >= fx + fw or x + w <= fx or y >= fy + fh or y + h <= fy:
            out.append((fx, fy, fw, fh))
            continue
        if x > fx:
            out.append((fx, fy, x - fx, fh))
        if x + w < fx + fw:
            out.append((x + w, fy, fx + fw - x - w, fh))
        if y > fy:
            out.append((fx, fy, fw, y - fy))
        if y + h < fy + fh:
            out.append((fx, y + h, fw, fy + fh - y - h))
    return [
        a
        for k, a in enumerate(out)
        if not any(
            j != k and b[0] <= a[0] and b[1] <= a[1] and a[0] + a[2] <= b[0] + b[2] and a[1] + a[3] <= b[1] + b[3]
            and (b != a or j < k)
            for j, b in enumerate(out)
        )
    ]


def cost(heights, page=PAGE):
    """Bytes of texture the pages take at Ebitengine's sizes."""
    return sum(4 * side(page) * side(h) for h in heights)


def pack(sizes, page=PAGE, pad=PAD):
    """The cheapest placement of sizes [(w, h), ...] over ORDERS:
    ({index: (page, x, y)}, [page heights])."""
    best = None
    for _, key in ORDERS:
        order = sorted(range(len(sizes)), key=lambda i: (key(sizes[i]), i))
        placed, heights = fill(sizes, order, page, pad)
        if best is None or cost(heights, page) < cost(best[1], page):
            best = (placed, heights)
    return best


def self_test():
    import random

    random.seed(162)
    sizes = [(random.randrange(4, 60), random.randrange(4, 60)) for _ in range(300)]
    placed, heights = pack(sizes, page=256, pad=2)
    cases = []

    def box(i):
        p, x, y = placed[i]
        return p, x - 2, y - 2, x + sizes[i][0], y + sizes[i][1]

    cases.append(("every rectangle is placed", sorted(placed) == list(range(len(sizes)))))
    inside = all(
        x >= 2 and y >= 2 and x + sizes[i][0] <= 256 - 2 and y + sizes[i][1] <= heights[p] - 2
        for i, (p, x, y) in placed.items()
    )
    cases.append(("each lies inside its page, the pad included", inside))
    overlap = False
    by_page = {}
    for i in placed:
        by_page.setdefault(placed[i][0], []).append(box(i))
    for boxes in by_page.values():
        for a in range(len(boxes)):
            for b in range(a + 1, len(boxes)):
                _, ax0, ay0, ax1, ay1 = boxes[a]
                _, bx0, by0, bx1, by1 = boxes[b]
                if ax0 < bx1 and bx0 < ax1 and ay0 < by1 and by0 < ay1:
                    overlap = True
    cases.append(("none overlaps another, the pad included", not overlap))
    cases.append(("the same sizes land in the same places", pack(sizes, page=256, pad=2) == (placed, heights)))
    try:
        fill([(300, 10)], [0], page=256)
        cases.append(("a sprite wider than a page is refused", False))
    except ValueError:
        cases.append(("a sprite wider than a page is refused", True))
    cases.append(("a page costs its power-of-two size", cost([700], page=2048) == 4 * 2048 * 1024))

    ok = True
    print("pack.py --self-test:")
    for name, passed in cases:
        print(f"  {'ok  ' if passed else 'FAIL'} {name}")
        ok = ok and passed
    return 0 if ok else 1


if __name__ == "__main__":
    if len(sys.argv) > 1 and sys.argv[1] in ("-h", "--help"):
        print(__doc__)
        print("Usage: tools/render-sprites/pack.py --self-test")
        sys.exit(0)
    if len(sys.argv) > 1 and sys.argv[1] == "--self-test":
        sys.exit(self_test())
    print("Usage: tools/render-sprites/pack.py --self-test", file=sys.stderr)
    sys.exit(2)

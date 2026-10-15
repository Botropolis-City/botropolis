#!/usr/bin/env python3
"""Compare the atlases in the working tree with the committed ones.

The render is not bit-exact. Eevee at 32 TAA samples under software GL
reproduces a page very nearly but not exactly, so a byte comparison
fails on a handful of pixels that nothing in the city can see. A gate
that cries wolf is a gate nobody reads.

So: the manifests must match byte for byte, because every number in
them is a decision the pipeline made; the pages are compared as decoded
pixels, and a page passes while fewer than --tolerance of its bytes
differ. Anything larger is a real change and fails.

Standard library only: PNG here is always 8-bit RGBA, so zlib and the
five PNG filters are the whole decoder.
"""

import argparse
import struct
import subprocess
import sys
import zlib

# Bytes of decoded pixel that may differ in a page before the check
# calls it a change. Measured on a no-op render; see
# roadmap item 65 in the Botropolis-City meta repository.
DEFAULT_TOLERANCE = 400


def chunks(data):
    """Every chunk of a PNG as (type, payload), in file order."""
    if data[:8] != b"\x89PNG\r\n\x1a\n":
        raise ValueError("not a PNG")
    out, i = [], 8
    while i + 8 <= len(data):
        (length,) = struct.unpack(">I", data[i : i + 4])
        kind = data[i + 4 : i + 8].decode("latin1")
        out.append((kind, data[i + 8 : i + 8 + length]))
        i += 12 + length
    return out


def decode(data):
    """An 8-bit RGBA PNG as raw pixel bytes, unfiltered."""
    header, idat = None, bytearray()
    for kind, payload in chunks(data):
        if kind == "IHDR":
            header = struct.unpack(">IIBBBBB", payload)
        elif kind == "IDAT":
            idat += payload
    if header is None:
        raise ValueError("no IHDR")
    width, height, depth, colour, compression, filtering, interlace = header
    if (depth, colour, compression, filtering, interlace) != (8, 6, 0, 0, 0):
        raise ValueError(f"only 8-bit RGBA, uninterlaced: got {header}")
    raw = zlib.decompress(bytes(idat))
    stride = width * 4
    out = bytearray(stride * height)
    previous = bytearray(stride)
    at = 0
    for row in range(height):
        method = raw[at]
        at += 1
        line = bytearray(raw[at : at + stride])
        at += stride
        if method == 1:  # Sub
            for i in range(4, stride):
                line[i] = (line[i] + line[i - 4]) & 0xFF
        elif method == 2:  # Up
            for i in range(stride):
                line[i] = (line[i] + previous[i]) & 0xFF
        elif method == 3:  # Average
            for i in range(stride):
                left = line[i - 4] if i >= 4 else 0
                line[i] = (line[i] + ((left + previous[i]) >> 1)) & 0xFF
        elif method == 4:  # Paeth
            for i in range(stride):
                left = line[i - 4] if i >= 4 else 0
                up = previous[i]
                upleft = previous[i - 4] if i >= 4 else 0
                p = left + up - upleft
                pa, pb, pc = abs(p - left), abs(p - up), abs(p - upleft)
                if pa <= pb and pa <= pc:
                    guess = left
                elif pb <= pc:
                    guess = up
                else:
                    guess = upleft
                line[i] = (line[i] + guess) & 0xFF
        elif method != 0:
            raise ValueError(f"unknown PNG filter {method}")
        out[row * stride : (row + 1) * stride] = line
        previous = line
    return width, height, bytes(out)


def differing(a, b):
    """How many bytes of two equal-length buffers differ."""
    if len(a) != len(b):
        return max(len(a), len(b))
    return sum(1 for x, y in zip(a, b) if x != y)


def committed(repo, path):
    """A path's content at HEAD, or None when it is not committed."""
    done = subprocess.run(
        ["git", "-C", repo, "show", "HEAD:" + path], capture_output=True
    )
    return done.stdout if done.returncode == 0 else None


def changed_paths(repo, folder):
    done = subprocess.run(
        ["git", "-C", repo, "diff", "--name-only", "--", folder],
        capture_output=True,
        text=True,
        check=True,
    )
    return [p for p in done.stdout.split() if p]


def compare(repo, folder, tolerance, out=sys.stdout):
    """Report every atlas file that differs. Returns the exit status."""
    paths = changed_paths(repo, folder)
    if not paths:
        print("atlas-diff: the re-render is byte-identical", file=out)
        return 0
    status, worst = 0, 0
    for path in sorted(paths):
        old = committed(repo, path)
        new = open(f"{repo}/{path}", "rb").read()
        if old is None:
            print(f"  {path}: not committed — a new file is a change", file=out)
            status = 1
            continue
        if path.endswith(".json"):
            print(f"  {path}: the manifest differs, byte for byte", file=out)
            status = 1
            continue
        try:
            wa, ha, pa = decode(old)
            wb, hb, pb = decode(new)
        except ValueError as err:
            print(f"  {path}: cannot decode ({err})", file=out)
            status = 1
            continue
        if (wa, ha) != (wb, hb):
            print(f"  {path}: {wa}x{ha} became {wb}x{hb}", file=out)
            status = 1
            continue
        moved = differing(pa, pb)
        worst = max(worst, moved)
        verdict = "within the noise floor" if moved <= tolerance else "A REAL CHANGE"
        print(
            f"  {path}: {moved} of {len(pa)} decoded bytes moved — {verdict}",
            file=out,
        )
        if moved > tolerance:
            status = 1
    if status == 0:
        print(
            f"atlas-diff: every page is within {tolerance} bytes "
            f"(worst {worst}); the manifests are identical",
            file=out,
        )
    return status


def self_test():
    """Exercise the decoder and the comparison on pages we build here,
    so the gate is checked before it judges anything."""
    import io
    import random

    def png(pixels, width, height, method=0):
        """Encode with one PNG filter throughout, so the decoder's five
        branches are all walked by the test rather than assumed."""
        raw = bytearray()
        stride = width * 4
        previous = bytearray(stride)
        for row in range(height):
            line = pixels[row * stride : (row + 1) * stride]
            raw.append(method)
            encoded = bytearray(stride)
            for i in range(stride):
                left = line[i - 4] if i >= 4 else 0
                up = previous[i]
                upleft = previous[i - 4] if i >= 4 else 0
                if method == 0:
                    guess = 0
                elif method == 1:
                    guess = left
                elif method == 2:
                    guess = up
                elif method == 3:
                    guess = (left + up) >> 1
                else:
                    p = left + up - upleft
                    pa, pb, pc = abs(p - left), abs(p - up), abs(p - upleft)
                    guess = left if (pa <= pb and pa <= pc) else (up if pb <= pc else upleft)
                encoded[i] = (line[i] - guess) & 0xFF
            raw += encoded
            previous = bytearray(line)
        body = zlib.compress(bytes(raw))
        out = bytearray(b"\x89PNG\r\n\x1a\n")
        for kind, payload in (
            (b"IHDR", struct.pack(">IIBBBBB", width, height, 8, 6, 0, 0, 0)),
            (b"IDAT", body),
            (b"IEND", b""),
        ):
            out += struct.pack(">I", len(payload)) + kind + payload
            out += struct.pack(">I", zlib.crc32(kind + payload) & 0xFFFFFFFF)
        return bytes(out)

    random.seed(20260921)
    width = height = 16
    pixels = bytes(random.randrange(256) for _ in range(width * height * 4))
    page = png(pixels, width, height)

    cases = []
    w, h, decoded = decode(page)
    cases.append(("a page decodes to its own pixels", decoded == pixels and (w, h) == (width, height)))

    for method, name in enumerate(("none", "sub", "up", "average", "paeth")):
        cases.append(
            (f"the {name} filter round-trips", decode(png(pixels, width, height, method))[2] == pixels)
        )

    cases.append(
        (
            "two filterings of one page compare as no change",
            differing(decode(png(pixels, width, height, 1))[2], decode(png(pixels, width, height, 4))[2]) == 0,
        )
    )

    nudged = bytearray(pixels)
    nudged[40] = (nudged[40] + 1) & 0xFF
    cases.append(("one nudged byte counts as one", differing(pixels, bytes(nudged)) == 1))

    other = bytes(random.randrange(256) for _ in range(width * height * 4))
    cases.append(("a different page counts many", differing(pixels, other) > 100))

    cases.append(("a truncated buffer is not silently equal", differing(pixels, pixels[:-4]) > 0))

    try:
        decode(b"not a png at all")
        cases.append(("rubbish is rejected", False))
    except ValueError:
        cases.append(("rubbish is rejected", True))

    report = io.StringIO()
    ok = True
    for name, passed in cases:
        print(f"  {'ok  ' if passed else 'FAIL'} {name}", file=report)
        ok = ok and passed
    print("atlas-diff --self-test:", file=sys.stdout)
    sys.stdout.write(report.getvalue())
    return 0 if ok else 1


def main():
    p = argparse.ArgumentParser(
        prog="tools/atlas-diff.py",
        description=__doc__,
        formatter_class=argparse.RawDescriptionHelpFormatter,
    )
    p.add_argument("--repo", default=".", help="the repository to compare in (default: .)")
    p.add_argument(
        "--folder",
        default="pkg/assets/kits",
        help="the folder holding the atlases (default: pkg/assets/kits)",
    )
    p.add_argument(
        "--tolerance",
        type=int,
        default=DEFAULT_TOLERANCE,
        help=f"decoded bytes a page may move before it is a change (default: {DEFAULT_TOLERANCE})",
    )
    p.add_argument("--self-test", action="store_true", help="check the decoder and exit")
    args = p.parse_args()
    if args.self_test:
        return self_test()
    return compare(args.repo, args.folder, args.tolerance)


if __name__ == "__main__":
    sys.exit(main())

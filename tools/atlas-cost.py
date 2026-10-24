#!/usr/bin/env python3
"""What the atlas costs, and what dropping pieces from it would save.

Answers "is this piece worth its page?" without a twenty-minute Blender
run: it replays the pipeline's own packer, pack.py, over the sprite sizes in
the shipped manifests. The check that it is replaying rather than
approximating is that it reproduces the shipped page count exactly, which
it prints first — if that line says MISMATCH the packer has changed and
this tool is lying.

  python3 tools/atlas-cost.py              # today, and every piece's status
  python3 tools/atlas-cost.py --drop-unused  # what dropping the never-drawn ones saves

A piece counts as drawn if its full name or its bare name appears as a
literal anywhere in the tracked .go files, which is how pkg/render names
them — there is no runtime construction of piece names.
"""
import argparse, glob, importlib.util, json, os, subprocess, sys, types

REPO = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
BLENDER_ONLY = ("bpy", "bpy_extras", "bpy_extras.object_utils", "mathutils", "numpy")


def renderer():
    """render.py as the renderer sees it, loaded without Blender.

    Its Blender modules are only used inside functions, so stand-ins let the
    module body run and PIECES is the list the renderer packs, however it is
    built.
    """
    for name in BLENDER_ONLY:
        try:
            importlib.import_module(name)
        except ImportError:
            sys.modules[name] = types.ModuleType(name)
    sys.modules["bpy_extras.object_utils"].__dict__.setdefault("world_to_camera_view", None)
    sys.modules["mathutils"].__dict__.setdefault("Vector", None)
    path = os.path.join(REPO, "tools/render-sprites/render.py")
    spec = importlib.util.spec_from_file_location("render", path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


def pipeline():
    r = renderer()
    order = [f"{kit}/{n}" for kit, names in r.PIECES.items() for n in names]
    return r.PAGE, r.PAD, order, r.pack.pack


def zooms(kits):
    manifests = glob.glob(os.path.join(kits, "kits-z*.json"))
    names = [os.path.basename(m)[len("kits-"):-len(".json")] for m in manifests]
    return sorted(names, key=lambda z: float(z[1:]))


def drawn_names():
    go = subprocess.run(["bash", "-c", "cat $(git ls-files '*.go')"], cwd=REPO,
                        capture_output=True, text=True).stdout
    return lambda n: f'"{n}"' in go or f'"{n.split("/")[-1]}"' in go


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--drop-unused", action="store_true",
                    help="also price the atlas without the pieces nothing draws")
    args = ap.parse_args()
    page, pad, order, packer = pipeline()

    def pack(sizes, page, pad):
        return len(packer(sizes, page=page, pad=pad)[1])

    drawn = drawn_names()
    kits = os.path.join(REPO, "pkg/assets/kits")
    total_now = total_lean = 0
    for z in zooms(kits):
        manifest = json.load(open(os.path.join(kits, f"kits-{z}.json")))
        sp = manifest["sprites"]
        def sizes(keep):
            return [(sp[n][h]["w"], sp[n][h]["h"]) for n, h in
                    sorted((n, h) for n in order if n in sp and keep(n) for h in sp[n])]
        now = pack(sizes(lambda n: True), page, pad)
        shipped = len(manifest["pages"])
        total_now += now
        note = "replays the shipped atlas" if now == shipped else f"MISMATCH — shipped {shipped}"
        print(f"{z}: {now} pages ({note})")
        if args.drop_unused:
            lean = pack(sizes(drawn), page, pad)
            total_lean += lean
            print(f"    without the never-drawn pieces: {lean} pages ({now - lean} fewer)")
    mb = sum(os.path.getsize(os.path.join(kits, f)) for f in os.listdir(kits)
             if f.endswith(".png")) / 1e6
    print(f"\n{mb:.1f} MB over {total_now} pages")
    if args.drop_unused:
        print(f"{mb * total_lean / total_now:.1f} MB over {total_lean} pages if the never-drawn go")
    never = [n for n in order if not drawn(n)]
    print(f"\nnever drawn: {len(never)} of {len(order)} pieces")
    for n in never:
        print("   ", n)


if __name__ == "__main__":
    main()

"""The city's own buildings: one kind per district type and storey count,
modelled here like the drone and the fountain so their palette is
decided where every other colour is. Item 109.

A storey is the same height in every type, so floors count the same
across the map, and footprints widen a little up each column.
"""

import math

import bpy
from mathutils import Vector

TYPES = ("residential", "shopping", "office", "industrial")
MAX_STOREYS = 5
STOREY = 0.36
PLINTH = 0.03
# A course is the slab one compaction slides under a building: a sixth
# of a storey, in the building's own material. Up to three are drawn,
# stepping in as they rise like steps up to a door: the top one a step
# wider than the building on every side, each below a step wider again.
COURSE = STOREY / 6
COURSE_STEP = 0.025
COURSES = 3
COURSE_MATERIAL = {"residential": ("brick", "brick-trim"), "shopping": ("render", "render-trim"),
                   "office": ("slab", "office-roof"), "industrial": ("cladding", "rib")}

# Every lit window is this colour, which the pipeline's pane finder
# looks for in our pieces, so a stopped building's glass still goes dark.
WINDOW = "#ffd56b"
WINDOW_GLOW = 0.5

PALETTE = {
    "plinth": "#5d5a55",
    "brick": "#c9876a",
    "brick-b": "#b9765a",
    "brick-c": "#d59a7c",
    "party-wall": "#7d4f3f",
    "brick-trim": "#efd9c7",
    "tile-roof": "#83443a",
    "chimney": "#9a5c48",
    "flat-roof": "#c9b8a8",
    "garden": "#6aa85f",
    "hedge": "#3f6b47",
    "render": "#e8cfa8",
    "render-trim": "#f4e6cf",
    "awning-red": "#d9473a",
    "awning-teal": "#2f8f7f",
    "banner": "#f2e3b8",
    "sign-white": "#f4f1ea",
    "billboard": "#f0a818",
    "display": "#cfe3ee",
    "curtain": "#7d93ab",
    "slab": "#9aa1ab",
    "office-roof": "#c3c9d2",
    "mast": "#6b7280",
    "cladding": "#aeb4a3",
    "rib": "#8d9384",
    "shed-roof": "#7a8070",
    "skylight": "#d7e2e6",
    "door": "#4a4f55",
    "stripe": "#e0b23c",
    "vent": "#c8ccc6",
    "tank": "#d7dcd6",
    "pipe": "#8a8f96",
    "crane": "#e0a400",
    "container-red": "#c0392b",
    "container-blue": "#3c6fb0",
    "truck-cab": "#c0392b",
    "truck-box": "#eceff1",
    "yard": "#6b6a64",
    "skip": "#e0a400",
    "skip-rim": "#b88400",
    "rubble": "#7a6f62",
    "rubble-dark": "#55504a",
    "rubble-light": "#a39a8c",
    "chassis": "#33373d",
    "tyre": "#1d2026",
    "hook": "#8a8f96",
    "mixer-cab": "#3c6fb0",
    "drum": "#eceff1",
    "drum-band": "#e0b23c",
    "chute": "#6b7280",
    "cart": "#e9e4d8",
    "steel": "#a7adb5",
    "wood": "#9a6a3f",
    "wood-light": "#c89a62",
    "goods-orange": "#e8892c",
    "goods-green": "#6aa84f",
    "goods-red": "#c8402f",
    "kiosk": "#3f6f8f",
    "cup": "#f4f1ea",
    "trailer": "#d8c25a",
    "trousers": "#3b4250",
    "coat-blue": "#3c6fb0",
    "coat-plum": "#8a4f7d",
    "coat-mustard": "#d0a23a",
    "hair": "#4a3428",
}


def _srgb_to_linear(c):
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def _rgba(hex_colour):
    h = hex_colour.lstrip("#")
    return tuple(_srgb_to_linear(int(h[i:i + 2], 16) / 255) for i in (0, 2, 4)) + (1.0,)


class Model:
    """One piece being built: its root, its parts and its materials."""

    def __init__(self, name):
        self.root = bpy.data.objects.new(name, None)
        bpy.context.scene.collection.objects.link(self.root)
        self.mats = {}
        # The building's own body, without what stands beside it: its
        # width along x, depth along y and centre along x, in kit units.
        self.body = (0.0, 0.0, 0.0)
        self.furniture = False

    def mat(self, key, emission=0.0):
        k = (key, emission)
        if k not in self.mats:
            m = bpy.data.materials.new(f"{self.root.name}/{key}")
            m.use_nodes = True
            bsdf = m.node_tree.nodes["Principled BSDF"]
            colour = _rgba(WINDOW if key == "window" else PALETTE[key])
            bsdf.inputs["Base Color"].default_value = colour
            bsdf.inputs["Roughness"].default_value = 0.8
            if emission:
                bsdf.inputs["Emission Color"].default_value = colour
                bsdf.inputs["Emission Strength"].default_value = emission
            self.mats[k] = m
        return self.mats[k]

    def add(self, obj, key, emission=0.0):
        obj.data.materials.append(self.mat(key, emission))
        obj.parent = self.root
        obj["furniture"] = self.furniture
        return obj

    def box(self, size, at, key, emission=0.0):
        """A box size (x, y, z) standing with its foot centred on at."""
        bpy.ops.mesh.primitive_cube_add(size=1, location=(at[0], at[1], at[2] + size[2] / 2))
        o = bpy.context.active_object
        o.scale = size
        return self.add(o, key, emission)

    def cylinder(self, radius, depth, at, key, axis="z", vertices=20):
        rot = {"z": (0, 0, 0), "x": (0, math.pi / 2, 0), "y": (math.pi / 2, 0, 0)}[axis]
        z = at[2] + (depth / 2 if axis == "z" else 0)
        bpy.ops.mesh.primitive_cylinder_add(radius=radius, depth=depth, location=(at[0], at[1], z), rotation=rot, vertices=vertices)
        return self.add(bpy.context.active_object, key)

    def mesh(self, verts, faces, key):
        me = bpy.data.meshes.new(f"{self.root.name}/mesh")
        me.from_pydata(verts, [], faces)
        me.update()
        o = bpy.data.objects.new(f"{self.root.name}/part", me)
        bpy.context.scene.collection.objects.link(o)
        return self.add(o, key)

    def gable(self, w, d, z, rise, key, along="x", at=(0, 0), overhang=0.03):
        """A pitched roof over a w x d footprint, its ridge along x or y."""
        x0, y0 = at
        hw, hd = w / 2 + overhang, d / 2 + overhang
        if along == "x":
            v = [(x0 - hw, y0 - hd, z), (x0 + hw, y0 - hd, z), (x0 + hw, y0 + hd, z), (x0 - hw, y0 + hd, z),
                 (x0 - hw, y0, z + rise), (x0 + hw, y0, z + rise)]
            f = [(0, 1, 5, 4), (3, 4, 5, 2), (0, 4, 3), (1, 2, 5), (0, 3, 2, 1)]
        else:
            v = [(x0 - hw, y0 - hd, z), (x0 + hw, y0 - hd, z), (x0 + hw, y0 + hd, z), (x0 - hw, y0 + hd, z),
                 (x0, y0 - hd, z + rise), (x0, y0 + hd, z + rise)]
            f = [(0, 4, 5, 3), (1, 2, 5, 4), (0, 1, 4), (3, 5, 2), (0, 3, 2, 1)]
        return self.mesh(v, f, key)

    def sawtooth(self, w, d, z, teeth, rise, key, glass_key, x0=0.0):
        """A sawtooth roof: teeth along x, each a slope rising to a
        vertical glazed face."""
        step = w / teeth
        hd = d / 2
        for i in range(teeth):
            xa = x0 - w / 2 + i * step
            xb = xa + step
            v = [(xa, -hd, z), (xb, -hd, z), (xb, -hd, z + rise), (xa, hd, z), (xb, hd, z), (xb, hd, z + rise)]
            f = [(0, 2, 5, 3), (0, 1, 2), (3, 5, 4), (0, 3, 4, 1)]
            self.mesh(v, f, key)
            self.box((0.012, d * 0.86, rise * 0.7), (xb - 0.008, 0, z + rise * 0.12), glass_key)

    def windows(self, w, d, z, per_x, per_y, size, key="window", emission=WINDOW_GLOW, inset=0.08):
        """A row of lit windows round all four faces at height z."""
        ww, wh = size
        depth = 0.012
        for i in range(per_x):
            x = -w / 2 + inset + (w - 2 * inset) * (i + 0.5) / per_x
            for y in (-d / 2 - depth / 2, d / 2 + depth / 2):
                self.box((ww, depth, wh), (x, y, z), key, emission)
        for j in range(per_y):
            y = -d / 2 + inset + (d - 2 * inset) * (j + 0.5) / per_y
            for x in (-w / 2 - depth / 2, w / 2 + depth / 2):
                self.box((depth, ww, wh), (x, y, z), key, emission)

    def band(self, w, d, z, h, key, out=0.012):
        return self.box((w + 2 * out, d + 2 * out, h), (0, 0, z), key)

    def done(self):
        bpy.context.view_layer.update()
        return self.root


def footprint(storeys, base, top):
    t = (storeys - 1) / (MAX_STOREYS - 1)
    return base + (top - base) * t


def storey_z(i):
    return PLINTH + i * STOREY


def residential(m, n):
    w = footprint(n, 0.5, 0.74)
    d = w * 0.8
    if n == 3:
        d = w * 0.62
    m.body = (w, d, 0.0)
    m.box((w + 0.03, d + 0.03, PLINTH), (0, 0, 0), "plinth")
    top = storey_z(n)
    if n == 3:
        townhouses(m, w, d, top)
        return
    m.box((w, d, n * STOREY), (0, 0, PLINTH), "brick")
    for i in range(n):
        m.windows(w, d, storey_z(i) + STOREY * 0.32, max(2, round(w / 0.17)), max(1, round(d / 0.2)), (0.07, 0.13))
        if i and n >= 4:
            m.band(w, d, storey_z(i) - 0.008, 0.016, "brick-trim")
    if n <= 2:
        m.gable(w, d, top, 0.24, "tile-roof", along="x")
        if n == 2:
            m.box((0.07, 0.07, 0.22), (w * 0.28, d * 0.15, top + 0.04), "chimney")
    else:
        m.band(w, d, top, 0.04, "brick-trim")
        if n == 4:
            m.box((w - 0.04, d - 0.04, 0.012), (0, 0, top + 0.01), "flat-roof")
        else:
            m.box((w - 0.05, d - 0.05, 0.03), (0, 0, top + 0.01), "garden")
            for x in (-w * 0.3, 0, w * 0.3):
                m.box((0.09, 0.09, 0.08), (x, -d * 0.22, top + 0.03), "hedge")
            m.box((w * 0.5, 0.05, 0.05), (0, d * 0.25, top + 0.03), "hedge")


def townhouses(m, w, d, top):
    """Three narrow houses in a terrace, each its own brick and gable,
    with the party walls between them showing down both faces."""
    unit = w / 3
    depth = 0.012
    for k, brick in enumerate(("brick", "brick-b", "brick-c")):
        x = -w / 2 + unit * (k + 0.5)
        m.box((unit, d, top - PLINTH), (x, 0, PLINTH), brick)
        m.gable(unit, d, top, 0.22, "tile-roof", along="y", at=(x, 0), overhang=0.008)
        for i in range(3):
            z = storey_z(i) + STOREY * 0.32
            for y in (-d / 2 - depth / 2, d / 2 + depth / 2):
                m.box((0.07, depth, 0.13), (x, y, z), "window", WINDOW_GLOW)
        m.box((0.06, depth, STOREY * 0.5), (x + unit * 0.28, -d / 2 - depth / 2, PLINTH), "door")
    for i in range(3):
        z = storey_z(i) + STOREY * 0.32
        for y in (-d * 0.2, d * 0.2):
            for x in (-w / 2 - depth / 2, w / 2 + depth / 2):
                m.box((depth, 0.07, 0.13), (x, y, z), "window", WINDOW_GLOW)
    for k in (1, 2):
        x = -w / 2 + unit * k
        for y in (-d / 2 - 0.006, d / 2 + 0.006):
            m.box((0.016, 0.012, top - PLINTH + 0.02), (x, y, PLINTH), "party-wall")


def awnings(m, w, d, z, drop, out, key):
    """Sloped fabric awnings over a ground floor's windows on all four
    faces, each with a pale valance along its lower edge."""
    zb = z - drop
    for sy in (-1, 1):
        y0, y1 = sy * d / 2, sy * (d / 2 + out)
        m.mesh([(-w / 2, y0, z), (w / 2, y0, z), (w / 2, y1, zb), (-w / 2, y1, zb)], [(0, 1, 2, 3)], key)
        m.box((w, 0.008, 0.03), (0, y1, zb - 0.03), "sign-white")
    for sx in (-1, 1):
        x0, x1 = sx * w / 2, sx * (w / 2 + out)
        m.mesh([(x0, -d / 2, z), (x0, d / 2, z), (x1, d / 2, zb), (x1, -d / 2, zb)], [(0, 1, 2, 3)], key)
        m.box((0.008, d, 0.03), (x1, 0, zb - 0.03), "sign-white")


def sale_sign(m, w, d, height, z):
    """The sale sign: one tall board flat against the face the camera
    shows on its left, near that face's right edge, with white squares
    down it for its lettering."""
    x = w / 2 - 0.075
    y = -d / 2 - 0.009
    m.box((0.085, 0.014, height), (x, y, z), "awning-red")
    for k in range(4):
        m.box((0.05, 0.02, height * 0.13), (x, y - 0.003, z + height * (0.1 + 0.22 * k)), "sign-white")


def banner(m, w, d, z, h):
    """An advertising banner across the upper floor's two front faces: a
    framed board with a picture and a strapline on it."""
    for along_x, (cx, cy) in ((True, (0, -d / 2 - 0.016)), (False, (-w / 2 - 0.016, 0))):
        span = (w if along_x else d) * 0.66

        def board(length, thick, height, zz, key, offset=0.0):
            size = (length, thick, height) if along_x else (thick, length, height)
            at = (cx + offset, cy, zz) if along_x else (cx, cy + offset, zz)
            m.box(size, at, key)

        board(span, 0.014, h, z, "awning-teal")
        board(span * 0.9, 0.02, h * 0.74, z + h * 0.13, "banner")
        board(span * 0.3, 0.026, h * 0.5, z + h * 0.25, "awning-red", offset=-span * 0.25)
        board(span * 0.42, 0.026, h * 0.1, z + h * 0.55, "awning-teal", offset=span * 0.16)
        board(span * 0.3, 0.026, h * 0.08, z + h * 0.3, "awning-teal", offset=span * 0.1)


def shopping(m, n):
    w = footprint(n, 0.5, 0.8)
    d = w * 0.82
    m.body = (w, d, 0.0)
    m.box((w + 0.03, d + 0.03, PLINTH), (0, 0, 0), "plinth")
    top = storey_z(n)
    m.box((w, d, n * STOREY), (0, 0, PLINTH), "render")
    awning = "awning-teal" if n == 3 else "awning-red"
    big = n >= 4
    m.box((w + 0.006, d + 0.006, STOREY * (0.62 if big else 0.5)), (0, 0, PLINTH + STOREY * 0.12), "display")
    m.windows(w, d, PLINTH + STOREY * 0.2, max(2, round(w / 0.2)), max(1, round(d / 0.22)), (0.1, 0.12))
    awnings(m, w, d, PLINTH + STOREY * 0.86, STOREY * 0.22, 0.08, awning)
    for i in range(1, n):
        m.windows(w, d, storey_z(i) + STOREY * 0.32, max(2, round(w / 0.17)), max(1, round(d / 0.2)), (0.08, 0.13))
    m.band(w, d, top, 0.04, "render-trim")
    m.box((w - 0.04, d - 0.04, 0.012), (0, 0, top + 0.01), "flat-roof")
    if n == 3:
        banner(m, w, d, storey_z(2) + STOREY * 0.12, STOREY * 0.62)
    if n >= 4:
        sign_h = (n - 1) * STOREY * 0.9
        sale_sign(m, w, d, sign_h, storey_z(1) + STOREY * 0.05)
    if n == 5:
        m.furniture = True
        for x in (-w * 0.25, w * 0.25):
            m.box((0.02, 0.02, 0.12), (x, 0, top + 0.02), "mast")
        m.box((w * 0.75, 0.03, 0.16), (0, 0, top + 0.14), "billboard")
        m.box((w * 0.5, 0.034, 0.07), (0, 0, top + 0.185), "awning-red")
        m.furniture = False


def office(m, n):
    w = footprint(n, 0.48, 0.68)
    d = w
    m.body = (w, d, 0.0)
    m.box((w + 0.03, d + 0.03, PLINTH), (0, 0, 0), "plinth")
    top = storey_z(n)
    m.box((w, d, n * STOREY), (0, 0, PLINTH), "curtain")
    for i in range(n):
        z = storey_z(i)
        m.band(w, d, z, 0.035, "slab", out=0.014)
        m.windows(w, d, z + STOREY * 0.22, max(3, round(w / 0.1)), max(3, round(d / 0.1)), (0.06, STOREY * 0.55), inset=0.04)
    m.band(w, d, top, 0.04, "slab", out=0.014)
    m.box((w - 0.02, d - 0.02, 0.014), (0, 0, top + 0.03), "office-roof")
    if n == 5:
        m.box((w * 0.7, d * 0.7, 0.1), (0, 0, top + 0.04), "slab")
        m.box((w * 0.45, d * 0.45, 0.08), (0, 0, top + 0.14), "office-roof")


def industrial(m, n):
    w = footprint(n, 0.56, 0.62)
    d = w * 0.8
    shift = 0.0
    if n == 5:
        shift = -0.16
    elif n == 4:
        shift = -0.1
    elif n == 3:
        shift = -0.06
    m.body = (w, d, shift)
    m.box((w + 0.03, d + 0.03, PLINTH), (shift, 0, 0), "plinth")
    top = storey_z(n)
    body = m.box((w, d, n * STOREY), (shift, 0, PLINTH), "cladding")
    ribs = max(4, round(w / 0.06))
    for k in range(ribs + 1):
        x = shift - w / 2 + w * k / ribs
        for y in (-d / 2 - 0.004, d / 2 + 0.004):
            m.box((0.01, 0.008, n * STOREY), (x, y, PLINTH), "rib")
    for i in range(n):
        z = storey_z(i) + STOREY * 0.52
        m.box((w + 0.012, d + 0.012, 0.07), (shift, 0, z), "window", WINDOW_GLOW)
        if i == 0:
            m.box((w + 0.012, d + 0.012, 0.012), (shift, 0, storey_z(i) + 0.02), "stripe")
    for k in range(min(n, 3) if n >= 2 else 1):
        x = shift - w / 2 + w * (k + 0.5) / max(1, min(n, 3))
        m.box((0.11, 0.012, STOREY * 0.42), (x, -d / 2 - 0.007, PLINTH), "door")
    if n in (1, 2, 5):
        unit = w / 3
        for k in range(3):
            m.gable(unit, d, top, 0.16, "shed-roof", along="y", at=(shift - w / 2 + unit * (k + 0.5), 0), overhang=0.004)
    else:
        m.box((w - 0.02, d - 0.02, 0.014), (shift, 0, top), "shed-roof")
        for x, y in ((-w * 0.25, -d * 0.2), (w * 0.2, d * 0.15), (w * 0.05, -d * 0.25)):
            m.box((0.07, 0.07, 0.06), (shift + x, y, top + 0.014), "vent")
            m.box((0.08, 0.08, 0.012), (shift + x, y, top + 0.074), "vent")
    if n == 3:
        bx = shift + w / 2 + 0.11
        m.box((0.13, 0.32, 0.17), (bx, 0.04, 0.02), "truck-box")
        m.box((0.13, 0.1, 0.13), (bx, -0.18, 0.02), "truck-cab")
        m.box((0.118, 0.02, 0.05), (bx, -0.232, 0.09), "window", 0.3)
    if n == 4:
        for k, y in enumerate((-0.14, 0.14)):
            x = shift + w / 2 + 0.14
            m.cylinder(0.095, 0.5 + 0.1 * k, (x, y, 0.02), "tank")
            m.cylinder(0.1, 0.02, (x, y, 0.52 + 0.1 * k), "pipe")
            m.cylinder(0.014, 0.1, (x - 0.08, y, 0.3), "pipe", axis="x", vertices=8)
    if n == 5:
        yard_x = shift + w / 2 + 0.2
        m.box((0.36, d + 0.06, 0.012), (yard_x, 0, 0), "yard")
        m.box((0.18, 0.11, 0.1), (yard_x - 0.03, -0.12, 0.012), "container-red")
        m.box((0.18, 0.11, 0.1), (yard_x + 0.02, 0.1, 0.012), "container-blue")
        h = 0.62
        for gx in (yard_x - 0.15, yard_x + 0.15):
            for gy in (-d / 2 + 0.02, d / 2 - 0.02):
                m.box((0.022, 0.022, h), (gx, gy, 0.012), "crane")
            m.box((0.022, d - 0.02, 0.026), (gx, 0, 0.012 + h - 0.026), "crane")
        m.box((0.33, 0.04, 0.035), (yard_x, 0.02, 0.012 + h), "crane")
        m.box((0.05, 0.05, 0.04), (yard_x - 0.03, 0.02, 0.012 + h - 0.04), "door")
        m.box((0.004, 0.004, h - 0.26), (yard_x - 0.03, 0.02, 0.26), "pipe")
        m.box((0.18, 0.11, 0.1), (yard_x - 0.03, 0.02, 0.16), "container-red")


BUILD = {"residential": residential, "shopping": shopping, "office": office, "industrial": industrial}


def names():
    return [f"{t}-{n}" for t in TYPES for n in range(1, MAX_STOREYS + 1)]


# The site's own vehicles and the skip that shows how full a session's
# context is (item 109): four fills, a hook-lift truck empty and loaded,
# and a cement mixer. They are built nose along +y and turned to face -y,
# as every kit's vehicles are, so the map turns them all by one rule
# (item 151).
SKIP_FILLS = 4
VEHICLES = ["skip-truck", "skip-truck-loaded", "mixer-truck"]


# A building whose session's transcript is gone: abandoned and fallen
# apart, walls broken to uneven heights, its roof down inside them and
# rubble spilled round it, in its district type's own material.
RUINS_MATERIAL = {
    "residential": ("brick", "tile-roof", "brick-b"),
    "shopping": ("render", "awning-red", "render-trim"),
    "office": ("slab", "curtain", "office-roof"),
    "industrial": ("cladding", "shed-roof", "rib"),
}


def ruins_names():
    return [f"{t}-ruins" for t in TYPES]


def is_ruins(name):
    return name in ruins_names()


def ruins(name, at=(0, 0)):
    t = name[: -len("-ruins")]
    wall, roof, trim = RUINS_MATERIAL[t]
    m = Model(f"botropolis/{name}")
    w, d = footprint(3, 0.5, 0.7), footprint(3, 0.5, 0.7) * 0.8
    m.body = (w, d, 0.0)
    m.box((w + 0.03, d + 0.03, PLINTH), (0, 0, 0), "plinth")
    thick = 0.03
    # Each wall is broken into stretches of uneven height, with gaps.
    walls = (
        ((-w / 2, -d / 2), (w / 2, -d / 2), (0.9, 0.35, 0.0, 0.6, 0.2)),
        ((w / 2, -d / 2), (w / 2, d / 2), (0.25, 0.55, 0.15, 0.0)),
        ((w / 2, d / 2), (-w / 2, d / 2), (0.7, 0.0, 0.4, 1.1, 0.5)),
        ((-w / 2, d / 2), (-w / 2, -d / 2), (0.45, 0.8, 0.0, 0.3)),
    )
    for (x0, y0), (x1, y1), heights in walls:
        n = len(heights)
        for k, frac in enumerate(heights):
            if frac <= 0:
                continue
            a, b = k / n, (k + 1) / n
            cx, cy = x0 + (x1 - x0) * (a + b) / 2, y0 + (y1 - y0) * (a + b) / 2
            seg = math.hypot(x1 - x0, y1 - y0) / n
            size = (seg, thick, frac * STOREY) if y0 == y1 else (thick, seg, frac * STOREY)
            m.box(size, (cx, cy, PLINTH), wall if k % 2 == 0 else trim)
    # The roof, fallen in: a slab leaning from one wall to the floor.
    slab = m.box((w * 0.7, d * 0.6, 0.025), (0.02, 0.03, PLINTH + 0.12), roof)
    slab.rotation_euler = (0.35, -0.25, 0.2)
    # Rubble inside and spilled out through the gaps.
    for x, y, s in ((-0.12, -0.05, 0.06), (0.1, 0.08, 0.05), (0.0, -0.12, 0.04), (0.16, -0.14, 0.035),
                    (-0.18, 0.12, 0.045), (w / 2 + 0.04, 0.02, 0.04), (-0.05, -d / 2 - 0.05, 0.035), (0.06, d / 2 + 0.04, 0.03)):
        m.box((s, s * 1.2, s * 0.6), (x, y, PLINTH), "rubble" if s > 0.04 else "rubble-dark")
    m.root.location = Vector((at[0], at[1], 0))
    m.root["body"] = [round(v, 4) for v in m.body]
    return m.done()


# A scripted run, shown: a street stall a third of a building's footprint
# and under a storey, one kind per district type (Aria, 2026-10-26, item
# 273). Its front, where it serves, faces -y, toward the camera's corner.
STALL_W = 0.24
# Drawn at this size, a stall reads on its lot and still stands lower
# than a one-storey house.
STALL_SCALE = 1.35


def stall_names():
    return [f"{t}-stall" for t in TYPES]


def is_stall(name):
    return name in stall_names()


def stripes(m, w, d, z, rise, keys, along="x"):
    """A pitched canvas over w x d at z, striped in bands of keys."""
    n = 6
    for k in range(n):
        if along == "x":
            x0, x1 = -w / 2 + w * k / n, -w / 2 + w * (k + 1) / n
            v = [(x0, -d / 2, z), (x1, -d / 2, z), (x1, 0, z + rise), (x0, 0, z + rise),
                 (x0, d / 2, z), (x1, d / 2, z)]
            m.mesh(v, [(0, 1, 2, 3), (3, 2, 5, 4)], keys[k % len(keys)])


def hot_dog_cart(m):
    """A two-wheeled cart with a handle and a striped umbrella."""
    w, d = STALL_W * 0.8, STALL_W * 0.5
    m.body = (w, d, 0.0)
    m.box((w, d, 0.1), (0, 0, 0.05), "cart")
    m.box((w + 0.01, d + 0.01, 0.012), (0, 0, 0.15), "steel")
    m.box((w * 0.5, d * 0.6, 0.03), (0, 0, 0.162), "steel")
    for x in (-w / 2 + 0.03, w / 2 - 0.03):
        m.cylinder(0.035, 0.012, (x, -d / 2 - 0.007, 0.035), "tyre", axis="y", vertices=12)
    m.box((0.012, 0.012, 0.06), (w / 2 + 0.04, 0, 0.1), "steel")
    m.box((0.05, 0.012, 0.012), (w / 2 + 0.02, 0, 0.12), "steel")
    m.box((0.008, 0.008, 0.13), (0, 0, 0.16), "mast")
    for k in range(8):
        a0, a1 = 2 * math.pi * k / 8, 2 * math.pi * (k + 1) / 8
        r, top, rim = 0.15, 0.31, 0.26
        v = [(0, 0, top), (r * math.cos(a0), r * math.sin(a0), rim), (r * math.cos(a1), r * math.sin(a1), rim)]
        m.mesh(v, [(0, 1, 2)], "awning-red" if k % 2 == 0 else "sign-white")


def bazaar_stall(m):
    """Four poles, a striped awning, and crates of goods on the counter."""
    w, d = STALL_W, STALL_W * 0.7
    m.body = (w, d, 0.0)
    m.box((w, d, 0.1), (0, 0, 0), "wood")
    m.box((w + 0.02, d * 0.4, 0.012), (0, -d * 0.3, 0.1), "wood-light")
    for x in (-w / 2, w / 2):
        for y in (-d / 2, d / 2):
            m.box((0.012, 0.012, 0.26), (x, y, 0.0), "wood")
    stripes(m, w + 0.04, d + 0.06, 0.25, 0.06, ("awning-teal", "sign-white"))
    for x, key in ((-w * 0.3, "goods-orange"), (0.0, "goods-green"), (w * 0.3, "goods-red")):
        m.box((0.05, 0.04, 0.03), (x, -d * 0.3, 0.112), "wood-light")
        m.box((0.042, 0.032, 0.014), (x, -d * 0.3, 0.142), key)


def coffee_kiosk(m):
    """A small booth with a serving hatch, a counter and an awning."""
    w, d = STALL_W * 0.75, STALL_W * 0.6
    m.body = (w, d, 0.0)
    m.box((w + 0.02, d + 0.02, PLINTH * 0.6), (0, 0, 0), "plinth")
    m.box((w, d, 0.26), (0, 0, PLINTH * 0.6), "kiosk")
    m.box((w * 0.7, 0.012, 0.09), (0, -d / 2 - 0.004, 0.12), "display")
    m.box((w * 0.8, 0.05, 0.012), (0, -d / 2 - 0.025, 0.115), "office-roof")
    m.mesh([(-w / 2 - 0.01, -d / 2, 0.25), (w / 2 + 0.01, -d / 2, 0.25), (w / 2 + 0.01, -d / 2 - 0.07, 0.21), (-w / 2 - 0.01, -d / 2 - 0.07, 0.21)],
           [(0, 1, 2, 3)], "awning-teal")
    m.box((w + 0.02, d + 0.02, 0.02), (0, 0, 0.27), "office-roof")
    m.box((w * 0.6, 0.012, 0.04), (0, 0, 0.29), "sign-white")
    m.cylinder(0.018, 0.04, (w * 0.25, -d / 2 - 0.035, 0.127), "cup", vertices=10)


def food_trailer(m):
    """A trailer on blocks with a drawbar and a propped-up serving hatch."""
    w, d = STALL_W, STALL_W * 0.55
    m.body = (w, d, 0.0)
    for x in (-w * 0.35, w * 0.35):
        m.box((0.03, d * 0.8, 0.04), (x, 0, 0), "plinth")
    m.cylinder(0.03, 0.014, (0, -d / 2 + 0.004, 0.03), "tyre", axis="y", vertices=12)
    m.box((w, d, 0.2), (0, 0, 0.04), "trailer")
    m.box((w, d + 0.004, 0.025), (0, 0, 0.1), "stripe")
    m.box((w * 0.6, 0.012, 0.07), (0, -d / 2 - 0.004, 0.14), "display")
    hatch = m.box((w * 0.62, 0.008, 0.07), (0, -d / 2 - 0.035, 0.21), "trailer")
    hatch.rotation_euler = (1.0, 0, 0)
    m.box((w * 0.66, 0.04, 0.01), (0, -d / 2 - 0.02, 0.13), "steel")
    m.box((0.1, 0.012, 0.012), (w / 2 + 0.05, 0, 0.05), "steel")
    m.box((w + 0.01, d + 0.01, 0.015), (0, 0, 0.24), "shed-roof")


STALLS = {"residential": hot_dog_cart, "shopping": bazaar_stall, "office": coffee_kiosk, "industrial": food_trailer}


def stall(name, at=(0, 0)):
    m = Model(f"botropolis/{name}")
    STALLS[name[: -len("-stall")]](m)
    m.root.scale = (STALL_SCALE, STALL_SCALE, STALL_SCALE)
    m.root.location = Vector((at[0], at[1], 0))
    m.root["body"] = [round(v * STALL_SCALE, 4) for v in m.body]
    return m.done()


# The customers who queue at a stall, one per subagent in flight: a
# figure a little under half a storey tall, in one of three coats.
CUSTOMERS = {"customer-a": "coat-blue", "customer-b": "coat-plum", "customer-c": "coat-mustard"}


def customer_names():
    return list(CUSTOMERS)


def is_customer(name):
    return name in CUSTOMERS


def customer(name, at=(0, 0)):
    m = Model(f"botropolis/{name}")
    m.body = (0.05, 0.04, 0.0)
    for x in (-0.012, 0.012):
        m.box((0.016, 0.02, 0.07), (x, 0, 0), "trousers")
    m.cylinder(0.024, 0.075, (0, 0, 0.07), CUSTOMERS[name], vertices=12)
    m.cylinder(0.016, 0.03, (0, 0, 0.148), "hair", vertices=12)
    m.root.location = Vector((at[0], at[1], 0))
    m.root["body"] = [round(v, 4) for v in m.body]
    return m.done()


def site_names():
    return [f"skip-{k}" for k in range(1, SKIP_FILLS + 1)] + VEHICLES


def is_site(name):
    return name in site_names()


def skip_body(m, at=(0.0, 0.0, 0.0), turn_y=True):
    """A skip's tapered steel body, open at the top, with a rim."""
    x0, y0, z0 = at
    bl, bw, tl, tw, h = 0.17, 0.10, 0.22, 0.12, 0.07
    if not turn_y:
        bl, bw, tl, tw = bw, bl, tw, tl
    v = [(x0 - bl / 2, y0 - bw / 2, z0), (x0 + bl / 2, y0 - bw / 2, z0), (x0 + bl / 2, y0 + bw / 2, z0), (x0 - bl / 2, y0 + bw / 2, z0),
         (x0 - tl / 2, y0 - tw / 2, z0 + h), (x0 + tl / 2, y0 - tw / 2, z0 + h), (x0 + tl / 2, y0 + tw / 2, z0 + h), (x0 - tl / 2, y0 + tw / 2, z0 + h)]
    m.mesh(v, [(0, 1, 2, 3), (0, 1, 5, 4), (1, 2, 6, 5), (2, 3, 7, 6), (3, 0, 4, 7)], "skip")
    m.box((tl + 0.008, 0.008, 0.008), (x0, y0 - tw / 2, z0 + h - 0.004), "skip-rim")
    m.box((tl + 0.008, 0.008, 0.008), (x0, y0 + tw / 2, z0 + h - 0.004), "skip-rim")
    m.box((0.008, tw + 0.008, 0.008), (x0 - tl / 2, y0, z0 + h - 0.004), "skip-rim")
    m.box((0.008, tw + 0.008, 0.008), (x0 + tl / 2, y0, z0 + h - 0.004), "skip-rim")
    return tl, tw, h


def rubble(m, level, at=(0.0, 0.0, 0.0), tl=0.22, tw=0.12, h=0.07):
    """What a skip holds at a fill level: a third, to the rim, heaped
    over it, and heaped with spill on the ground beside it."""
    x0, y0, z0 = at
    fill = {1: 0.4, 2: 0.95, 3: 1.0, 4: 1.0}[level]
    m.box((tl * 0.86, tw * 0.84, h * fill), (x0, y0, z0 + 0.004), "rubble")
    if level >= 3:
        rise = 0.05 if level == 3 else 0.08
        top = z0 + h
        m.mesh([(x0 - tl * 0.42, y0 - tw * 0.4, top), (x0 + tl * 0.42, y0 - tw * 0.4, top), (x0 + tl * 0.42, y0 + tw * 0.4, top),
                (x0 - tl * 0.42, y0 + tw * 0.4, top), (x0 - tl * 0.1, y0, top + rise), (x0 + tl * 0.12, y0, top + rise * 0.9)],
               [(0, 1, 5, 4), (1, 2, 5), (2, 3, 4, 5), (3, 0, 4)], "rubble-dark")
        for dx, dy, s in ((-0.06, -0.02, 0.03), (0.05, 0.02, 0.025), (0.0, 0.03, 0.02)):
            m.box((s, s, s), (x0 + dx, y0 + dy, top + rise * 0.4), "rubble-light")
    if level == 4:
        for dx, dy, s in ((tl * 0.6, 0.02, 0.035), (tl * 0.66, -0.04, 0.025), (-tl * 0.62, 0.03, 0.03), (0.02, tw * 0.75, 0.028)):
            m.box((s, s, s * 0.7), (x0 + dx, y0 + dy, z0), "rubble-dark")


def skip(m, level):
    tl, tw, h = skip_body(m)
    rubble(m, level, tl=tl, tw=tw, h=h)


def wheels(m, length, width, axles):
    for y in axles:
        for x in (-width / 2 - 0.004, width / 2 + 0.004):
            m.cylinder(0.026, 0.02, (x, y, 0.026), "tyre", axis="x", vertices=12)


def skip_truck(m, loaded):
    """A hook-lift truck: cab at the front (+y), a chassis, and the hook
    arm that drags a skip up onto it."""
    width, length = 0.13, 0.42
    m.box((width, length, 0.03), (0, 0, 0.03), "chassis")
    m.box((width, 0.1, 0.1), (0, length / 2 - 0.05, 0.05), "truck-cab")
    m.box((width * 0.92, 0.012, 0.045), (0, length / 2 + 0.001, 0.09), "window", 0.3)
    m.box((0.02, 0.26, 0.012), (-width * 0.3, -0.06, 0.062), "hook")
    m.box((0.02, 0.26, 0.012), (width * 0.3, -0.06, 0.062), "hook")
    m.box((width * 0.7, 0.02, 0.09), (0, 0.08, 0.062), "hook")
    wheels(m, length, width, (length / 2 - 0.06, -length / 2 + 0.08, -length / 2 + 0.14))
    if loaded:
        tl, tw, h = skip_body(m, at=(0, -0.06, 0.075), turn_y=False)
        rubble(m, 3, at=(0, -0.06, 0.075), tl=tw, tw=tl, h=h)


def mixer_truck(m):
    """A cement mixer: cab at the front (+y), the drum tilted up toward
    the back, and the chute it pours down."""
    width, length = 0.13, 0.42
    m.box((width, length, 0.03), (0, 0, 0.03), "chassis")
    m.box((width, 0.1, 0.1), (0, length / 2 - 0.05, 0.05), "mixer-cab")
    m.box((width * 0.92, 0.012, 0.045), (0, length / 2 + 0.001, 0.09), "window", 0.3)
    drum = m.cylinder(0.06, 0.26, (0, -0.05, 0.13), "drum", axis="y", vertices=20)
    drum.rotation_euler.x = 1.5708 + 0.25
    for y in (-0.1, -0.02, 0.06):
        band = m.cylinder(0.063, 0.014, (0, y, 0.13 - (y + 0.05) * 0.25), "drum-band", axis="y", vertices=20)
        band.rotation_euler.x = 1.5708 + 0.25
    chute = m.box((0.03, 0.12, 0.008), (0, -length / 2 - 0.04, 0.09), "chute")
    chute.rotation_euler.x = 0.4
    wheels(m, length, width, (length / 2 - 0.06, -length / 2 + 0.08, -length / 2 + 0.14))


def site(name, at=(0, 0)):
    """botropolis/<skip-k | skip-truck | skip-truck-loaded | mixer-truck>."""
    m = Model(f"botropolis/{name}")
    if name.startswith("skip-") and name[len("skip-"):].isdigit():
        skip(m, int(name[len("skip-"):]))
    elif name == "mixer-truck":
        mixer_truck(m)
    else:
        skip_truck(m, name.endswith("loaded"))
    m.root.location = Vector((at[0], at[1], 0))
    if name in VEHICLES:
        m.root.rotation_euler.z = math.pi
    return m.done()


def course_names():
    return [f"{n}-course-{k}" for n in names() for k in range(1, COURSES + 1)]


def is_kind(name):
    return name in names()


def is_course(name):
    return name in course_names()


def course(name, at=(0, 0)):
    """Course k under botropolis/<type>-<storeys>: a slab of the
    building's own material, its footprint and k steps round it, with a
    lighter joint along its top so a stack of them reads as layers."""
    kind, k = name.rsplit("-course-", 1)
    margin = COURSE_STEP * int(k)
    t = kind.rsplit("-", 1)[0]
    probe = build(kind)
    w, d, x = probe["body"]
    for o in list(probe.children_recursive) + [probe]:
        bpy.data.objects.remove(o, do_unlink=True)
    m = Model(f"botropolis/{name}")
    body, joint = COURSE_MATERIAL[t]
    m.box((w + 2 * margin, d + 2 * margin, COURSE * 0.8), (x, 0, 0), body)
    m.box((w + 2 * margin, d + 2 * margin, COURSE * 0.2), (x, 0, COURSE * 0.8), joint)
    m.body = (w + 2 * margin, d + 2 * margin, x)
    m.root.location = Vector((at[0], at[1], 0))
    m.root["body"] = [round(v, 4) for v in m.body]
    m.root["roof"] = round(COURSE, 4)
    return m.done()


def roof_height(root, x):
    """How high the roof is at the body's centre: straight down from
    above, the first surface met that is not rooftop furniture."""
    bpy.context.view_layer.update()
    depsgraph = bpy.context.evaluated_depsgraph_get()
    origin = Vector((root.location.x + x, root.location.y, 50.0))
    down = Vector((0, 0, -1))
    while True:
        hit, at, _, _, obj, _ = bpy.context.scene.ray_cast(depsgraph, origin, down)
        if not hit:
            return 0.0
        if obj.get("furniture") or obj.parent is not root:
            origin = at + down * 1e-4
            continue
        return at.z


def build(name, at=(0, 0)):
    """The piece botropolis/<type>-<storeys>, standing on at."""
    kind, n = name.rsplit("-", 1)
    m = Model(f"botropolis/{name}")
    BUILD[kind](m, int(n))
    m.root.location = Vector((at[0], at[1], 0))
    m.root["body"] = [round(v, 4) for v in m.body]
    root = m.done()
    root["roof"] = round(roof_height(root, m.body[2]), 4)
    return root

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
    "billboard": "#f2c230",
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


def sale_sign(m, at, angle, height, z):
    """A blade sign down a corner, standing out from it, with white
    squares for its lettering."""
    blade = m.box((0.1, 0.018, height), (at[0], at[1], z), "awning-red")
    blade.rotation_euler.z = angle
    for k in range(4):
        sq = m.box((0.056, 0.024, height * 0.13), (at[0], at[1], z + height * (0.1 + 0.22 * k)), "sign-white")
        sq.rotation_euler.z = angle


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
        reach = 0.05 / math.sqrt(2)
        for sx, sy in ((1, -1), (-1, 1), (1, 1), (-1, -1)):
            at = (sx * (w / 2 + reach), sy * (d / 2 + reach))
            sale_sign(m, at, math.atan2(sy, sx), sign_h, storey_z(1) + STOREY * 0.05)
    if n == 5:
        for x in (-w * 0.25, w * 0.25):
            m.box((0.02, 0.02, 0.12), (x, 0, top + 0.02), "mast")
        m.box((w * 0.75, 0.03, 0.16), (0, 0, top + 0.14), "billboard")
        m.box((w * 0.5, 0.034, 0.07), (0, 0, top + 0.185), "awning-red")


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


def is_kind(name):
    return name in names()


def build(name, at=(0, 0)):
    """The piece botropolis/<type>-<storeys>, standing on at."""
    kind, n = name.rsplit("-", 1)
    m = Model(f"botropolis/{name}")
    BUILD[kind](m, int(n))
    m.root.location = Vector((at[0], at[1], 0))
    m.root["body"] = [round(v, 4) for v in m.body]
    return m.done()

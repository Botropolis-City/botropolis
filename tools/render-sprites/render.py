"""Headless Blender pipeline for Botropolis' sprites.

    blender -b --python tools/render-sprites/render.py -- scene --out out.png
    blender -b --python tools/render-sprites/render.py -- atlas --out pkg/assets/kits
    blender -b --python tools/render-sprites/render.py -- kinds --out kinds.png
    blender -b --python tools/render-sprites/render.py -- --help

The `scene` mode composes one district from Kenney's City Kits — Commercial
for the buildings, Roads for the avenues, Industrial for the plant — and
renders it once from the isometric heading, so the look can be judged
beside the current map before any atlas is cut.

The `atlas` mode renders every piece the map uses at four headings and
each zoom level into atlas pages with a JSON manifest: for each sprite
its page, its rectangle and where the piece's ground origin lands, so
the renderer can set it on a cell.
"""

import argparse
import glob
import json
import math
import os
import subprocess
import sys

import bpy
import numpy as np
from bpy_extras.object_utils import world_to_camera_view
from mathutils import Vector

sys.dont_write_bytecode = True
sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import buildings  # noqa: E402

KITS = os.path.join(os.path.dirname(os.path.abspath(__file__)), "..", "kits")

# The map's 2:1 diamond: the camera looks down from 30 degrees above the
# ground (sin 30 = 1/2 is the foreshortening that makes a square's
# diagonal twice as wide as it is tall; atan(1/2) is the diamond's edge
# angle on screen, a different number, and tilting the camera by it cut
# every tile a tenth too short for its cell) and turned 45 degrees, so
# a unit tile projects as a diamond 132 px wide and 66 tall at zoom 1.
ISO_TILT = 60.0
ISO_TURN = 45.0

# One kit unit is one map cell; the map draws a cell as a diamond
# TILE_PX wide at zoom 1 (city.IsoTileWidth), and a unit's diagonal lies
# flat across that diamond.
TILE_PX = 132.0
PX_PER_UNIT = TILE_PX / math.sqrt(2)
PAGE = 2048
PAGE_BUDGET = 8
PAD = 2

# Every piece the map draws, by kit: the atlas is cut from this list.
# Every piece cut here is paid for four times over — once per heading —
# in atlas area, page count and the size of the client binary that embeds
# it. A piece nothing draws is also a piece nothing validates: two of the
# four whose anchors moved most when bug 20 derived them were pieces no
# code named, so that fix was partly measured against art nobody looks
# at. Ten were dropped on 2026-09-25 for exactly that reason (bug 24):
# four industrial building variants, the tank, the windmill, the solar
# panels, two train wagons and the rowing boat.
#
# Item 59 was that phase arriving. chimney-large went: bug 24 had kept it
# as the plant's stack, and item 36 then picked chimney-medium instead, so
# the piece three entries reasoned about was the largest thing here that
# nothing drew. sign-highway-detailed went with it, never drawn at all —
# kitSignGantry names the plain sign-highway.
#
# The six kept but not yet drawn are kept on purpose, each with a use in
# view: electricity-pole and electricity-wires are what the power lines
# should become (they are vector strokes today), traffic-light,
# construction-cone and light-curved are street furniture the `detail`
# setting now gives a home to, and the truck is a second vehicle for
# Traffic. That set is asserted by TestAtlasCarriesNothingUndeclared
# rather than only written here, because item 59 existed at all through a
# comment going stale when chimney-medium was drawn.
PIECES = {
    # Item 109 replaced the buildings context fill chose between with the
    # city's own kinds; the hall and the library are all that is left.
    "city-kit-commercial": ["building-l", "building-n"],
    "city-kit-industrial": ["building-a", "chimney-medium", "water-tower",
                            "shipping-container-a", "shipping-container-b", "shipping-container-c"],
    "city-kit-roads": ["road-straight", "road-bend", "road-crossroad", "road-intersection", "road-end", "road-square",
                       "light-square", "light-curved", "electricity-pole", "electricity-wires", "traffic-light", "construction-cone",
                       # Phase 21 item 38: signage prototypes. A gantry beside
                       # the plot, a post-mounted board, a plaque for a wall.
                       "sign-highway", "road-sign-empty", "road-sign-empty-hanging"],
    "city-kit-suburban": ["tree-large", "tree-small", "planter"],
    "nature-kit": ["tree_default", "tree_oak", "tree_thin", "tree_tall", "tree_pineRoundA", "tree_small",
                   "plant_bush", "plant_bushLarge",
                   # Item 86: what grows on the lot of a session quiet
                   # for long enough, and a sapling among it.
                   "grass", "grass_large", "grass_leafs", "plant_flatShort", "tree_small_dark"],
    "car-kit": ["sedan", "van", "taxi", "suv", "hatchback-sports", "truck", "delivery"],
    "train-kit": ["train-diesel-a", "train-diesel-b", "train-diesel-c",
                  "train-carriage-container-red", "train-carriage-container-blue", "train-carriage-container-green"],
    # The gauge boats need a hull that is not a tug: the river already
    # carries tugs for sessions arriving and leaving, and two meanings on
    # one waterway only work if the silhouettes differ at a glance. The
    # kit has no sternwheeler, so this liner stands in for Aria's
    # steamboat -- long hull and funnels against the tug's stubby
    # wheelhouse. Item 49.
    "watercraft-kit": ["boat-tug-a", "boat-tug-b", "ship-ocean-liner-small",
                       # Channel markers for the usage gauge. Aria, on
                       # r248: "why do we have lines in the river?" --
                       # painted stripes down a waterway read as road
                       # markings. A buoy is the object that marks
                       # lateral position on water. Bug 50.
                       "buoy", "buoy-flag",
                       # One hull per gauge rank, so the boats differ in
                       # profile and not only in scale. Item 51: long
                       # and tall, long and flat, small and vertical.
                       # The full-size liner is left out -- the shrink
                       # per rank already separates the sizes, and it is
                       # a large sprite against one page of headroom.
                       "ship-cargo-a", "boat-sail-a",
                       # The refresh courier. Item 56: it says "you
                       # asked for fresh numbers", so it must not be
                       # mistaken for a tug, which says "a session came
                       # or went". Chosen by measurement rather than by
                       # eye -- the discriminator is height, since the
                       # tugs are 2.24 tall and blocky while every speed
                       # hull is 1.2-1.7 and flat. boat-speed-j is the
                       # longest at 4.27 and the most slender at 2.39:1,
                       # against boat-tug-a's 3.47 and 1.94:1. Low and
                       # fast against tall and squat, on both axes.
                       "boat-speed-j"],
    # Item 109: the city's own buildings, one kind per district type and
    # storey count, modelled in buildings.py.
    "botropolis": ["drone", "fountain-a", "fountain-b", "fountain-c"] + buildings.names() + buildings.course_names() + buildings.site_names() + buildings.ruins_names(),
}

# Kits that are not modelled at one unit per cell are scaled on import:
# the Car Kit is in metres, a sedan 2.55 long, and a car on the map is a
# third of a cell; the Watercraft Kit's tug is 3.5, and a barge on the map
# is two thirds of a cell.
#
# The Train Kit is scaled against the cars (item 134). Its wagon is 1.0 by
# 2.7 and the sedan 1.5 by 2.55, so no one scale gives a wagon both a real
# wagon's width beside a car (about 1.7 times) and its length (about 3.3
# times): 0.35 is about a tenth off each, and 0.40 makes the body wider
# than a real wagon. pkg/assets/kits_test.go holds the ratios.
SCALE = {"car-kit": 0.12, "train-kit": 0.35, "watercraft-kit": 0.25}

# Kits whose pieces are vehicles, modelled facing along y: the atlas
# records their footprint, so the map spaces a train by its wagons' own
# length rather than by a constant that can drift from the scale.
VEHICLES = {"car-kit", "train-kit"}

# The Nature Kit's geometry, on this city's terms. Its trees are the
# variety the two Suburban ones cannot give, but they are modelled a
# storey and a half taller than the buildings and their leaves are
# teal, so the pipeline — which is where colour is decided — scales
# each piece to a height in the Suburban trees' range and repaints its
# materials in the Suburban greens. The tallest comes in a hair over
# tree-large's 0.77, well under two storeys.
NATURE_HEIGHT = {
    "tree_default": 0.80,
    "tree_oak": 0.70,
    "tree_thin": 0.76,
    "tree_tall": 0.84,
    "tree_pineRoundA": 0.78,
    "tree_small": 0.56,
    "plant_bush": 0.26,
    "plant_bushLarge": 0.30,
    "grass": 0.16,
    "grass_large": 0.22,
    "grass_leafs": 0.18,
    "plant_flatShort": 0.20,
    "tree_small_dark": 0.40,
}

# The Suburban trees' own colours, read off the atlas they are cut into:
# leaf, shaded leaf and bark. The Nature Kit names its materials, so the
# repaint is by name rather than by pixel.
NATURE_TINT = {
    "leafsGreen": "#3c8f6e",
    "leafsDark": "#2d7458",
    "grass": "#3c8f6e",
    "woodBark": "#9c6349",
    "woodBarkDark": "#82523c",
}


GRASS = (0.22, 0.33, 0.17, 1)
PLAZA = (0.54, 0.53, 0.49, 1)
CONCRETE = (0.45, 0.45, 0.47, 1)


def parse_args():
    argv = sys.argv[sys.argv.index("--") + 1 :] if "--" in sys.argv else []
    p = argparse.ArgumentParser(prog="render.py", description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("mode", choices=["scene", "atlas", "kinds"], help="what to render")
    p.add_argument("--out", required=True, help="PNG to write (scene, kinds) or directory for the atlases (atlas)")
    p.add_argument("--heading", type=int, default=0, help="the heading to show the kinds from (kinds)")
    p.add_argument("--zooms", default="1,2", help="zoom levels to cut atlases for (atlas)")
    p.add_argument("--only", default="", help="render only pieces whose name contains this (atlas, for trying things)")
    p.add_argument("--kits", default=KITS, help="where the Kenney kits are (tools/kits)")
    p.add_argument("--width", type=int, default=1600)
    p.add_argument("--height", type=int, default=1000)
    p.add_argument("--scale", type=float, default=13.0, help="orthographic width in tiles")
    return p.parse_args(argv)


def reset():
    bpy.ops.wm.read_factory_settings(use_empty=True)


ACCENT = (0.91, 0.63, 0.24, 1)
SLATE = (0.16, 0.18, 0.22, 1)
OFFWHITE = (0.85, 0.86, 0.88, 1)
# The plaza's own stone, as an sRGB hex like the rest of the palette.
STONE = "#9a9791"
# The plant's steel blue, ui.Palette.Plant: the fountain's water is the
# same water the plant's tanks hold.
PLANT_BLUE = "#6a8cb8"


def solid(name, colour, emission=0.0):
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs["Base Color"].default_value = colour
    bsdf.inputs["Roughness"].default_value = 0.7
    if emission:
        bsdf.inputs["Emission Color"].default_value = colour
        bsdf.inputs["Emission Strength"].default_value = emission
    return mat


def drone(at):
    """Our own drone: a slate body, two off-white rotors, one accent
    light — the subagent in flight. Modelled here so it shares the kits'
    palette and needs no third-party asset."""
    root = bpy.data.objects.new("botropolis/drone", None)
    bpy.context.scene.collection.objects.link(root)
    parts = []
    bpy.ops.mesh.primitive_cylinder_add(radius=0.09, depth=0.07, location=(0, 0, 0.32))
    body = bpy.context.active_object
    body.data.materials.append(solid("drone-body", SLATE))
    parts.append(body)
    bpy.ops.mesh.primitive_cube_add(size=1, location=(0, 0, 0.35))
    arm = bpy.context.active_object
    arm.scale = (0.34, 0.03, 0.015)
    arm.data.materials.append(solid("drone-arm", SLATE))
    parts.append(arm)
    for x in (-0.16, 0.16):
        bpy.ops.mesh.primitive_cylinder_add(radius=0.1, depth=0.012, location=(x, 0, 0.37))
        rotor = bpy.context.active_object
        rotor.data.materials.append(solid("drone-rotor", OFFWHITE))
        parts.append(rotor)
    bpy.ops.mesh.primitive_uv_sphere_add(radius=0.035, location=(0, 0, 0.27))
    lamp = bpy.context.active_object
    lamp.data.materials.append(solid("drone-light", ACCENT, emission=3.0))
    parts.append(lamp)
    for o in parts:
        o.parent = root
    root.location = Vector((at[0], at[1], 0))
    bpy.context.view_layer.update()
    return root



# How high the jet stands and how wide the droplets fly in each of the
# three spray frames: up, over, and falling back. The city cycles them
# slowly, and holds the first when motion is reduced.
SPRAY = {"a": (0.10, 0.09), "b": (0.19, 0.15), "c": (0.14, 0.21)}


def fountain(at, frame):
    """The plaza's fountain: a stone basin with a lip, a column, a disc
    of water in the plant's steel blue, and a spray cut in three frames.
    No kit on disk has one, so it is modelled here as the drone is,
    where the palette is decided."""
    root = bpy.data.objects.new(f"botropolis/fountain-{frame}", None)
    bpy.context.scene.collection.objects.link(root)
    parts = []
    stone = solid("fountain-stone", rgba(STONE))
    water = solid("fountain-water", rgba(PLANT_BLUE))
    spray = solid("fountain-spray", rgba(PLANT_BLUE), emission=0.8)

    def cyl(radius, depth, z, mat):
        bpy.ops.mesh.primitive_cylinder_add(radius=radius, depth=depth, location=(0, 0, z), vertices=28)
        o = bpy.context.active_object
        o.data.materials.append(mat)
        parts.append(o)

    cyl(0.38, 0.10, 0.05, stone)     # the basin
    cyl(0.41, 0.05, 0.10, stone)     # the lip round its rim
    cyl(0.36, 0.02, 0.135, water)    # the water standing in it
    cyl(0.055, 0.26, 0.23, stone)    # the column
    cyl(0.13, 0.03, 0.365, stone)    # the dish it holds up
    rise, spread = SPRAY[frame]
    cyl(0.022, rise, 0.38 + rise / 2, spray)
    for i in range(6):
        angle = i * math.pi / 3
        bpy.ops.mesh.primitive_uv_sphere_add(
            radius=0.024,
            location=(spread * math.cos(angle), spread * math.sin(angle), 0.38 + rise * (0.8 if i % 2 else 0.55)),
            segments=12,
            ring_count=8,
        )
        drop = bpy.context.active_object
        drop.data.materials.append(spray)
        parts.append(drop)
    for o in parts:
        o.parent = root
    root.location = Vector((at[0], at[1], 0))
    bpy.context.view_layer.update()
    return root

def model_path(kits, kit, name):
    for folder in ("GLB format", "GLTF format"):
        path = os.path.join(kits, kit, "Models", folder, name + ".glb")
        if os.path.exists(path):
            return path
    raise FileNotFoundError(f"{kit}/{name}.glb is not in tools/kits; run make kits")


def piece(kits, kit, name, at, turn=0.0):
    """Import one kit piece at a tile position, turned in degrees about z;
    the botropolis kit is modelled here rather than imported."""
    if kit == "botropolis":
        if name == "drone":
            return drone(at)
        if name.startswith("fountain-") and name[len("fountain-"):] in SPRAY:
            return fountain(at, name[len("fountain-"):])
        if buildings.is_kind(name):
            return buildings.build(name, at)
        if buildings.is_course(name):
            return buildings.course(name, at)
        if buildings.is_site(name):
            return buildings.site(name, at)
        if buildings.is_ruins(name):
            return buildings.ruins(name, at)
        raise FileNotFoundError(f"botropolis/{name} is not a piece we model")
    before = set(bpy.context.scene.objects)
    bpy.ops.import_scene.gltf(filepath=model_path(kits, kit, name))
    new = [o for o in bpy.context.scene.objects if o not in before]
    root = bpy.data.objects.new(f"{kit}/{name}", None)
    bpy.context.scene.collection.objects.link(root)
    for o in new:
        if o.parent is None:
            o.parent = root
    root.location = Vector((at[0], at[1], at[2] if len(at) > 2 else 0))
    root.rotation_euler = (0, 0, math.radians(turn))
    k = SCALE.get(kit, 1.0)
    if kit == "nature-kit":
        repaint(new, NATURE_TINT)
        k = nature_scale(root, name)
    root.scale = (k, k, k)
    bpy.context.view_layer.update()
    return root


def srgb_to_linear(c):
    return c / 12.92 if c <= 0.04045 else ((c + 0.055) / 1.055) ** 2.4


def rgba(hex_colour):
    """A #rrggbb string as Blender's linear RGBA."""
    h = hex_colour.lstrip("#")
    return tuple(srgb_to_linear(int(h[i:i + 2], 16) / 255) for i in (0, 2, 4)) + (1.0,)


def repaint(objects, tints):
    """Set the base colour of every material named in tints. Kits that
    paint with plain materials rather than a colour map can be brought
    onto this city's palette here, where colour is decided."""
    seen = set()
    for o in objects:
        for slot in getattr(o, "material_slots", []):
            m = slot.material
            if m is None or m.name in seen or not m.use_nodes:
                continue
            seen.add(m.name)
            want = tints.get(m.name.split(".")[0])
            if want is None:
                continue
            for node in m.node_tree.nodes:
                if node.type == "BSDF_PRINCIPLED":
                    node.inputs["Base Color"].default_value = rgba(want)


def nature_scale(root, name):
    """The scale that brings a Nature Kit piece to its height in
    NATURE_HEIGHT, so no tree stands taller than the city's own."""
    want = NATURE_HEIGHT.get(name)
    if want is None:
        return 1.0
    bpy.context.view_layer.update()
    lo, hi = bounds(root)
    tall = hi.z - lo.z
    return want / tall if tall > 0 else 1.0


def flat(name, size, at, colour):
    """A coloured plane: ground, plaza floor, concrete apron."""
    bpy.ops.mesh.primitive_plane_add(size=1, location=(at[0], at[1], at[2] if len(at) > 2 else 0))
    o = bpy.context.active_object
    o.name = name
    o.scale = (size[0], size[1], 1)
    mat = bpy.data.materials.new(name)
    mat.use_nodes = True
    bsdf = mat.node_tree.nodes["Principled BSDF"]
    bsdf.inputs["Base Color"].default_value = colour
    bsdf.inputs["Roughness"].default_value = 1.0
    o.data.materials.append(mat)
    return o


def light():
    sun = bpy.data.lights.new("sun", "SUN")
    sun.energy = 4.0
    sun.angle = math.radians(3)
    o = bpy.data.objects.new("sun", sun)
    bpy.context.scene.collection.objects.link(o)
    # One light direction for every sprite: high, from the front-left.
    o.rotation_euler = (math.radians(48), math.radians(12), math.radians(-35))
    world = bpy.data.worlds.new("world")
    world.use_nodes = True
    bg = world.node_tree.nodes["Background"]
    bg.inputs["Color"].default_value = (0.7, 0.78, 0.9, 1)
    bg.inputs["Strength"].default_value = 0.55
    bpy.context.scene.world = world


def camera(centre, scale, width, height):
    cam = bpy.data.cameras.new("iso")
    cam.type = "ORTHO"
    cam.ortho_scale = scale
    cam.clip_end = 500
    o = bpy.data.objects.new("iso", cam)
    bpy.context.scene.collection.objects.link(o)
    o.rotation_euler = (math.radians(ISO_TILT), 0, math.radians(ISO_TURN))
    # Back the camera off along its own view axis so everything is in front.
    back = o.rotation_euler.to_matrix() @ Vector((0, 0, 60))
    o.location = Vector(centre) + back
    bpy.context.scene.camera = o
    r = bpy.context.scene.render
    r.resolution_x, r.resolution_y = width, height
    r.resolution_percentage = 100


def settings():
    scene = bpy.context.scene
    scene.render.engine = "BLENDER_EEVEE"
    scene.render.film_transparent = False
    scene.render.image_settings.file_format = "PNG"
    scene.render.image_settings.color_mode = "RGBA"
    scene.view_settings.view_transform = "Standard"
    scene.view_settings.look = "None"
    eevee = scene.eevee
    for attr, value in (("taa_render_samples", 32), ("use_gtao", True), ("use_shadows", True), ("use_soft_shadows", True), ("shadow_cube_size", "2048"), ("shadow_cascade_size", "4096")):
        if hasattr(eevee, attr):
            try:
                setattr(eevee, attr, value)
            except Exception:
                pass


def district(kits):
    """One live district on its block, the plant on the plaza next door,
    avenues round both, lamps at the crossings."""
    flat("ground", (40, 40), (3.5, 1, -0.02), GRASS)
    # Blocks: the district at x 0..4, y 0..3; the plaza at x 5..8.
    flat("block", (4, 3), (2, 1.5, 0.0), PLAZA)
    flat("plaza", (3, 3), (6.5, 1.5, 0.0), CONCRETE)
    # Roads: two rows and three columns of avenue round the blocks.
    for x in range(0, 4):
        piece(kits, "city-kit-roads", "road-straight", (x + 0.5, -0.5), 0)
        piece(kits, "city-kit-roads", "road-straight", (x + 0.5, 3.5), 0)
    for x in range(5, 8):
        piece(kits, "city-kit-roads", "road-straight", (x + 0.5, -0.5), 0)
        piece(kits, "city-kit-roads", "road-straight", (x + 0.5, 3.5), 0)
    for y in range(0, 3):
        for x in (-0.5, 4.5, 8.5):
            piece(kits, "city-kit-roads", "road-straight", (x, y + 0.5), 90)
    piece(kits, "city-kit-roads", "road-bend", (-0.5, -0.5), 180)
    piece(kits, "city-kit-roads", "road-bend", (8.5, -0.5), 270)
    piece(kits, "city-kit-roads", "road-bend", (8.5, 3.5), 0)
    piece(kits, "city-kit-roads", "road-bend", (-0.5, 3.5), 90)
    piece(kits, "city-kit-roads", "road-intersection", (4.5, -0.5), 90)
    piece(kits, "city-kit-roads", "road-intersection", (4.5, 3.5), 270)
    # Lamps at the crossings.
    for at, turn in (((-0.15, -0.15), 0), ((4.15, -0.15), 90), ((8.15, -0.15), 90), ((-0.15, 3.15), 270), ((4.15, 3.15), 180), ((8.15, 3.15), 180)):
        piece(kits, "city-kit-roads", "light-square", at, turn)
    # Sessions: six buildings on the block, one per session.
    names = ["building-a", "building-b", "building-c", "building-d", "building-e", "building-f"]
    i = 0
    for row in range(2):
        for col in range(3):
            piece(kits, "city-kit-commercial", names[i], (0.7 + col * 1.3, 0.75 + row * 1.5), 0)
            i += 1
    # The plant: an industrial hall, its stack, a tank and the water tower.
    piece(kits, "city-kit-industrial", "building-a", (6.3, 1.0), 0)
    piece(kits, "city-kit-industrial", "chimney-large", (7.4, 2.3), 0)
    piece(kits, "city-kit-industrial", "detail-tank-large", (5.6, 2.4), 0)
    piece(kits, "city-kit-industrial", "water-tower", (7.5, 0.6), 0)


def remove(root):
    """Delete an imported piece and everything under it."""
    for o in list(root.children_recursive) + [root]:
        bpy.data.objects.remove(o, do_unlink=True)
    for block in (bpy.data.meshes, bpy.data.materials, bpy.data.images):
        for datum in list(block):
            if datum.users == 0:
                block.remove(datum)


def bounds(root):
    lo = Vector((1e9, 1e9, 1e9))
    hi = Vector((-1e9, -1e9, -1e9))
    for o in root.children_recursive:
        if o.type != "MESH":
            continue
        for c in o.bound_box:
            w = o.matrix_world @ Vector(c)
            lo = Vector(map(min, lo, w))
            hi = Vector(map(max, hi, w))
    return lo, hi


def ground_point(root):
    """Where a piece meets the ground: the centre of its footprint in x
    and y, and in z the ground plane itself — or the foot of the piece
    when it never reaches the ground.

    The atlas anchors a sprite here, so the map can put a piece on a
    point and have the piece stand on it. Deriving this from the mesh
    rather than trusting the model's origin is the fix for bug 20: a kit
    that models geometry away from its origin was drawn wherever that
    origin happened to fall.

    z is `max(0, lowest)` rather than the lowest point, because a kit
    that dips below the ground means it: the Nature Kit sets its trees
    0.023 to 0.062 of a tile into the earth so they do not read as
    standing on a plane, and its bushes deepest of all. Anchoring those
    at their lowest point would lift them out of the ground, which is
    the same floating this bug is about, in the other direction. A piece
    that is wholly above the ground — the drone — is anchored at its own
    foot, because there is no ground under it to meet."""
    lo, hi = bounds(root)
    return Vector(((lo.x + hi.x) / 2, (lo.y + hi.y) / 2, max(0.0, lo.z)))


def frame_piece(root, heading, zoom, ground):
    """Point the camera at a piece from a heading and size the frame to
    hold it: returns the resolution and where its ground point lands."""
    scene = bpy.context.scene
    cam = scene.camera
    cam.rotation_euler = (math.radians(ISO_TILT), 0, math.radians(ISO_TURN + heading))
    lo, hi = bounds(root)
    centre = (lo + hi) / 2
    back = cam.rotation_euler.to_matrix() @ Vector((0, 0, 60))
    cam.location = centre + back
    bpy.context.view_layer.update()
    corners = [Vector((x, y, z)) for x in (lo.x, hi.x) for y in (lo.y, hi.y) for z in (lo.z, hi.z)]
    # Ortho scale is the frame's width in camera units; find the extent
    # of the piece in camera space and add a margin.
    view = cam.rotation_euler.to_matrix().inverted()
    xs = [(view @ (c - cam.location)).x for c in corners]
    ys = [(view @ (c - cam.location)).y for c in corners]
    margin = 0.15
    w = max(xs) - min(xs) + 2 * margin
    h = max(ys) - min(ys) + 2 * margin
    size = max(w, h)
    cam.data.ortho_scale = size
    px = int(math.ceil(size * PX_PER_UNIT * zoom))
    scene.render.resolution_x = px
    scene.render.resolution_y = px
    bpy.context.view_layer.update()
    ndc = world_to_camera_view(scene, cam, ground)
    return px, (ndc.x * px, (1 - ndc.y) * px)


def render_pixels(path):
    """Render the frame to path and return it as an HxWx4 uint8 array."""
    scene = bpy.context.scene
    scene.render.filepath = path
    bpy.ops.render.render(write_still=True)
    img = bpy.data.images.load(path)
    w, h = img.size
    buf = np.empty(w * h * 4, dtype=np.float32)
    img.pixels.foreach_get(buf)
    bpy.data.images.remove(img)
    rgba = (buf.reshape(h, w, 4)[::-1] * 255 + 0.5).astype(np.uint8)
    return rgba


def crop(rgba):
    alpha = rgba[:, :, 3]
    rows = np.where(alpha.any(axis=1))[0]
    cols = np.where(alpha.any(axis=0))[0]
    if len(rows) == 0:
        return rgba[:1, :1], (0, 0)
    y0, y1 = rows[0], rows[-1] + 1
    x0, x1 = cols[0], cols[-1] + 1
    return rgba[y0:y1, x0:x1], (x0, y0)


# A window pane in a rendered building: glass is the one surface in the
# commercial kit that is clearly blue, and its panes are painted into the
# palette texture rather than modelled, so they are found in the picture.
PANE_MIN_AREA = 4


def kit_glass(r, g, b, a):
    return (a > 200) & (b - r > 40) & (b > 120)


def warm_glass(r, g, b, a):
    """Our own buildings' lit windows, buildings.WINDOW as rendered."""
    return (a > 200) & (r > 225) & (g > 190) & (b > 90) & (b < 185) & (r - b > 70)


def panes(rgba, glass_of=kit_glass):
    """Each window pane in a cut sprite, as the four corners of the
    parallelogram it covers -- top left, top right, bottom right, bottom
    left -- in the sprite's own pixels. The corners are the topmost and
    bottommost glass pixel in the pane's leftmost and rightmost columns,
    so a pane on either face carries that face's slope. Item 86."""
    r, g, b, a = (rgba[:, :, i].astype(int) for i in range(4))
    glass = glass_of(r, g, b, a)
    seen = np.zeros(glass.shape, dtype=bool)
    h, w = glass.shape
    out = []
    for y0, x0 in zip(*np.nonzero(glass)):
        if seen[y0, x0]:
            continue
        stack, pixels = [(y0, x0)], []
        seen[y0, x0] = True
        while stack:
            y, x = stack.pop()
            pixels.append((y, x))
            for ny, nx in ((y - 1, x), (y + 1, x), (y, x - 1), (y, x + 1)):
                if 0 <= ny < h and 0 <= nx < w and glass[ny, nx] and not seen[ny, nx]:
                    seen[ny, nx] = True
                    stack.append((ny, nx))
        if len(pixels) < PANE_MIN_AREA:
            continue
        xs = [x for _, x in pixels]
        left, right = min(xs), max(xs)
        lys = [y for y, x in pixels if x == left]
        rys = [y for y, x in pixels if x == right]
        out.append([left, min(lys), right + 1, min(rys), right + 1, max(rys) + 1, left, max(lys) + 1])
    out.sort()
    return [[int(v) for v in q] for q in out]


class Pages:
    """A shelf packer over PAGE-square pages."""

    def __init__(self):
        self.pages = []
        self.shelf_y = 0
        self.shelf_h = 0
        self.x = 0

    def new_page(self):
        self.pages.append(np.zeros((PAGE, PAGE, 4), dtype=np.uint8))
        self.shelf_y = self.shelf_h = self.x = 0

    def put(self, rgba):
        h, w = rgba.shape[:2]
        if w + 2 * PAD > PAGE or h + 2 * PAD > PAGE:
            raise ValueError(f"a sprite of {w}x{h} does not fit a {PAGE} page; lower the zoom or split the piece")
        if not self.pages:
            self.new_page()
        if self.x + w + PAD > PAGE:
            self.shelf_y += self.shelf_h + PAD
            self.shelf_h = 0
            self.x = 0
        if self.shelf_y + h + PAD > PAGE:
            self.new_page()
        page = len(self.pages) - 1
        x, y = self.x + PAD, self.shelf_y + PAD
        self.pages[page][y : y + h, x : x + w] = rgba
        self.x += w + PAD
        self.shelf_h = max(self.shelf_h, h)
        return page, x, y

    def save(self, out, stem):
        names = []
        for i, page in enumerate(self.pages):
            name = f"{stem}-{i}.png"
            img = bpy.data.images.new(name, PAGE, PAGE, alpha=True)
            img.pixels.foreach_set((page[::-1].astype(np.float32) / 255).ravel())
            img.filepath_raw = os.path.join(out, name)
            img.file_format = "PNG"
            img.save()
            bpy.data.images.remove(img)
            names.append(name)
        shrink([os.path.join(out, n) for n in names])
        return names


def shrink(paths):
    """Re-compress the pages we just wrote, before anyone can ship them.

    Blender writes PNGs at a low deflate level and `make sprites` used to
    be the only thing that fixed that, on the line after this script ran.
    Two steps that must both happen, with nothing checking that they did:
    a staged re-cut driving this file directly — which is how the zoom
    levels are cut one at a time, because each takes about a hundred
    seconds — skipped the second one silently. Item 63 found nine pages
    carried forward unshrunk, 6.6 MB in every client binary, and the
    check that was supposed to catch it could not: an unshrunk page has
    no ImageMagick date chunk either, so looking for the chunk passed
    whether the step had run or not.

    Doing it here rather than after means there is one step. The Makefile
    still calls the tool, which is now a no-op that costs a re-encode.
    """
    if not paths:
        return
    tool = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "shrink-pngs")
    subprocess.run([tool, *paths], check=True)


def atlas(args):
    """Cut the atlases: every piece, four headings, each zoom level."""
    out = os.path.abspath(args.out)
    os.makedirs(out, exist_ok=True)
    tmp = os.path.join(out, ".render.png")
    zooms = [float(z) for z in args.zooms.split(",") if z]
    reset()
    settings()
    bpy.context.scene.render.film_transparent = True
    light()
    camera((0, 0, 0), 4, 64, 64)
    for zoom in zooms:
        pages = Pages()
        sprites = {}
        count = 0
        for kit, names in PIECES.items():
            for name in names:
                if args.only and args.only not in name:
                    continue
                root = piece(args.kits, kit, name, (0, 0))
                key = f"{kit}/{name}"
                sprites[key] = {}
                # One ground point per piece, projected once per heading:
                # the camera turns, the piece does not.
                ground = ground_point(root)
                lo, hi = bounds(root)
                for heading in (0, 90, 180, 270):
                    px, (ax, ay) = frame_piece(root, heading, zoom, ground)
                    rgba = render_pixels(tmp)
                    cut, (x0, y0) = crop(rgba)
                    page, x, y = pages.put(cut)
                    sprites[key][str(heading)] = {
                        "page": page, "x": int(x), "y": int(y), "w": int(cut.shape[1]), "h": int(cut.shape[0]),
                        "ax": round(float(ax - x0), 2), "ay": round(float(ay - y0), 2),
                    }
                    if kit == "city-kit-commercial" and name.startswith("building"):
                        sprites[key][str(heading)]["panes"] = panes(cut)
                    if kit == "botropolis" and buildings.is_kind(name):
                        sprites[key][str(heading)]["panes"] = panes(cut, warm_glass)
                        sprites[key][str(heading)]["body"] = list(root["body"]) + [root["roof"]]
                    if kit == "botropolis" and buildings.is_course(name):
                        sprites[key][str(heading)]["body"] = list(root["body"])
                    if kit in VEHICLES:
                        sprites[key][str(heading)]["foot"] = [round(hi.x - lo.x, 4), round(hi.y - lo.y, 4)]
                    count += 1
                remove(root)
        stem = f"kits-z{zoom:g}"
        names = pages.save(out, stem)
        # A re-cut that needs fewer pages than the last one leaves the
        # surplus behind, and nothing notices: the manifest stops naming
        # it, but it is still on disk, still tracked, and still embedded
        # by go:embed into every client binary. Dropping ten pieces in
        # bug 24 orphaned kits-z2-6.png exactly this way — 922 KB of a
        # page nothing could reach.
        for stale in sorted(glob.glob(os.path.join(out, f"{stem}-*.png"))):
            if os.path.basename(stale) not in names:
                os.remove(stale)
                print(f"STALE removed {os.path.basename(stale)}, no longer in the atlas", file=sys.stderr)
        if len(names) > PAGE_BUDGET:
            print(f"BUDGET zoom {zoom:g} needs {len(names)} pages; the budget is {PAGE_BUDGET}", file=sys.stderr)
            sys.exit(1)
        manifest = {"zoom": zoom, "tile": TILE_PX * zoom, "page": PAGE, "pages": names, "sprites": sprites}
        with open(os.path.join(out, stem + ".json"), "w") as f:
            json.dump(manifest, f, indent=1, sort_keys=True)
            f.write("\n")
        print(f"WROTE {stem}: {count} sprites on {len(names)} page(s)")
    if os.path.exists(tmp):
        os.remove(tmp)


def kinds(args):
    """Every kind of building in one sheet: a row per district type, a
    column per storey count, each standing on one ground line, so the
    set can be held against the art it was drawn from. Prints each
    piece's height and pane count, which the map's tests also read."""
    out = os.path.abspath(args.out)
    tmp = out + ".render.png"
    reset()
    settings()
    bpy.context.scene.render.film_transparent = True
    light()
    camera((0, 0, 0), 4, 64, 64)
    zoom = 2.0
    cuts = {}
    for name in buildings.names():
        root = buildings.build(name)
        ground = ground_point(root)
        _, (ax, ay) = frame_piece(root, args.heading, zoom, ground)
        cut, (x0, y0) = crop(render_pixels(tmp))
        cuts[name] = (cut, ax - x0, ay - y0)
        print(f"KIND {name}: {cut.shape[1]}x{cut.shape[0]} px, {len(panes(cut, warm_glass))} panes")
        remove(root)
    cell_w = max(c.shape[1] for c, _, _ in cuts.values()) + 24
    above = max(ay for _, _, ay in cuts.values())
    below = max(c.shape[0] - ay for c, _, ay in cuts.values())
    cell_h = int(above + below) + 24
    rows, cols = len(buildings.TYPES), buildings.MAX_STOREYS
    sheet = np.zeros((rows * cell_h, cols * cell_w, 4), dtype=np.uint8)
    sheet[:, :] = (17, 22, 28, 255)
    for name, (cut, ax, ay) in cuts.items():
        kind, n = name.rsplit("-", 1)
        r, c = buildings.TYPES.index(kind), int(n) - 1
        x = int(c * cell_w + cell_w / 2 - ax)
        y = int(r * cell_h + 12 + above - ay)
        region = sheet[y:y + cut.shape[0], x:x + cut.shape[1]]
        alpha = cut[:, :, 3:4].astype(np.float32) / 255
        region[:, :, :3] = (cut[:, :, :3] * alpha + region[:, :, :3] * (1 - alpha)).astype(np.uint8)
    h, w = sheet.shape[:2]
    img = bpy.data.images.new("kinds", w, h, alpha=True)
    img.pixels.foreach_set((sheet[::-1].astype(np.float32) / 255).ravel())
    img.filepath_raw = out
    img.file_format = "PNG"
    img.save()
    if os.path.exists(tmp):
        os.remove(tmp)
    print("WROTE", out)


def main():
    args = parse_args()
    if args.mode == "atlas":
        atlas(args)
        return
    if args.mode == "kinds":
        kinds(args)
        return
    reset()
    settings()
    light()
    district(args.kits)
    camera((4.0, 1.5, 0.4), args.scale, args.width, args.height)
    bpy.context.scene.render.filepath = os.path.abspath(args.out)
    bpy.ops.render.render(write_still=True)
    print("WROTE", bpy.context.scene.render.filepath)


if __name__ == "__main__":
    main()

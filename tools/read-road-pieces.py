#!/usr/bin/env python3
"""Which sides each road piece is open on, read off the atlas.

Run this rather than assuming, which is how bug 25 survived three
reports: the premise in recipes.go about the kit's own geometry was a
quarter turn out, and the table was derived from the premise.

  python3 tools/read-road-pieces.py

Prints a piece-by-turn table of open sides, and the check that makes it
trustworthy — every piece must come out with exactly the number of open
sides its name implies, at every turn. pkg/render's
TestRoadPieceOpensWhereItJoins holds roadPiece against these values.


At camera heading 0 the projection is screen=((x-y)s,(x+y)s/2) with +X
east and +Y south, so the tile diamond's upper-right edge faces north,
lower-right east, lower-left south, upper-left west. A piece drawn with
turn T uses the sprite baked at heading (0-T).

The test is the edge midpoint: a closed side has the kit's raised kerb
across it, which is near-white; an open side has road surface, which is
mid grey. A midpoint outside the sprite means the piece does not reach
that edge at all, which is closed by another name — the bend is a
partial tile and that is how it reads.
"""
import argparse
import glob
import json
from PIL import Image
LEVEL=max(glob.glob('pkg/assets/kits/kits-z*.json'), key=lambda p: json.load(open(p))['zoom'])
m=json.load(open(LEVEL))
sp=m['sprites']; TILE=m['tile']; HW,HH=TILE/2,TILE/4
MID={"N":(HW/2,-HH/2),"E":(HW/2,HH/2),"S":(-HW/2,HH/2),"W":(-HW/2,-HH/2)}
KERB=200
EXPECT={"road-straight":2,"road-crossroad":4,"road-bend":2,"road-intersection":3,"road-end":1}

def sides(name, turn):
    e=sp["city-kit-roads/"+name][str((0-turn)%360)]
    page=Image.open(f"pkg/assets/kits/{m['pages'][e['page']]}").convert("RGBA")
    img=page.crop((e['x'],e['y'],e['x']+e['w'],e['y']+e['h'])); px=img.load()
    ax,ay=e['ax'],e['ay']; out=[]
    for d,(dx,dy) in MID.items():
        vals=[]
        for t in (0.80,0.86,0.92,0.98):
            X,Y=int(round(ax+dx*t)),int(round(ay+dy*t))
            if 0<=X<img.width and 0<=Y<img.height:
                r,g,b,a=px[X,Y]
                if a>200: vals.append((r+g+b)/3)
        if vals and max(vals)<KERB: out.append(d)
    return "".join(d for d in "NESW" if d in out)

ok=True
argparse.ArgumentParser(description=__doc__.splitlines()[0]).parse_args()
for name,want in EXPECT.items():
    row=[]
    for t in (0,90,180,270):
        s=sides(name,t); row.append(f"turn {t:3d}: {s or '-':5s}")
        if len(s)!=want: ok=False; row[-1]+="!"
    print(f"{name:18s} " + " ".join(row))
print("\nevery piece has the number of open sides its name implies:", ok)

#!/usr/bin/env python3
"""Import a generated 5x8 pet sheet into assets/pets/<pet_id>/animations/*.png.

This is an asset tool, not runtime code. It keeps the Go runtime small.

Important design choice:
- Default mode does NOT remove background by color threshold. Color-threshold alpha
  caused holes in face/skin/highlights because pale pixels looked like background.
- The safe default crops cells from the known 5x8 sheet geometry, then fits the
  whole frame into a 256x256 transparent canvas. It preserves all character pixels.
- Optional border flood-fill background removal is available with --remove-bg, but
  it only removes pixels connected to the outer border, so inner face pixels are
  preserved.
"""
from __future__ import annotations

import argparse
import json
import shutil
from collections import deque
from pathlib import Path
from typing import Iterable, Tuple

try:
    from PIL import Image, ImageChops, ImageStat
except ImportError as exc:
    raise SystemExit("Missing Pillow. Install once with: py -m pip install pillow") from exc

DEFAULT_ACTS = ["idle", "walk", "run", "happy", "cry", "angry", "wave", "sleepy"]
DEFAULT_FRAME = 256
DEFAULT_COLS = 5
DEFAULT_ROWS = 8
BOX = Tuple[int, int, int, int]


def clamp(v: int, lo: int, hi: int) -> int:
    return max(lo, min(hi, v))


def parse_acts(value: str) -> list[str]:
    acts = [x.strip() for x in value.split(",") if x.strip()]
    if len(acts) != DEFAULT_ROWS:
        raise ValueError(f"--acts must contain exactly {DEFAULT_ROWS} names")
    return acts


def sample_corner_bg(img: Image.Image, sample: int = 24) -> tuple[int, int, int]:
    rgb = img.convert("RGB")
    w, h = rgb.size
    patches = [
        rgb.crop((0, 0, sample, sample)),
        rgb.crop((w - sample, 0, w, sample)),
        rgb.crop((0, h - sample, sample, h)),
        rgb.crop((w - sample, h - sample, w, h)),
    ]
    vals = []
    for p in patches:
        stat = ImageStat.Stat(p)
        vals.append(tuple(int(x) for x in stat.median))
    return tuple(sorted(vals)[len(vals) // 2])  # type: ignore[return-value]


def color_dist(a: tuple[int, int, int], b: tuple[int, int, int]) -> int:
    return abs(a[0] - b[0]) + abs(a[1] - b[1]) + abs(a[2] - b[2])


def remove_border_background(frame: Image.Image, threshold: int = 42) -> Image.Image:
    """Remove only background connected to image border; do not punch inner holes."""
    rgba = frame.convert("RGBA")
    w, h = rgba.size
    bg = sample_corner_bg(rgba)
    pix = rgba.load()
    visited = set()
    q: deque[tuple[int, int]] = deque()

    def maybe_add(x: int, y: int) -> None:
        if x < 0 or y < 0 or x >= w or y >= h or (x, y) in visited:
            return
        r, g, b, a = pix[x, y]
        if a == 0 or color_dist((r, g, b), bg) <= threshold:
            visited.add((x, y))
            q.append((x, y))

    for x in range(w):
        maybe_add(x, 0)
        maybe_add(x, h - 1)
    for y in range(h):
        maybe_add(0, y)
        maybe_add(w - 1, y)

    while q:
        x, y = q.popleft()
        maybe_add(x + 1, y)
        maybe_add(x - 1, y)
        maybe_add(x, y + 1)
        maybe_add(x, y - 1)

    out = rgba.copy()
    opix = out.load()
    for x, y in visited:
        r, g, b, _ = opix[x, y]
        opix[x, y] = (r, g, b, 0)
    return out


def alpha_bbox(img: Image.Image, pad: int = 6) -> BOX:
    box = img.getchannel("A").getbbox()
    if not box:
        return (0, 0, img.width, img.height)
    x0, y0, x1, y1 = box
    return (
        clamp(x0 - pad, 0, img.width),
        clamp(y0 - pad, 0, img.height),
        clamp(x1 + pad, 0, img.width),
        clamp(y1 + pad, 0, img.height),
    )


def grid_cell_box(img: Image.Image, col: int, row: int, cols: int, rows: int, margin_x: int, margin_y: int) -> BOX:
    """Crop by stable sheet geometry, with optional fixed margin trim.

    This intentionally avoids detecting foreground by color, because generated
    chibi sheets have pale face/highlight pixels near the white background.
    """
    w, h = img.size
    usable_x0 = margin_x
    usable_y0 = margin_y
    usable_w = w - margin_x * 2
    usable_h = h - margin_y * 2
    cw = usable_w / cols
    rh = usable_h / rows
    x0 = round(usable_x0 + col * cw)
    y0 = round(usable_y0 + row * rh)
    x1 = round(usable_x0 + (col + 1) * cw)
    y1 = round(usable_y0 + (row + 1) * rh)
    return (clamp(x0, 0, w), clamp(y0, 0, h), clamp(x1, 0, w), clamp(y1, 0, h))


def normalize_frame(frame: Image.Image, frame_size: int, remove_bg: bool, bg_threshold: int) -> Image.Image:
    frame = frame.convert("RGBA")
    if remove_bg:
        frame = remove_border_background(frame, threshold=bg_threshold)
        frame = frame.crop(alpha_bbox(frame, pad=8))

    max_w = int(frame_size * 0.92)
    max_h = int(frame_size * 0.94)
    frame.thumbnail((max_w, max_h), Image.Resampling.LANCZOS)

    out = Image.new("RGBA", (frame_size, frame_size), (0, 0, 0, 0))
    x = (frame_size - frame.width) // 2
    y = frame_size - frame.height - int(frame_size * 0.03)
    out.alpha_composite(frame, (x, y))
    return out


def write_pet_json_minimal(pet_dir: Path, pet_id: str, overwrite_json: bool = False) -> None:
    pet_json = pet_dir / "pet.json"
    if pet_json.exists() and not overwrite_json:
        return
    data = {
        "schema": 4,
        "id": pet_id,
        "name": pet_id,
        "animation_dir": "animations",
        "animations": {},
        "act_blacklist": ["source", "preview", "*_draft", "*_tmp"],
    }
    pet_json.write_text(json.dumps(data, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")


def import_sheet(
    src: Path,
    assets_root: Path,
    pet_id: str,
    frame_size: int,
    cols: int,
    rows: int,
    acts: list[str],
    overwrite: bool,
    margin_x: int,
    margin_y: int,
    remove_bg: bool,
    bg_threshold: int,
) -> None:
    if not src.exists():
        raise FileNotFoundError(src)
    if rows != len(acts):
        raise ValueError(f"rows={rows} but acts={len(acts)}")

    pet_dir = assets_root / "pets" / pet_id
    anim_dir = pet_dir / "animations"
    anim_dir.mkdir(parents=True, exist_ok=True)
    shutil.copy2(src, pet_dir / f"source_{src.stem}.png")

    img = Image.open(src).convert("RGBA")
    strip_size = (frame_size * cols, frame_size)

    for row, act in enumerate(acts):
        out_path = anim_dir / f"{act}.png"
        if out_path.exists() and not overwrite:
            print(f"skip existing: {out_path}")
            continue
        strip = Image.new("RGBA", strip_size, (0, 0, 0, 0))
        for col in range(cols):
            box = grid_cell_box(img, col, row, cols, rows, margin_x, margin_y)
            frame = normalize_frame(img.crop(box), frame_size, remove_bg, bg_threshold)
            strip.alpha_composite(frame, (col * frame_size, 0))
        strip.save(out_path)
        print(f"wrote: {out_path}")

    write_pet_json_minimal(pet_dir, pet_id)
    cache = pet_dir / ".petcache.json"
    if cache.exists():
        cache.unlink()
    print(f"done: {pet_dir}")


def main() -> None:
    parser = argparse.ArgumentParser(description="Split a 5x8 pet sheet into per-act animation strips.")
    parser.add_argument("--src", required=True, type=Path, help="Source sheet path, e.g. assets/pet2.png")
    parser.add_argument("--pet", required=True, help="Pet id/folder name, e.g. pet2")
    parser.add_argument("--assets", default=Path("assets"), type=Path, help="Assets root, default: assets")
    parser.add_argument("--acts", default=",".join(DEFAULT_ACTS), help="8 comma-separated row names")
    parser.add_argument("--frame", default=DEFAULT_FRAME, type=int, help="Output frame size, default: 256")
    parser.add_argument("--cols", default=DEFAULT_COLS, type=int, help="Sheet columns, default: 5")
    parser.add_argument("--rows", default=DEFAULT_ROWS, type=int, help="Sheet rows, default: 8")
    parser.add_argument("--margin-x", default=0, type=int, help="Fixed left/right sheet margin to ignore, default: 0")
    parser.add_argument("--margin-y", default=0, type=int, help="Fixed top/bottom sheet margin to ignore, default: 0")
    parser.add_argument("--remove-bg", action="store_true", help="Optional border flood-fill background removal. Safer than threshold punching, but off by default.")
    parser.add_argument("--bg-threshold", default=42, type=int, help="Border background color threshold for --remove-bg")
    parser.add_argument("--overwrite", action="store_true", help="Overwrite existing animation strips")
    args = parser.parse_args()

    import_sheet(
        src=args.src,
        assets_root=args.assets,
        pet_id=args.pet,
        frame_size=args.frame,
        cols=args.cols,
        rows=args.rows,
        acts=parse_acts(args.acts),
        overwrite=args.overwrite,
        margin_x=args.margin_x,
        margin_y=args.margin_y,
        remove_bg=args.remove_bg,
        bg_threshold=args.bg_threshold,
    )


if __name__ == "__main__":
    main()

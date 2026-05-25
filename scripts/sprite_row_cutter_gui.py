# -*- coding: utf-8 -*-
"""
Sprite Row Cutter GUI

Mục tiêu:
- Mở 1 ảnh sprite sheet.
- Dùng lưới 1 hàng x 1..20 frame để chọn đúng một hàng animation.
- Vùng được chọn sáng rõ, phần ngoài bị dim xám.
- Export hàng đó thành <act>.png dạng strip ngang.
- Mỗi frame output mặc định là 256x256, phù hợp desktop-pet-lite.

Dependency:
    py -m pip install pillow

Run:
    py scripts\\sprite_row_cutter_gui.py
"""
from __future__ import annotations

import json
import re
from collections import deque
from pathlib import Path
import tkinter as tk
from tkinter import ttk, filedialog, messagebox

from PIL import Image, ImageTk, ImageDraw, ImageStat


try:
    RESAMPLE = Image.Resampling.LANCZOS
except AttributeError:  # Pillow cũ
    RESAMPLE = Image.LANCZOS


MIN_FRAME_COUNT = 1
MAX_FRAME_COUNT = 20
DEFAULT_FRAME_COUNT = 5


DEFAULT_ACTS = [
    "idle",
    "walk",
    "run",
    "happy",
    "cry",
    "angry",
    "wave",
    "sleepy",
    "surprised",
    "shy",
    "thinking",
    "cheer",
    "scared",
    "dizzy",
    "dance",
    "sit_idle",
    "jump",
    "fly",
    "attack",
    "hurt",
]


def clamp(value: float, lo: float, hi: float) -> float:
    return max(lo, min(hi, value))


def safe_name(name: str) -> str:
    name = name.strip().lower()
    name = re.sub(r"[^a-z0-9_\-]+", "_", name)
    name = name.strip("_-")
    return name or "animation"


def color_distance(a: tuple[int, int, int], b: tuple[int, int, int]) -> int:
    return abs(a[0] - b[0]) + abs(a[1] - b[1]) + abs(a[2] - b[2])


def infer_initial_layout(width: int, height: int, preferred_frame_size: int) -> tuple[int, float, str]:
    """Infer frame count and initial row height for common sprite layouts."""
    if width <= 0 or height <= 0:
        return DEFAULT_FRAME_COUNT, 100.0, "fallback"

    preferred_frame_size = int(clamp(preferred_frame_size, 32, 1024))

    # Most exported pet strips are square frames in one horizontal row, e.g.
    # 2304x256 => 9 frames, 3072x256 => 12 frames.
    if width % height == 0:
        frames = width // height
        if MIN_FRAME_COUNT <= frames <= MAX_FRAME_COUNT:
            return frames, float(height), f"detected square strip: {width}/{height}={frames}"

    # If frame size is known, infer frames from width/frame_size. This also
    # handles multi-row sheets where each row is one animation.
    if width % preferred_frame_size == 0:
        frames = width // preferred_frame_size
        if MIN_FRAME_COUNT <= frames <= MAX_FRAME_COUNT:
            row_height = float(preferred_frame_size if height >= preferred_frame_size else height)
            return frames, row_height, f"detected by frame size: {width}/{preferred_frame_size}={frames}"

    # Fallback keeps the older behavior: assume the image contains several
    # animation rows and start with row 0.
    return DEFAULT_FRAME_COUNT, float(height) / 8.0, "fallback 8-row estimate"


def sample_corner_bg(img: Image.Image, sample: int = 24) -> tuple[int, int, int]:
    """Lấy màu nền ước lượng từ 4 góc."""
    rgb = img.convert("RGB")
    w, h = rgb.size
    sample = int(clamp(sample, 4, min(w, h)))

    patches = [
        rgb.crop((0, 0, sample, sample)),
        rgb.crop((w - sample, 0, w, sample)),
        rgb.crop((0, h - sample, sample, h)),
        rgb.crop((w - sample, h - sample, w, h)),
    ]

    values = []
    for patch in patches:
        stat = ImageStat.Stat(patch)
        values.append(tuple(int(x) for x in stat.median))

    return tuple(sorted(values)[len(values) // 2])  # type: ignore[return-value]


def remove_border_background(frame: Image.Image, threshold: int = 42) -> Image.Image:
    """
    Xóa nền bằng flood-fill từ viền ngoài.

    Khác với threshold toàn ảnh, cách này không khoét lỗ bên trong mặt/da/highlight.
    Nó chỉ xóa pixel giống nền và có kết nối với viền ngoài của frame.
    """
    rgba = frame.convert("RGBA")
    w, h = rgba.size
    if w <= 0 or h <= 0:
        return rgba

    bg = sample_corner_bg(rgba)
    pix = rgba.load()

    visited = bytearray(w * h)
    q: deque[tuple[int, int]] = deque()

    def idx(x: int, y: int) -> int:
        return y * w + x

    def maybe_add(x: int, y: int) -> None:
        if x < 0 or y < 0 or x >= w or y >= h:
            return

        i = idx(x, y)
        if visited[i]:
            return

        r, g, b, a = pix[x, y]
        if a == 0 or color_distance((r, g, b), bg) <= threshold:
            visited[i] = 1
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

    for y in range(h):
        row = y * w
        for x in range(w):
            if visited[row + x]:
                r, g, b, _ = opix[x, y]
                opix[x, y] = (r, g, b, 0)

    return out


def alpha_bbox(img: Image.Image, pad: int = 8) -> tuple[int, int, int, int]:
    box = img.getchannel("A").getbbox()
    if not box:
        return 0, 0, img.width, img.height

    x0, y0, x1, y1 = box
    return (
        int(clamp(x0 - pad, 0, img.width)),
        int(clamp(y0 - pad, 0, img.height)),
        int(clamp(x1 + pad, 0, img.width)),
        int(clamp(y1 + pad, 0, img.height)),
    )


class SpriteRowCutterApp(tk.Tk):
    def __init__(self) -> None:
        super().__init__()

        self.title("Sprite Row Cutter - 1x1 to 1x20 Animation Cutter")
        self.geometry("1220x840")
        self.minsize(980, 680)

        self.image_path: Path | None = None
        self.output_dir: Path = Path.cwd()
        self.output_dir_user_set = False

        self.image: Image.Image | None = None
        self.tk_image: ImageTk.PhotoImage | None = None

        self.scale = 1.0
        self.fit_scale = 1.0
        self.user_zoom = 1.0
        self.offset_x = 0
        self.offset_y = 0
        self.pan_x = 0.0
        self.pan_y = 0.0

        # Selection in original image coordinate: x, y, w, h.
        self.sel = [0.0, 0.0, 100.0, 100.0]

        self.drag_mode: str | None = None
        self.drag_start_img = (0.0, 0.0)
        self.drag_start_sel = [0.0, 0.0, 100.0, 100.0]
        self.drag_start_pan = (0.0, 0.0)

        self.act_name_var = tk.StringVar(value="idle")
        self.frame_count_var = tk.IntVar(value=DEFAULT_FRAME_COUNT)
        self.frame_size_var = tk.IntVar(value=256)
        self.remove_bg_var = tk.BooleanVar(value=True)
        self.fit_content_var = tk.BooleanVar(value=True)
        self.save_frames_var = tk.BooleanVar(value=False)
        self.bg_threshold_var = tk.IntVar(value=42)
        self.row_index_var = tk.IntVar(value=0)

        self.status_var = tk.StringVar(value="Open Image để bắt đầu.")

        self._build_ui()
        self._bind_events()

    # ---------------- UI ----------------

    def _build_ui(self) -> None:
        root = ttk.Frame(self)
        root.pack(fill="both", expand=True)

        toolbar = ttk.Frame(root, padding=(8, 6))
        toolbar.pack(fill="x")

        ttk.Button(toolbar, text="Open Image", command=self.open_image).pack(side="left", padx=(0, 6))
        ttk.Button(toolbar, text="Output Dir", command=self.choose_output_dir).pack(side="left", padx=(0, 12))

        ttk.Label(toolbar, text="Act name:").pack(side="left")
        act_box = ttk.Combobox(toolbar, textvariable=self.act_name_var, width=16, values=DEFAULT_ACTS)
        act_box.pack(side="left", padx=(4, 12))

        ttk.Label(toolbar, text="Frames:").pack(side="left")
        frame_spin = ttk.Spinbox(
            toolbar,
            from_=MIN_FRAME_COUNT,
            to=MAX_FRAME_COUNT,
            textvariable=self.frame_count_var,
            width=4,
            command=self.redraw,
        )
        frame_spin.pack(side="left", padx=(4, 12))
        frame_spin.bind("<KeyRelease>", lambda _e: self.redraw())

        ttk.Label(toolbar, text="Frame size:").pack(side="left")
        ttk.Entry(toolbar, textvariable=self.frame_size_var, width=6).pack(side="left", padx=(4, 12))

        ttk.Checkbutton(toolbar, text="Remove BG", variable=self.remove_bg_var).pack(side="left", padx=(0, 8))
        ttk.Checkbutton(toolbar, text="Fit content", variable=self.fit_content_var).pack(side="left", padx=(0, 8))
        ttk.Checkbutton(toolbar, text="Save frames", variable=self.save_frames_var).pack(side="left", padx=(0, 12))

        ttk.Label(toolbar, text="BG threshold:").pack(side="left")
        ttk.Entry(toolbar, textvariable=self.bg_threshold_var, width=5).pack(side="left", padx=(4, 12))

        ttk.Button(toolbar, text="Auto Row", command=self.auto_row_by_index).pack(side="right", padx=(8, 0))
        ttk.Spinbox(toolbar, from_=0, to=99, textvariable=self.row_index_var, width=4).pack(side="right")
        ttk.Label(toolbar, text="Row:").pack(side="right", padx=(8, 4))
        ttk.Button(toolbar, text="Export + Next", command=self.export_row_and_next).pack(side="right", padx=(8, 0))
        ttk.Button(toolbar, text="Export Row", command=self.export_row).pack(side="right", padx=(8, 0))

        hint = ttk.Label(
            root,
            text=(
                "Mouse: kéo vùng sáng để di chuyển; kéo viền/góc lớn để resize. "
                "Wheel: zoom ảnh. Shift+Wheel: đổi chiều cao. Ctrl+Wheel: đổi chiều rộng. Middle/Right drag: pan. "
                "Arrow: di chuyển. Shift+Arrow: resize. PageUp/PageDown: nhảy 1 hàng. Enter: export. Ctrl+Enter: export+next."
            ),
            padding=(8, 0),
        )
        hint.pack(fill="x")

        self.canvas = tk.Canvas(root, bg="#111111", highlightthickness=0)
        self.canvas.pack(fill="both", expand=True, padx=8, pady=8)

        status = ttk.Label(root, textvariable=self.status_var, anchor="w", padding=(8, 4))
        status.pack(fill="x")

    def _bind_events(self) -> None:
        self.canvas.bind("<Configure>", lambda _e: self.redraw())
        self.canvas.bind("<ButtonPress-1>", self.on_mouse_down)
        self.canvas.bind("<B1-Motion>", self.on_mouse_move)
        self.canvas.bind("<ButtonRelease-1>", self.on_mouse_up)
        self.canvas.bind("<ButtonPress-2>", self.on_pan_down)
        self.canvas.bind("<B2-Motion>", self.on_pan_move)
        self.canvas.bind("<ButtonRelease-2>", self.on_pan_up)
        self.canvas.bind("<ButtonPress-3>", self.on_pan_down)
        self.canvas.bind("<B3-Motion>", self.on_pan_move)
        self.canvas.bind("<ButtonRelease-3>", self.on_pan_up)
        self.canvas.bind("<MouseWheel>", self.on_mouse_wheel)

        # Linux wheel fallback.
        self.canvas.bind("<Button-4>", lambda e: self.on_mouse_wheel(e, delta=120))
        self.canvas.bind("<Button-5>", lambda e: self.on_mouse_wheel(e, delta=-120))

        self.bind("<Left>", lambda e: self.nudge(-1, 0, e))
        self.bind("<Right>", lambda e: self.nudge(1, 0, e))
        self.bind("<Up>", lambda e: self.nudge(0, -1, e))
        self.bind("<Down>", lambda e: self.nudge(0, 1, e))

        self.bind("<Prior>", lambda _e: self.move_selection(0, -self.sel[3]))  # PageUp
        self.bind("<Next>", lambda _e: self.move_selection(0, self.sel[3]))  # PageDown
        self.bind("<plus>", lambda _e: self.zoom_image(1.15))
        self.bind("<KP_Add>", lambda _e: self.zoom_image(1.15))
        self.bind("<minus>", lambda _e: self.zoom_image(1 / 1.15))
        self.bind("<KP_Subtract>", lambda _e: self.zoom_image(1 / 1.15))
        self.bind("<Control-0>", lambda _e: self.reset_zoom())
        self.bind("<Return>", lambda _e: self.export_row())
        self.bind("<Control-Return>", lambda _e: self.export_row_and_next())

    # ---------------- File actions ----------------

    def open_image(self) -> None:
        path = filedialog.askopenfilename(
            title="Select sprite sheet",
            filetypes=[
                ("Image files", "*.png *.jpg *.jpeg *.webp *.bmp"),
                ("All files", "*.*"),
            ],
        )
        if not path:
            return

        try:
            img = Image.open(path).convert("RGBA")
        except Exception as exc:
            messagebox.showerror("Open image failed", str(exc))
            return

        self.image_path = Path(path)
        self.image = img
        if not self.output_dir_user_set:
            self.output_dir = self.image_path.parent
        self.user_zoom = 1.0
        self.pan_x = 0.0
        self.pan_y = 0.0

        frames, row_height, detect_note = infer_initial_layout(img.width, img.height, self.get_frame_size())
        self.frame_count_var.set(frames)
        self.sel = [
            0.0,
            0.0,
            float(img.width),
            row_height,
        ]
        self.row_index_var.set(0)
        self.act_name_var.set(DEFAULT_ACTS[0])

        self.status_var.set(
            f"Loaded: {self.image_path.name} | size={img.width}x{img.height} | frames={frames} | {detect_note} | output={self.output_dir}"
        )
        self.redraw()

    def choose_output_dir(self) -> None:
        path = filedialog.askdirectory(title="Select output animations directory")
        if not path:
            return
        self.output_dir = Path(path)
        self.output_dir.mkdir(parents=True, exist_ok=True)
        self.output_dir_user_set = True
        self.status_var.set(f"Output dir: {self.output_dir}")

    # ---------------- Coordinate helpers ----------------

    def get_frame_count(self) -> int:
        try:
            return int(clamp(int(self.frame_count_var.get()), MIN_FRAME_COUNT, MAX_FRAME_COUNT))
        except Exception:
            return DEFAULT_FRAME_COUNT

    def get_frame_size(self) -> int:
        try:
            return int(clamp(int(self.frame_size_var.get()), 32, 1024))
        except Exception:
            return 256

    def get_bg_threshold(self) -> int:
        try:
            return int(clamp(int(self.bg_threshold_var.get()), 0, 255))
        except Exception:
            return 42

    def image_to_canvas(self, x: float, y: float) -> tuple[float, float]:
        return self.offset_x + x * self.scale, self.offset_y + y * self.scale

    def canvas_to_image(self, x: float, y: float) -> tuple[float, float]:
        return (x - self.offset_x) / self.scale, (y - self.offset_y) / self.scale

    def clamp_selection(self) -> None:
        if self.image is None:
            return

        x, y, w, h = self.sel

        min_w = max(40.0, self.get_frame_count() * 12.0)
        min_h = 24.0

        w = clamp(w, min_w, self.image.width)
        h = clamp(h, min_h, self.image.height)

        x = clamp(x, 0, self.image.width - w)
        y = clamp(y, 0, self.image.height - h)

        self.sel = [x, y, w, h]

    # ---------------- Render ----------------

    def redraw(self) -> None:
        self.canvas.delete("all")

        if self.image is None:
            self.canvas.create_text(
                self.canvas.winfo_width() // 2,
                self.canvas.winfo_height() // 2,
                text="Open Image",
                fill="#eeeeee",
                font=("Segoe UI", 22, "bold"),
            )
            return

        self.clamp_selection()

        cw = max(1, self.canvas.winfo_width())
        ch = max(1, self.canvas.winfo_height())
        iw, ih = self.image.size

        self.fit_scale = min(cw / iw, ch / ih) * 0.96
        self.fit_scale = max(0.05, self.fit_scale)
        self.user_zoom = clamp(self.user_zoom, 0.1, 16.0)
        self.scale = max(0.02, self.fit_scale * self.user_zoom)

        dw = max(1, int(iw * self.scale))
        dh = max(1, int(ih * self.scale))
        self.offset_x = int((cw - dw) // 2 + self.pan_x)
        self.offset_y = int((ch - dh) // 2 + self.pan_y)

        base = self.image.resize((dw, dh), RESAMPLE).convert("RGBA")

        x, y, w, h = self.sel
        sx0 = int(x * self.scale)
        sy0 = int(y * self.scale)
        sx1 = int((x + w) * self.scale)
        sy1 = int((y + h) * self.scale)

        sx0 = int(clamp(sx0, 0, dw))
        sy0 = int(clamp(sy0, 0, dh))
        sx1 = int(clamp(sx1, 0, dw))
        sy1 = int(clamp(sy1, 0, dh))

        # Dim outside selection.
        dim = Image.new("RGBA", base.size, (0, 0, 0, 155))
        preview = Image.alpha_composite(base, dim)

        if sx1 > sx0 and sy1 > sy0:
            crop = base.crop((sx0, sy0, sx1, sy1))
            preview.paste(crop, (sx0, sy0))

        draw = ImageDraw.Draw(preview)

        # Selection border.
        draw.rectangle((sx0, sy0, sx1, sy1), outline=(255, 255, 255, 255), width=3)
        draw.rectangle((sx0 + 2, sy0 + 2, sx1 - 2, sy1 - 2), outline=(255, 220, 80, 255), width=1)

        # Grid.
        frames = self.get_frame_count()
        if frames > 1:
            for i in range(1, frames):
                gx = int(sx0 + (sx1 - sx0) * i / frames)
                draw.line((gx, sy0, gx, sy1), fill=(255, 255, 255, 220), width=2)

        # Handles.
        handle = 18
        for hx, hy in [
            (sx0, sy0),
            (sx1, sy0),
            (sx0, sy1),
            (sx1, sy1),
            ((sx0 + sx1) // 2, sy0),
            ((sx0 + sx1) // 2, sy1),
            (sx0, (sy0 + sy1) // 2),
            (sx1, (sy0 + sy1) // 2),
        ]:
            draw.rectangle(
                (hx - handle // 2, hy - handle // 2, hx + handle // 2, hy + handle // 2),
                fill=(255, 255, 255, 230),
                outline=(0, 0, 0, 180),
            )

        self.status_var.set(
            f"{self.act_name_var.get() or 'animation'} | {frames} frames | "
            f"crop x={int(x)} y={int(y)} w={int(w)} h={int(h)} | "
            f"zoom={self.user_zoom:.2f}x | output={self.output_dir}"
        )

        self.tk_image = ImageTk.PhotoImage(preview)
        self.canvas.create_image(self.offset_x, self.offset_y, image=self.tk_image, anchor="nw")

    # ---------------- Mouse interaction ----------------

    def hit_test(self, ix: float, iy: float) -> str:
        x, y, w, h = self.sel
        x0, y0, x1, y1 = x, y, x + w, y + h

        threshold = max(14.0 / self.scale, 6.0)

        near_l = abs(ix - x0) <= threshold
        near_r = abs(ix - x1) <= threshold
        near_t = abs(iy - y0) <= threshold
        near_b = abs(iy - y1) <= threshold
        inside = x0 <= ix <= x1 and y0 <= iy <= y1

        if near_l and near_t:
            return "resize_tl"
        if near_r and near_t:
            return "resize_tr"
        if near_l and near_b:
            return "resize_bl"
        if near_r and near_b:
            return "resize_br"
        if near_l and inside:
            return "resize_l"
        if near_r and inside:
            return "resize_r"
        if near_t and inside:
            return "resize_t"
        if near_b and inside:
            return "resize_b"
        if inside:
            return "move"
        return "move_center"

    def on_mouse_down(self, event: tk.Event) -> None:
        if self.image is None:
            return

        ix, iy = self.canvas_to_image(event.x, event.y)
        self.drag_mode = self.hit_test(ix, iy)
        self.drag_start_img = (ix, iy)
        self.drag_start_sel = list(self.sel)

        if self.drag_mode == "move_center":
            self.sel[0] = ix - self.sel[2] / 2
            self.sel[1] = iy - self.sel[3] / 2
            self.clamp_selection()
            self.drag_mode = "move"
            self.drag_start_img = (ix, iy)
            self.drag_start_sel = list(self.sel)

        self.redraw()

    def on_mouse_move(self, event: tk.Event) -> None:
        if self.image is None or not self.drag_mode:
            return

        ix, iy = self.canvas_to_image(event.x, event.y)
        sx, sy = self.drag_start_img
        dx = ix - sx
        dy = iy - sy

        x, y, w, h = self.drag_start_sel
        mode = self.drag_mode

        if mode == "move":
            self.sel = [x + dx, y + dy, w, h]
        elif mode == "resize_l":
            self.sel = [x + dx, y, w - dx, h]
        elif mode == "resize_r":
            self.sel = [x, y, w + dx, h]
        elif mode == "resize_t":
            self.sel = [x, y + dy, w, h - dy]
        elif mode == "resize_b":
            self.sel = [x, y, w, h + dy]
        elif mode == "resize_tl":
            self.sel = [x + dx, y + dy, w - dx, h - dy]
        elif mode == "resize_tr":
            self.sel = [x, y + dy, w + dx, h - dy]
        elif mode == "resize_bl":
            self.sel = [x + dx, y, w - dx, h + dy]
        elif mode == "resize_br":
            self.sel = [x, y, w + dx, h + dy]

        self.clamp_selection()
        self.redraw()

    def on_mouse_up(self, _event: tk.Event) -> None:
        self.drag_mode = None

    def on_mouse_wheel(self, event: tk.Event, delta: int | None = None) -> None:
        if self.image is None:
            return

        if delta is None:
            delta = event.delta

        direction = 1 if delta > 0 else -1
        step = 8
        ctrl_pressed = bool(getattr(event, "state", 0) & 0x0004)

        x, y, w, h = self.sel

        shift_pressed = bool(getattr(event, "state", 0) & 0x0001)

        if ctrl_pressed:
            # Resize width from center.
            new_w = w + direction * step * 2
            cx = x + w / 2
            self.sel = [cx - new_w / 2, y, new_w, h]
            self.clamp_selection()
            self.redraw()
            return
        if shift_pressed:
            # Resize height from center.
            new_h = h + direction * step * 2
            cy = y + h / 2
            self.sel = [x, cy - new_h / 2, w, new_h]
            self.clamp_selection()
            self.redraw()
            return

        self.zoom_image(1.12 if direction > 0 else 1 / 1.12, event.x, event.y)

    def zoom_image(self, factor: float, canvas_x: float | None = None, canvas_y: float | None = None) -> None:
        if self.image is None:
            return

        if canvas_x is None:
            canvas_x = self.canvas.winfo_width() / 2
        if canvas_y is None:
            canvas_y = self.canvas.winfo_height() / 2

        before = self.canvas_to_image(canvas_x, canvas_y)
        self.user_zoom = clamp(self.user_zoom * factor, 0.1, 16.0)
        self.redraw()
        after_canvas_x, after_canvas_y = self.image_to_canvas(*before)
        self.pan_x += canvas_x - after_canvas_x
        self.pan_y += canvas_y - after_canvas_y
        self.redraw()

    def reset_zoom(self) -> None:
        self.user_zoom = 1.0
        self.pan_x = 0.0
        self.pan_y = 0.0
        self.redraw()

    def on_pan_down(self, event: tk.Event) -> None:
        self.drag_mode = "pan"
        self.drag_start_img = (float(event.x), float(event.y))
        self.drag_start_pan = (self.pan_x, self.pan_y)

    def on_pan_move(self, event: tk.Event) -> None:
        if self.drag_mode != "pan":
            return
        sx, sy = self.drag_start_img
        px, py = self.drag_start_pan
        self.pan_x = px + float(event.x) - sx
        self.pan_y = py + float(event.y) - sy
        self.redraw()

    def on_pan_up(self, _event: tk.Event) -> None:
        if self.drag_mode == "pan":
            self.drag_mode = None

    def nudge(self, dx: int, dy: int, event: tk.Event) -> None:
        if self.image is None:
            return

        shift_pressed = bool(getattr(event, "state", 0) & 0x0001)
        ctrl_pressed = bool(getattr(event, "state", 0) & 0x0004)
        step = 10 if ctrl_pressed else 1

        if shift_pressed:
            x, y, w, h = self.sel
            self.sel = [x, y, w + dx * step, h + dy * step]
        else:
            self.move_selection(dx * step, dy * step)

        self.clamp_selection()
        self.redraw()

    def move_selection(self, dx: float, dy: float) -> None:
        if self.image is None:
            return
        x, y, w, h = self.sel
        self.sel = [x + dx, y + dy, w, h]
        self.clamp_selection()
        self.redraw()

    def auto_row_by_index(self) -> None:
        """Đặt selection vào row N theo chiều cao hiện tại hoặc chia đều 8 hàng nếu mới mở ảnh."""
        if self.image is None:
            return
        try:
            row = max(0, int(self.row_index_var.get()))
        except Exception:
            row = 0
        x, _y, w, h = self.sel
        y = row * h
        if y + h > self.image.height:
            y = max(0, self.image.height - h)
        self.sel = [x, y, w, h]
        if row < len(DEFAULT_ACTS):
            self.act_name_var.set(DEFAULT_ACTS[row])
        self.clamp_selection()
        self.redraw()

    # ---------------- Export ----------------

    def prepare_frame(self, crop: Image.Image) -> Image.Image:
        frame_size = self.get_frame_size()
        bg_threshold = self.get_bg_threshold()
        frame = crop.convert("RGBA")

        if self.remove_bg_var.get():
            frame = remove_border_background(frame, threshold=bg_threshold)

        if self.fit_content_var.get():
            box = alpha_bbox(frame, pad=8)
            frame = frame.crop(box)

        max_w = int(frame_size * 0.92)
        max_h = int(frame_size * 0.94)
        frame.thumbnail((max_w, max_h), RESAMPLE)

        out = Image.new("RGBA", (frame_size, frame_size), (0, 0, 0, 0))
        px = (frame_size - frame.width) // 2

        # Baseline-align: nhân vật đứng gần đáy frame.
        py = frame_size - frame.height - int(frame_size * 0.035)
        out.alpha_composite(frame, (px, py))
        return out

    def export_row(self) -> bool:
        if self.image is None:
            messagebox.showwarning("No image", "Bạn chưa mở ảnh.")
            return False

        act = safe_name(self.act_name_var.get())
        frames = self.get_frame_count()
        frame_size = self.get_frame_size()

        self.clamp_selection()
        x, y, w, h = self.sel

        out_dir = self.output_dir
        out_dir.mkdir(parents=True, exist_ok=True)

        strip = Image.new("RGBA", (frame_size * frames, frame_size), (0, 0, 0, 0))
        individual_frames: list[Image.Image] = []

        for i in range(frames):
            fx0 = int(round(x + w * i / frames))
            fy0 = int(round(y))
            fx1 = int(round(x + w * (i + 1) / frames))
            fy1 = int(round(y + h))

            crop = self.image.crop((fx0, fy0, fx1, fy1))
            prepared = self.prepare_frame(crop)
            strip.alpha_composite(prepared, (i * frame_size, 0))
            individual_frames.append(prepared)

        out_path = out_dir / f"{act}.png"
        if out_path.exists():
            if not messagebox.askyesno("Overwrite image?", f"File đã tồn tại:\n{out_path}\n\nGhi đè file này?"):
                self.status_var.set(f"Export cancelled: {out_path}")
                return False
        strip.save(out_path)

        if self.save_frames_var.get():
            frames_dir = out_dir / f"{act}_frames"
            frames_dir.mkdir(parents=True, exist_ok=True)
            for i, frame in enumerate(individual_frames):
                frame.save(frames_dir / f"{act}_{i:02d}.png")

        metadata = {
            "source": str(self.image_path) if self.image_path else "",
            "output": str(out_path),
            "act": act,
            "frames": frames,
            "frame_size": frame_size,
            "selection": {
                "x": round(x, 2),
                "y": round(y, 2),
                "w": round(w, 2),
                "h": round(h, 2),
            },
            "remove_bg": bool(self.remove_bg_var.get()),
            "fit_content": bool(self.fit_content_var.get()),
            "bg_threshold": self.get_bg_threshold(),
        }

        history_path = out_dir / "_cut_history.jsonl"
        with history_path.open("a", encoding="utf-8") as f:
            f.write(json.dumps(metadata, ensure_ascii=False) + "\n")

        self.status_var.set(f"Exported: {out_path}")
        return True

    def export_row_and_next(self) -> None:
        if not self.export_row():
            return
        try:
            current_row = int(self.row_index_var.get())
        except Exception:
            current_row = 0
        next_row = current_row + 1
        self.row_index_var.set(next_row)
        if next_row < len(DEFAULT_ACTS):
            self.act_name_var.set(DEFAULT_ACTS[next_row])
        self.auto_row_by_index()


def main() -> None:
    app = SpriteRowCutterApp()
    app.mainloop()


if __name__ == "__main__":
    main()

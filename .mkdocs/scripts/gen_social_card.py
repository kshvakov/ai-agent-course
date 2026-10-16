"""Generate the 1200x630 social card PNG.

One-off local generator (expects Pillow and macOS Arial fonts); the output
PNG is committed, so CI does not need to run this.

Run: .venv/bin/python .mkdocs/scripts/gen_social_card.py
"""
import os

from PIL import Image, ImageDraw, ImageFont

W, H = 1200, 630
OUT = os.path.join(
    os.path.dirname(__file__), "..", "overrides", "assets", "images", "social-card.png"
)

TOP = (69, 90, 220)     # indigo A200
BOTTOM = (26, 35, 126)  # indigo 900

FONT_DIR = "/System/Library/Fonts/Supplemental"


def font(name, size):
    return ImageFont.truetype(os.path.join(FONT_DIR, name), size)


def centered(draw, text, fnt, cy, fill):
    box = draw.textbbox((0, 0), text, font=fnt)
    w, h = box[2] - box[0], box[3] - box[1]
    draw.text(((W - w) / 2 - box[0], cy - h / 2 - box[1]), text, font=fnt, fill=fill)


img = Image.new("RGB", (W, H))
draw = ImageDraw.Draw(img)
for y in range(H):
    t = y / (H - 1)
    color = tuple(int(a + (b - a) * t) for a, b in zip(TOP, BOTTOM))
    draw.line([(0, y), (W, y)], fill=color)

centered(draw, "AI Agent Course", font("Arial Bold.ttf", 96), 240, "white")
centered(
    draw,
    "Designing Autonomous AI Agents in Go",
    font("Arial.ttf", 46),
    380,
    (197, 202, 233),
)
centered(
    draw,
    "kshvakov.github.io/ai-agent-course",
    font("Arial.ttf", 30),
    560,
    (150, 160, 220),
)

os.makedirs(os.path.dirname(OUT), exist_ok=True)
img.save(OUT, "PNG")
print("saved", os.path.abspath(OUT))

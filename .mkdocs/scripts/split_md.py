#!/usr/bin/env python3
"""Split a markdown file into slices at fence-parity boundaries.

Usage: split_md.py SRC OUT_DIR TARGET_LINES
Creates OUT_DIR/slice-01.md, slice-02.md, ... and prints them.
"""
import sys
from pathlib import Path

src = Path(sys.argv[1])
out_dir = Path(sys.argv[2])
target = int(sys.argv[3])

lines = src.read_text(encoding="utf-8").splitlines(keepends=True)

# positions where fence count before the line is even (safe cut points)
cut_ok = []
count = 0
for i, line in enumerate(lines):
    if line.startswith("```"):
        # cut allowed BEFORE this line only if fences so far are even
        if count % 2 == 0:
            cut_ok.append(i)
        count += 1

# choose split points greedily
slices = []
start = 0
i = 0
while i < len(lines):
    # find furthest safe cut within target window
    best = None
    for pos in cut_ok:
        if pos <= start:
            continue
        if pos - start <= target:
            best = pos
        else:
            break
    if best is None or best <= start:
        best = len(lines)
    slices.append((start, best))
    start = best
    if start >= len(lines):
        break
    # avoid infinite loop: ensure progress
    i += 1

out_dir.mkdir(parents=True, exist_ok=True)
for n, (a, b) in enumerate(slices, 1):
    p = out_dir / f"slice-{n:02d}.md"
    p.write_text("".join(lines[a:b]), encoding="utf-8")
    print(f"{p} [{a+1}-{b}] {b-a} lines")

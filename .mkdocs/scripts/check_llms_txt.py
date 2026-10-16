#!/usr/bin/env python3
"""Sanity-check the llms_txt plugin artifacts in the built site.

The plugin depends on undocumented mkdocs-static-i18n 1.x behavior
(on_nav rerun per locale, en first; on_post_build inside the last
locale build), so a dependency upgrade could silently truncate
llms.txt to the en-only snapshot. This check fails loudly instead:
llms.txt must cover both locales, llms-full.txt must carry RU content,
and the patched 404 must be en + noindex.
"""
import sys
from pathlib import Path

MIN_ENTRIES = 54  # 28 EN + 28 RU pages currently in the nav


def fail(msg: str) -> int:
    print(f"error: {msg}")
    return 1


def main() -> int:
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    root = Path(args[0]) if args else Path("build/site")
    if not root.is_dir():
        print(f"error: {root} is not a directory (run mkdocs build first)")
        return 1

    llms = root / "llms.txt"
    if not llms.exists():
        return fail("llms.txt not found")
    text = llms.read_text(encoding="utf-8")

    entries = [line for line in text.splitlines() if line.startswith("- [")]
    if len(entries) < MIN_ENTRIES:
        return fail(f"llms.txt has {len(entries)} entries, expected >= {MIN_ENTRIES} "
                    "(RU locale missing? plugin/i18n contract broken?)")
    if "## Русская версия" not in text:
        return fail('llms.txt lacks the "## Русская версия" section')

    full = root / "llms-full.txt"
    if not full.exists():
        return fail("llms-full.txt not found")
    full_text = full.read_text(encoding="utf-8")
    if len(full_text) < 100_000:
        return fail(f"llms-full.txt is only {len(full_text)} chars, looks truncated")
    if "## Русская версия" not in full_text and "Предисловие" not in full_text:
        return fail("llms-full.txt lacks RU content")

    page_404 = root / "404.html"
    if not page_404.exists():
        return fail("404.html not found")
    html_404 = page_404.read_text(encoding="utf-8")
    if '<html lang="en"' not in html_404:
        return fail("404.html is not lang=en (404 patch not applied)")
    if "noindex" not in html_404:
        return fail("404.html lacks noindex")

    print(f"OK: llms.txt ({len(entries)} entries), llms-full.txt "
          f"({len(full_text)} chars), 404.html patched [site, ru/ included]")
    return 0


if __name__ == "__main__":
    sys.exit(main())

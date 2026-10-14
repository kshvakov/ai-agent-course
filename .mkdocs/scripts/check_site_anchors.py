#!/usr/bin/env python3
"""Verify every cross-page href fragment in the built site against target ids.

Exact fragment comparison: no slash normalization, so a plugin artifact
like href="...#anchor/" is reported as broken. Run AFTER
fix_anchor_slashes.py.

The ru/ subtree is included: RU headings use unicode slugify
(markdown.extensions.toc.slugify_unicode, configured in mkdocs.yml),
so Cyrillic slugs are stable and resolvable. --include-ru is kept for
compatibility and is now a no-op (both scopes check the same pages).
"""
import re
import sys
import urllib.parse
from pathlib import Path

HREF = re.compile(r'href="([^"]*#[^"]*)"')
ID = re.compile(r'id="([^"]+)"')
NAME = re.compile(r'name="([^"]+)"')


def main() -> int:
    args = [a for a in sys.argv[1:] if not a.startswith("--")]
    root = Path(args[0]) if args else Path("build/site")
    if not root.is_dir():
        print(f"error: {root} is not a directory (run mkdocs build first)")
        return 1

    def scoped(page: Path) -> bool:
        rel = page.relative_to(root).as_posix()
        return True  # ru/ uses unicode slugs now; the whole site is checked

    # id/name per page, keyed by posix path relative to site root
    anchors: dict[str, set[str]] = {}
    pages: list[Path] = []
    for page in sorted(root.rglob("*.html")):
        if not scoped(page):
            continue
        rel = page.relative_to(root).as_posix()
        html = page.read_text(encoding="utf-8")
        anchors[rel] = set(ID.findall(html)) | set(NAME.findall(html))
        pages.append(page)
    broken: list[tuple[str, str]] = []
    for page in pages:
        rel = page.relative_to(root).as_posix()
        for href in HREF.findall(page.read_text(encoding="utf-8")):
            target, _, frag = href.partition("#")
            if not frag:
                continue
            if target.startswith(("http://", "https://")):
                continue
            frag = urllib.parse.unquote(frag)
            if target:
                resolved = (page.parent / target).resolve().relative_to(root.resolve())
                tp = urllib.parse.unquote(resolved.as_posix())
                if (root / tp).is_dir():
                    tp = tp.rstrip("/") + "/index.html"
            else:
                tp = rel
            if tp not in anchors or frag not in anchors[tp]:
                broken.append((rel, href))
    if broken:
        print(f"BROKEN: {len(broken)} fragment link(s):")
        for page, href in sorted(set(broken)):
            print(f"  {page}: {href}")
        return 1
    print(f"OK: all fragment links resolve to existing ids in {root} [site, ru/ included]")
    return 0


if __name__ == "__main__":
    sys.exit(main())

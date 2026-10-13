#!/usr/bin/env python3
"""Fix trailing slashes in href fragments produced by the i18n plugin.

The i18n plugin rewrites README.md#anchor links into directory URLs and
appends the directory trailing slash to the fragment as well:
href="../00-preface/#anchor/" -> must be href="../00-preface/#anchor".

Run AFTER `mkdocs build`; fixes build/site/**/*.html in place.
"""
import re
import sys
from pathlib import Path

# Matches href="...#fragment/" where the trailing slash belongs to the
# plugin's URL join, not the fragment itself. Fragments never contain "/".
HREF_SLASH = re.compile(r'(href="[^"#]*#[^"]*?)/"')


def main() -> int:
    root = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("build/site")
    if not root.is_dir():
        print(f"error: {root} is not a directory (run mkdocs build first)")
        return 1
    fixed = 0
    for page in sorted(root.rglob("*.html")):
        html = page.read_text(encoding="utf-8")
        new_html, n = HREF_SLASH.subn(r'\1"', html)
        if n:
            page.write_text(new_html, encoding="utf-8")
            fixed += n
    print(f"fixed {fixed} href fragment(s) with trailing slash in {root}")
    return 0


if __name__ == "__main__":
    sys.exit(main())

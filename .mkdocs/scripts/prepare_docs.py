#!/usr/bin/env python3
"""
Prepare documentation structure for MkDocs build.

This script:
1. Copies README.md files to index.md (for clean URLs)
2. Copies MANUAL.md and SOLUTION.md files as-is
3. Copies STYLE.md to style.md
4. Fixes internal links (README.md -> index.md or remove README.md)
5. Creates structure for both EN and RU versions
"""

import os
import re
import shutil
from pathlib import Path

# Base paths
# Script is now in .mkdocs/scripts/, so we need to go up 3 levels to reach repo root
REPO_ROOT = Path(__file__).parent.parent.parent
DOCS_DIR = REPO_ROOT / "build" / "docs"
DOCS_RU_DIR = DOCS_DIR / "ru"

# Source paths
BOOK_DIR = REPO_ROOT / "book"
LABS_DIR = REPO_ROOT / "labs"
TRANSLATIONS_RU_DIR = REPO_ROOT / "translations" / "ru"
README_FILE = REPO_ROOT / "README.md"

# GitHub repository info
GITHUB_REPO = "kshvakov/ai-agent-course"
GITHUB_BASE_URL = f"https://github.com/{GITHUB_REPO}/blob/main"
SITE_BASE_PATH = "/ai-agent-course"

MKDOCS_CONFIG_FILE = REPO_ROOT / "mkdocs.yml"


def read_site_url() -> str:
    """
    Read `site_url` from mkdocs.yml without adding extra dependencies.
    Falls back to empty string if not found.
    """
    if not MKDOCS_CONFIG_FILE.exists():
        return ""
    content = MKDOCS_CONFIG_FILE.read_text(encoding="utf-8")
    m = re.search(r"(?m)^\s*site_url:\s*(.+?)\s*$", content)
    if not m:
        return ""
    url = m.group(1).strip().strip('"').strip("'")
    return url


def urljoin_base(base: str, path: str) -> str:
    """Join base URL with a relative path."""
    if not base:
        return path
    return base.rstrip("/") + "/" + path.lstrip("/")


def build_sitemap_urls(docs_root: Path, site_url: str) -> list[str]:
    """
    Build a minimal sitemap list based on generated `index.md` pages.
    We intentionally include only `index.md` to keep URLs stable and clean.
    """
    urls: list[str] = []
    for index_md in docs_root.rglob("index.md"):
        rel_dir = index_md.parent.relative_to(docs_root).as_posix()
        if rel_dir == ".":
            rel_dir = ""
        # Directory URL (use_directory_urls: true)
        url_path = rel_dir + ("/" if rel_dir else "")
        urls.append(urljoin_base(site_url, url_path))
    # Deduplicate + stable order
    return sorted(set(urls))


def fix_links(content: str, is_ru: bool = False) -> str:
    """
    Fix internal markdown links:
    - Replace README.md with index.md or remove README.md from paths
    - Fix relative paths for docs structure
    - Handle translation links (translations/ru/... -> ru/...)
    - Convert labs links to GitHub links
    - Fix Translations section links to point to site language versions
    """
    # Fix Translations section - replace GitHub branch links with site links
    if is_ru:
        # Russian version: replace GitHub main branch link with site root (default language)
        content = re.sub(
            r'(\*\*English \(EN\)\*\* — )\[`main` branch\]\(https://github\.com/[^)]+\)',
            f'\\1[English version]({SITE_BASE_PATH}/)',
            content
        )
        content = re.sub(
            r'(\*\*English \(EN\)\*\* — )\[`main` branch\]\([^)]+\)',
            f'\\1[English version]({SITE_BASE_PATH}/)',
            content
        )
        # Russian version: replace any local "English version" link in the Translations section with site root
        # (On GitHub it's often a relative path to /book/README.md, but on the site it must be /ai-agent-course/)
        content = re.sub(
            r'(?m)^- (\*\*English \(EN\)\*\* — )\[English version\]\([^)]+\)\s*$',
            f'- \\1[English version]({SITE_BASE_PATH}/)',
            content
        )
        # Remove "ru (эта ветка)" reference
        content = re.sub(
            r'(\*\*Русский \(RU\)\*\* — )`ru` \(эта ветка\)',
            f'\\1[Русская версия]({SITE_BASE_PATH}/ru/)',
            content
        )
    else:
        # English version: replace Russian version link with site /ru/ link
        content = re.sub(
            r'(\*\*Русский \(RU\)\*\* — )\[Russian version\]\(\./translations/ru/README\.md\)',
            f'\\1[Russian version]({SITE_BASE_PATH}/ru/)',
            content
        )
        content = re.sub(
            r'(\*\*Русский \(RU\)\*\* — )\[Russian version\]\([^)]+\)',
            f'\\1[Russian version]({SITE_BASE_PATH}/ru/)',
            content
        )
        # Remove "main (this branch)" reference
        content = re.sub(
            r'(\*\*English \(EN\)\*\* — )`main` \(this branch\)',
            f'\\1[English version]({SITE_BASE_PATH}/)',
            content
        )
    
    # Pattern to match markdown links: [text](path/to/file.md)
    link_pattern = r'\[([^\]]+)\]\(([^)]+)\)'
    
    def replace_link(match):
        text = match.group(1)
        link_path = match.group(2)
        
        # Skip external links (http/https) - but we already processed Translations section above
        if link_path.startswith(('http://', 'https://', 'mailto:')):
            return match.group(0)
        
        # Skip anchor-only links (#section)
        if link_path.startswith('#'):
            return match.group(0)
        
        # Convert labs links to GitHub links
        # Match patterns like: ./labs/lab00-capability-check, labs/lab01-basics, ../labs/lab02-tools
        if 'labs/lab' in link_path or './labs/' in link_path or '../labs/' in link_path:
            # Extract lab name from path
            lab_match = re.search(r'labs/(lab\d+[^/\)]+)', link_path)
            if lab_match:
                lab_name = lab_match.group(1)
                # Convert to GitHub link
                # For RU version, use translations/ru/labs path
                if is_ru:
                    github_path = f"{GITHUB_BASE_URL}/translations/ru/labs/{lab_name}/README.md"
                else:
                    github_path = f"{GITHUB_BASE_URL}/labs/{lab_name}/README.md"
                return f'[{text}]({github_path})'
        
        # Remove book/ prefix from links (since book is now root)
        if '/book/' in link_path:
            new_path = link_path.replace('/book/', '/')
            new_path = new_path.replace('./book/', './')
            new_path = new_path.replace('../book/', '../')
        elif link_path.startswith('book/'):
            new_path = link_path.replace('book/', '')
        else:
            new_path = link_path
        
        # Handle translation links
        # For EN: translations/ru/book/README.md -> ../ru/
        # For RU: ../translations/ru/book/README.md -> ../ (already handled above)
        if 'translations/ru/' in new_path:
            if is_ru:
                # In RU version, remove translations/ru/ prefix
                new_path = new_path.replace('translations/ru/', '')
                new_path = new_path.replace('../translations/ru/', '../')
            else:
                # In EN version, convert to ../ru/...
                new_path = new_path.replace('translations/ru/', '../ru/')
                new_path = new_path.replace('../translations/ru/', '../ru/')
        
        # Replace README.md with directory URLs (for use_directory_urls: true)
        # Convert ../dir/README.md -> ../dir/ (preferred by MkDocs)
        if new_path.endswith('/README.md'):
            # Convert /README.md to /
            new_path = new_path[:-10] + '/'
        elif new_path.endswith('README.md'):
            # For relative paths like ../dir/README.md, convert to ../dir/
            if '/' in new_path and not new_path.startswith('http'):
                # Check if it's a directory reference (has ../ or ./)
                if '../' in new_path or './' in new_path:
                    new_path = new_path.replace('README.md', '')
                    # Ensure it ends with /
                    if not new_path.endswith('/'):
                        new_path += '/'
                else:
                    # Same directory reference
                    new_path = new_path.replace('README.md', '')
                    if not new_path.endswith('/'):
                        new_path += '/'
            else:
                # Root level README.md -> index.md
                new_path = 'index.md'
        elif '/README.md' in new_path:
            # Replace /README.md in the middle of path with /
            new_path = new_path.replace('/README.md', '/')
        
        # Clean up relative paths
        # Remove leading ./ if present
        if new_path.startswith('./'):
            new_path = new_path[2:]
        
        # Fix directory links that don't end with / or .md
        # Links like "labs/lab00-capability-check" should be "labs/lab00-capability-check/"
        # But skip if it's already a file reference (.md, .html, etc.) or has anchor (#)
        if (not new_path.startswith(('http://', 'https://', 'mailto:', '#')) and
            not new_path.endswith(('.md', '.html', '.pdf', '.png', '.jpg', '.jpeg', '.gif', '.svg', '/')) and
            '/' in new_path):
            # It's likely a directory reference, add trailing /
            new_path += '/'
        
        return f'[{text}]({new_path})'
    
    return re.sub(link_pattern, replace_link, content)


def normalize_lists(content: str) -> str:
    """
    Normalize markdown lists by ensuring there's a blank line before list items.
    This fixes the issue where Python-Markdown doesn't recognize lists without
    a preceding blank line, causing them to render as a single paragraph.
    
    Works only outside of code fences (```/~~~) to avoid breaking code examples.
    Also handles blockquote lists (lines starting with >).
    """
    lines = content.split('\n')
    result = []
    in_code_fence = False
    code_fence_char = None
    
    i = 0
    while i < len(lines):
        line = lines[i]
        
        # Track code fence state
        stripped = line.strip()
        if stripped.startswith('```') or stripped.startswith('~~~'):
            # Check if this is a closing fence
            if in_code_fence and stripped.startswith(code_fence_char * 3):
                in_code_fence = False
                code_fence_char = None
            elif not in_code_fence:
                # Opening fence - extract the language/fence type
                code_fence_char = stripped[0]
                in_code_fence = True
            result.append(line)
            i += 1
            continue
        
        # If we're inside a code fence, don't process
        if in_code_fence:
            result.append(line)
            i += 1
            continue
        
        # Check if this line starts a list
        # Pattern: optional blockquote (>), optional spaces, then list marker
        # List markers: 1. 2. etc (ordered) or - * + (unordered)
        list_pattern = re.compile(r'^(\s*>?\s*)(\d+\.\s+|\-\s+|\*\s+|\+\s+).*')
        match = list_pattern.match(line)
        
        if match:
            # This is a list item
            prefix = match.group(1)  # Includes blockquote and indentation
            list_marker = match.group(2)
            
            # Check if previous line is blank or also a list
            if result:
                prev_line = result[-1]
                prev_stripped = prev_line.strip()
                
                # If previous line is not blank and not a list, insert blank line
                if prev_stripped and not re.match(r'^(\s*>?\s*)(\d+\.\s+|\-\s+|\*\s+|\+\s+).*', prev_line):
                    # Insert blank line with same blockquote prefix if present
                    if prefix.startswith('>'):
                        # Extract blockquote prefix (may have spaces)
                        blockquote_match = re.match(r'^(\s*>)', prefix)
                        if blockquote_match:
                            result.append(blockquote_match.group(1))
                    else:
                        result.append('')
        
        result.append(line)
        i += 1
    
    return '\n'.join(result)


def copy_readme_to_index(src_dir: Path, dst_dir: Path, is_ru: bool = False):
    """Copy README.md to index.md and fix links."""
    readme_file = src_dir / "README.md"
    if not readme_file.exists():
        return
    
    dst_dir.mkdir(parents=True, exist_ok=True)
    index_file = dst_dir / "index.md"
    
    content = readme_file.read_text(encoding='utf-8')
    content = fix_links(content, is_ru)
    content = normalize_lists(content)
    
    index_file.write_text(content, encoding='utf-8')
    print(f"  ✓ {src_dir.name}/README.md -> {dst_dir.name}/index.md")


def copy_other_files(src_dir: Path, dst_dir: Path):
    """Copy MANUAL.md and SOLUTION.md files."""
    for filename in ['MANUAL.md', 'SOLUTION.md']:
        src_file = src_dir / filename
        if src_file.exists():
            dst_dir.mkdir(parents=True, exist_ok=True)
            dst_file = dst_dir / filename
            
            content = src_file.read_text(encoding='utf-8')
            content = fix_links(content)
            content = normalize_lists(content)
            
            dst_file.write_text(content, encoding='utf-8')
            print(f"  ✓ {src_dir.name}/{filename} -> {dst_dir.name}/{filename}")


def process_directory(src_base: Path, dst_base: Path, is_ru: bool = False):
    """Process a directory tree recursively."""
    if not src_base.exists():
        return
    
    # Process current directory
    copy_readme_to_index(src_base, dst_base, is_ru)
    
    # Process subdirectories
    for item in src_base.iterdir():
        if item.is_dir() and not item.name.startswith('.'):
            dst_subdir = dst_base / item.name
            process_directory(item, dst_subdir, is_ru)
            
            # Copy other files from subdirectories
            copy_other_files(item, dst_subdir)


def main():
    """Main function to prepare docs structure."""
    print("Preparing documentation structure for MkDocs...")
    
    # Clean existing docs directory
    if DOCS_DIR.exists():
        shutil.rmtree(DOCS_DIR)
    DOCS_DIR.mkdir(parents=True)

    # Create SEO helper files at site root (copied by MkDocs as static files)
    site_url = read_site_url()
    sitemap_url = urljoin_base(site_url, "sitemap.xml") if site_url else "sitemap.xml"
    
    # Copy book/README.md as index.md (EN)
    print("\n[EN] Processing book/ directory...")
    book_readme = BOOK_DIR / "README.md"
    if book_readme.exists():
        content = book_readme.read_text(encoding='utf-8')
        content = fix_links(content, is_ru=False)
        content = normalize_lists(content)
        (DOCS_DIR / "index.md").write_text(content, encoding='utf-8')
        print("  ✓ book/README.md -> index.md")
    
    # Process book chapters directly in docs/ (EN)
    print("\n[EN] Processing book chapters...")
    for item in BOOK_DIR.iterdir():
        if item.is_dir() and not item.name.startswith('.'):
            process_directory(item, DOCS_DIR / item.name, is_ru=False)
    
    # Labs are not copied - links will point to GitHub
    
    # Process Russian translation
    print("\n[RU] Processing Russian translation...")
    
    # Copy translations/ru/book/README.md as ru/index.md
    print("\n[RU] Processing translations/ru/book/ directory...")
    DOCS_RU_DIR.mkdir(parents=True, exist_ok=True)
    ru_book_dir = TRANSLATIONS_RU_DIR / "book"
    ru_book_readme = ru_book_dir / "README.md"
    if ru_book_readme.exists():
        content = ru_book_readme.read_text(encoding='utf-8')
        content = fix_links(content, is_ru=True)
        content = normalize_lists(content)
        (DOCS_RU_DIR / "index.md").write_text(content, encoding='utf-8')
        print("  ✓ translations/ru/book/README.md -> ru/index.md")
    
    # Process book chapters directly in docs/ru/ (RU)
    print("\n[RU] Processing book chapters...")
    for item in ru_book_dir.iterdir():
        if item.is_dir() and not item.name.startswith('.'):
            process_directory(item, DOCS_RU_DIR / item.name, is_ru=True)
    
    # Labs are not copied - links will point to GitHub
    
    # Write robots.txt and sitemap.xml after all pages are generated
    robots_txt = f"""User-agent: *
Allow: /

Sitemap: {sitemap_url}
"""
    (DOCS_DIR / "robots.txt").write_text(robots_txt, encoding="utf-8")

    sitemap_urls = build_sitemap_urls(DOCS_DIR, site_url)
    sitemap_xml_lines = [
        '<?xml version="1.0" encoding="UTF-8"?>',
        '<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
    ]
    for u in sitemap_urls:
        sitemap_xml_lines.append("  <url>")
        sitemap_xml_lines.append(f"    <loc>{u}</loc>")
        sitemap_xml_lines.append("  </url>")
    sitemap_xml_lines.append("</urlset>")
    sitemap_xml = "\n".join(sitemap_xml_lines) + "\n"
    (DOCS_DIR / "sitemap.xml").write_text(sitemap_xml, encoding="utf-8")

    # Multilingual helper: provide a minimal sitemap at /ru/sitemap.xml.
    # This avoids 404 when clients/tools try to access a language-local sitemap,
    # while keeping a single authoritative sitemap at /sitemap.xml.
    if site_url:
        root_sitemap_loc = urljoin_base(site_url, "sitemap.xml")
        ru_sitemap_index_xml = "\n".join([
            '<?xml version="1.0" encoding="UTF-8"?>',
            '<sitemapindex xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">',
            "  <sitemap>",
            f"    <loc>{root_sitemap_loc}</loc>",
            "  </sitemap>",
            "</sitemapindex>",
            "",
        ])
        DOCS_RU_DIR.mkdir(parents=True, exist_ok=True)
        (DOCS_RU_DIR / "sitemap.xml").write_text(ru_sitemap_index_xml, encoding="utf-8")

    # Service Worker to prevent 404 for nested */sitemap.xml requests in the browser.
    # We keep a single authoritative sitemap at the site root, and serve it for any
    # request whose path ends with /sitemap.xml within the SW scope.
    service_worker_js = """\
/* eslint-disable no-restricted-globals */
// This Service Worker exists solely to map nested */sitemap.xml requests to the root sitemap.
// It is useful for MkDocs Material + i18n setups where the theme requests sitemap.xml relative
// to the current page URL (leading to 404 on static hosting).

self.addEventListener("install", () => {
  // Activate the updated worker ASAP
  self.skipWaiting();
});

self.addEventListener("activate", (event) => {
  // Take control of existing pages ASAP
  event.waitUntil(self.clients.claim());
});

self.addEventListener("fetch", (event) => {
  const req = event.request;
  if (!req || req.method !== "GET") return;

  let url;
  try {
    url = new URL(req.url);
  } catch (_) {
    return;
  }

  // Only same-origin requests
  if (url.origin !== self.location.origin) return;

  // Only */sitemap.xml
  if (!url.pathname.endsWith("/sitemap.xml")) return;

  // Root sitemap inside the current SW scope (works for / and GitHub Pages project sites)
  const scopePath = new URL(self.registration.scope).pathname; // always ends with '/'
  const rootSitemapPath = scopePath + "sitemap.xml";

  // Let the root sitemap go to network normally (avoid loops)
  if (url.pathname === rootSitemapPath) return;

  event.respondWith(fetch(rootSitemapPath, { cache: "reload" }));
});
"""
    (DOCS_DIR / "service-worker.js").write_text(service_worker_js, encoding="utf-8")

    print("\n✓ Documentation structure prepared successfully!")
    print(f"  Output directory: {DOCS_DIR}")


if __name__ == "__main__":
    main()


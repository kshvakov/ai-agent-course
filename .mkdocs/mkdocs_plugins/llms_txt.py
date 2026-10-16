"""
MkDocs plugin generating llms.txt and llms-full.txt at the site root.

- llms.txt:      short structured index for LLM crawlers (https://llmstxt.org)
- llms-full.txt: the same index followed by the full markdown of every page

Also patches the site-wide 404 page: GitHub Pages serves a single 404.html,
which mkdocs-static-i18n renders from the last-built locale (ru). The patch
forces lang="en" and adds noindex (404 pages must never be indexed).
"""
import os
import re

from mkdocs.plugins import BasePlugin


class LlmsTxtPlugin(BasePlugin):
    def __init__(self):
        self._pages = {}        # page.url -> Page
        self._markdown = {}     # page.url -> raw markdown
        self._nav_snapshot = []  # [("section"|"page", title, url)] from on_nav

    def on_nav(self, nav, **kwargs):
        # mkdocs-static-i18n re-runs the on_nav chain once per locale (en, ru)
        # and mutates the nav each time. Keep only the FIRST snapshot: the
        # default locale (en) comes first in the languages list.
        if not self._nav_snapshot:
            self._snapshot_nav(nav.items, self._nav_snapshot)
        return nav

    def _snapshot_nav(self, items, out):
        for item in items:
            if item.is_section:
                out.append(("section", item.title, None))
                self._snapshot_nav(item.children, out)
            elif item.is_page:
                out.append(("page", item.title, item.url))

    def on_page_markdown(self, markdown, page, **kwargs):
        self._pages[page.url] = page
        self._markdown[page.url] = markdown
        return markdown

    # ------------------------------------------------------------------ #

    def on_post_build(self, **kwargs):
        config = kwargs["config"]
        site_url = (config.site_url or "").rstrip("/")
        site_dir = config.site_dir

        en_lines, ru_entries = self._collect(site_url)
        llms = self._render_index(config, site_url, en_lines, ru_entries)
        llms_full = self._render_full(config, site_url)

        self._write(os.path.join(site_dir, "llms.txt"), llms)
        self._write(os.path.join(site_dir, "llms-full.txt"), llms_full)
        self._patch_404(site_dir)

    def _collect(self, site_url):
        """Return (en_lines, ru_entries) following the nav snapshot order."""
        en_lines = []
        ru_entries = []
        seen = set()

        def add(page):
            seen.add(page.url)
            desc = (page.meta or {}).get("description", "")
            entry = f"- [{page.title}]({self._abs(site_url, page.url)})"
            if desc:
                entry += f": {desc}"
            if page.url.startswith("ru/"):
                ru_entries.append(entry)
            else:
                en_lines.append(entry)

        for kind, title, url in self._nav_snapshot:
            if kind == "section":
                en_lines.append(f"## {title}")
                en_lines.append("")
            else:
                page = self._pages.get(url)
                if page is not None:
                    add(page)

        # Locale variants and any pages missing from the nav, in stable order.
        for url in sorted(self._pages):
            if url not in seen:
                add(self._pages[url])

        return en_lines, ru_entries

    def _render_index(self, config, site_url, en_lines, ru_entries):
        lines = [
            f"# {config.site_name}",
            "",
            f"> {config.site_description}",
            "",
            "A practical, open course on building autonomous AI agents in Go: "
            "26 chapters from LLM fundamentals to production readiness, with "
            "laboratory assignments in the source repository.",
            "",
            f"Site: {site_url}/",
        ]
        if config.repo_url:
            lines.append(f"Source: {config.repo_url}")
        lines += [
            "Languages: en (primary), ru",
            "",
            "## Course contents",
            "",
        ]
        lines += en_lines
        if ru_entries:
            lines += [
                "",
                "## Русская версия",
                "",
            ]
            lines += ru_entries
        lines.append("")
        return "\n".join(lines)

    def _render_full(self, config, site_url):
        header = [
            f"# {config.site_name}",
            "",
            f"> {config.site_description}",
            "",
            f"Site: {site_url}/",
            "",
        ]
        body = []

        for url in self._ordered_urls():
            page = self._pages[url]
            md = (self._markdown.get(url) or "").strip()
            if not md:
                continue
            body += [
                "---",
                "",
                f"## {page.title}",
                "",
                f"URL: {self._abs(site_url, url)}",
                "",
                md,
                "",
            ]
        return "\n".join(header + body)

    def _ordered_urls(self):
        """EN pages first in nav order, then RU pages sorted by URL."""
        urls = []
        for kind, _title, url in self._nav_snapshot:
            if kind == "page" and url in self._pages and not url.startswith("ru/"):
                urls.append(url)
        for url in sorted(self._pages):
            if url not in urls:
                urls.append(url)
        return urls

    def _abs(self, site_url, url):
        return f"{site_url}/{url}" if site_url else f"/{url}"

    def _write(self, path, text):
        with open(path, "w", encoding="utf-8") as f:
            f.write(text)

    def _patch_404(self, site_dir):
        path = os.path.join(site_dir, "404.html")
        if not os.path.exists(path):
            return
        with open(path, encoding="utf-8") as f:
            html = f.read()
        orig = html
        # i18n builds the single site-wide 404 from the last-built locale (ru).
        html = html.replace('<html lang="ru"', '<html lang="en"', 1)
        # Flip the existing robots meta (emitted by the SEO override in
        # extrahead) instead of injecting a second, conflicting one.
        if 'name="robots"' in html:
            html = re.sub(
                r'(<meta name="robots" content=")[^"]*(")',
                r'\1noindex\2',
                html,
                count=1,
            )
        else:
            html = html.replace(
                "</title>",
                "</title>\n    <meta name=\"robots\" content=\"noindex\">",
                1,
            )
        if html != orig:
            with open(path, "w", encoding="utf-8") as f:
                f.write(html)

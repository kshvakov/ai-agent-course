"""
MkDocs plugin for automatic meta description generation.

Generates unique meta descriptions for each page based on content,
extracting the first meaningful paragraph after the title.
"""
import re
from typing import Any, Dict
from mkdocs.plugins import BasePlugin
from mkdocs.structure.pages import Page


class AutoDescriptionPlugin(BasePlugin):
    """Plugin to auto-generate meta descriptions from page content."""

    def __init__(self):
        self.max_length = 160
        self.min_length = 100

    def on_config(self, config, **kwargs):
        """Load plugin configuration."""
        if 'max_length' in self.config:
            self.max_length = self.config['max_length']
        if 'min_length' in self.config:
            self.min_length = self.config['min_length']
        return config

    def on_page_markdown(self, markdown: str, page: Page, **kwargs) -> str:
        """
        Called when markdown is processed. Adds description to page.meta if not present.
        Also updates page.title from the first H1 heading if it differs.
        """
        # Extract title from first H1 heading
        title_from_markdown = self._extract_title(markdown)
        if title_from_markdown and title_from_markdown != page.title:
            # Update page title to match the actual heading in markdown
            # This ensures translated titles are used correctly
            page.title = title_from_markdown

        # Skip if description already exists
        if page.meta and page.meta.get('description'):
            return markdown

        # Extract description from markdown content
        description = self._extract_description(markdown, page.title)

        if description:
            # Initialize meta if needed
            if not page.meta:
                page.meta = {}
            page.meta['description'] = description

        return markdown

    def _extract_title(self, markdown: str) -> str:
        """
        Extract the first H1 heading from markdown.
        Returns empty string if no H1 found.
        """
        if not markdown:
            return ""
        
        lines = markdown.split('\n')
        for line in lines:
            stripped = line.strip()
            # Find first H1 heading
            if stripped.startswith('# '):
                # Remove # and leading/trailing whitespace
                title = stripped[2:].strip()
                # Remove markdown formatting from title
                title = re.sub(r'\*\*([^\*]+)\*\*', r'\1', title)  # Bold
                title = re.sub(r'\*([^\*]+)\*', r'\1', title)      # Italic
                title = re.sub(r'`([^`]+)`', r'\1', title)         # Inline code
                return title
        
        return ""

    def _extract_description(self, markdown: str, title: str) -> str:
        """
        Extract a meaningful description from markdown content.
        
        Strategy:
        1. Skip the first H1 title
        2. Find first paragraph after title (skip empty lines, code blocks, etc.)
        3. Extract text, remove markdown formatting
        4. Trim to target length
        """
        if not markdown:
            return ""

        # Remove code blocks first (they shouldn't be in description)
        markdown = re.sub(r'```[\s\S]*?```', '', markdown)
        markdown = re.sub(r'~~~[\s\S]*?~~~', '', markdown)
        
        # Remove inline code
        markdown = re.sub(r'`[^`]+`', '', markdown)
        
        # Remove links but keep text: [text](url) -> text
        markdown = re.sub(r'\[([^\]]+)\]\([^\)]+\)', r'\1', markdown)
        
        # Remove images
        markdown = re.sub(r'!\[[^\]]*\]\([^\)]+\)', '', markdown)
        
        # Remove HTML tags
        markdown = re.sub(r'<[^>]+>', '', markdown)
        
        # Split into lines
        lines = markdown.split('\n')
        
        # Skip H1 title (first line starting with #)
        content_start = 0
        for i, line in enumerate(lines):
            stripped = line.strip()
            # Skip empty lines and H1
            if stripped.startswith('# '):
                content_start = i + 1
                continue
            # Skip other headers at start
            if stripped.startswith('##'):
                continue
            # Skip empty lines
            if not stripped:
                continue
            # Found first content line
            if stripped:
                content_start = i
                break
        
        # Extract first meaningful paragraph
        paragraph_lines = []
        for i in range(content_start, len(lines)):
            line = lines[i].strip()
            
            # Stop at next major section (H2)
            if line.startswith('## '):
                break
            
            # Skip empty lines, lists, code blocks, horizontal rules
            if (not line or 
                line.startswith('- ') or 
                line.startswith('* ') or 
                line.startswith('1. ') or
                line.startswith('---') or
                line.startswith('***') or
                line.startswith('```')):
                continue
            
            # Collect paragraph text (prefer longer sentences)
            if line and len(line) > 20:  # Skip very short lines
                paragraph_lines.append(line)
                # Stop after collecting enough text
                combined = ' '.join(paragraph_lines)
                if len(combined) > self.min_length:
                    break
        
        if not paragraph_lines:
            return ""
        
        # Join and clean up
        text = ' '.join(paragraph_lines)
        
        # Remove extra whitespace
        text = re.sub(r'\s+', ' ', text).strip()
        
        # Remove markdown formatting
        text = re.sub(r'\*\*([^\*]+)\*\*', r'\1', text)  # Bold
        text = re.sub(r'\*([^\*]+)\*', r'\1', text)      # Italic
        text = re.sub(r'_{2}([^_]+)_{2}', r'\1', text)   # Bold alt
        text = re.sub(r'_([^_]+)_', r'\1', text)          # Italic alt
        
        # Replace straight quotes with typographic quotes or remove them for HTML safety
        # This prevents breaking HTML attributes
        text = text.replace('"', "'")  # Replace double quotes with single quotes
        text = text.replace('"', "'")  # Replace typographic double quotes
        text = text.replace('"', "'")  # Replace typographic double quotes (right)
        text = text.replace('«', "'")  # Replace guillemets
        text = text.replace('»', "'")  # Replace guillemets
        
        # Trim to max length, but try to end at word boundary
        if len(text) > self.max_length:
            text = text[:self.max_length].rsplit(' ', 1)[0]
            # Add ellipsis if truncated
            if len(text) < len(' '.join(paragraph_lines)):
                text += '...'
        
        return text

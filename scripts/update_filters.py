#!/usr/bin/env python3
"""
Updates website/index.html filter dropdowns with values from config/feeds.json
"""

import json
import re
from pathlib import Path

ROOT = Path(__file__).parent.parent
FEEDS_JSON = ROOT / "config" / "feeds.json"
INDEX_HTML = ROOT / "website" / "index.html"


def main():
    # Load feeds
    with open(FEEDS_JSON, "r") as f:
        data = json.load(f)

    # Collect unique values
    countries = set()
    languages = set()
    categories = set()
    tags = set()
    sources = set()

    for feed in data["feeds"]:
        if feed.get("country"):
            countries.add(feed["country"])
        if feed.get("language"):
            languages.add(feed["language"])
        if feed.get("category"):
            categories.add(feed["category"])
        for tag in feed.get("tags", []):
            tags.add(tag)
        if feed.get("name"):
            sources.add(feed["name"])

    # Sort
    countries = sorted(countries)
    languages = sorted(languages)
    categories = sorted(categories)
    tags = sorted(tags)
    sources = sorted(sources)

    # Generate option HTML
    def make_options(values):
        opts = ['<option value="">ALL</option>']
        for v in values:
            opts.append(f'<option value="{v}">{v.upper()}</option>')
        return "\n          ".join(opts)

    country_options = make_options(countries)
    language_options = make_options(languages)
    category_options = make_options(categories)
    tag_options = make_options(tags)
    source_options = make_options(sources)

    # Read index.html
    with open(INDEX_HTML, "r") as f:
        html = f.read()

    # Replace filter options using regex
    def replace_select(html, select_id, options):
        pattern = rf'(<select id="{select_id}">)\s*.*?\s*(</select>)'
        replacement = rf'\1\n          {options}\n        \2'
        return re.sub(pattern, replacement, html, flags=re.DOTALL)

    html = replace_select(html, "filter-country", country_options)
    html = replace_select(html, "filter-language", language_options)
    html = replace_select(html, "filter-category", category_options)
    html = replace_select(html, "filter-tag", tag_options)
    html = replace_select(html, "filter-source", source_options)

    # Write back
    with open(INDEX_HTML, "w") as f:
        f.write(html)

    print(f"Updated filters: {len(countries)} countries, {len(languages)} languages, {len(categories)} categories, {len(tags)} tags, {len(sources)} sources")


if __name__ == "__main__":
    main()

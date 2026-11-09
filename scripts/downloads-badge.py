#!/usr/bin/env python3
"""Sum the public GitHub download counters for every configured GHCR image."""

import argparse
import json
import os
import re
import sys
import time
from decimal import Decimal
from html.parser import HTMLParser
from pathlib import Path
from urllib.error import URLError
from urllib.parse import quote
from urllib.request import Request, urlopen


ROOT = Path(__file__).resolve().parents[1]
MULTIPLIERS = {"": 1, "K": 1_000, "M": 1_000_000, "B": 1_000_000_000}


class PackageText(HTMLParser):
    def __init__(self):
        super().__init__()
        self.parts = []
        self.hidden = 0

    def handle_starttag(self, tag, attrs):
        if tag in {"script", "style"}:
            self.hidden += 1
        if not self.hidden:
            # GitHub can expose the full count in a title beside a rounded value.
            for name, value in attrs:
                if name == "title" and value and re.fullmatch(r"[\d,]+", value):
                    self.parts.append(value)

    def handle_endtag(self, tag):
        if tag in {"script", "style"}:
            self.hidden = max(0, self.hidden - 1)

    def handle_data(self, data):
        if not self.hidden:
            self.parts.append(data)


def parse_downloads(page):
    parser = PackageText()
    parser.feed(page)
    text = " ".join(" ".join(parser.parts).split())
    match = re.search(
        r"\bTotal downloads\s*:?\s*((?:[0-9]{1,3}(?:,[0-9]{3})+|[0-9]+)(?:\.[0-9]+)?)"
        r"\s*([KMB])?(?![\w.,])",
        text,
        re.IGNORECASE,
    )
    if not match:
        raise ValueError("No total downloads found; GitHub's package-page layout may have changed")
    value = Decimal(match[1].replace(",", "")) * MULTIPLIERS[(match[2] or "").upper()]
    if value != value.to_integral_value():
        raise ValueError(f"Invalid download count: {match[0]}")
    return int(value)


def configured_images(root=ROOT):
    images = set()
    for config in sorted(root.glob("beszel_*/config.yaml")):
        match = re.search(r'^image:[ \t]*["\']?(ghcr\.io/[^"\'\s]+)["\']?[ \t]*$', config.read_text(), re.M)
        if not match:
            raise ValueError(f"{config}: missing GHCR image")
        images.add(match[1])
    if not images:
        raise ValueError("No app images found")
    return sorted(images)


def fetch_downloads(repository, image):
    _, owner, package = image.split("/", 2)
    if owner.lower() != repository.split("/", 1)[0].lower():
        raise ValueError(f"{image}: package owner differs from {repository}")
    url = f"https://github.com/{repository}/pkgs/container/{quote(package, safe='')}"
    request = Request(url, headers={"User-Agent": "Beszel-downloads-badge", "Accept-Language": "en"})
    for attempt in range(3):
        try:
            with urlopen(request, timeout=30) as response:
                return parse_downloads(response.read().decode("utf-8"))
        except URLError:
            if attempt == 2:
                raise
            time.sleep(attempt + 1)


def format_count(count):
    for suffix, multiplier in reversed(list(MULTIPLIERS.items())):
        if suffix and count >= multiplier:
            return f"{Decimal(count) / multiplier:.1f}".rstrip("0").rstrip(".") + suffix
    return str(count)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repository", default=os.environ.get("GITHUB_REPOSITORY", "vineetchoudhary/home-assistant-beszel-agent"))
    args = parser.parse_args()
    total = 0
    images = configured_images()
    for image in images:
        count = fetch_downloads(args.repository, image)
        print(f"{image}: {count:,}", file=sys.stderr)
        total += count
    print(f"Total across {len(images)} images: {total:,}", file=sys.stderr)
    print(json.dumps({"schemaVersion": 1, "label": "downloads", "message": format_count(total), "color": "2f6fed"}))


if __name__ == "__main__":
    main()

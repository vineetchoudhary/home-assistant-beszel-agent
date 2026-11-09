#!/usr/bin/env python3
"""Build HA Apps Preview from repository metadata, without install state."""

import argparse
import base64
import html
from pathlib import Path

import yaml


ROOT = Path(__file__).resolve().parents[1]
ORDER = (
    "beszel_agent",
    "beszel_agent_amd",
    "beszel_agent_intel",
    "beszel_agent_nvidia",
    "beszel_agent_smart",
    "beszel_bridge",
    "beszel_hub",
)


def render():
    configured = {path.parent.name for path in ROOT.glob("beszel_*/config.yaml")}
    if configured != set(ORDER):
        raise ValueError("Update ORDER to include every configured app")
    cards = []
    for directory in ORDER:
        config = yaml.safe_load((ROOT / directory / "config.yaml").read_text())
        icon = base64.b64encode((ROOT / directory / "icon.png").read_bytes()).decode("ascii")
        cards.append(
            f'<article class="card" data-app="{directory}">'
            f'<img alt="" src="data:image/png;base64,{icon}">'
            f'<div><h2>{html.escape(config["name"])}</h2>'
            f'<p>{html.escape(config["description"])}</p></div></article>'
        )
    return """<!doctype html>
<html lang="en">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>HA Apps Preview</title>
<style>
  * { box-sizing: border-box; }
  html { background: #111; color-scheme: dark; }
  body { margin: 0; font-family: Roboto, Arial, sans-serif; }
  main {
    width: 1800px;
    padding: 8px;
    display: flex;
    flex-wrap: wrap;
    justify-content: flex-start;
    gap: 16px;
  }
  .card {
    width: calc((100% - 48px) / 4);
    min-height: 172px;
    padding: 24px;
    display: flex;
    align-items: flex-start;
    gap: 20px;
    border: 1px solid #383838;
    border-radius: 16px;
    background: #1c1c1c;
    box-shadow: 0 2px 4px #0003;
  }
  .card img { width: 48px; height: 48px; flex-shrink: 0; }
  .card div { min-width: 0; }
  h2 { margin: 0 0 8px; color: #dedede; font-size: 20px; line-height: 26px; font-weight: 400; }
  p { margin: 0; color: #aaa; font-size: 18px; line-height: 26px; }
</style>
<main aria-label="Beszel Agent variants, Bridge and Hub">
""" + "\n".join(cards) + "\n</main>\n</html>\n"


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--output", type=Path, default=ROOT / ".artifacts/bridge/screenshots/ha-apps-preview/index.html")
    args = parser.parse_args()
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(render())
    print(args.output)


if __name__ == "__main__":
    main()

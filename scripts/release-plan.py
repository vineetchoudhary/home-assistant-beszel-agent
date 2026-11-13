#!/usr/bin/env python3
"""Resolve publication scope and versions without building or publishing anything."""

import argparse
from pathlib import Path
import re


ROOT = Path(__file__).resolve().parent.parent
FULL_SCOPES = {"all", "all-except-bridge"}
IMAGE_TYPES = FULL_SCOPES | {"stable", "smart", "intel", "amd", "nvidia", "hub", "bridge"}


def version_key(version):
    if not re.fullmatch(r"[0-9]+(?:\.[0-9]+){2,3}", version):
        raise ValueError("Expected a numeric app version such as 0.21.2 or 0.21.2.1")
    parts = tuple(map(int, version.split(".")))
    return parts + (0,) * (4 - len(parts))


def plan_release(*, event, ref, current_app, current_bridge, current_beszel,
                 latest_beszel="", image_type="all", bridge_version="", force=False):
    # Dispatch inputs have no effect on scheduled checks or pushed tags.
    if event == "workflow_dispatch":
        trigger = "manual"
        if not ref.startswith("refs/heads/"):
            raise ValueError("Run manual releases from a branch so version updates can be committed")
        if image_type not in IMAGE_TYPES:
            raise ValueError("Unknown image type: " + image_type)
        if bridge_version and image_type != "bridge":
            raise ValueError("bridge_version is only available with image_type=bridge")
    elif event == "schedule":
        trigger, image_type, bridge_version, force = "scheduled", "all", "", False
    elif event == "push" and ref.startswith("refs/tags/v"):
        trigger, image_type, bridge_version, force = "tag", "all", "", False
    else:
        raise ValueError("Expected a scheduled check, manual run, or v-prefixed release tag")

    version_key(current_app)
    version_key(current_bridge)
    app_version, beszel_version = current_app, current_beszel
    if trigger == "tag":
        app_version = ref.removeprefix("refs/tags/v")
    elif trigger == "manual" and image_type not in FULL_SCOPES:
        if image_type == "bridge":
            app_version = bridge_version or current_bridge
            if app_version != current_bridge and version_key(app_version) <= version_key(current_bridge):
                raise ValueError("A new Bridge release must have a newer version than the current Bridge")
    else:
        version_key(latest_beszel)
        if latest_beszel != current_beszel:
            app_version = beszel_version = latest_beszel
    version_key(app_version)

    # A full rebuild must preserve a newer independent Bridge release.
    bridge_app_version = current_bridge
    if image_type == "all":
        bridge_app_version = max((current_bridge, app_version), key=version_key)
    elif image_type == "bridge":
        bridge_app_version = app_version

    run_steps = (trigger == "tag" or force
                 or (trigger == "manual" and image_type not in FULL_SCOPES)
                 or app_version != current_app)
    release_tag = ""
    if run_steps and trigger != "tag":
        if image_type in FULL_SCOPES and not force:
            release_tag = "v" + app_version
        elif image_type == "bridge" and app_version != current_bridge:
            release_tag = "bridge-v" + app_version

    return {
        "type": trigger,
        "image_type": image_type,
        "APP_VERSION": app_version,
        "BRIDGE_APP_VERSION": bridge_app_version,
        "BESZEL_VERSION": beszel_version,
        "run_steps": str(run_steps).lower(),
        "verify_bridge": str(run_steps and image_type in {"all", "bridge"}).lower(),
        "RELEASE_TAG": release_tag,
        "RELEASE_NAME": ("Beszel Bridge v" if image_type == "bridge" else "Release v") + app_version,
    }


def config_version(directory):
    match = re.search(r'^version: "([^"]+)"$', (ROOT / directory / "config.yaml").read_text(), re.M)
    if not match:
        raise ValueError(directory + "/config.yaml has no quoted version")
    return match.group(1)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--event", required=True)
    parser.add_argument("--ref", required=True)
    parser.add_argument("--image-type", default="all")
    parser.add_argument("--bridge-version", default="")
    parser.add_argument("--latest-beszel", default="")
    parser.add_argument("--force-rebuild", choices=("true", "false"), default="false")
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        plan = plan_release(
            event=args.event, ref=args.ref, image_type=args.image_type or "all",
            bridge_version=args.bridge_version.strip(), latest_beszel=args.latest_beszel,
            force=args.force_rebuild == "true", current_app=config_version("beszel_agent"),
            current_bridge=config_version("beszel_bridge"),
            current_beszel=(ROOT / "beszel_agent/beszel_version").read_text().strip(),
        )
    except ValueError as error:
        parser.error(str(error))
    with args.output.open("a") as output:
        for key, value in plan.items():
            output.write(f"{key}={value}\n")
            print(f"{key}={value}")


if __name__ == "__main__":
    main()

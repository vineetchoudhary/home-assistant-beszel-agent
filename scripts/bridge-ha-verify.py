#!/usr/bin/env python3
"""Inspect an isolated HA test instance through its real REST/WebSocket APIs.

Run inside the test HA container; credentials stay in its private /config file.
This is test tooling, not a component installed in a user's Home Assistant.
"""
import argparse
import asyncio
import json
from pathlib import Path

import aiohttp
import yaml


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dashboard", action="store_true")
    parser.add_argument("--customize", action="store_true", help="Rename the test CPU entity through HA's registry")
    args = parser.parse_args()
    settings = json.loads(Path("/config/bridge-test-auth.json").read_text())
    namespace = settings["namespace"]
    async with aiohttp.ClientSession() as session:
        async with session.ws_connect("http://127.0.0.1:8123/api/websocket") as ws:
            await ws.receive_json()
            await ws.send_json({"type": "auth", "access_token": settings["access_token"]})
            auth = await ws.receive_json()
            assert auth["type"] == "auth_ok", "HA test authentication failed"
            sequence = 0

            async def command(kind, **data):
                nonlocal sequence
                sequence += 1
                await ws.send_json({"id": sequence, "type": kind, **data})
                result = await ws.receive_json()
                assert result.get("success"), f"HA {kind} failed: {result.get('error')}"
                return result.get("result")

            registry = await command("config/entity_registry/list")
            if args.customize:
                cpu_id = namespace + "_" + settings["system_id"] + "_cpu_usage"
                cpu = next(entity for entity in registry if entity["platform"] == "mqtt" and entity["unique_id"] == cpu_id)
                await command("config/entity_registry/update", entity_id=cpu["entity_id"],
                              new_entity_id="sensor.bridge_lab_custom_cpu", name="My lab CPU")
                registry = await command("config/entity_registry/list")
            entities = [entity for entity in registry if entity["platform"] == "mqtt" and entity["unique_id"].startswith(namespace + "_")]
            device_ids = {entity["device_id"] for entity in entities}
            devices = [device for device in await command("config/device_registry/list") if device["id"] in device_ids]
            async with session.get("http://127.0.0.1:8123/api/states", headers={"Authorization": "Bearer " + settings["access_token"]}) as response:
                response.raise_for_status()
                all_states = await response.json()
            entity_ids = {entity["entity_id"] for entity in entities}
            states = {state["entity_id"]: state for state in all_states if state["entity_id"] in entity_ids}
            if args.dashboard:
                mapping = {}
                for entity in entities:
                    unique_id = entity["unique_id"]
                    prefix = namespace + "_" + settings["system_id"] + "_"
                    if unique_id.startswith(prefix):
                        key = unique_id.removeprefix(prefix)
                        domain = entity["entity_id"].split(".")[0]
                        mapping[f"{domain}.beszel_server_{key}"] = entity["entity_id"]
                    bridge_prefix = namespace + "_bridge_"
                    if unique_id.startswith(bridge_prefix):
                        key = unique_id.removeprefix(bridge_prefix)
                        domain = entity["entity_id"].split(".")[0]
                        mapping[f"{domain}.beszel_bridge_{key}"] = entity["entity_id"]

                def substitute(value):
                    if isinstance(value, dict):
                        return {key: substitute(item) for key, item in value.items()}
                    if isinstance(value, list):
                        return [substitute(item) for item in value]
                    if isinstance(value, str):
                        return mapping.get(value, value.replace("BESZEL_HUB_URL", settings["hub_browser_url"]))
                    return value

                views = []
                for name in ("server-overview.yaml", "server-detail.yaml"):
                    document = yaml.safe_load(Path("/repo/docs/dashboards/beszel/" + name).read_text())
                    views.extend(substitute(document)["views"])
                path = "beszel-bridge-lab"
                dashboards = await command("lovelace/dashboards/list")
                if not any(dashboard["url_path"] == path for dashboard in dashboards):
                    await command("lovelace/dashboards/create", url_path=path, title="Beszel servers", icon="mdi:server-network", show_in_sidebar=True, require_admin=False)
                await command("lovelace/config/save", url_path=path, config={"title": "Beszel servers", "views": views})
            print(json.dumps({"entities": entities, "devices": devices, "states": states}, sort_keys=True))


asyncio.run(main())

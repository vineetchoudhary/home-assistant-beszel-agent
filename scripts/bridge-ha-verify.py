#!/usr/bin/env python3
"""Inspect an isolated HA test instance through its real REST/WebSocket APIs.

Run inside the test HA container; credentials stay in its private /config file.
This is test tooling, not a component installed in a user's Home Assistant.
"""
import argparse
import asyncio
import json
import hashlib
from pathlib import Path

import aiohttp
import yaml


async def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--dashboard", action="store_true")
    parser.add_argument("--customize", action="store_true", help="Rename the test CPU entity through HA's registry")
    parser.add_argument("--enable-key", action="append", default=[], help="Enable a detailed test entity by its Bridge metric key")
    parser.add_argument("--disable-key", action="append", default=[], help="Disable a test entity explicitly as a user")
    parser.add_argument("--storage-dashboard", action="store_true", help="Import the storage template with the expanded fixture's actual entity IDs")
    parser.add_argument("--reset-customizations", action="store_true", help="Restore the lab CPU entity's default name and ID for setup screenshots")
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
            if args.reset_customizations:
                cpu_id = namespace + "_" + settings["system_id"] + "_cpu_usage"
                cpu = next(entity for entity in registry if entity["unique_id"] == cpu_id)
                await command("config/entity_registry/update", entity_id=cpu["entity_id"],
                              new_entity_id="sensor." + cpu_id, name=None)
                registry = await command("config/entity_registry/list")
            for key, disabled_by in [(key, None) for key in args.enable_key] + [(key, "user") for key in args.disable_key]:
                unique_id = namespace + "_" + settings["system_id"] + "_" + key
                entity = next(item for item in registry if item["unique_id"] == unique_id)
                await command("config/entity_registry/update", entity_id=entity["entity_id"], disabled_by=disabled_by)
            if args.enable_key or args.disable_key:
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
            if args.dashboard or args.storage_dashboard:
                mapping = {}
                metric_entities = {}
                for entity in entities:
                    unique_id = entity["unique_id"]
                    prefix = namespace + "_" + settings["system_id"] + "_"
                    if unique_id.startswith(prefix):
                        key = unique_id.removeprefix(prefix)
                        domain = entity["entity_id"].split(".")[0]
                        mapping[f"{domain}.beszel_server_{key}"] = entity["entity_id"]
                        metric_entities[key] = entity["entity_id"]
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

                templates = ["server-overview.yaml", "server-detail.yaml"]
                if args.storage_dashboard:
                    def component_key(family, identity, metric):
                        return family + "_" + hashlib.sha256(identity.encode()).hexdigest()[:24] + "_" + metric
                    aliases = {"extra_disk_usage": component_key("filesystems", "data", "usage")}
                    for field in ("status", "capacity", "power_on_seconds", "power_on_human", "cycles", "temperature", "last_updated", "updated_human"):
                        aliases["smart_" + field] = component_key("smart", "smt000000000001", field)
                    for alias, metric in aliases.items():
                        assert metric in metric_entities, "Storage template requires the expanded fixture's entities"
                        mapping["sensor.beszel_server_" + alias] = metric_entities[metric]
                    templates.append("storage-health.yaml")
                views = []
                for name in templates:
                    document = yaml.safe_load(Path("/repo/docs/dashboards/beszel/" + name).read_text())
                    views.extend(substitute(document)["views"])
                path = "beszel-bridge-lab"
                dashboards = await command("lovelace/dashboards/list")
                if not any(dashboard["url_path"] == path for dashboard in dashboards):
                    await command("lovelace/dashboards/create", url_path=path, title="Beszel servers", icon="mdi:server-network", show_in_sidebar=True, require_admin=False)
                await command("lovelace/config/save", url_path=path, config={"title": "Beszel servers", "views": views})
            print(json.dumps({"entities": entities, "devices": devices, "states": states}, sort_keys=True))


asyncio.run(main())

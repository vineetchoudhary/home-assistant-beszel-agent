#!/usr/bin/env python3
"""Isolated real Hub/Agent -> Bridge -> Mosquitto -> Home Assistant test.

Uses disposable containers and loopback-only published ports. Production HA
and Supervisor installations are never contacted. Run with Docker available.
"""
import argparse
import hashlib
import json
import os
import re
from pathlib import Path
import secrets
import shutil
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

ROOT = Path(__file__).resolve().parent.parent
HA_IMAGE = "ghcr.io/home-assistant/home-assistant:2026.10.1"

HA_API_PROXY = r'''
import asyncio
from aiohttp import ClientSession, WSMsgType, web

async def websocket_proxy(request):
    client = web.WebSocketResponse(max_msg_size=32 << 20)
    await client.prepare(request)
    async with ClientSession() as session:
        async with session.ws_connect("http://ha:8123/api/websocket", max_msg_size=32 << 20) as upstream:
            async def forward(source, target):
                async for message in source:
                    if message.type == WSMsgType.TEXT:
                        await target.send_str(message.data)
                    elif message.type == WSMsgType.BINARY:
                        await target.send_bytes(message.data)
            tasks = [asyncio.create_task(forward(client, upstream)), asyncio.create_task(forward(upstream, client))]
            _, pending = await asyncio.wait(tasks, return_when=asyncio.FIRST_COMPLETED)
            for task in pending:
                task.cancel()
            await asyncio.gather(*tasks, return_exceptions=True)
    return client

app = web.Application()
app.router.add_get("/core/websocket", websocket_proxy)
web.run_app(app, host="0.0.0.0", port=80, print=None)
'''
PROXY = '''import datetime,http.server,json,pathlib,ssl,urllib.request,urllib.error,urllib.parse
class Proxy(http.server.BaseHTTPRequestHandler):
    def handle_request(self):
        if not self.path.startswith('/beszel/'):
            self.send_error(404); return
        headers={k:v for k,v in self.headers.items() if k.lower() not in ('host','content-length','connection')}
        body=self.rfile.read(int(self.headers.get('Content-Length',0))) if self.command=='POST' else None
        req=urllib.request.Request('http://hub:8090/'+self.path.removeprefix('/beszel/'),data=body,headers=headers,method=self.command)
        try:
            upstream=urllib.request.urlopen(req,timeout=15)
        except urllib.error.HTTPError as error:
            upstream=error
        data=upstream.read()
        status=upstream.status
        fault_path=pathlib.Path('/lab/proxy-fault.json')
        fault=json.loads(fault_path.read_text()) if fault_path.exists() else {}
        if fault.get('expanded'):
            fixture=json.loads(pathlib.Path('/repo/beszel_bridge/testdata/expanded/monitoring.json').read_text())
            system_id=pathlib.Path('/lab/expanded-system-id').read_text().strip()
            now=datetime.datetime.now(datetime.timezone.utc)
            timestamp=lambda delta=0: (now-datetime.timedelta(hours=delta)).isoformat()
            millis=int(now.timestamp()*1000)
            parsed=urllib.parse.urlparse(self.path)
            collection=parsed.path.split('/')[-2] if parsed.path.endswith('/records') else ''
            if collection=='systems':
                response=json.loads(data)
                for system in response.get('items',[]):
                    if system['id']==system_id:
                        system['info'].update(fixture['system']['info'])
                        system['status']='up'
                data=json.dumps(response).encode()
            elif collection=='system_stats':
                sample=fixture['peak'] if "type='10m'" in urllib.parse.parse_qs(parsed.query).get('filter',[''])[0] else fixture['sample']
                sample.update(system=system_id,created=timestamp())
                if fault.get('expanded_bad_disk'): sample['stats'].setdefault('efs',{}).setdefault('data',{})['d']='changed-shape'
                data=json.dumps({'page':1,'totalPages':1,'totalItems':1,'items':[sample]}).encode(); status=200
            elif collection in fixture and isinstance(fixture[collection],list):
                rows=fixture[collection]
                for row in rows:
                    row['system']=system_id
                    if 'updated' in row: row['updated']=timestamp(10) if collection=='smart_devices' else millis if isinstance(row['updated'],int) else timestamp()
                    if 'details_updated' in row: row['details_updated']=timestamp(10)
                    if 'created' in row: row['created']=millis if isinstance(row['created'],int) else timestamp()
                    if row.get('certInfo'): row['certInfo']['expires']=millis+30*86400000
                data=json.dumps({'page':1,'totalPages':1 if rows else 0,'totalItems':len(rows),'items':rows}).encode(); status=200
            elif parsed.path.endswith('/api/beszel/package-updates'):
                row=fixture['package_updates']; row['checkedAt']=int(now.timestamp())-36000
                data=json.dumps(row).encode(); status=200
        if fault:
            response=json.loads(data)
            if self.path.endswith('/api/beszel/info') and fault.get('future_version'):
                response['v']='9.99.0'
                response['future_field']={'format':'new'}
            if '/systems/records' in self.path:
                if fault.get('invalid_status'):
                    for system in response.get('items',[]): system['status']={'format':'changed'}
                if fault.get('incomplete_inventory'):
                    response['items'].append({'id':False,'name':'unreadable'})
                    response['totalItems']+=1
            if '/system_stats/records' in self.path:
                for sample in response.get('items',[]):
                    if fault.get('stale'): sample['created']='2000-01-01 00:00:00.000Z'
                    if fault.get('invalid_timestamp'): sample['created']='changed-timestamp-format'
                    if fault.get('invalid_cpu'): sample['stats']['cpu']={'changed':'upstream-secret-do-not-echo'}
                    if fault.get('missing_memory'): sample['stats'].pop('mp',None)
                if fault.get('invalid_sample'): response['items']=['changed-record-format']
            data=json.dumps(response).encode()
        self.send_response(status)
        self.send_header('Content-Type','application/json')
        self.send_header('Content-Length',str(len(data)))
        self.end_headers(); self.wfile.write(data)
    do_GET=handle_request
    do_POST=handle_request
    def log_message(self,*args): pass
server=http.server.ThreadingHTTPServer(('0.0.0.0',8443),Proxy)
ctx=ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain('/lab/proxy.crt','/lab/proxy.key')
server.socket=ctx.wrap_socket(server.socket,server_side=True)
server.serve_forever()
'''


class Lab:
    def __init__(self, args):
        self.args = args
        self.directory = Path(tempfile.mkdtemp(prefix="beszel-bridge-e2e-"))
        self.tag = f"beszel-bridge-e2e-{os.getpid()}-{secrets.token_hex(3)}"
        self.containers = []
        self.network_created = False
        self.checks = []
        self.ha_url = ""

    def build(self):
        import re
        app_version = re.search(r'^version: "([^"]+)"', (ROOT / "beszel_bridge/config.yaml").read_text(), re.M).group(1)
        hub_source = self.directory / "hub-source"
        shutil.copytree(ROOT / "beszel_hub", hub_source)
        (hub_source / "beszel_version").write_text(self.args.hub_version + "\n")
        for name, image, source in [("bridge", self.args.image, ROOT / "beszel_bridge"), ("hub", self.args.hub_image, hub_source)]:
            log = self.directory / (name + "-build.log")
            with log.open("w") as output:
                result = subprocess.run(["docker", "build", "--progress=plain", "--build-arg", "BUILD_FROM=ghcr.io/home-assistant/base:latest", "--build-arg", "BUILD_VERSION=" + app_version, "-t", image, str(source)], stdout=output, stderr=subprocess.STDOUT)
            if result.returncode:
                descriptor, preserved_name = tempfile.mkstemp(prefix="beszel-bridge-failed-build-", suffix=".log")
                os.close(descriptor)
                preserved = Path(preserved_name)
                shutil.copyfile(log, preserved)
                raise RuntimeError(f"{name} image build failed; complete log saved to {preserved}")
            print("PASS built " + name + " image", flush=True)

    def docker(self, *args, input=None):
        # Capture output so errors never print test account tokens or options.
        result = subprocess.run(["docker", *map(str, args)], input=input, text=True, capture_output=True)
        if result.returncode:
            raise RuntimeError(f"Docker {args[0]} failed: {result.stderr[-1800:]}")
        return result.stdout.strip()

    def start(self, suffix, image, *options):
        name = self.tag + "-" + suffix
        self.containers.append(name)
        self.docker("run", "-d", "--name", name, "--network", self.tag, *options, image)
        return name

    def port(self, container, internal):
        return self.docker("port", container, internal).rsplit(":", 1)[1]

    def request(self, base, path, payload=None, token=None, method=None, form=False):
        headers = {}
        if token:
            headers["Authorization"] = "Bearer " + token if base == self.ha_url else token
        data = None
        if payload is not None:
            data = urllib.parse.urlencode(payload).encode() if form else json.dumps(payload).encode()
            headers["Content-Type"] = "application/x-www-form-urlencoded" if form else "application/json"
        request = urllib.request.Request(base + path, data=data, headers=headers, method=method)
        with urllib.request.urlopen(request, timeout=20) as response:
            return json.load(response)

    def wait(self, title, condition, timeout=90):
        deadline = time.monotonic() + timeout
        last_error = "condition not satisfied"
        while time.monotonic() < deadline:
            try:
                result = condition()
                if result:
                    self.checks.append(title)
                    print("PASS " + title, flush=True)
                    return result
            except Exception as error:
                # Do not print HTTP response bodies or auth-bearing commands.
                last_error = type(error).__name__
            time.sleep(2)
        raise RuntimeError(title + " timed out (" + last_error + ")")

    def snapshot(self, dashboard=False, customize=False, enable_key=None, disable_key=None):
        args = ["exec", self.ha, "python", "/repo/scripts/bridge-ha-verify.py"]
        if dashboard:
            args.append("--dashboard")
        if customize:
            args.append("--customize")
        if enable_key:
            args.extend(["--enable-key", enable_key])
        if disable_key:
            args.extend(["--disable-key", disable_key])
        return json.loads(self.docker(*args))

    def entity(self, key, system_id=None):
        snapshot = self.snapshot()
        unique_id = self.namespace + "_" + (system_id or self.system_id) + "_" + key
        entity = next((entity for entity in snapshot["entities"] if entity["unique_id"] == unique_id), None)
        return snapshot["states"].get(entity["entity_id"]) if entity else None

    def metric_ready(self):
        state = self.entity("cpu_usage")
        return state and state["state"] not in ("unknown", "unavailable")

    def metric_unavailable(self):
        state = self.entity("cpu_usage")
        return state and state["state"] == "unavailable"

    def status(self):
        return json.loads(self.docker("exec", self.bridge, "/usr/local/bin/bridge", "status"))

    def bridge_state(self):
        # The container writes this private file as root. Read it with the same
        # container user, rather than the unprivileged Linux CI host account.
        return json.loads(self.docker("exec", self.bridge, "cat", "/data/bridge-state.json"))

    def readable(self, key):
        entity = self.entity(key)
        return entity and entity["state"] not in ("unavailable", "unknown")

    def assert_identities_preserved(self, identities):
        current = {entity["unique_id"]: entity["entity_id"] for entity in self.snapshot()["entities"]}
        assert all(current.get(key) == value for key, value in identities.items()), "Existing entity identity changed"

    @staticmethod
    def component_key(family, identity, metric):
        return family + "_" + hashlib.sha256(identity.encode()).hexdigest()[:24] + "_" + metric

    def expanded_metric_checks(self):
        # Synthetic hardware responses pass through the authenticated HTTPS Hub
        # proxy, then the actual Bridge, broker and HA discovery/registry stack.
        fault = self.directory / "proxy-fault.json"
        fault.write_text(json.dumps({"expanded": True}))
        combined = "filesystems_combined_usage"
        self.wait("weighted multi-disk usage reaches HA", lambda: self.entity(combined) and self.entity(combined)["state"] == "14")
        smart = self.component_key("smart", "smt000000000001", "status")
        seconds = self.component_key("smart", "smt000000000001", "power_on_seconds")
        human = self.component_key("smart", "smt000000000001", "power_on_human")
        self.wait("ten-hour-old SMART status remains available", lambda: self.entity(smart) and self.entity(smart)["state"] == "PASSED", timeout=120)
        assert self.entity(seconds)["state"] in ("72000", "72000.0")
        assert self.entity(human)["state"] == "20 hours"
        assert self.entity(smart)["attributes"]["device"] == "/dev/sda"
        assert self.entity(smart)["attributes"]["model"] == "CT1000BX500SSD1"
        assert self.entity("uptime_human")["state"] == "20 hours"
        self.checks.append("SMART metadata, raw seconds and readable durations reach HA")
        core = self.component_key("cpu", "0", "usage")
        registry = self.snapshot()["entities"]
        entity = next(row for row in registry if row["unique_id"] == self.namespace + "_" + self.system_id + "_" + core)
        assert entity["disabled_by"] == "integration", "Detailed entity was enabled by default"
        self.snapshot(enable_key=core)
        self.wait("user can enable a detailed CPU entity", lambda: self.entity(core) and self.entity(core)["state"] in ("10", "10.0"))
        raw_metric = "attribute_" + hashlib.sha256(b"id:199").hexdigest()[:24]
        raw_key = self.component_key("smart_attributes", "smt000000000001", raw_metric)
        self.snapshot(enable_key=raw_key)
        self.wait("raw SMART attribute preserves large integer metadata", lambda: self.entity(raw_key) and self.entity(raw_key)["attributes"].get("raw_value") == "9007199254740993")
        self.enable_all_entity_checks()
        self.history_retention_checks()
        self.grouped_entity_checks()
        fault.write_text(json.dumps({"expanded": True, "expanded_bad_disk": True}))
        self.wait("invalid disk makes aggregate unavailable without hiding CPU", lambda: self.entity(combined)["state"] == "unavailable" and self.readable("cpu_usage"))
        fault.write_text(json.dumps({"expanded": True}))
        self.wait("valid disk recovers combined usage", lambda: self.entity(combined)["state"] == "14")
        fault.unlink()

    def grouped_entity_checks(self):
        baseline = self.snapshot()
        identities = {entity["unique_id"]: entity["entity_id"] for entity in baseline["entities"]}
        smart = self.component_key("smart", "smt000000000001", "status")
        container = self.component_key("containers", "abcdef123456", "status")
        service = self.component_key("services", "svc000000000001", "state")
        details = set()
        for entity in self.bridge_state()["topics"].values():
            family = entity.get("family")
            component = entity.get("component", "")
            if family not in ("smart", "smart_attributes", "containers", "services") or ":" not in component:
                continue
            identity = component.split(":", 1)[1]
            if not identity:
                continue
            primary = self.component_key("smart" if family == "smart_attributes" else family, identity,
                                         "state" if family == "services" else "status")
            if entity["metric"] != primary:
                details.add(self.namespace + "_" + self.system_id + "_" + entity["metric"])
        assert details, "Expanded fixture needs individual detail entities before grouping"
        self.stop_bridge()
        self.options["group_entities"] = True
        self.save_options()
        self.start_bridge()

        def grouped():
            snapshot = self.snapshot()
            entities = {entity["unique_id"]: entity["entity_id"] for entity in snapshot["entities"]}
            if details.intersection(entities):
                return False
            def state(key):
                entity_id = entities.get(self.namespace + "_" + self.system_id + "_" + key)
                return snapshot["states"].get(entity_id, {})
            disk, app, daemon = state(smart), state(container), state(service)
            attrs = disk.get("attributes", {})
            return (disk.get("state") == "PASSED" and attrs.get("power_on_seconds") == 72000
                    and attrs.get("temperature_celsius") == 32
                    and any(row.get("raw_value") == "9007199254740993" for row in attrs.get("smart_attributes", []))
                    and app.get("attributes", {}).get("memory_bytes") == 134217728
                    and app.get("attributes", {}).get("receive_bytes_per_second") == 200
                    and daemon.get("state") == "failed"
                    and daemon.get("attributes", {}).get("cpu_usage_percent") == 0)

        self.wait("optional grouping replaces SMART, container and service details with attributes", grouped, timeout=120)
        preserved = {key: value for key, value in identities.items() if key not in details}
        self.assert_identities_preserved(preserved)
        assert self.entity("cpu_usage")["attributes"]["friendly_name"] == "My lab CPU"
        self.checks.append("grouped summaries preserve IDs, unrelated entities and exact SMART integers")
        self.stop_bridge()
        self.options["group_entities"] = False
        self.save_options()
        self.start_bridge()

        def individual():
            registered = {entity["unique_id"] for entity in self.snapshot()["entities"]}
            return details.issubset(registered) and self.readable(self.component_key("smart", "smt000000000001", "power_on_seconds"))

        self.wait("disabling grouping restores individual detail discovery", individual, timeout=120)
        self.assert_identities_preserved(preserved)

    def enable_all_entity_checks(self):
        baseline = self.snapshot()
        # Pick an actual detailed GPU entity rather than assuming its source key.
        gpu = next(entity for entity in baseline["entities"] if "_gpus_" in entity["unique_id"] and entity["disabled_by"] == "integration")
        user_disabled = gpu["unique_id"].removeprefix(self.namespace + "_" + self.system_id + "_")
        baseline = self.snapshot(disable_key=user_disabled)
        identities = {entity["unique_id"]: entity["entity_id"] for entity in baseline["entities"]}
        disabled = {entity["unique_id"] for entity in baseline["entities"] if entity["disabled_by"] == "integration"}
        assert disabled, "Expanded fixture needs integration-disabled entities"
        self.stop_bridge()
        self.options["enable_all_entities"] = True
        self.save_options()
        self.start_bridge()

        def all_enabled():
            entities = {entity["unique_id"]: entity for entity in self.snapshot()["entities"]}
            return all(key in entities and entities[key]["disabled_by"] is None for key in disabled)

        self.wait("enable_all_entities enables existing disabled entities in real HA", all_enabled, timeout=120)
        current = self.snapshot()
        assert next(entity for entity in current["entities"] if entity["unique_id"] == gpu["unique_id"])["disabled_by"] == "user", "Explicit user disable was overwritten"
        self.assert_identities_preserved(identities)
        assert self.entity("cpu_usage")["attributes"]["friendly_name"] == "My lab CPU"
        self.checks.append("bulk enabling preserves existing entity IDs, user names and explicit disables")
        self.stop_bridge()
        self.options["enable_all_entities"] = False
        self.save_options()
        self.start_bridge()
        self.wait("turning enable_all_entities off keeps existing entities enabled", all_enabled)

    def history_retention_checks(self):
        baseline = self.snapshot()
        cpu = next(entity for entity in baseline["entities"] if entity["unique_id"] == self.namespace + "_" + self.system_id + "_cpu_usage")
        identities = {entity["unique_id"]: (entity["entity_id"], entity["disabled_by"]) for entity in baseline["entities"]}

        def history(seed=False):
            command = ["exec", self.ha, "python", "/repo/scripts/bridge-ha-history.py", "--entity-id", cpu["entity_id"]]
            if seed:
                command.append("--seed")
            return json.loads(self.docker(*command))

        seeded = self.wait("disposable Recorder history fixture is committed", lambda: history(seed=True))
        assert all(seeded[name] == 1 for name in ("owned_old", "owned_middle", "owned_recent", "unrelated_old"))
        for days in (5, 2):
            self.stop_bridge()
            self.options["history_retention_days"] = days
            self.save_options()
            self.start_bridge()

            def retained():
                rows = history()
                return rows["owned_old"] == 0 and rows["owned_middle"] == int(days == 5) and rows["owned_recent"] == 1 and rows["unrelated_old"] == 1

            self.wait(f"{days}-day Bridge retention purges only older owned history", retained, timeout=120)
        state = self.bridge_state()
        assert state["history_keep_days"] == 2 and state["history_purge_at"], "Cleanup schedule was not persisted"
        self.stop_bridge()
        self.options["history_retention_days"] = 0
        self.save_options()
        self.start_bridge()
        self.wait("Bridge resumes with history retention disabled", self.metric_ready)
        after = self.snapshot()
        current = {entity["unique_id"]: (entity["entity_id"], entity["disabled_by"]) for entity in after["entities"]}
        assert all(current.get(key) == value for key, value in identities.items()), "History retention changed entity identity or enablement"
        assert self.bridge_state()["history_purge_at"] == state["history_purge_at"], "Disabled retention requested another cleanup"
        self.checks.append("history retention preserves recent and unrelated data, entity settings and the persisted cleanup schedule")

    def data_fault_checks(self):
        fault = self.directory / "proxy-fault.json"
        scenarios = [
            ("future_version", "future Hub version metadata permits normal polling", (), ("cpu_usage", "memory_usage")),
            ("invalid_cpu", "malformed CPU leaves memory and disk available in HA", ("cpu_usage",), ("memory_usage", "disk_usage")),
            ("missing_memory", "missing memory leaves CPU and disk available in HA", ("memory_usage",), ("cpu_usage", "disk_usage")),
            ("invalid_status", "unknown server status leaves valid readings available in HA", ("system_status", "connectivity"), ("cpu_usage", "memory_usage")),
            ("invalid_sample", "broken sample preserves devices and Bridge liveness", ("cpu_usage", "memory_usage"), ()),
            ("invalid_timestamp", "invalid source timestamp fails closed without stopping Bridge", ("cpu_usage", "memory_usage"), ()),
            ("incomplete_inventory", "unreadable inventory record preserves valid HA readings", (), ("cpu_usage", "memory_usage")),
        ]
        for mode, title, unavailable, available in scenarios:
            before = self.status()["last_sync"]
            temporary = fault.with_suffix(".tmp")
            temporary.write_text(json.dumps({mode: True}))
            temporary.replace(fault)
            def observed():
                status = self.status()
                if status.get("last_sync") == before or not status.get("hub_connected"):
                    return False
                if mode == "future_version":
                    version = self.entity("hub_version", "bridge")
                    if not version or version["state"] != "9.99.0": return False
                if mode == "incomplete_inventory" and not status.get("inventory_incomplete"): return False
                if unavailable and status.get("degraded_systems") != 1: return False
                for key in unavailable:
                    entity = self.entity(key)
                    if not entity or entity["state"] != "unavailable": return False
                return all(self.readable(key) for key in available)
            self.wait(title, observed)
            self.docker("exec", self.bridge, "/usr/local/bin/bridge", "health")
            fault.unlink()
            self.wait("valid data recovers after " + mode, lambda: not self.status().get("error") and self.metric_ready() and self.readable("memory_usage"))
        logs = subprocess.run(["docker", "logs", self.bridge], text=True, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=True).stdout
        assert "upstream-secret-do-not-echo" not in logs, "Malformed API value leaked into logs"
        self.checks.append("malformed values are omitted from Bridge logs")

    def start_bridge(self):
        environment = []
        if getattr(self, "ha_token", None):
            env_file = self.directory / "ha-api.env"
            env_file.write_text("SUPERVISOR_TOKEN=" + self.ha_token + "\n")
            env_file.chmod(0o600)
            environment = ["--env-file", str(env_file)]
        self.bridge = self.start("bridge", self.args.image, *environment, "-v", f"{self.bridge_dir}:/data", "-v", f"{self.directory}:/ssl:ro")

    def stop_bridge(self):
        self.docker("rm", "-fv", self.bridge)
        self.containers.remove(self.bridge)

    def save_options(self):
        target = self.bridge_dir / "options.json"
        target.write_text(json.dumps(self.options))
        target.chmod(0o600)

    def prepare(self):
        self.docker("network", "create", self.tag)
        self.network_created = True
        self.ha_url = ""
        self.bridge_dir = self.directory / "bridge"
        self.ha_dir = self.directory / "ha"
        self.bridge_dir.mkdir()
        self.ha_dir.mkdir()
        (self.directory / "mosquitto.conf").write_text("listener 1883 0.0.0.0\nallow_anonymous true\npersistence true\npersistence_location /mosquitto/data/\n")
        self.broker = self.start("broker", "eclipse-mosquitto:2.0.22", "--network-alias", "broker", "-v", f"{self.directory / 'mosquitto.conf'}:/mosquitto/config/mosquitto.conf:ro")
        password = secrets.token_urlsafe(24)
        self.hub = self.start("hub", self.args.hub_image, "--network-alias", "hub", "-e", "BESZEL_HUB_USER_EMAIL=admin@example.test", "-e", "BESZEL_HUB_USER_PASSWORD=" + password)
        # Hub port 8090 is deliberately not published. A temporary client uses
        # the same internal DNS/port a Bridge app would use inside HA.
        self.client = self.tag + "-client"
        self.docker("run", "-d", "--name", self.client, "--network", self.tag, "--entrypoint", "sleep", "-v", f"{self.directory}:/lab", "-v", f"{ROOT}:/repo:ro", HA_IMAGE, "infinity")
        self.containers.append(self.client)
        # Expose a test-only API client over the Docker CLI; no Hub host port.
        client_script = '''import json,sys,urllib.request
data=json.load(sys.stdin)
headers={'Content-Type':'application/json'}
if data.get('token'): headers['Authorization']=data['token']
body=json.dumps(data['payload']).encode() if data.get('payload') is not None else None
r=urllib.request.Request('http://hub:8090'+data['path'],data=body,headers=headers,method=data.get('method'))
with urllib.request.urlopen(r,timeout=15) as response: print(response.read().decode())
'''
        (self.directory / "hub-client.py").write_text(client_script)

        def api(path, payload=None, token=None, method=None):
            return json.loads(self.docker("exec", "-i", self.client, "python", "/lab/hub-client.py", input=json.dumps({"path": path, "payload": payload, "token": token, "method": method})))

        self.hub_api = api
        self.wait("local Hub API reachable without a published host port", lambda: api("/api/health"))
        auth = api("/api/collections/users/auth-with-password", {"identity": "admin@example.test", "password": password})
        self.admin_token = auth["token"]
        info = api("/api/beszel/info", token=self.admin_token)
        registration = api("/api/beszel/universal-token?enable=1", token=self.admin_token)["token"]
        self.agent = self.start("agent", f"henrygd/beszel-agent:{self.args.hub_version}", "--hostname", "bridge-test-server", "-e", "KEY=" + info["key"], "-e", "TOKEN=" + registration, "-e", "HUB_URL=http://hub:8090")
        system = self.wait("real Agent reports metrics to Hub", lambda: next((row for row in api("/api/collections/systems/records", token=self.admin_token)["items"] if row["status"] == "up"), None))
        self.system_id = system["id"]
        super_token = api("/api/collections/_superusers/auth-with-password", {"identity": "admin@example.test", "password": password})["token"]
        reader_password = secrets.token_urlsafe(24)
        reader = api("/api/collections/users/records", {"email": "reader@example.test", "password": reader_password, "passwordConfirm": reader_password, "role": "readonly", "verified": True}, super_token)
        api("/api/collections/systems/records/" + self.system_id, {"users": [auth["record"]["id"], reader["id"]]}, super_token, "PATCH")
        self.options = {"hub_url": "http://hub:8090", "hub_email": "reader@example.test", "hub_password": reader_password,
                        "mqtt_source": "manual", "mqtt_host": "broker", "mqtt_port": 1883,
                        "poll_interval": 15, "stale_after": 180, "discovery_prefix": "homeassistant", "ha_status_topic": "homeassistant/status"}
        self.save_options()
        if self.args.capture_fixtures:
            self.capture_fixtures(reader_password, auth["record"]["id"], reader["id"])
        self.start_bridge()
        self.wait("Bridge persists stable Hub identity", lambda: (self.bridge_dir / "bridge-state.json").exists())
        self.namespace = self.bridge_state()["namespace"]

    def capture_fixtures(self, password, admin_id, reader_id):
        api = self.hub_api
        reader = api("/api/collections/users/auth-with-password", {"identity": "reader@example.test", "password": password})
        token = reader["token"]
        info = api("/api/beszel/info", token=token)
        assert info["v"] == self.args.hub_version, "Hub is not running the requested release"
        systems = api("/api/collections/systems/records?page=1&perPage=100&sort=id", token=token)
        query = urllib.parse.urlencode({"filter": "system='" + self.system_id + "' && type='1m'", "sort": "-created", "perPage": 1})
        samples = self.wait("readonly account can read a real source sample", lambda: self.source_sample(api, token, query))
        replacements = {self.system_id: "sys000000000001", samples["items"][0]["id"]: "sta000000000001", admin_id: "usr000000000001", reader_id: "usr000000000002"}
        def sanitize(value):
            if isinstance(value, dict):
                return {key: "example-host" if key == "host" else sanitize(item) for key, item in value.items()}
            if isinstance(value, list):
                return [sanitize(item) for item in value]
            return replacements.get(value, value) if isinstance(value, str) else value
        info["key"] = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIFixturePublicKeyForBridgeIdentityOnly"
        for system in systems["items"]:
            system["name"] = "Example server"
        destination = self.args.capture_fixtures
        destination.mkdir(parents=True, exist_ok=False)
        for name, response in [("info", info), ("systems", systems), ("system_stats", samples)]:
            (destination / (name + ".json")).write_text(json.dumps(sanitize(response), indent=2) + "\n")
        print("PASS captured sanitized " + self.args.hub_version + " readonly API fixtures", flush=True)

    @staticmethod
    def source_sample(api, token, query):
        response = api("/api/collections/system_stats/records?" + query, token=token)
        return response if response.get("items") else None

    def setup_ha(self):
        # A full Core instance exercises its built-in MQTT integration and UI.
        (self.ha_dir / "configuration.yaml").write_text("default_config:\nrecorder:\n  db_url: sqlite:////recorder/home-assistant_v2.db\nhomeassistant:\n  name: Beszel Bridge Lab\n  time_zone: UTC\n  country: US\n  latitude: 0\n  longitude: 0\n  elevation: 0\n  unit_system: metric\n")
        # Keep SQLite/WAL on a native volume. Shared macOS mounts can give the
        # Recorder and the history-fixture process different WAL file handles.
        self.ha = self.start("ha", HA_IMAGE, "--network-alias", "ha", "-p", "127.0.0.1::8123", "-v", f"{self.ha_dir}:/config", "-v", "/recorder", "-v", f"{ROOT}:/repo:ro")
        self.ha_url = "http://127.0.0.1:" + self.port(self.ha, "8123/tcp")
        self.wait("Home Assistant onboarding available", lambda: self.request(self.ha_url, "/api/onboarding"), timeout=240)
        self.client_id = self.ha_url + "/"
        response = self.request(self.ha_url, "/api/onboarding/users", {"client_id": self.client_id, "name": "Bridge Lab", "username": "bridge_lab", "password": "bridge-lab-only-password", "language": "en"})
        auth = self.request(self.ha_url, "/auth/token", {"grant_type": "authorization_code", "code": response["auth_code"], "client_id": self.client_id}, form=True)
        self.ha_token = auth["access_token"]
        token_file = self.ha_dir / "bridge-test-auth.json"
        token_file.write_text(json.dumps({"access_token": self.ha_token, "namespace": self.namespace, "system_id": self.system_id, "hub_browser_url": "https://monitoring.example.com"}))
        token_file.chmod(0o600)
        for path, payload in [("/api/onboarding/core_config", {}), ("/api/onboarding/analytics", {}), ("/api/onboarding/integration", {"client_id": self.client_id, "redirect_uri": self.client_id})]:
            self.request(self.ha_url, path, payload, self.ha_token)
        flow = self.request(self.ha_url, "/api/config/config_entries/flow", {"handler": "mqtt", "show_advanced_options": False}, self.ha_token)
        result = self.request(self.ha_url, "/api/config/config_entries/flow/" + flow["flow_id"],
                              {"broker": "broker", "port": 1883, "protocol": "5", "other_settings": {"set_ca_cert": "off", "set_client_cert": False, "transport": "tcp"}}, self.ha_token)
        assert result["type"] == "create_entry", "MQTT config flow did not complete: " + str(result.get("errors"))
        (self.directory / "ha-api.py").write_text(HA_API_PROXY)
        self.ha_api = self.tag + "-ha-api"
        self.containers.append(self.ha_api)
        self.docker("run", "-d", "--name", self.ha_api, "--network", self.tag, "--network-alias", "supervisor", "--entrypoint", "python", "-v", f"{self.directory}:/lab:ro", HA_IMAGE, "/lab/ha-api.py")
        self.wait("HA creates real MQTT metric entities", self.metric_ready)

    def exercise(self):
        baseline = self.snapshot(customize=True)
        identities = {entity["unique_id"]: entity["entity_id"] for entity in baseline["entities"]}
        assert len(baseline["devices"]) == 2, "Expected Bridge and monitored server devices"
        assert self.entity("cpu_usage")["attributes"]["unit_of_measurement"] == "%"
        assert not any(entity["unique_id"].endswith("_temperature") for entity in baseline["entities"]), "invented temperature without a source sensor"
        self.checks.append("device grouping, units, and absent sensor capability")
        # Rename in the actual Hub; stable IDs and HA customizations must survive.
        self.hub_api("/api/collections/systems/records/" + self.system_id, {"name": "Renamed lab server"}, self.admin_token, "PATCH")
        self.wait("server rename updates device name", lambda: any(device["name"] == "Renamed lab server" for device in self.snapshot()["devices"]))
        self.assert_identities_preserved(identities)
        self.checks.append("server rename preserves entity identities")
        assert self.entity("cpu_usage")["attributes"]["friendly_name"] == "My lab CPU"
        self.checks.append("HA entity ID and name customization survive rediscovery")
        # HTTPS reverse proxy with a base path represents a remote Hub endpoint.
        config = "[req]\ndistinguished_name=dn\nx509_extensions=extensions\nprompt=no\n[dn]\nCN=proxy\n[extensions]\nsubjectAltName=DNS:proxy,IP:127.0.0.1\nbasicConstraints=critical,CA:TRUE\nkeyUsage=critical,digitalSignature,keyEncipherment,keyCertSign\n"
        (self.directory / "openssl.cnf").write_text(config)
        subprocess.run(["openssl", "req", "-x509", "-newkey", "rsa:2048", "-nodes", "-days", "1", "-config", str(self.directory / "openssl.cnf"), "-keyout", str(self.directory / "proxy.key"), "-out", str(self.directory / "proxy.crt")], check=True, capture_output=True)
        (self.directory / "proxy.py").write_text(PROXY)
        self.proxy = self.tag + "-proxy"
        (self.directory / "expanded-system-id").write_text(self.system_id)
        self.docker("run", "-d", "--name", self.proxy, "--network", self.tag, "--network-alias", "proxy", "--entrypoint", "python", "-v", f"{self.directory}:/lab:ro", "-v", f"{ROOT}:/repo:ro", HA_IMAGE, "/lab/proxy.py")
        self.containers.append(self.proxy)
        self.stop_bridge()
        self.options.update(hub_url="https://proxy:8443/beszel", hub_ca_file="/ssl/proxy.crt")
        self.save_options()
        self.start_bridge()
        self.wait("remote HTTPS base-path Hub with private CA", self.metric_ready)
        self.data_fault_checks()
        self.expanded_metric_checks()
        # Require a new successful sync after restart, not a restored HA value.
        self.wait("remote Hub performs a successful sync", lambda: '"hub_connected":true' in self.docker("exec", self.bridge, "/usr/local/bin/bridge", "status").replace(" ", ""))
        self.assert_identities_preserved(identities)
        assert self.entity("cpu_usage")["attributes"]["friendly_name"] == "My lab CPU"
        self.checks.append("Hub address change and Bridge restart preserve identity")
        self.docker("stop", self.proxy)
        self.wait("Hub outage marks real HA metrics unavailable", self.metric_unavailable)
        self.docker("exec", self.bridge, "/usr/local/bin/bridge", "health")
        self.checks.append("Bridge liveness stays healthy during Hub outage")
        self.docker("start", self.proxy)
        self.wait("Hub recovery restores existing entities", self.metric_ready)
        (self.directory / "proxy-fault.json").write_text(json.dumps({"stale": True}))
        self.wait("old source sample expires despite successful polls", self.metric_unavailable)
        (self.directory / "proxy-fault.json").unlink()
        self.wait("fresh source sample restores telemetry", self.metric_ready)
        before = json.loads(self.docker("exec", self.bridge, "/usr/local/bin/bridge", "status"))["last_sync"]
        self.docker("restart", self.broker)
        def reconnected():
            status = json.loads(self.docker("exec", self.bridge, "/usr/local/bin/bridge", "status"))
            return status.get("mqtt_connected") and status.get("last_sync") != before and self.metric_ready()
        self.wait("broker restart recovers entity state", reconnected, timeout=120)
        self.docker("restart", self.ha)
        self.ha_url = "http://127.0.0.1:" + self.port(self.ha, "8123/tcp")
        self.wait("HA restart rediscovers existing entities", self.metric_ready, timeout=180)
        self.assert_identities_preserved(identities)
        self.snapshot(dashboard=True)
        self.checks.append("both standard-card dashboard templates imported with actual entity IDs")
        if not self.args.keep:
            self.stop_bridge()
            self.options["cleanup"] = True
            self.save_options()
            self.start_bridge()
            self.wait("cleanup removes owned HA entities", lambda: len(self.snapshot()["entities"]) == 0)

    def prepare_screenshots(self):
        """Leave a verified, fixture-backed UI for real browser screenshots."""
        self.hub_api("/api/collections/systems/records/" + self.system_id,
                     {"name": "Home server"}, self.admin_token, "PATCH")
        (self.directory / "proxy-fault.json").write_text(json.dumps({"expanded": True}))
        self.stop_bridge()
        self.start_bridge()
        smart = self.component_key("smart", "smt000000000001", "status")
        self.wait("screenshot SMART overview ready", lambda: self.entity(smart) and self.entity(smart)["state"] == "PASSED", timeout=120)
        self.wait("screenshot disk aggregate ready", lambda: self.entity("filesystems_combined_usage") and self.entity("filesystems_combined_usage")["state"] == "14")
        result = json.loads(self.docker("exec", self.ha, "python", "/repo/scripts/bridge-ha-verify.py", "--storage-dashboard", "--reset-customizations"))
        assert any(entity["unique_id"].endswith("_uptime_human") for entity in result["entities"])
        self.checks.append("storage dashboard imports with actual fixture-backed entity IDs")

    def close(self):
        if self.args.keep:
            print(json.dumps({"ha_url": self.ha_url, "dashboard": self.ha_url + "/beszel-bridge-lab/servers", "directory": str(self.directory), "network": self.tag, "containers": self.containers}, indent=2), flush=True)
            return
        for container in reversed(self.containers):
            subprocess.run(["docker", "rm", "-fv", container], capture_output=True)
        if self.network_created:
            subprocess.run(["docker", "network", "rm", self.tag], capture_output=True)
        self.remove_test_directory()

    def remove_test_directory(self):
        try:
            shutil.rmtree(self.directory)
        except PermissionError:
            # HA creates private, root-owned directories on Linux bind mounts.
            # Remove only this disposable lab's contents inside a container.
            self.docker("run", "--rm", "--entrypoint", "python", "-v",
                        f"{self.directory}:/lab", HA_IMAGE, "-c",
                        "import pathlib,shutil\n"
                        "for path in pathlib.Path('/lab').iterdir():\n"
                        "    if path.is_symlink() or not path.is_dir(): path.unlink()\n"
                        "    else: shutil.rmtree(path)\n")
            self.directory.rmdir()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", default="beszel-bridge-dev:local")
    parser.add_argument("--hub-image", default="beszel-bridge-hub-dev:local")
    parser.add_argument("--hub-version", default=(ROOT / "beszel_hub/beszel_version").read_text().strip())
    parser.add_argument("--keep", action="store_true")
    parser.add_argument("--build", action="store_true", help="Build the Bridge and a Hub app pinned to --hub-version first")
    parser.add_argument("--report", type=Path)
    parser.add_argument("--capture-fixtures", type=Path, help="Save sanitized readonly API responses in a new version directory")
    parser.add_argument("--prepare-screenshots", action="store_true", help="With --keep, prepare expanded example data and the storage dashboard for browser screenshots")
    args = parser.parse_args()
    if not re.fullmatch(r"[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?", args.hub_version):
        parser.error("Expected a Hub release version such as 0.21.1")
    if args.prepare_screenshots and not args.keep:
        parser.error("--prepare-screenshots requires --keep")
    lab = Lab(args)
    try:
        if args.build:
            lab.build()
        lab.prepare()
        lab.setup_ha()
        if args.keep:
            print("Browser test URL: " + lab.ha_url, flush=True)
        lab.exercise()
        if args.prepare_screenshots:
            lab.prepare_screenshots()
        report = {"app_version": lab.docker("run", "--rm", "--entrypoint", "/usr/local/bin/bridge", args.image, "version"),
                  "hub_version": args.hub_version, "ha_image": HA_IMAGE,
                  "tested_at_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
                  "bridge_image_id": lab.docker("image", "inspect", args.image, "--format", "{{.Id}}"),
                  "bridge_architecture": lab.docker("image", "inspect", args.image, "--format", "{{.Architecture}}"),
                  "hub_image_id": lab.docker("image", "inspect", args.hub_image, "--format", "{{.Id}}"),
                  "checks": lab.checks,
                  "supervisor_validated": False, "remote_machine": "isolated HTTPS proxy topology; physical remote deployment not tested"}
        if args.keep:
            report["retained_environment"] = {"ha_url": lab.ha_url, "directory": str(lab.directory), "network": lab.tag, "containers": lab.containers}
            report["screenshot_data"] = "synthetic hardware fixtures" if args.prepare_screenshots else "live Agent readings"
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_text(json.dumps(report, indent=2) + "\n")
        print(f"PASS {len(lab.checks)} real-stack checks", flush=True)
    except Exception as error:
        # Bridge logs deliberately exclude credentials and raw Hub values. Keep
        # a small diagnostic report before disposable containers are removed.
        if getattr(lab, "bridge", None):
            log = subprocess.run(["docker", "logs", "--tail", "30", lab.bridge], text=True,
                                 stdout=subprocess.PIPE, stderr=subprocess.STDOUT).stdout
            print("Bridge diagnostics:\n" + log, flush=True)
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_text(json.dumps({"hub_version": args.hub_version, "checks": lab.checks,
                                               "failure_type": type(error).__name__}, indent=2) + "\n")
        raise
    finally:
        lab.close()


if __name__ == "__main__":
    main()

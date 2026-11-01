#!/usr/bin/env python3
"""Test Agent key sources with real Bashio and SSH authentication.

Uses the standard Agent image with each variant's actual startup script mounted
in turn. This checks authentication without requiring GPU or S.M.A.R.T. devices.
Docker, Python 3, ssh and ssh-keygen are required.
"""

import argparse
import json
import os
from pathlib import Path
import subprocess
import tempfile
import time


ROOT = Path(__file__).resolve().parent.parent


def run(*args, **kwargs):
    return subprocess.run(args, check=True, capture_output=True, **kwargs)


def ssh_authenticates(port, key):
    command = [
        "ssh", "-F", "/dev/null", "-v", "-T", "-i", str(key), "-p", str(port),
        "-o", "BatchMode=yes", "-o", "IdentitiesOnly=yes",
        "-o", "IdentityAgent=none", "-o", "StrictHostKeyChecking=no",
        "-o", "UserKnownHostsFile=/dev/null", "-o", "GlobalKnownHostsFile=/dev/null",
        "-o", "ConnectTimeout=2", "root@127.0.0.1", "true",
    ]
    try:
        result = subprocess.run(command, input=b"", capture_output=True, timeout=3)
        diagnostic = result.stderr
    except subprocess.TimeoutExpired as exc:
        diagnostic = exc.stderr or b""
    # The Agent speaks its own metrics protocol after SSH authentication, so an
    # ordinary shell command's exit status is not an authentication result.
    if b"Authenticated to " in diagnostic:
        return True
    if b"Permission denied (publickey)" in diagnostic:
        return False
    raise AssertionError(diagnostic.decode(errors="replace"))


def check_case(image, app, work, network, options, accepted=(), rejected=(), error=None,
               environment=None, user=None):
    options_file = work / "supervisor" / "addons" / "self" / "options" / "config"
    config = {
        "hub_url": "http://127.0.0.1:1", "token": "key-regression-test",
        "environment_vars": [], "custom_volumes": [], **options,
    }
    options_file.write_text(json.dumps({"result": "ok", "data": config}))
    command = [
        "docker", "run", "-d", "--entrypoint", "/usr/bin/bashio",
        "--network", network,
        "-e", "SUPERVISOR_TOKEN=key-regression-test",
        "-e", "DOCKER_HOST=", "-e", "SKIP_GPU=true",
        "-e", "PACKAGE_UPDATES_INTERVAL=0", "-e", "DATA_DIR=/tmp/beszel-agent",
        "-v", f"{ROOT / app / 'run.sh'}:/run.sh:ro",
        "-v", f"{work / 'ssl'}:/ssl:ro",
        "-p", "127.0.0.1::45876",
    ]
    for name, value in (environment or {}).items():
        command.extend(["-e", f"{name}={value}"])
    if user:
        command.extend(["--user", user])
    command.extend([image, "/run.sh"])
    container = run(*command).stdout.decode().strip()
    try:
        deadline = time.monotonic() + 15
        logs = ""
        while time.monotonic() < deadline:
            state = json.loads(run("docker", "inspect", "--format", "{{json .State}}",
                                   container).stdout)
            # Fetch logs after the state snapshot. The Agent may exit between
            # these calls, so a stopped state must use its final diagnostic.
            output = run("docker", "logs", container)
            logs = (output.stdout + output.stderr).decode(errors="replace")
            if not state["Running"]:
                if error and state["ExitCode"] != 0 and error in logs:
                    return
                raise AssertionError(f"Agent exited unexpectedly: {state}\n{logs}")
            if error is None and "Starting SSH server" in logs:
                break
            time.sleep(0.1)
        else:
            raise AssertionError(f"Agent did not reach expected startup state\n{logs}")

        address = run("docker", "port", container, "45876/tcp").stdout.decode().strip()
        port = int(address.rsplit(":", 1)[1])
        for key in accepted:
            assert ssh_authenticates(port, key), f"Authorized key was rejected: {key.name}"
        for key in rejected:
            assert not ssh_authenticates(port, key), f"Unlisted key was accepted: {key.name}"
    finally:
        run("docker", "rm", "-fv", container)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("apps", nargs="*", help="Agent directories (default: all five)")
    parser.add_argument("--image", help="Use an existing Agent app image instead of building")
    args = parser.parse_args()
    apps = args.apps or sorted(p.name for p in ROOT.glob("beszel_agent*") if p.is_dir())
    for app in apps:
        if app not in [p.name for p in ROOT.glob("beszel_agent*") if p.is_dir()]:
            parser.error(f"Unknown Agent app: {app}")

    image = args.image or f"beszel-agent-key-test:{os.getpid()}"
    network = f"beszel-agent-key-test-{os.getpid()}"
    mock = None
    network_created = False
    built = False
    passed = 0
    failures = []
    try:
        if not args.image:
            print("Building the standard Agent image for authentication tests...", flush=True)
            subprocess.run([
                "docker", "build", "--build-arg", "BUILD_FROM=ghcr.io/home-assistant/base:latest",
                "-t", image, str(ROOT / "beszel_agent"),
            ], check=True)
            built = True

        with tempfile.TemporaryDirectory(prefix="beszel-agent-keys-") as temp:
            work = Path(temp)
            work.chmod(0o755)
            supervisor = work / "supervisor"
            config_dir = supervisor / "addons" / "self" / "options"
            config_dir.mkdir(parents=True)
            (config_dir.parent / "info").write_text(json.dumps({
                "result": "ok", "data": {"watchdog": False},
            }))
            run("docker", "network", "create", network)
            network_created = True
            mock = run(
                "docker", "run", "-d", "--network", network,
                "--network-alias", "supervisor", "--entrypoint", "/bin/bash",
                "-v", f"{supervisor}:/supervisor-api:ro", image,
                "-c", "exec httpd -f -p 80 -h /supervisor-api",
            ).stdout.decode().strip()
            ssl = work / "ssl"
            ssl.mkdir(mode=0o755)
            keys = [work / f"hub-{number}" for number in range(3)]
            for key in keys:
                run("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", str(key))
            public = [Path(f"{key}.pub").read_text().strip() for key in keys]
            multiple = f"  # Two independent Hubs\r\n\r\n  {public[0]}\r\n{public[1]}\r\n"
            (ssl / "hub keys").write_text(multiple)
            (ssl / "empty").touch()
            (ssl / "comments").write_text("\n  # No keys\r\n\t\n")
            (ssl / "invalid").write_text("this-is-not-an-ssh-public-key\n")
            (ssl / "unreadable").write_text(multiple)
            (ssl / "unreadable").chmod(0)
            key_file = "/ssl/hub keys"
            both = {"accepted": keys[:2], "rejected": keys[2:]}

            cases = [
                ("legacy single key", {"key": public[0]},
                 {"accepted": keys[:1], "rejected": keys[1:]}),
                ("inline keys, comments and CRLF", {"key": multiple}, both),
                ("file without a key option", {"key_file": key_file}, both),
                ("file overrides inline keys and aliases", {
                    "key": public[2], "key_file": key_file,
                    "environment_vars": [{"name": "BESZEL_AGENT_KEY", "value": public[2]}],
                }, both),
                ("KEY environment override", {
                    "key": public[2],
                    "environment_vars": [{"name": "KEY", "value": multiple}],
                }, both),
                ("KEY_FILE environment overrides both options", {
                    "key": public[2], "key_file": "/ssl/missing",
                    "environment_vars": [{"name": "KEY_FILE", "value": key_file}],
                }, both),
                ("prefixed KEY environment override", {
                    "key": public[2],
                    "environment_vars": [{"name": "BESZEL_AGENT_KEY", "value": multiple}],
                }, both),
                ("prefixed KEY_FILE environment override", {
                    "key": public[2], "key_file": "/ssl/missing",
                    "environment_vars": [{"name": "BESZEL_AGENT_KEY_FILE", "value": key_file}],
                }, both),
                ("inherited KEY_FILE", {}, {**both, "environment": {"KEY_FILE": key_file}}),
                ("BESZEL_KEY test override", {}, {
                    "accepted": keys[:1], "rejected": keys[1:],
                    "environment": {"BESZEL_KEY": public[0]},
                }),
                ("BESZEL_KEY_FILE test override", {}, {
                    **both, "environment": {"BESZEL_KEY_FILE": key_file},
                }),
                ("missing key sources", {}, {"error": "either 'key' or 'key_file' is required"}),
                ("comment-only inline keys", {"key": " \n # No keys\n"},
                 {"error": "either 'key' or 'key_file' is required"}),
                ("missing file cannot fall back to a key", {
                    "key": public[0], "key_file": "/ssl/missing",
                }, {"error": "must point to a readable regular file"}),
                ("directory is not a key file", {"key_file": "/ssl"},
                 {"error": "must point to a readable regular file"}),
                ("unreadable key file", {"key_file": "/ssl/unreadable"}, {
                    "error": "must point to a readable regular file", "user": "65534:65534",
                }),
                ("empty key file", {"key_file": "/ssl/empty"},
                 {"error": "contains no public keys"}),
                ("comment-only key file", {"key_file": "/ssl/comments"},
                 {"error": "contains no public keys"}),
                ("malformed key file", {"key_file": "/ssl/invalid"},
                 {"error": "Failed to load public keys"}),
            ]
            for app in apps:
                for name, options, expectations in cases:
                    try:
                        check_case(image, app, work, network, options, **expectations)
                    except (AssertionError, subprocess.SubprocessError, ValueError) as exc:
                        failures.append(f"{app}: {name}: {exc}")
                        print(f"FAIL {app}: {name}: {exc}", flush=True)
                    else:
                        passed += 1
                        print(f"PASS {app}: {name}", flush=True)
    finally:
        if mock:
            run("docker", "rm", "-fv", mock)
        if network_created:
            run("docker", "network", "rm", network)
        if built:
            run("docker", "rmi", image)

    print(f"\n{passed} key-source cases passed, {len(failures)} failed", flush=True)
    return bool(failures)


if __name__ == "__main__":
    raise SystemExit(main())

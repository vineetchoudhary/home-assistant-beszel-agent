# Testing

Run commands from the repository root. Container tests need Docker, Git and Python 3 with PyYAML. Bridge checks also need Go 1.26+ or the script's Docker fallback. Bridge end-to-end tests require OpenSSL.

## App checks

| Check | Command |
| --- | --- |
| Configuration, versions and lint | `./scripts/test-apps.sh --static` |
| Build and startup for every app | `./scripts/test-apps.sh` |
| Build and startup for one app | `./scripts/test-apps.sh beszel_hub` |
| Mock Supervisor configuration loading | `./scripts/test-apps.sh --supervisor` |
| Agent key sources and SSH authorization | `python3 scripts/test-agent-keys.py` |
| Hub registration and readings from every Agent | `./scripts/test-e2e.sh --all-agents` |

For `test-apps.sh`, add `--no-smoke` to build without running containers, or `--keep` to retain built images. Use `--supervisor` for startup-script changes.

For `test-e2e.sh`, omit `--all-agents` to test only the standard Agent. `--keep` prints the Hub URL, login and cleanup commands. Run `./scripts/test-e2e.sh --clean` to remove retained test resources.

`test-agent-keys.py` builds the standard Agent image and runs every variant's startup script with real Bashio and SSH keys. It checks inline keys, file precedence, environment overrides and invalid files, including rejection of unlisted keys. Pass Agent directory names to select variants or `--image IMAGE` to reuse a built app image. These tests do not exercise hardware collectors.

## Bridge entities and dashboards

```bash
./scripts/test-bridge.sh
python3 scripts/test-bridge-e2e.py --build --hub-version 0.20.0
python3 scripts/test-bridge-e2e.py --build --hub-version 0.21.0
python3 scripts/test-bridge-e2e.py --build --hub-version 0.21.1
```

The first command runs Go tests, vet, formatting and amd64/arm64 compilation. The end-to-end commands run Hub, Agent, MQTT and Home Assistant containers to check entities, dashboards, malformed fields, newer version metadata, outages, restarts and cleanup.

`--build` builds the Bridge and the selected Hub version without changing repository pins. If `--hub-version` is omitted, the Hub's pin is used. Any release version can be tested. The [compatibility file](../beszel_bridge/compatibility.json) records coverage and does not restrict runtime versions.

Add `--report .artifacts/bridge/report.json` to save results. `--keep` retains the environment and prints its URLs, container names, network and temporary directory. Remove those resources when finished. Otherwise cleanup is automatic.

Add `--capture-fixtures .artifacts/bridge/fixtures/0.21.1` to capture sanitized readonly API responses in a new directory. Review them before adding fixtures and recording tested versions.

### Bridge fixtures

[Versioned fixtures](../beszel_bridge/testdata/README.md) contain `info.json` for `/api/beszel/info`, `systems.json` for inventory and `system_stats.json` for the latest `1m` sample.

Replace identifying fields consistently, exclude credentials and preserve response shapes, metric values and timestamps. Freshness uses `system_stats.created`. Zero temperature does not prove sensor support, and idle samples may omit `b` while retaining zero `ni` rates. Keep malformed, stale and permission-limited test cases separate from captured responses.

## Run a Hub locally

```bash
docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/base:latest -t beszel-hub-test beszel_hub/
docker run --rm -it -p 8090:8090 -v /tmp/beszel-hub-data:/var/lib/beszel-hub beszel-hub-test
```

Open `http://127.0.0.1:8090` and create the first account. Data remains in `/tmp/beszel-hub-data` after the container stops.

## Bridge diagnostics

Inside a running Bridge container:

```sh
/usr/local/bin/bridge status
/usr/local/bin/bridge check
/usr/local/bin/bridge health
```

`status` shows connection readiness, `check` lists selected systems and sample freshness without publishing, and `health` checks the process. For a retained test environment, use `docker exec` with the Bridge container name printed by `--keep` to run these commands. Remove identifying information before sharing the output.

## Continuous integration

The [validation workflow](../.github/workflows/validate.yml) runs static and Agent SSH authentication checks on relevant changes. Its manual full run adds builds for every app, startup checks, mock Supervisor and end-to-end tests. Containers do not cover Supervisor installation or physical hardware. Complete the [HA OS checklist](MANUAL_CHECKLIST.md) before release.

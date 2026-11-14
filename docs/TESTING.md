# Testing

Run commands from the repository root. Container tests need Docker, Git and Python 3 with PyYAML. Bridge checks also need Go 1.26+ or the script's Docker fallback. Bridge end-to-end tests require OpenSSL.

## App checks

| Check | Command |
| --- | --- |
| Configuration, versions and lint | `./scripts/test-apps.sh --static` |
| Build and startup for every app | `./scripts/test-apps.sh` |
| Build and startup for one app | `./scripts/test-apps.sh beszel_hub` |
| Mock Supervisor configuration loading | `./scripts/test-apps.sh --supervisor` |
| Container test runner regressions | `python3 -B scripts/test-ci-runners.py` |
| Release scope, versions and publication regressions | `python3 -B scripts/test-release-workflow.py` |
| Agent key sources and SSH authorization | `python3 scripts/test-agent-keys.py` |
| Hub registration and readings from every Agent | `./scripts/test-e2e.sh --all-agents` |

For `test-apps.sh`, add `--no-smoke` to build without running containers, or `--keep` to retain built images. Use `--supervisor` for startup-script changes.

For `test-e2e.sh`, omit `--all-agents` to test only the standard Agent. `--keep` prints the Hub URL, login and cleanup commands. Run `./scripts/test-e2e.sh --clean` to remove retained test resources.

`test-release-workflow.py` checks manual scope selection, independent Bridge versions, scheduled/tag behavior, version preservation and release metadata. It executes the workflow's actual selection, version-update and publication scripts against disposable app files with simulated build, Git and GitHub commands; it does not publish images, push commits or create releases.

`test-agent-keys.py` builds the standard Agent image and runs every variant's startup script with real Bashio and SSH keys. It checks inline keys, file precedence, environment overrides and invalid files, including rejection of unlisted keys. Pass Agent directory names to select variants or `--image IMAGE` to reuse a built app image. These tests do not exercise hardware collectors.

## Bridge entities and dashboards

```bash
./scripts/test-bridge.sh
python3 scripts/test-bridge-e2e.py --build --hub-version 0.20.0
python3 scripts/test-bridge-e2e.py --build --hub-version 0.21.0
python3 scripts/test-bridge-e2e.py --build --hub-version 0.21.1
python3 scripts/test-bridge-e2e.py --build --hub-version 0.21.2
```

The first command runs Go tests, vet, formatting and amd64/arm64 compilation. The end-to-end commands run Hub, Agent, MQTT and Home Assistant containers to check entities, dashboards, malformed fields, newer version metadata, outages, restarts and cleanup.

The baseline Agent explicitly disables temperature collection with `SENSORS=` and the test verifies that its source sample has no temperature sensors before checking discovery. This keeps the missing-capability check independent of sensors exposed by the Docker host. Expanded proxy fixtures separately verify temperature discovery and a valid zero reading; SMART and GPU temperature entities are outside the baseline sensor check.

Expanded tests cover disk aggregation/exclusions, SMART status/precision, large counters, hardware maps, containers/services, pool details, probe failures, package classification, account-owned alerts, discovery defaults and component-removal grace. The [expanded fixture](../beszel_bridge/testdata/expanded/monitoring.json) is synthetic and does not claim physical-hardware validation. Real-stack tests inject those responses through the authenticated HTTPS test proxy and verify the actual Bridge, MQTT integration, registry and states, including enabling disabled details.

Readable-metric tests cover decimal byte and rate scaling, rounding across unit boundaries, zero/missing/invalid readings, exact raw counters, family and narrow legacy selection, default discovery and shared freshness. The real-stack test verifies readable companions alongside numeric originals, units changing after new samples, percentage precision and readable attributes in grouped summaries.

Grouping tests check the disabled default, one entity per SMART disk/container/service, complete vendor properties, exact raw integers, valid versus unsupported zeros, filtering, stale/ambiguous rates and migration only after a complete fresh replacement. The real-stack test enables grouping, verifies summary attributes and removal of replaced detail discovery in Home Assistant, checks summary IDs and unrelated customizations, then restores individual mode.

Unit tests exercise bulk enabling of 512 integration-disabled entities, exact MQTT ownership, metric/system filters, API failure handling and preservation of explicit user/device disables. The real-stack test forwards a simulated Supervisor Core WebSocket endpoint to Home Assistant and checks `enable_all_entities` against existing registry entries, including unchanged IDs, names and user disables. This does not validate permission provisioning by an installed Supervisor.

History-retention tests check the disabled default, day bounds, exact renamed-entity targeting, empty-scope and wildcard rejection, persisted daily scheduling, changed limits and retry after API failure. The real-stack test seeds aged state rows only in its disposable Recorder database and verifies cleanup at multiple configured retention periods through the real `recorder.purge_entities` action. Recent owned rows and older unrelated rows must remain, with entity identities and enablement unchanged. The database uses a native anonymous Docker volume to preserve SQLite/WAL coordination on macOS shared filesystems. Container cleanup removes that volume. `bridge-ha-history.py` is test-only and refuses to run without the lab's authentication marker.

`--build` builds the Bridge and the selected Hub version without changing repository pins. If `--hub-version` is omitted, the Hub's pin is used. Any release version can be tested. The [compatibility file](../beszel_bridge/compatibility.json) records coverage and does not restrict runtime versions.

Add `--report .artifacts/bridge/report.json` to save results. `--keep` retains the environment and prints its URLs, container names, network and temporary directory. Remove those resources when finished. Otherwise cleanup is automatic.

Add `--capture-fixtures .artifacts/bridge/fixtures/0.21.2` to capture sanitized readonly API responses in a new directory. Review them before adding fixtures and recording tested versions.

### Setup screenshots

Run `python3 scripts/test-bridge-e2e.py --build --keep --prepare-screenshots --report .artifacts/bridge/screenshots/session.json` to complete the real-stack checks and leave a local Home Assistant instance with controlled example disk/SMART data. The report records its URL, container names and temporary directory without credentials. Use the test login defined in `setup_ha` to verify the UI and capture screenshots. The screenshot preparation also imports all three dashboard templates using actual test entity IDs.

Capture real browser screens for MQTT devices, enabling a detailed entity, SMART attributes and desktop/mobile dashboards. Keep passwords, tokens and production data out of images. Identify the hardware readings as synthetic examples. Close the browser tab and remove only the retained test containers, network and temporary directory after capture. Container screenshots do not validate Supervisor installation or physical SMART hardware.

### HA Apps Preview

The README's [HA Apps Preview](screenshots/beszel-apps.jpg) is rendered from the names, descriptions and icons in this repository using [ha-apps-preview.py](../scripts/ha-apps-preview.py). It follows Home Assistant's order: Agent, AMD, Intel, NVIDIA, S.M.A.R.T., Bridge and Hub. It shows four cards above three left-aligned cards, with no installation or host-availability badges.

```bash
python3 scripts/ha-apps-preview.py
python3 -m http.server 18765 --bind 127.0.0.1 --directory .artifacts/bridge/screenshots/ha-apps-preview
```

Open `http://127.0.0.1:18765/` in a browser with an 1800 × 400 viewport. Capture the `main` element at 1800 × 376 pixels as `docs/screenshots/beszel-apps.jpg`, then close the tab and stop the server. Update the script's app order when adding another app.

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

The [validation workflow](../.github/workflows/validate.yml) runs static and Agent SSH authentication checks on relevant changes. Its manual full run adds builds for every app, startup checks, mock Supervisor, Agent/Hub end-to-end tests and a Bridge/Home Assistant matrix for Hub 0.20.0, 0.21.0, 0.21.1 and 0.21.2. Containers do not cover Supervisor installation or physical hardware. Use the [HA OS checklist](MANUAL_CHECKLIST.md) to record those checks separately.

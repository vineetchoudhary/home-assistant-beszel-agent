# Beszel Bridge readiness

Recorded **16 October 2026**: app candidate **0.21.0.1**, bundled Beszel **0.21.0**. **Unreleased development preview.** No public Bridge image or production installation change was recorded. `release_ready: false` blocks full/Bridge publishing.

## Verified behavior

| Area | Recorded evidence and boundary |
| --- | --- |
| Hub compatibility | Real Hub/Agent 0.20.0 and 0.21.0 with readonly sharing |
| Entities/lifecycle | 21 real HA Core 2026.10.1 checks per Hub version with Mosquitto 2.0.22: units, missing metrics, freshness, outages, restarts and cleanup |
| Customization | Entity IDs/names survive rediscovery and restarts on Hub 0.21.0 |
| Networking | Internal Docker hostname without published API port, HTTPS base path and trusted private CA proxy |
| Builds | Native arm64 and emulated amd64 smoke tests, app/binary version 0.21.0.1, Go tests/vet/formatting and both Linux targets |
| Shared tooling | All seven apps pass static/build/startup/mock-Supervisor checks, all five Agents register and supply metrics |
| Dashboards | Both templates imported in real HA, desktop/mobile, missing temperature and unavailable states checked |

## 0.21.0.1 audit

- Required Hub options prompt configuration. Image metadata reports the app's version.
- Preview publishing fails before version mutation/login. Unsupported Hub versions fail before test setup.
- The dated documentation audit checked 129 references across 28 Markdown files, 21 desktop images at 1920 × 1080 and three phone images at 430 × 932. Raw pixels outside hints/token masks match. Thirteen older Supervisor captures await refresh.

[Reproduce tests](TESTING.md#bridge-entities-and-dashboards). Recorded sanitized `audit-*` logs/reports stay under ignored `.artifacts/bridge/`. [Fixtures](../beszel_bridge/testdata/README.md) remain test inputs. See [screenshots](SCREENSHOTS.md) for capture provenance.

## Remaining release gates

Complete the [manual checklist](MANUAL_CHECKLIST.md): real Supervisor for both Hub locations, both MQTT sources/TLS, read-only certificates, watchdog, backups and scoped cleanup. Also verify native amd64, physical GPU/disk claims, production session/password changes, coexistence and longer-running history. Broader HA versions remain unverified.

Container/mocked coverage leaves those checks open. Follow the [release gate](RELEASE.md#bridge-release-gate), retain `stage: experimental` and verify matching public image tags/pull/install after acceptance.

## Removal and rollback

Run [scoped cleanup](../beszel_bridge/DOCS.md#clean-up-or-replace-a-hubbroker) before uninstalling, switching brokers or replacing/rotating the Hub key. If the old broker is offline, preserve its settings and `/data` for retry.

Rollback restores the previous image, compatible options and `/data` manifest. Preserve the Hub's key/database when restoring it. Dashboards remain user-owned configurations.

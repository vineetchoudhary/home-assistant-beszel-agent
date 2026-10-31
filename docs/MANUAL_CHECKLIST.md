# Manual HA OS acceptance

Candidate: app **0.21.0.1**, bundled Beszel **0.21.0**. The Bridge is unpublished. Use a disposable or backed-up HA OS/Supervisor system and record dated results in [readiness](BRIDGE_READINESS.md).

## HA OS acceptance

- [ ] Install using the [development instructions](../beszel_bridge/DOCS.md#development-installation-before-images-are-published), with normal protection enabled. Record HA OS, Supervisor, Core and architecture versions.
- [ ] Connect a local Hub by its internal hostname on `8090`, with its host port enabled/disabled. Verify readonly sharing, base paths and Ingress independence.
- [ ] Connect a physical remote Hub over LAN and HTTPS/base path. Verify private CA access through read-only `/ssl` and reject untrusted certificates.
- [ ] Test Supervisor/manual MQTT, TLS, custom discovery prefixes, rejected credentials and delayed startup. Match HA's broker/prefix.
- [ ] Restart HA, Hub, broker and Bridge in different orders. Verify fresh recovery, no duplicates and no watchdog loops during outages.
- [ ] Verify device grouping, units, source timestamps, valid zeros, missing metrics, paused/down/unknown status and system/metric filters.
- [ ] Rename servers/entities and change the same Hub's URL. Restore options and `/data`, test replacement/exclusion/grace/cleanup and preserve unrelated topics.
- [ ] Test no systems, reduced permissions, session expiry and password changes. Check recovery and longer-running history/coexistence.
- [ ] Import both dashboards on desktop/mobile with actual customized IDs, missing metrics and unavailable readings.
- [ ] Verify native amd64/aarch64 where claimed. Test GPU/S.M.A.R.T. on physical hardware. Record emulation separately.

Keep private logs and raw captures under ignored `.artifacts/bridge/`. Compare results against the [full acceptance matrix](HA_ENTITIES_DASHBOARDS_PLAN.md#required-release-validation).

## HA OS screenshots

Capture real UI at **1920 × 1080** with consistent scale, readable text and optional red rectangles. Preserve UI text, mask secrets and export lossless WebP. Replace existing files in place, including the historical `agenet` spelling. Follow [capture notes](SCREENSHOTS.md).

Paths below are relative to `docs/screenshots/`:

| Done | Replace | Capture/highlight |
| --- | --- | --- |
| [ ] | `2-ha-apps.webp` | Apps, Install app |
| [ ] | `3-ha-apps-store-repo.webp` | Store menu, Repositories |
| [ ] | `4-ha-apps-add-repo.webp` | Repository URL and Add |
| [ ] | `5-ha-apps-repo-added.webp` | Added repository |
| [ ] | `5-beszel-agent-home-assistant.webp` | Standard Agent card |
| [ ] | `5-beszel-agent-smart-home-assistant.webp` | S.M.A.R.T. Agent card |
| [ ] | `6-ha-beszel-agenet-install.webp` | Install |
| [ ] | `7-ha-beszel-agenet-config.webp` | Configuration tab |
| [ ] | `8-ha-beszel-agenet-config-fill.webp` | Required options, masked secrets |
| [ ] | `9-ha-beszel-agenet-custom-volumes.webp` | Volume-check entries |
| [ ] | `10-ha-beszel-agenet-install-success.webp` | Stopped app, Start |
| [ ] | `11-ha-beszel-agenet-start.webp` | Running app/status |
| [ ] | `12-ha-beszel-agenet-protection-mode.webp` | Protection mode |

`1-ha-settings.webp`, `8-beszel-config.webp` and `13-beszel-dashboard.webp` are refreshed 1080p captures. Refresh `11-beszel-agent-smart-log.webp` and `13-beszel-smart.webp` during physical disk checks.

Add these under `docs/screenshots/beszel-bridge/` and link them beside the matching [Bridge steps](../beszel_bridge/DOCS.md):

| Done | New file | Capture/highlight |
| --- | --- | --- |
| [ ] | `haos-bridge-install.webp` | Bridge Install |
| [ ] | `haos-bridge-local-options.webp` | Internal Hub URL, readonly account, Supervisor MQTT |
| [ ] | `haos-bridge-remote-options.webp` | Remote URL/base path and private CA |
| [ ] | `haos-bridge-manual-mqtt.webp` | Manual broker, TLS and CA |
| [ ] | `haos-bridge-running.webp` | Running status and watchdog |
| [ ] | `haos-bridge-log.webp` | Bridge synchronized, system count/version |

Existing MQTT, account-sharing, entity and dashboard captures remain usable. Update capture versions/dates, verify guide links and inspect full-size images.

## Release handoff

- [ ] Resolve acceptance failures, record evidence and set [`release_ready`](../beszel_bridge/compatibility.json) to `true` only in the reviewed release change. Keep `stage: experimental` during preview.
- [ ] Follow the [release gate](RELEASE.md#bridge-release-gate), align candidate versions, publish both architecture images and verify public pull/install. Full/Bridge publishing stays blocked until acceptance. Existing-app rebuilds remain available.

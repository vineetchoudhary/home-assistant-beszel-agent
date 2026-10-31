# Beszel Bridge plan

Record dated **16 October 2026**. Implementation and container/HA Core validation are recorded. **Next: P04 real Supervisor acceptance, then P06 release.**

The original baseline was Hub `0.20.0` and app `0.20.0.1`. The recorded candidate is app `0.21.0.1` across seven apps, with Beszel `0.21.0` in the six bundled apps. Check current pins and [readiness](BRIDGE_READINESS.md) before resuming.

## Scope

One independent Bridge connects one local HA Hub app or remote LAN/VPN/HTTPS Hub to HA's MQTT integration. Support amd64/aarch64, Supervisor/manual brokers, system/metric selection, redacted diagnostics and standard-card [dashboards](dashboards/beszel/README.md).

Defer container, GPU, S.M.A.R.T. and extra filesystem entities, multiple Hubs, historical backfill, automatic dashboard provisioning and control actions. A [community integration](https://github.com/cgfm/Beszel-HASS) offers a non-MQTT route. Check coexistence before exposing duplicate metrics.

## Design decisions

- Use a normal password-authenticated `readonly` account with shared systems. Unsupported auth/API capabilities require a documented scope or compatibility change, never superuser or direct-database fallback.
- Use explicit Hub URLs, preserving base paths. Local access must work through the installed app's internal hostname on `8090` with its host port disabled. Ingress authentication is independent.
- Verify TLS and optional read-only `/ssl` CAs. Protect credentials in app options/backups, keep passwords out of process arguments and redact tokens/query secrets from diagnostics.
- Use typed Go adapters, bounded requests and retry backoff. Compatibility requires real versioned fixtures. Recorded support is Hub `0.20.0`/`0.21.0`, HA Core `2026.10.1` and Paho Go `1.5.1`.
- Use source `1m` sample timestamps for freshness. Retain discovery, publish live states without retention, use finite expiry and wait for fresh data after reconnect. Separate Bridge, Hub and server availability.
- Derive identity from the Hub's public SSH key and stable system IDs. Persist identity/owned topics in `/data`, preserve customization and scope cleanup to owned topics. Complete successful inventories and a five-minute grace period govern inaccessible-system removal.

Configuration, metric units and cleanup behavior are maintained in the [Bridge guide](../beszel_bridge/DOCS.md), [schema policy](../beszel_bridge/compatibility.json) and [architecture](ARCHITECTURE.md).

## Implementation checkpoints

Statuses below are the recorded 16 October results. Container progress leaves Supervisor acceptance open.

| ID | Depends on | Result and remaining work |
| --- | --- | --- |
| P00 | None | Scope and architecture complete. |
| P01 | P00 | [API/auth/fixtures](../beszel_bridge/testdata/) pass on both Hub versions. Supervisor/physical networking acceptance remains in P04. |
| P02 | P01 | [Collector/adapters](../beszel_bridge/metrics.go), unit tests, vet and Linux amd64/arm64 builds pass. |
| P03 | P02 | [Real HA Core lifecycle](TESTING.md#bridge-entities-and-dashboards) passes. Production auth/coexistence checks remain. |
| P04 | P03 | Both images and shared tooling checked. Real Supervisor install, networking, certificates, watchdog and backup remain pending. |
| P05 | P03, P04 | [Dashboards](dashboards/beszel/SETUP.md) checked on desktop/mobile, missing metrics and outages. Both real installations remain pending. |
| P06 | P01–P05 | [Readiness recorded](BRIDGE_READINESS.md). Release withheld until acceptance and matching public images are verified. |

## Required release validation

Record dated pass/fail/not-tested evidence for every row using the [manual checklist](MANUAL_CHECKLIST.md).

| Area | Required checks |
| --- | --- |
| Deployments | Local internal hostname with host port on/off, local base path, physical remote LAN and HTTPS/base path/private CA |
| Versions | Each declared Hub schema, actionable unsupported-version failures and explicit tested HA minimum |
| Authentication | Readonly sharing, no systems, expired/invalid tokens, changed password and reduced permissions |
| MQTT | Supervisor/manual sources, custom discovery prefix, TLS/credentials, broker restart, retained discovery and HA rediscovery |
| Identity | Restarts, server rename, Hub URL change, backup/restore, Hub replacement and integration coexistence |
| Data | Units, source freshness, paused/down/unknown status, valid zeros, missing sensors, pagination and malformed/partial responses |
| Resilience | All restart orders, bounded retries, stale-data unavailability and no watchdog loop during upstream outages |
| Cleanup | Exclusions, complete-inventory grace, owned-topic removal and retry with an offline old broker |
| Dashboards | Actual customized entity IDs, standard cards, desktop/mobile, missing metrics and newly collected history |
| Platforms | Native/emulated evidence kept distinct, both architectures and real Supervisor installations for both Hub locations |
| Repository | Existing-app regressions, workflow coverage, fixtures, matching app/image versions and guide links |

## Resume

1. Inspect the working tree, current compatibility policy and readiness report.
2. Complete P04 and production-auth/history/coexistence checks on real installations. Keep private evidence under ignored `.artifacts/bridge/`.
3. Record unresolved limits before P06. Follow the [release process](RELEASE.md#bridge-release-gate) and verify public images/installations before claiming availability.

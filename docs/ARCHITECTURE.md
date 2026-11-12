# Architecture

Agents collect host metrics, the Hub stores and displays them, and the optional Bridge exposes them to Home Assistant.

```mermaid
flowchart LR
    A[Beszel Agents] --> H[Local or remote Hub]
    H -->|Authenticated API| B[Beszel Bridge]
    B -->|Discovery and readings| M[MQTT broker]
    M --> E[HA devices and dashboards]
```

| App | Runtime | Ports and storage |
| --- | --- | --- |
| Agent | `run.sh` reads bashio options and starts Beszel plus an HTTP health helper | Agent `45876`, watchdog `45877`, app data persists |
| Hub | Beszel server with an internal Ingress proxy | API/UI/watchdog `8090`, Ingress `8099`, database and keys in app data |
| Bridge | Go HTTP collector, typed metric adapters and MQTT publisher | Internal `8099/health` and redacted `8099/status`, identity/manifest under `/data` |

Agent/Hub images use Alpine except NVIDIA, which uses Debian for injected glibc tools and the amd64 NVML build. Intel is amd64-only because `intel_gpu_top` is unavailable on aarch64. See [app variants](APPS.md).

The Bridge reads the Hub using a normal account and decodes known API fields independently. Its [compatibility policy](../beszel_bridge/compatibility.json) records tested versions. New versions are tried automatically. It uses read-only `/ssl` and normal container networking, with no published port or hardware privileges.

The Hub's public SSH key and system IDs identify MQTT devices. Discovery messages are retained, live readings expire using source timestamps, and Watchdog checks the Bridge process separately from Hub/MQTT connectivity.

Bad readings and failed per-server requests affect only their sensors. Incomplete inventories preserve discovery. Connection, authentication and identity failures make Hub readings unavailable while the Bridge keeps retrying.

Expanded adapters read current samples, native 10-minute peaks and independent inventories for SMART, containers, services, storage pools, probes, system details and account-owned alerts. Package inventory uses the Hub's read-only GET endpoint. At most four Hub requests run concurrently. MQTT publication and manifest writes are serialized. Component definitions extend the ownership manifest without changing original IDs or topics.

Fast readings use source timestamps and normal freshness limits. SMART, pool details and package inventory have a separate age limit, while timestamps and readable ages explain old data. Components need five minutes of complete successful absence before discovery removal. Detailed entities are discovered disabled by default and user choices survive rediscovery.

With `enable_all_entities: true`, MQTT discovery enables new entities and the Bridge uses the Supervisor Core WebSocket proxy to enable existing integration-disabled entries. Registry updates match exact owned MQTT unique IDs and respect metric/system filters. Explicit user and device disables are preserved. Core API failures are retried on the next poll without interrupting metric publication.

Optional `history_retention_days` uses the same Core API path to request `recorder.purge_entities` for exact currently registered and selected Bridge entity IDs. The default `0` makes no cleanup requests. Successful requests persist the day limit and timestamp in the ownership manifest, limiting cleanup to once every 24 hours unless the configured limit changes. The Bridge never edits the Recorder database or its global configuration, and does not clear statistics.

Hub backups stop the server for SQLite consistency. Bridge backups preserve identity and owned topics. See [Hub configuration](../beszel_hub/DOCS.md) and [Bridge setup](../beszel_bridge/DOCS.md).

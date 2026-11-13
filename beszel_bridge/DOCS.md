# Beszel Bridge

Bring readings from a local or remote Beszel Hub into Home Assistant as MQTT devices and entities.

## Requirements

- Home Assistant **2026.10.1 or newer** and a Beszel Hub.
- An MQTT broker connected to Home Assistant's MQTT integration.
- A dedicated Hub account with a password and shared systems, preferably with the `readonly` role. Superuser credentials, Agent tokens, OAuth-only accounts and interactive MFA are unsupported.

Supports 64-bit Intel/AMD (**amd64**) and ARM (**aarch64**) with Protection mode enabled. Use one Bridge per Hub per Home Assistant installation. No Agent changes or HACS components are needed.

## Setup

[Add this repository](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md#1-add-the-repository), then open **Settings > Apps > Install app**, choose **Beszel Bridge** and select **Install**.

### 1. Share systems with the Bridge account

Open the Hub's `/_/` admin panel, including any base path, and sign in as a superuser. Create a normal account in **users** with an email, password, name and **readonly** role.

![Create a readonly Hub account](https://raw.githubusercontent.com/vineetchoudhary/home-assistant-beszel-agent/main/docs/screenshots/beszel-bridge/hub-readonly-account.webp)

In **systems**, open each server's **users** picker and add the new account without removing the owner. Choose **Set selection** and save. Use this normal account in the Bridge, keeping the superuser account for administration.

![Share a server with the Bridge account](https://raw.githubusercontent.com/vineetchoudhary/home-assistant-beszel-agent/main/docs/screenshots/beszel-bridge/hub-system-sharing.webp)

### 2. Connect Home Assistant to MQTT

Skip this if Home Assistant already uses your broker. Otherwise, open **Settings > Devices & services > Add integration**, search for **MQTT** and enter your broker's address, port and credentials. Match its TLS settings, then finish setup.

![MQTT broker settings](https://raw.githubusercontent.com/vineetchoudhary/home-assistant-beszel-agent/main/docs/screenshots/beszel-bridge/03-mqtt-broker.webp)

![MQTT credentials and other settings](https://raw.githubusercontent.com/vineetchoudhary/home-assistant-beszel-agent/main/docs/screenshots/beszel-bridge/04-mqtt-credentials.webp)

The example uses a local test broker. Enter your broker's own address and credentials; expand **Other settings** for certificate options.

### 3. Configure the Bridge

For a Hub running as a Home Assistant app, replace `192.168.1.100` with Home Assistant's IP address under **Settings > System > Network**. Use the dedicated account from step 1:

```yaml
hub_url: "http://192.168.1.100:8090"
hub_email: "ha-monitor@example.com"
hub_password: "your-dedicated-account-password"
mqtt_source: supervisor
```

For a Hub on another machine, use its reachable LAN, VPN or HTTPS address, such as `https://monitoring.example.com/beszel`. Include any base path. Ingress URLs and `localhost` do not point to the Hub from the Bridge.

If the Hub's published port is disabled, use its [internal app hostname](https://contributors.home-assistant.io/apps/guides/communication/#the-internal-network) with port `8090`. The Hub's Watchdog needs its published port, so disable Watchdog on the Hub if removing that mapping.

`mqtt_source: supervisor` gets settings from the installed broker app. For another broker, add these options:

```yaml
mqtt_source: manual
mqtt_host: "mqtt.example.com"
mqtt_port: 8883
mqtt_username: "beszel-bridge"
mqtt_password: "your-broker-password"
mqtt_tls: true
mqtt_ca_file: "/ssl/mqtt-ca.crt"
```

Use the same broker and discovery prefix as Home Assistant. For private CAs, copy PEM certificates into `/ssl` and set `hub_ca_file` or `mqtt_ca_file`. Leave CA fields empty for publicly trusted certificates. Certificate and hostname verification remain enabled. Keep credentials private when sharing options, logs or backups.

### 4. Start and check devices

Save and start the Bridge. Check for `Bridge synchronized` in its log, then open **Settings > Devices & services > MQTT**. You should see a Bridge device and one device per selected server. Save and restart after configuration changes.

![Bridge and server devices in MQTT](https://raw.githubusercontent.com/vineetchoudhary/home-assistant-beszel-agent/main/docs/screenshots/beszel-bridge/06-mqtt-devices.webp)

Use the [dashboard setup guide](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/dashboards/beszel/SETUP.md#add-a-dashboard) to display these readings.

## Configuration reference

### Hub connection

| Option | Default | Purpose |
| --- | --- | --- |
| `hub_url` | Not set, required | Direct HTTP or HTTPS Hub URL, including its port and base path. Credentials, query strings and fragments are not accepted in the URL. |
| `hub_email` | Not set, required | Email of the normal Hub account with access to the selected systems. |
| `hub_password` | Not set, required | That account's password. |
| `hub_ca_file` | `""` | Optional PEM CA file under `/ssl` for a privately signed HTTPS Hub. Empty uses system trust. |

### MQTT connection

| Option | Default | Purpose |
| --- | --- | --- |
| `mqtt_source` | `supervisor` | Use the broker app's service settings, or choose `manual` for the fields below. |
| `mqtt_host` | `""` | Required in manual mode. Hostname or IP address, without a scheme, port or path. |
| `mqtt_port` | `1883` | Manual broker port, from `1` to `65535`. TLS commonly uses `8883`. |
| `mqtt_username` | `""` | Manual broker username, if required. |
| `mqtt_password` | `""` | Manual broker password, if required. |
| `mqtt_tls` | `false` | Enable TLS for a manual connection. Match the broker's port. |
| `mqtt_ca_file` | `""` | Optional PEM CA file under `/ssl` for MQTT TLS in either source mode. Empty uses system trust. |

In Supervisor mode, manual host, port, credentials and TLS settings are ignored. If the broker service is unavailable, start the broker app or switch to manual mode.

### Selection and updates

| Option | Default | Purpose |
| --- | --- | --- |
| `include_systems` | `[]` | System record IDs to monitor. Empty selects all systems the account can access. |
| `exclude_systems` | `[]` | System record IDs to omit. Exclusions take precedence. |
| `metrics` | `[]` | Individual keys or families below. Empty selects all supported readings. Status, connectivity and Bridge diagnostics are always included. |
| `group_entities` | `false` | Optional single status entity per SMART disk, container and systemd service, with detailed properties as attributes. Save and restart after changing this option. See grouped entities below. |
| `enable_all_entities` | `false` | Enable all selected entities, including existing entities disabled by the integration. Explicit user disables are preserved. Save and restart the Bridge after changing this option. |
| `history_retention_days` | `0` | Optional detailed-history cleanup for this Bridge's selected entities. `0` leaves Home Assistant's retention unchanged. Set `1` to `365` days to remove older recorded states daily. See the retention notes below. |
| `poll_interval` | `30` | Seconds between polls, from `15` to `300`. |
| `stale_after` | `0` | Maximum sample age in seconds. `0` uses `max(180, 3 × poll_interval)`. Explicit values must meet that minimum and be at most `86400`. |
| `detail_stale_after` | `172800` | Maximum age of SMART, pool details and package inventory in seconds. Default is 48 hours. Range `3600` to `31536000`. |
| `disk_aggregate_exclude` | `[]` | System/filesystem pairs to omit from combined totals. Individual filesystem entities remain available. See the example below. |
| `discovery_prefix` | `homeassistant` | Home Assistant's MQTT discovery prefix. Use letters, numbers, underscores or hyphens. Clean up before changing it. |
| `ha_status_topic` | `homeassistant/status` | Home Assistant's MQTT birth topic. Match any custom setting. Must be nonempty, without wildcards. |
| `cleanup` | `false` | Remove this Bridge's MQTT entities and exit. Follow the cleanup steps below, then restore `false`. |

System IDs contain 15 lowercase letters or digits. In the Hub's `/_/` admin panel, open **systems** and copy the server's **id**. Replace `abc123def456ghi` with that value:

```yaml
include_systems:
  - "abc123def456ghi"
metrics:
  - cpu_usage
  - memory_usage
```

## Readings and history

| Metric key | Reading |
| --- | --- |
| `cpu_usage` | CPU usage, percent. |
| `memory_usage` | Memory usage, percent. |
| `disk_usage` | Primary disk usage, percent. |
| `temperature` | Highest observed sensor temperature, °C. Appears only after a real sensor is reported. |
| `network_receive` | Receive rate, bytes per second (`B/s`). |
| `network_transmit` | Transmit rate, bytes per second (`B/s`). |
| `uptime` | Uptime, seconds. |
| `last_sample` | Source sample timestamp. |

These original keys and identities remain supported. Selecting a family includes its corresponding original readings. Selecting only an original key retains its narrow selection.

### Metric families

| Family selector | Readings and metadata |
| --- | --- |
| `cpu` | Overall/per-core usage, user/system/I/O wait/steal/idle/IRQ/soft IRQ/nice percentages, 1/5/15-minute load averages and native 10-minute usage peak. |
| `memory` | Usage percentage, total/used/free bytes, buffer/cache, ZFS ARC and native 10-minute peak used memory. Free is total minus used, rather than the OS's raw free-memory counter. |
| `swap` | Total/used/free bytes and usage percentage when capacity is positive. |
| `filesystems` | Primary and extra filesystem capacity/used/free bytes and usage percentage, plus combined capacity/used/free/usage. |
| `disk_io` | Primary/extra filesystem read/write rates and cumulative byte counters, read/write time factors, utilization, read/write latency, weighted I/O and native 10-minute peaks. |
| `network` | Aggregate receive/transmit and combined bandwidth, native 10-minute peaks, each interface's receive/transmit rates and cumulative byte counters. |
| `temperatures` | Highest observed temperature and every named temperature sensor, including GPU sensors reported in the temperature map. |
| `fans` | Each named fan's RPM. |
| `gpus` | Highest GPU utilization, each GPU's utilization, memory used/total/free/percentage, power/package power and engine utilization. Model and source identity are attributes. |
| `batteries` | Representative battery percentage/state and each named battery's percentage. The Hub does not supply a separate charge state for every battery. |
| `wifi` | Signal strength in dBm per interface, with interface identity and SSID when available. |
| `smart` | Per-disk status, capacity, power-on seconds/readable duration, cycles, temperature, update timestamp and numeric/readable update age. Device/model/type/firmware/serial are status attributes. |
| `smart_attributes` | Every reported SMART raw attribute, with source name/ID, normalized value, worst value, threshold, raw string and failure flag. Exact raw integers also appear in the `raw_value` string attribute. |
| `containers` | Per-container status, health, CPU, memory, receive/transmit rates, combined bandwidth, image-update availability and last update. Name, ID, image and ports are attributes. |
| `services` | Total/failed service counts and each systemd service's state/substate, current/peak CPU, current/peak memory and last update. |
| `storage_pools` | ZFS/Btrfs pool health/advisory, capacity/allocated/free bytes, usage, read/write rates, scrub state/progress/errors and detail-update time. Per-vdev state/read/write/checksum errors and per-dataset used/available bytes retain source names and mount metadata. |
| `network_monitors` | Probe response average/minimum/maximum, packet loss, hourly summaries, current-window probe/success counts, response sum and sample timestamp. HTTPS certificate expiry/days remaining/issuer and target/protocol/port/DNS/interval/enabled/TLS metadata when available. |
| `package_updates` | Total/security update counts, update availability, last-check timestamp/age and the complete package inventory as attributes, including installed/available versions and security classification. |
| `alerts` | Only the Bridge account's triggered alerts, active count, latest event and that event's resolution. Threshold, duration, pending state and event values are attributes. |
| `system_info` | Uptime/source timestamp, Agent version/connection and system-information attributes: hostname, OS, kernel, architecture, CPU model, cores/threads/sockets, installed memory, container runtime and advertised capabilities. |
| `uptime_human` | Additional readable uptime alongside the original seconds entity. |

The Bridge device also exposes monitored/degraded system counts and an incomplete-inventory indicator. Sample age is an optional diagnostic entity on each server. These diagnostics are always selected.

Existing entities, readable uptime, extra filesystem usage, combined storage overview and SMART overview entities are enabled by default. Other new entities are discovered disabled unless `enable_all_entities` is enabled.

To enable all available readings at once, set this option in the Bridge's **Configuration**, save and restart:

```yaml
enable_all_entities: true
```

The Bridge enables new entities through MQTT discovery and existing integration-disabled entities through Home Assistant's API using its Supervisor token. No separate Home Assistant token is needed. Only this Bridge's selected entities are changed. Metric and system filters still apply, and entities disabled explicitly by a user or a disabled device remain disabled. Existing IDs, names and history are preserved. Home Assistant may take a short time to load the enabled entities. Turning the option off restores defaults for new discoveries and leaves existing entity settings intact.

For individual readings, open the server under **Settings > Devices & services > MQTT** and enable the entity in its settings. See the [illustrated entity and dashboard setup](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/dashboards/beszel/SETUP.md) for the steps.

Expanded readings have a `source_timestamp` attribute. Sizes use bytes, rates use `B/s`, latencies use milliseconds and temperatures use °C. Native bytes take precedence over legacy GiB/MiB fields. Home Assistant handles cumulative counter resets. Vendor SMART attributes retain their source encoding without guessed units or statistics classes.

### Optional grouped entities

The default layout keeps individual readings. To reduce the number of entities, enable **group_entities** in the Bridge app's **Configuration**, save and restart:

```yaml
group_entities: true
```

Grouping keeps one existing status entity per SMART disk and container, and one existing state entity per systemd service. The status/state entity keeps its ID and name. Its other readings become attributes, and replaced detail discovery is removed after a complete, fresh summary has been published. Other metric families and host-wide container/service counts keep their current entities. Here, services means the Hub's systemd inventory; arbitrary OS processes are not collected.

| Summary | State | Attributes when reported |
| --- | --- | --- |
| SMART disk status | Disk health such as `PASSED` or `UNKNOWN`. | Device, model, type, firmware, serial, `capacity_bytes`, `temperature_celsius`, `power_cycles`, `power_on_hours`, `power_on_seconds`, `power_on_human`, `power_on_source_precision_seconds`, `last_updated` and `smart_attributes`. |
| Container status | The Hub's reported status. | Name, container ID, image, ports, `health`, `cpu_usage_percent`, `memory_bytes`, `bandwidth_bytes_per_second`, `receive_bytes_per_second`, `transmit_bytes_per_second`, `update_available` and `last_updated`. |
| Service state | State such as `active` or `failed`. | Service name, `substate`, `cpu_usage_percent`, `cpu_peak_percent`, `memory_bytes`, `memory_peak_bytes` and `last_updated`. |

`smart_attributes` is a list of the drive's reported properties. Each item includes its name, ID when available, normalized/worst values, threshold, raw value/string and failure flag when reported. Unrecognized vendor fields are preserved. Large raw integers use an exact `raw_value` string. Selecting only `smart_attributes` also selects the disk status needed to carry them. Selecting only `smart` omits the vendor list. Unsupported values stay absent, and stale or ambiguous container receive/transmit samples are omitted. `attribute_timestamps` identifies values with a source timestamp different from the summary's source. Use `last_updated` to calculate age; changing readable ages are not included in grouped attributes.

Grouping and enabling are separate settings. The existing defaults still apply: SMART status is enabled by default, while container/service summaries are optional. Use `enable_all_entities` to enable selected integration-disabled summaries in bulk. Turning `group_entities` off restores individual discovery after the next successful fetch. Update dashboards and automations referencing replaced detail entities to use the summary's attributes. Existing detail history is not moved into the summary or deleted by grouping; Home Assistant's configured retention continues to apply.

Attributes can be shown in standard dashboard entity cards and used in templates or automations. They do not get independent numeric history charts or long-term statistics. For a reading that needs its own graph, keep individual mode or create a [template sensor](https://www.home-assistant.io/integrations/template/). For example, replace the ID below with your container status entity:

```yaml
template:
  - sensor:
      - name: "Container CPU"
        unit_of_measurement: "%"
        state_class: measurement
        state: "{{ state_attr('sensor.beszel_server_container_status', 'cpu_usage_percent') }}"
        availability: >-
          {{ has_value('sensor.beszel_server_container_status')
             and state_attr('sensor.beszel_server_container_status', 'cpu_usage_percent') is number }}
```

Fewer entities do not guarantee a smaller Recorder database. Attribute changes also produce state writes, so large or frequently changing attribute lists still have a history cost. See Home Assistant's [MQTT sensor attribute guidance](https://www.home-assistant.io/integrations/sensor.mqtt/#json-attributes-template-configuration).

### Multiple disks and SMART

The original `disk_usage` remains primary filesystem usage. Each extra filesystem gets its own entity. Combined usage is `100 × sum(used) / sum(capacity)` from one source sample. Missing or invalid capacity/usage makes the aggregate unavailable.

Combined capacity describes monitored filesystems. The Hub lacks a complete physical-disk map, so overlapping mounts or shared datasets can count storage twice. SMART capacities and storage pools stay out of the sum. Exclude overlaps using source keys from the filesystem entity's attributes. Use `root` for the primary filesystem:

```yaml
disk_aggregate_exclude:
  - system: "abc123def456ghi"
    filesystem: "data-backup"
```

SMART entities describe physical devices independently from filesystem usage. A status entity can show `PASSED`, with `/dev/sda`, `CT1000BX500SSD1` and `sat` in its attributes. Numeric entities provide capacity, temperature, cycles and power-on seconds. Readable entities show values such as `20 hours` and `10 hours ago`.

Select the status sensor, then **⋮ > Details**, to view its disk metadata. The [storage dashboard template](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/dashboards/beszel/storage-health.yaml) groups filesystem usage, SMART health and raw/readable uptime. The example below is captured from Home Assistant using controlled test readings.

![Storage, SMART health and uptime dashboard](https://raw.githubusercontent.com/vineetchoudhary/home-assistant-beszel-agent/main/docs/screenshots/beszel-bridge/storage-desktop.webp)

SMART seconds are converted from the Hub's hours value. `source_precision_seconds: 3600` records that limit. Unsupported power metrics are not created from default zeros. Reported attributes provide evidence for legitimate zero hours/cycles. Missing health data never becomes passed.

Readable durations use whole seconds, minutes, hours, days below 60 days, approximate 30-day months below a year, then 365-day years. Examples: `10 mins`, `20 hours`, `1 day`, `30 days`, `6 months`, `1 year` and `10 years`.

### Freshness and availability

Missing or stale readings are unavailable. Source time determines freshness, so frequent polls do not make old data fresh. Current telemetry uses `1m` samples. Peak entities use the latest native `10m` rollup with at least 30 minutes allowed for that interval. Probe freshness allows three configured intervals, with `stale_after` as the minimum. Failed probes do not report zero latency.

SMART, pool details and package inventory use `detail_stale_after`. SMART/pool records are checked every `max(60, poll_interval)` seconds, package inventory every five minutes. The Bridge reads stored SMART data without requesting scans. Timestamps and ages remain readable to explain old data. Pool details use `details_updated`, separately from current metrics. Security-update counts are unavailable when classification is unknown.

Expanded entities stay under their server device. Hub record IDs identify record-backed components. Source-map keys identify filesystems, interfaces and sensors. Display-name changes preserve identity when the source identity stays the same. A changed source-map key creates a replacement component.

HA history starts when entities report. Older history stays in Beszel. Live container inspection, extended service runtime inspection, logs, administrative diagnostics and control actions are outside this monitoring interface.

### History retention

Set `history_retention_days` to your desired retention period in the Bridge's **Configuration**, then save and restart. Enter a whole number from `1` to `365` days. The default, `0`, turns Bridge-managed cleanup off and uses Home Assistant's normal retention.

Positive values request `recorder.purge_entities` for the exact current entity IDs owned and selected by this Bridge, including renamed entities. Other integrations, the global Recorder configuration and the Beszel Hub database are unchanged.

Cleanup is requested after the first successful sync, when the day limit changes, and then once every 24 hours while the Bridge runs. The last request is persisted so restarting does not repeat it. Home Assistant performs cleanup asynchronously, so this is a daily policy rather than an exact rolling cutoff.

Enabling this option or lowering the limit permanently removes older recorded states and their attributes. Increasing the limit or turning cleanup off cannot restore deleted history. Short-term and long-term statistics keep Home Assistant's own retention rules. This option cannot keep history longer than Home Assistant's global `purge_keep_days`, or record entities excluded from Recorder. See [Recorder](https://www.home-assistant.io/integrations/recorder/) and [history data sources](https://www.home-assistant.io/integrations/history/#about-the-data-sources).

Renaming a server or changing the same Hub's URL preserves devices and HA customizations. Excluded systems are removed at the next successful sync. Inaccessible systems and absent components are removed after five minutes of complete successful inventory checks. Incomplete or malformed inventories preserve discovery. An unavailable optional collection affects its readings while other valid data continues.

## Troubleshooting

| Problem | Check |
| --- | --- |
| Hub connection or TLS error | Direct URL, base path, network access and CA file. |
| Login fails or no systems appear | Use a normal account with a password and shared systems. Check filters. |
| MQTT service unavailable | Start the broker app or choose manual settings. |
| Missing Home Assistant entities | Use the same broker and discovery prefix as Home Assistant, then check for a successful sync. |
| Existing entities stay disabled with `enable_all_entities: true` | Check the Bridge log for a Home Assistant API warning and update/restart the app so Supervisor grants its Core API permission. Entities explicitly disabled by a user or device remain disabled. |
| History retention is not applied | Check the Bridge log for a retention warning, ensure Recorder is enabled, and update/restart the app to grant its Core API permission. Cleanup runs daily and cannot extend Home Assistant's global history limit. |
| Unavailable readings | Check the Agent, Hub connection and age of the source sample. |
| Readings missing after a Hub update | Check the app log for changed or invalid fields. Other valid readings continue, and affected readings recover automatically when valid data returns. |

The Bridge's Watchdog checks its process and can remain healthy during Hub or broker outages. Check the log and connection diagnostics for those failures.

![Hub outage with unavailable readings](https://raw.githubusercontent.com/vineetchoudhary/home-assistant-beszel-agent/main/docs/screenshots/beszel-bridge/overview-unavailable-desktop.webp)

For container commands, see [advanced diagnostics](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/TESTING.md#bridge-diagnostics). Remove system names, IDs and credentials before sharing diagnostics.

## Clean up or replace a Hub/broker

Before uninstalling, switching brokers or discovery prefixes, replacing the Hub or rotating its SSH key:

1. Keep the original Hub and broker settings. Temporarily disable Watchdog and automatic start.
2. Set `cleanup: true`, save and restart. Confirm removal of this Bridge's MQTT discovery entries in the log.
3. Set `cleanup: false`, apply any new settings and start the app. Restore Watchdog and automatic start if continuing to use it.

If the old broker is offline, keep its settings and app data, then retry cleanup later. Include the app in Home Assistant backups. Deleting its data first can leave old MQTT entities behind.

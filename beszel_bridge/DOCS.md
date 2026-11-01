# Beszel Bridge

Bring readings from a local or remote Beszel Hub into Home Assistant as MQTT devices and entities.

## Requirements

- Home Assistant **2026.10.0 or newer** and a Beszel Hub.
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

![MQTT credentials and TLS settings](https://raw.githubusercontent.com/vineetchoudhary/home-assistant-beszel-agent/main/docs/screenshots/beszel-bridge/04-mqtt-credentials.webp)

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
| `metrics` | `[]` | Metric keys from the table below. Empty enables all supported readings. Status, connectivity and Bridge diagnostics are always included. |
| `poll_interval` | `30` | Seconds between polls, from `15` to `300`. |
| `stale_after` | `0` | Maximum sample age in seconds. `0` uses `max(180, 3 × poll_interval)`. Explicit values must meet that minimum and be at most `86400`. |
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

Missing or stale readings are unavailable. Freshness uses the Hub's source sample time, so polling more often does not make old data fresh. HA history starts when entities report. Older history stays in Beszel. Container, GPU, S.M.A.R.T. and extra filesystem entities are not available.

Renaming a server or changing the same Hub's URL preserves its devices. HA entity customizations also survive restarts. Excluded systems are removed at the next successful sync. Inaccessible systems are removed after five minutes of complete, successful inventory checks. Incomplete or malformed inventories preserve existing devices.

## Troubleshooting

| Problem | Check |
| --- | --- |
| Hub connection or TLS error | Direct URL, base path, network access and CA file. |
| Login fails or no systems appear | Use a normal account with a password and shared systems. Check filters. |
| MQTT service unavailable | Start the broker app or choose manual settings. |
| Missing Home Assistant entities | Use the same broker and discovery prefix as Home Assistant, then check for a successful sync. |
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

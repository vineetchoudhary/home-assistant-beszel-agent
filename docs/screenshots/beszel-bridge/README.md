# Bridge setup screenshots

The expanded-monitoring screenshots were captured from the real Home Assistant 2026.10.1 browser UI with Hub 0.21.1, Mosquitto, the Bridge and a live Beszel Agent in an isolated local test environment. The Bridge app/release pins remain unchanged.

The extra filesystem, SMART disks, GPU and other hardware examples come from [synthetic monitoring fixtures](../../../beszel_bridge/testdata/expanded/monitoring.json), injected through the authenticated Hub test proxy. They verify publication, discovery, entity settings and rendering. They do not validate physical SMART hardware or installation through Home Assistant OS/Supervisor.

| Images | What they show |
| --- | --- |
| `03-mqtt-broker.webp`, `04-mqtt-credentials.webp` | Actual MQTT connection form; the test broker is named `broker`. Credential fields are empty. |
| `06-mqtt-devices.webp`, `07-server-entities.webp`, `08-copy-entity-id.webp` | Bridge/server discovery and copying a real test entity ID. |
| `09-dashboard-settings.webp` through `13-dashboard-ready.webp` | Dashboard setup forms, raw configuration editor and saved template. |
| `14-enable-detail.webp`, `16-detail-enabled.webp` | Enabling a default-disabled GPU sensor and its resulting 25 W reading. |
| `15-smart-overview.webp` | SMART status and physical-disk metadata. The serial is a test value. |
| `overview-desktop.webp`, `overview-mobile.webp` | Overview and connection diagnostics. |
| `detail-desktop.webp`, `detail-mobile.webp` | Current readings, readable uptime, combined filesystems and history. |
| `storage-desktop.webp`, `storage-mobile.webp` | Individual/combined filesystem usage, SMART and raw/readable durations. |

Desktop captures use the browser's default 1280-pixel viewport; mobile captures use a 390 × 844 viewport and include the full page. Some desktop views have the sidebar collapsed to show all card labels. Screenshots are converted from browser JPEG captures to WebP without altering their content.

The Hub-account, system-sharing and outage screenshots, and unused earlier setup captures, retain their earlier capture versions. To reproduce the expanded examples, follow [Testing: setup screenshots](../../TESTING.md#setup-screenshots).

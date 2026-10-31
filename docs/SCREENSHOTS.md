# Screenshot capture notes

Use actual UI captures with locally drawn red rectangles and masked secrets. Preserve UI text and pixels. Keep raw captures/annotations under ignored `.artifacts/bridge/` and final images under `docs/screenshots/`.

## Installation guide

Refreshed on **16 October 2026** at **1920 × 1080**: `1-ha-settings.webp` (HA Core 2026.10.1), `8-beszel-config.webp` and `13-beszel-dashboard.webp` (Beszel 0.21.0).

[`beszel-apps.jpg`](screenshots/beszel-apps.jpg): captured **27 October 2026** from HA OS **18.3** and Core **2026.10.4** after adding the public repository. The native JPEG crop shows its title and six published apps. Intel GPU is unavailable on the aarch64 capture instance.

Thirteen older Supervisor captures still show **Add-ons**, now **Apps**. Keep original S.M.A.R.T. images until fresh hardware captures are available. Use the [replacement checklist](MANUAL_CHECKLIST.md#ha-os-screenshots).

## Bridge and dashboard captures

Captured on **16 October 2026** using HA Core **2026.10.1**, Hub/Agent **0.21.0** and Mosquitto **2.0.22**. Desktop: **1920 × 1080**. Native phone: **430 × 932**.

| Files | Setup step |
| --- | --- |
| `hub-readonly-account.webp` | Readonly account |
| `hub-system-sharing.webp` | System sharing |
| `01-ha-settings.webp` | Devices & services |
| `02-add-mqtt.webp` | Add MQTT |
| `03-mqtt-broker.webp`, `04-mqtt-credentials.webp` | Broker, credentials and TLS |
| `05-mqtt-connected.webp` | Finish MQTT setup |
| `06-mqtt-devices.webp` | Bridge/server devices |
| `07-server-entities.webp` | Live readings |
| `08-copy-entity-id.webp` | Copy Entity ID |
| `09-dashboard-settings.webp`, `10-create-dashboard.webp` | Create dashboard |
| `11-raw-editor-menu.webp` | Raw editor |
| `12-dashboard-yaml.webp` | Actual entity IDs |
| `13-dashboard-ready.webp` | Saved dashboard |
| `overview-desktop.webp`, `detail-desktop.webp` | Desktop previews |
| `overview-mobile.webp`, `detail-mobile.webp` | Phone previews |
| `overview-unavailable-desktop.webp`, `overview-unavailable-mobile.webp` | Hub outage |

Accounts, IDs and `broker` belong to disposable test fixtures. Anonymous MQTT was isolated without a published host port. Users must supply their own reachable broker, credentials and TLS settings. Images omit passwords, session tokens and private keys.

These captures cover real Hub, MQTT and dashboard UI. Supervisor installation/options remain pending. Disposable containers, network and private state were removed after capture.

Used by [Bridge setup](../beszel_bridge/DOCS.md) and [dashboards](dashboards/beszel/SETUP.md). Preserve reusable [API fixtures](../beszel_bridge/testdata/README.md). The obsolete `docs/bridge-evidence/` directory remains ignored.

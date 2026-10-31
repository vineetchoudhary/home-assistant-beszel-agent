# Beszel dashboard setup

Use the [overview](server-overview.yaml) or [detail](server-detail.yaml) template after [Bridge setup](../../../beszel_bridge/DOCS.md). Both use standard HA cards.

## Add a dashboard

### 1. Copy the entity IDs

Open the server under **Settings > Devices & services > MQTT**. Select a sensor, open its **Settings** cog and copy its complete Entity ID. Repeat for other readings and the Bridge device's diagnostics.

![Server sensors](../../screenshots/beszel-bridge/07-server-entities.webp)

![Copy the Entity ID](../../screenshots/beszel-bridge/08-copy-entity-id.webp)

### 2. Create an empty dashboard

Open **Settings > Dashboards > Add dashboard > New dashboard from scratch**. Enter a title and URL containing a hyphen and no spaces. Keep **Add to sidebar** enabled and choose **Create**.

![Dashboard settings](../../screenshots/beszel-bridge/09-dashboard-settings.webp)

![Create a dashboard](../../screenshots/beszel-bridge/10-create-dashboard.webp)

### 3. Open the raw configuration editor

Open the dashboard, choose **Edit dashboard**, then **⋮ > Raw configuration editor**. Take control of the dashboard first if prompted.

![Raw configuration editor menu](../../screenshots/beszel-bridge/11-raw-editor-menu.webp)

### 4. Paste a template and replace its placeholders

Paste a complete template and replace all server and Bridge entity placeholders with the actual IDs. In the detail template, replace `BESZEL_HUB_URL` with the Hub's browser address. To include both views, combine their items under one `views:` list.

![Template with actual entity IDs](../../screenshots/beszel-bridge/12-dashboard-yaml.webp)

### 5. Save and check the readings

Choose **Save**, close the editor and choose **Done**. Verify readings and connection indicators. Importing both views adds **Servers** and **Server detail** tabs.

![Saved dashboard](../../screenshots/beszel-bridge/13-dashboard-ready.webp)

## Adjust the dashboard

Duplicate card groups for additional servers and substitute their IDs. HA may change suggested IDs after collisions or renaming. Keep your own edits after import.

Remove unsupported or disabled metrics. Add Temperature after the Bridge observes a sensor. Network history requires rate entities. History starts with HA collection, while older data stays in Beszel. Outages show unavailable readings, as in the [Hub outage example](../../../beszel_bridge/DOCS.md#diagnostics-and-recovery).

## Dashboard previews

Real HA Core 2026.10.1 and Beszel 0.21.0 captures. See [capture notes](../../SCREENSHOTS.md#bridge-and-dashboard-captures).

### Server overview

![Overview and connection diagnostics](../../screenshots/beszel-bridge/overview-desktop.webp)

### Server detail

![Current readings and history](../../screenshots/beszel-bridge/detail-desktop.webp)

### Mobile layout

| Overview | Server detail |
| --- | --- |
| ![Mobile overview](../../screenshots/beszel-bridge/overview-mobile.webp) | ![Mobile detail](../../screenshots/beszel-bridge/detail-mobile.webp) |

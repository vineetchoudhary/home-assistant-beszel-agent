# Beszel dashboard setup

Complete [Bridge setup](../../../beszel_bridge/DOCS.md) and confirm its MQTT devices first. Choose the [overview](server-overview.yaml) or [detail](server-detail.yaml) template. Both use standard Home Assistant cards.

## Add a dashboard

### 1. Copy the entity IDs

Open the server under **Settings > Devices & services > MQTT**. Select a sensor, open its **Settings** cog and copy its complete Entity ID. Repeat for other readings and the Bridge device's diagnostics.

![Server sensors](../../screenshots/beszel-bridge/07-server-entities.webp)

![Copy the Entity ID](../../screenshots/beszel-bridge/08-copy-entity-id.webp)

### 2. Create an empty dashboard

Open **Settings > Dashboards > Add dashboard > New dashboard from scratch**. Enter a title and a URL path such as `beszel-servers`, enable **Add to sidebar** and choose **Create**.

![Dashboard settings](../../screenshots/beszel-bridge/09-dashboard-settings.webp)

![Create a dashboard](../../screenshots/beszel-bridge/10-create-dashboard.webp)

### 3. Open the raw configuration editor

Open the dashboard, choose **Edit dashboard**, then **⋮ > Raw configuration editor**. Take control of the dashboard first if prompted.

![Raw configuration editor menu](../../screenshots/beszel-bridge/11-raw-editor-menu.webp)

### 4. Paste a template and replace its placeholders

Paste one complete template. Replace every example ID beginning with `sensor.beszel_server`, `binary_sensor.beszel_server`, `sensor.beszel_bridge` or `binary_sensor.beszel_bridge` with the matching Entity ID from step 1. In the detail template, replace `BESZEL_HUB_URL` with your Hub's web address.

To use both views, copy the view entries from the second template into the first template's `views:` list. Keep a single `title:` and `views:` at the top level.

![Template with actual entity IDs](../../screenshots/beszel-bridge/12-dashboard-yaml.webp)

### 5. Save and check the readings

Choose **Save**, close the editor and choose **Done**. Check that readings appear. If you combined both templates, you will see **Servers** and **Server detail** tabs.

![Saved dashboard](../../screenshots/beszel-bridge/13-dashboard-ready.webp)

## Adjust the dashboard

For more servers, copy a card group and replace its Entity IDs. Always use IDs from Home Assistant because renaming or name collisions can change them.

Remove readings you have disabled or your hardware does not report. History starts when Home Assistant receives data. Older history stays in Beszel. If readings show **Unavailable**, check the [Bridge troubleshooting guide](../../../beszel_bridge/DOCS.md#troubleshooting).

## Dashboard examples

### Server overview

![Overview and connection diagnostics](../../screenshots/beszel-bridge/overview-desktop.webp)

### Server detail

![Current readings and history](../../screenshots/beszel-bridge/detail-desktop.webp)

### Mobile layout

| Overview | Server detail |
| --- | --- |
| ![Mobile overview](../../screenshots/beszel-bridge/overview-mobile.webp) | ![Mobile detail](../../screenshots/beszel-bridge/detail-mobile.webp) |

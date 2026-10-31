# Beszel dashboards

Standard Home Assistant cards for Beszel Bridge's MQTT entities. HACS and custom cards are not required.

- [Server overview](server-overview.yaml): status, resource usage and connection diagnostics.
- [Server detail](server-detail.yaml): current readings, history and a link to Beszel.

## Add a dashboard

1. Complete the [Bridge setup](../../../beszel_bridge/DOCS.md) and confirm its MQTT devices in Home Assistant.
2. Create a dashboard under **Settings > Dashboards**, then open its **Raw configuration editor**.
3. Paste a template, replace every entity placeholder with your actual entity IDs, set `BESZEL_HUB_URL` in the detail template, and save.

See the [illustrated setup, customization and previews](SETUP.md) for the full walkthrough.

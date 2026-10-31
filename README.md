# Beszel for Home Assistant

Run [Beszel](https://github.com/henrygd/beszel) Hub and Agent apps in Home Assistant to monitor your system and other hosts.

![Beszel repository in the Home Assistant app store, showing the Hub and five Agent variants](docs/screenshots/beszel-apps.jpg)

## Quick installation

Add this repository with the button below, or paste `https://github.com/vineetchoudhary/home-assistant-beszel-agent` into **Settings > Apps > Install app > ⋮ > Repositories**.

[![Open Home Assistant and add this app repository.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https://github.com/vineetchoudhary/home-assistant-beszel-agent)

1. Use your existing Hub, or install and start **Beszel Hub**, then create an account at `http://<home-assistant-host>:8090`.
2. Install **Beszel Agent**. In the Hub, choose **Add System**, copy its public key and token into the Agent's `key` and `token` options, and set `hub_url` to your Hub's address.
3. Save and start the Agent, then check the Hub for live metrics.

For screenshots and advanced options, see the [installation guide](docs/INSTALLATION.md).

## Beszel Bridge

[Beszel Bridge](beszel_bridge/DOCS.md) brings Hub readings into Home Assistant through MQTT.

[![Beszel Bridge dashboard showing server readings and Bridge and Hub connection status](docs/screenshots/beszel-bridge/overview-desktop.webp)](beszel_bridge/DOCS.md)

## Documentation

- [App selection and configuration](docs/APPS.md)
- [Home Assistant entities and dashboards](beszel_bridge/DOCS.md) (unreleased Bridge preview)
- [Contributing](docs/CONTRIBUTING.md) · [Testing](docs/TESTING.md) · [Architecture](docs/ARCHITECTURE.md)
- [Releases](docs/RELEASE.md) · [Manual HA OS checks](docs/MANUAL_CHECKLIST.md)

[Report an issue](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) · [MIT license](LICENSE)

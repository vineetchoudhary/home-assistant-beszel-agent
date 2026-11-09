[![Sponsor on GitHub](https://img.shields.io/badge/Sponsor%20on%20GitHub-EA4AAA?style=for-the-badge&logo=github&logoColor=white)](https://github.com/sponsors/vineetchoudhary)
[![Buy me a coffee](https://img.shields.io/badge/Buy%20me%20a%20coffee-FFDD00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black)](https://buymeacoffee.com/vineetchoudhary)
[![Build status](https://img.shields.io/github/actions/workflow/status/vineetchoudhary/home-assistant-beszel-agent/publish.yml?style=for-the-badge&logo=githubactions&logoColor=white&label=build)](https://github.com/vineetchoudhary/home-assistant-beszel-agent/actions/workflows/publish.yml)
[![Docker image downloads across all seven apps](https://img.shields.io/endpoint?url=https%3A%2F%2Fraw.githubusercontent.com%2Fvineetchoudhary%2Fhome-assistant-beszel-agent%2Fbadges%2Fdownloads.json&style=for-the-badge&logo=docker&logoColor=white)](https://github.com/vineetchoudhary/home-assistant-beszel-agent/packages)

# Beszel for Home Assistant

Monitor Home Assistant and other machines with [Beszel](https://github.com/henrygd/beszel). Run a Hub to view readings and an Agent on each monitored host.

![Beszel Hub and five Agent variants in the Home Assistant app store, arranged in two rows of three](docs/screenshots/beszel-apps.jpg)

## Quick installation

Add this repository with the button below, or paste `https://github.com/vineetchoudhary/home-assistant-beszel-agent` into **Settings > Apps > Install app > ⋮ > Repositories**.

[![Open Home Assistant and add this app repository.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https://github.com/vineetchoudhary/home-assistant-beszel-agent)

1. Use an existing Hub, or [set up Beszel Hub](beszel_hub/DOCS.md) and create its first account.
2. Install **Beszel Agent**. In the Hub, choose **Add System**, copy its public key and token into the Agent's `key` and `token` options, and set `hub_url` to your Hub's address.
3. Save and start the Agent, then check the Hub for live metrics.

See the [illustrated installation guide](docs/INSTALLATION.md) for the full setup.

## Beszel Bridge

[Beszel Bridge](beszel_bridge/DOCS.md) brings Hub readings into Home Assistant through MQTT.

[![Beszel Bridge dashboard showing server readings and Bridge and Hub connection status](docs/screenshots/beszel-bridge/overview-desktop.webp)](beszel_bridge/DOCS.md)

## Documentation

- [App selection and configuration](docs/APPS.md)
- [Home Assistant dashboard setup](docs/dashboards/beszel/SETUP.md)

[Report an issue](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) · [Contributing](docs/CONTRIBUTING.md) · [MIT license](LICENSE)

## Support

If this project has been useful to you, consider supporting its continued development.

<a href="https://github.com/sponsors/vineetchoudhary">
  <img src="https://img.shields.io/badge/Sponsor%20on%20GitHub-EA4AAA?style=for-the-badge&logo=github&logoColor=white" alt="Sponsor on GitHub" height="50">
</a>

<a href="https://buymeacoffee.com/vineetchoudhary">
  <img src="https://img.shields.io/badge/Buy%20me%20a%20coffee-FFDD00?style=for-the-badge&logo=buy-me-a-coffee&logoColor=black" alt="Buy Me a Coffee" height="50">
</a>

Thank you for supporting open source! 🙏

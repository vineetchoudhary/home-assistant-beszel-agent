# Beszel Add-ons for Home Assistant

Run Beszel inside Home Assistant with add-ons for both the Hub and the Agent. Monitor your Home Assistant system and other hosts with a lightweight monitoring stack.

## Available Add-ons

This repository provides six add-ons:

| Add-on | What it adds | Architectures |
| --- | --- | --- |
| **Beszel Hub** | The Beszel web UI and API, inside Home Assistant | amd64, aarch64 |
| **Beszel Agent** | Standard lightweight monitoring | amd64, aarch64 |
| **Beszel Agent (S.M.A.R.T.)** | S.M.A.R.T. disk health monitoring | amd64, aarch64 |
| **Beszel Agent (Intel GPU)** | Intel GPU metrics + S.M.A.R.T. | amd64 only |
| **Beszel Agent (AMD GPU)** | AMD GPU metrics + S.M.A.R.T. | amd64, aarch64 |
| **Beszel Agent (NVIDIA GPU)** | NVIDIA GPU metrics + S.M.A.R.T. - [see the caveats first](beszel_agent_nvidia/DOCS.md) | amd64, aarch64 |

Every GPU variant bundles S.M.A.R.T. support, matching upstream Beszel - which ships smartmontools in all of its non-scratch images rather than offering separate with/without builds.

## Quick Start

Click this button to add the repository:

[![Open your Home Assistant instance and show the add add-on repository dialog with this repository URL pre-filled.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https://github.com/vineetchoudhary/home-assistant-beszel-agent)

Or add it manually by following the [step-by-step installation and setup guide with screenshots](docs/INSTALLATION.md).

## Documentation
- [Step by Step Setup and Configuration Guide with Screenshots](docs/INSTALLATION.md)
- [Beszel Hub - Setup and Configuration](beszel_hub/DOCS.md)
- [Beszel Agent - Setup and Configuration](beszel_agent/DOCS.md)
- [Beszel Agent S.M.A.R.T. - Setup with S.M.A.R.T. Monitoring](beszel_agent_smart/DOCS.md)
- [Beszel Agent Intel GPU - Setup and Configuration](beszel_agent_intel/DOCS.md)
- [Beszel Agent AMD GPU - Setup and Configuration](beszel_agent_amd/DOCS.md)
- [Beszel Agent NVIDIA GPU - Setup and Configuration](beszel_agent_nvidia/DOCS.md)
- [Testing Instructions](docs/TESTING.md)
- [Contributing Guide](docs/CONTRIBUTING.md)
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Release Process](docs/RELEASE.md)

## Which Version Should I Use?

**Use Beszel Hub if:**
- You want to run the Beszel server directly inside Home Assistant
- You prefer a single-box setup for a small lab or home deployment

**Use Beszel Agent (Standard) if:**
- You just need basic system monitoring
- You want the smallest image size

**Use Beszel Agent (S.M.A.R.T.) if:**
- You want to monitor disk health
- You need S.M.A.R.T. data from your drives, but have no GPU to watch

**Use a GPU variant if:**
- You want GPU metrics as well as disk health - they all include S.M.A.R.T.
- Pick **Intel GPU** for Intel integrated or Arc graphics (amd64 only)
- Pick **AMD GPU** for Radeon cards - it reads sysfs, so no vendor tooling is needed
- Pick **NVIDIA GPU** only after reading [its caveats](beszel_agent_nvidia/DOCS.md); Home Assistant cannot attach an NVIDIA GPU to an add-on except on a Supervised host configured with the NVIDIA runtime as Docker's default

All GPU variants need **Protection mode disabled** to reach GPU and disk devices.

## What's Beszel?

Beszel is a lightweight monitoring solution that keeps track of your server's vital signs. It's pretty efficient and doesn't hog resources. Check out the [main Beszel project](https://github.com/henrygd/beszel) to learn more.

## Getting Help

Having trouble? [Open an issue](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) and I'll try to help.

## License

MIT - see [LICENSE](LICENSE) for the boring legal stuff.

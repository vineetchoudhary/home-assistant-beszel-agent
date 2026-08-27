# Beszel Add-ons for Home Assistant

Run Beszel inside Home Assistant with add-ons for both the Hub and the Agent. Monitor your Home Assistant system and other hosts with a lightweight monitoring stack.

## Available Add-ons

This repository provides five add-ons:

1. **Beszel Hub** - Run the Beszel web UI and API inside Home Assistant
2. **Beszel Hub (Test)** - Development/testing version of the Hub
3. **Beszel Agent** - Standard lightweight monitoring
4. **Beszel Agent (S.M.A.R.T.)** - With S.M.A.R.T. disk health monitoring
5. **Beszel Agent (Test)** - Development/testing version

## Quick Start

Click this button to add the repository:

[![Open your Home Assistant instance and show the add add-on repository dialog with this repository URL pre-filled.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https://github.com/vineetchoudhary/home-assistant-beszel-agent)

Or add it manually by following the [step-by-step installation and setup guide with screenshots](docs/INSTALLATION.md).

## Documentation
- [Step by Step Setup and Configuration Guide with Screenshots](docs/INSTALLATION.md)
- [Beszel Hub - Setup and Configuration](beszel_hub/DOCS.md)
- [Beszel Hub (Test) - Setup and Configuration](beszel_hub_dev/DOCS.md)
- [Beszel Agent - Setup and Configuration](beszel_agent/DOCS.md)
- [Beszel Agent S.M.A.R.T. - Setup with S.M.A.R.T. Monitoring](beszel_agent_smart/DOCS.md)
- [Testing Instructions](docs/TESTING.md)
- [Contributing Guide](docs/CONTRIBUTING.md)
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Release Process](docs/RELEASE.md)

## Which Version Should I Use?

**Use Beszel Hub if:**
- You want to run the Beszel server directly inside Home Assistant
- You prefer a single-box setup for a small lab or home deployment

**Use Beszel Agent (S.M.A.R.T.) if:**
- You want to monitor disk health
- You need S.M.A.R.T. data from your drives

**Use Beszel Agent (Standard) if:**
- You just need basic system monitoring
- You want the smallest image size

**Use a Test variant if:**
- You want to validate upcoming Hub or Agent changes
- You are comfortable testing development builds before using them in production

## What's Beszel?

Beszel is a lightweight monitoring solution that keeps track of your server's vital signs. It's pretty efficient and doesn't hog resources. Check out the [main Beszel project](https://github.com/henrygd/beszel) to learn more.

## Getting Help

Having trouble? [Open an issue](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) and I'll try to help.

## License

MIT - see [LICENSE](LICENSE) for the boring legal stuff.

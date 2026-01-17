# Beszel Agent for Home Assistant

Monitor your Home Assistant system with Beszel. Tracks CPU, memory, disk, network, and Docker containers - lightweight and efficient.

## Available Add-ons

This repository provides three add-on variants:

1. **Beszel Agent** - Standard lightweight monitoring
2. **Beszel Agent Dev** - Development/testing version
3. **Beszel Agent (S.M.A.R.T.)** - With S.M.A.R.T. disk health monitoring

## Quick Start

Click this button to add the repository:

[![Open your Home Assistant instance and show the add add-on repository dialog with this repository URL pre-filled.](https://my.home-assistant.io/badges/supervisor_add_addon_repository.svg)](https://my.home-assistant.io/redirect/supervisor_add_addon_repository/?repository_url=https://github.com/vineetchoudhary/home-assistant-beszel-agent)

Or add it manually:
- Go to **Supervisor** → **Add-on Store** → **⋮** → **Repositories**
- Paste: `https://github.com/vineetchoudhary/home-assistant-beszel-agent`
- Find "Beszel Agent" in the store and hit Install

## Documentation
- [Beszel Agent - Setup and Configuration](beszel_agent/DOCS.md)
- [Beszel Agent S.M.A.R.T. - Setup with S.M.A.R.T. Monitoring](beszel_agent_smart/DOCS.md)
- [Step by Step Setup and Configuration Guide with Screenshots](docs/INSTALLATION.md)
- [Testing Instructions](docs/TESTING.md)
- [Contributing Guide](docs/CONTRIBUTING.md)
- [Architecture Overview](docs/ARCHITECTURE.md)
- [Release Process](docs/RELEASE.md)

## Which Version Should I Use?

**Use Beszel Agent (S.M.A.R.T.) if:**
- You want to monitor disk health (temperature, wear, errors)
- You need S.M.A.R.T. data from your drives

**Use Beszel Agent (Standard) if:**
- You just need basic system monitoring
- You want the smallest image size

## What's Beszel?

Beszel is a lightweight monitoring solution that keeps track of your server's vital signs. It's pretty efficient and doesn't hog resources. Check out the [main Beszel project](https://github.com/henrygd/beszel) to learn more.

## Getting Help

Having trouble? [Open an issue](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) and I'll try to help.

## License

MIT - see [LICENSE](LICENSE) for the boring legal stuff.
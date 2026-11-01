# Choose a Beszel app

Use one Hub to view readings and one Agent per monitored host. Choose an Agent for your hardware. Each app below links to its setup and options.

| App | Adds | Architectures |
| --- | --- | --- |
| [Hub](../beszel_hub/DOCS.md) | Web UI, API and database | amd64, aarch64 |
| [Agent](../beszel_agent/DOCS.md) | CPU, memory, disk, network and app usage | amd64, aarch64 |
| [S.M.A.R.T.](../beszel_agent_smart/DOCS.md) | System metrics and disk health | amd64, aarch64 |
| [Intel GPU](../beszel_agent_intel/DOCS.md) | Intel GPU metrics and disk health | amd64 |
| [AMD GPU](../beszel_agent_amd/DOCS.md) | Radeon metrics and disk health | amd64, aarch64 |
| [NVIDIA GPU](../beszel_agent_nvidia/DOCS.md) | NVIDIA metrics and disk health, restricted host support | amd64, aarch64 |
| [Bridge](../beszel_bridge/DOCS.md) | Home Assistant MQTT entities and dashboards | amd64, aarch64 |

`amd64` means 64-bit Intel/AMD, and `aarch64` means 64-bit ARM. All GPU variants include S.M.A.R.T. NVIDIA GPU monitoring is unavailable on Home Assistant OS and requires the host setup in its guide.

See [installation](INSTALLATION.md) for setup and Protection mode requirements, or [dashboards](dashboards/beszel/README.md) for Bridge templates.

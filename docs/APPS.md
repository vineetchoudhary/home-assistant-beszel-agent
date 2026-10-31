# Choose a Beszel app

Run a Hub inside Home Assistant or use one on another machine. Choose an Agent for the metrics your host supports. App names link to configuration guides.

| App | Adds | Architectures |
| --- | --- | --- |
| [Hub](../beszel_hub/DOCS.md) | Web UI, API and database | amd64, aarch64 |
| [Agent](../beszel_agent/DOCS.md) | Basic system metrics, smallest image | amd64, aarch64 |
| [S.M.A.R.T.](../beszel_agent_smart/DOCS.md) | System metrics and disk health | amd64, aarch64 |
| [Intel GPU](../beszel_agent_intel/DOCS.md) | Intel GPU metrics and disk health | amd64 |
| [AMD GPU](../beszel_agent_amd/DOCS.md) | Radeon metrics and disk health | amd64, aarch64 |
| [NVIDIA GPU](../beszel_agent_nvidia/DOCS.md) | NVIDIA metrics and disk health, restricted host support | amd64, aarch64 |
| [Bridge](../beszel_bridge/DOCS.md) | MQTT entities and dashboards in HA, development preview | amd64, aarch64 |

All GPU variants include S.M.A.R.T. NVIDIA GPU monitoring is unavailable on HA OS and requires a specially configured Supervised host. Read its guide before installing.

For setup and Protection mode requirements, see [installation](INSTALLATION.md). The Bridge remains unpublished pending [Supervisor acceptance](BRIDGE_READINESS.md). Its [dashboard templates](dashboards/beszel/README.md) use standard HA cards.

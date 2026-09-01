# How It Works

Since you're here, you probably want to know what's going on under the hood.

This repository ships two kinds of app:

- **Agent apps** collect metrics and report to a Beszel Hub
- **Hub apps** run the Beszel server itself - the web UI, the API, and the database

They are independent. You can run only agents and point them at a hub elsewhere, run only the hub and feed it from agents on other machines, or run both on the same Home Assistant box.

## Agent app

```
┌──────────────────────────────────────────────┐
│         Home Assistant Supervisor            │
│  ┌────────────────────────────────────────┐  │
│  │      Beszel Agent App Container     │  │
│  │  ┌──────────────────────────────────┐  │  │
│  │  │      bashio (config reader)      │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │       run.sh (entrypoint)        │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │       Beszel Agent Binary        │  │  │
│  │  │  (monitors system/containers)    │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │    HTTP Healthcheck Helper       │  │  │
│  │  │    (watchdog only, optional)     │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  └───────────────────┼────────────────────┘  │
│     Ports 45876 (agent) / 45877 (health)     │
└───────────────────────┼──────────────────────┘
                        │
                        ▼
               ┌──────────────────┐
               │    Beszel Hub    │
               │ (this app, or │
               │    elsewhere)    │
               └──────────────────┘
```

Variants:

| Directory | Adds | Base | Platforms |
| --- | --- | --- | --- |
| `beszel_agent` | - | Alpine | amd64, arm64 |
| `beszel_agent_smart` | smartmontools | Alpine | amd64, arm64 |
| `beszel_agent_intel` | `intel_gpu_top`, `nvtop`, smartmontools | Alpine | amd64 |
| `beszel_agent_amd` | `amdgpu.ids`, `nvtop`, smartmontools | Alpine | amd64, arm64 |
| `beszel_agent_nvidia` | `nvtop`, smartmontools | **Debian** | amd64, arm64 |

Two things drive those choices:

- `igt-gpu-tools`, which provides `intel_gpu_top`, is packaged for x86_64 only, so the Intel variant is amd64-only.
- The NVIDIA variant uses the Debian base because the NVIDIA Container Toolkit injects a glibc-linked `nvidia-smi` and driver libraries, which will not run against musl. It also pulls the `_glibc` agent build on amd64, since Beszel compiles its NVML collector for `linux/amd64` with glibc only. See [`beszel_agent_nvidia/DOCS.md`](../beszel_agent_nvidia/DOCS.md) for why this variant is unusable on Home Assistant OS.

## Hub app

```
┌──────────────────────────────────────────────┐
│         Home Assistant Supervisor            │
│  ┌────────────────────────────────────────┐  │
│  │       Beszel Hub App Container      │  │
│  │  ┌──────────────────────────────────┐  │  │
│  │  │      bashio (config reader)      │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │       run.sh (entrypoint)        │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │        Beszel Hub Binary         │  │  │
│  │  │   serve --http 0.0.0.0:8090      │  │  │
│  │  │   web UI + REST API + /api/health│  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │  /var/lib/beszel-hub/beszel_data │  │  │
│  │  │  SQLite DB + hub SSH key         │  │  │
│  │  │  (app data dir, backed up)    │  │  │
│  │  └──────────────────────────────────┘  │  │
│  └────────────────────────────────────────┘  │
│                  Port 8090                   │
└───────────────────────┼──────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        ▼                               ▼
┌────────────────┐             ┌────────────────┐
│  Agent app  │             │  Agents on     │
│  (same HA box) │             │  other hosts   │
└────────────────┘             └────────────────┘
```

One variant: `beszel_hub`, Alpine, amd64 + arm64.

Port `8090` serves the web UI, the REST API, and the `/api/health` endpoint the Home Assistant watchdog polls. The app is marked `backup: cold` so Home Assistant stops it while taking a backup, which keeps the SQLite database consistent in the snapshot.
# Beszel Agent (NVIDIA GPU) for Home Assistant

Monitor your Home Assistant system with Beszel, including **NVIDIA GPU** metrics and **S.M.A.R.T. disk health**.

> ## Read this before installing
>
> **NVIDIA GPU monitoring does not work on Home Assistant OS, and does not work on a default Supervised install.**
>
> Home Assistant apps cannot request the NVIDIA container runtime. The Supervisor has no `--gpus` flag, no `runtime` option, and no device-request option in its app schema, so there is no per-app way to attach a GPU. This is the same limitation that stops the Frigate app from supporting NVIDIA.
>
> This app works **only** on Home Assistant **Supervised** installs where you have made the NVIDIA runtime the Docker *default*:
>
> ```json
> {
>   "default-runtime": "nvidia",
>   "runtimes": {
>     "nvidia": {
>       "path": "nvidia-container-runtime",
>       "runtimeArgs": []
>     }
>   }
> }
> ```
>
> in `/etc/docker/daemon.json`, followed by a Docker restart. That makes the NVIDIA Container Toolkit inject the driver into *every* container on the host, including this one. It is a host-level change Home Assistant does not officially support, and it affects all your containers.
>
> On Home Assistant OS this is not possible at all: the OS is immutable and ships no NVIDIA drivers.
>
> **If you cannot do the above,** run upstream's `henrygd/beszel-agent-nvidia` container directly on the GPU machine and point it at your Hub. Everything else in this app (CPU, memory, disk, network, Docker, S.M.A.R.T.) still works normally without a GPU - it simply reports no GPU.

## What it monitors:

- CPU, memory, disk, and network usage
- Home Assistant Apps (via Docker API)
- NVIDIA GPU utilisation, temperature, power, and memory *(only under the conditions above)*
- S.M.A.R.T. disk health data
- Historical data with trends

## How to tell whether the GPU is visible

The app log prints its GPU status at startup. If the runtime injected the driver you will see:

```
INFO: ✓ nvidia-smi available
INFO:   - NVIDIA GeForce RTX 3060
```

If not, you get an explicit error block explaining why, and the agent carries on reporting everything else.

## NVIDIA GPU Monitoring

Beszel picks a GPU collector automatically. To pin one, set `GPU_COLLECTOR` under `environment_vars`:

```yaml
environment_vars:
  - name: "GPU_COLLECTOR"
    value: "nvidia-smi"
```

Supported values are `nvidia-smi`, `nvml`, and `nvtop`.

`nvml` is **amd64 only** - Beszel builds its NVML collector for `linux/amd64` with glibc, so it is unavailable on aarch64 regardless of your setup. Use `nvidia-smi` there.

This app is built on the Home Assistant Debian base rather than the Alpine one, because the toolkit injects a glibc-linked `nvidia-smi` and driver libraries that will not run against musl.

To turn GPU monitoring off entirely, set `SKIP_GPU` to `true`.

See the [upstream GPU guide](https://beszel.dev/guide/gpu) for the full collector reference.

## Requirements

**Disable Protection mode** for S.M.A.R.T. disk access - Home Assistant only honours `full_access` for an unprotected app. GPU visibility here depends on the host Docker runtime, not on protection mode.

## S.M.A.R.T. Monitoring

`smartmontools` is bundled, matching upstream's own NVIDIA image. Disks are detected automatically:

- SATA/SAS drives (`/dev/sd*`)
- NVMe drives (`/dev/nvme*`)

## Watchdog and Healthcheck

This app uses two internal ports:

- `45876/tcp` for the Beszel agent
- `45877/tcp` for a lightweight HTTP watchdog endpoint

## Installation and Setup

Follow the [Installation and Setup Guide](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md) to install the app.

## Need Help?

- [Report issues on GitHub](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues)
- [Check out Beszel docs](https://github.com/henrygd/beszel)

## License

MIT - see [LICENSE](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/LICENSE)

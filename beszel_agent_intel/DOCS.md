# Beszel Agent (Intel GPU) for Home Assistant

Monitor your Home Assistant system with Beszel, including **Intel GPU** metrics and **S.M.A.R.T. disk health**. This add-on runs the Beszel agent with `intel_gpu_top`, `nvtop`, and smartmontools, and reports stats to your Beszel Hub.

## What it monitors:

- CPU, memory, disk, and network usage
- Home Assistant Add-ons (via Docker API)
- Intel GPU utilisation, power, and frequency
- S.M.A.R.T. disk health data
- Historical data with trends

## Requirements

**This add-on is amd64 only.** The `intel_gpu_top` tool it depends on is not packaged for aarch64, and Intel GPUs are x86 hardware in practice.

**Protection mode.** What it gates is narrower than you might expect:

| Needs Protection mode off? | |
| --- | --- |
| `/dev/dri` render nodes (`video: true`) | No - Home Assistant applies the device rules either way |
| `PERFMON` capability, used by the i915 backend | No - capabilities are always applied |
| Disk access for S.M.A.R.T. (`full_access`) | **Yes** |
| Add-on stats from the Docker API (`docker_api`) | **Yes** |

So an older i915 Intel GPU may report fine with protection left on. **Arc and Xe GPUs, and S.M.A.R.T., need it off.** The startup log lists what the add-on can actually see.

## Intel GPU Monitoring

Beszel picks a GPU collector automatically. To pin one, set `GPU_COLLECTOR` under `environment_vars`:

```yaml
environment_vars:
  - name: "GPU_COLLECTOR"
    value: "intel_gpu_top"
```

Supported values on this add-on are `intel_gpu_top` and `nvtop`.

If you have more than one render node, point the agent at a specific device:

```yaml
environment_vars:
  - name: "INTEL_GPU_DEVICE"
    value: "drm:/dev/dri/card0"
```

To turn GPU monitoring off entirely, set `SKIP_GPU` to `true`.

The add-on log lists the render nodes it can see at startup, which is the quickest way to tell whether the container has GPU access.

See the [upstream GPU guide](https://beszel.dev/guide/gpu) for the full collector reference.

## S.M.A.R.T. Monitoring

`smartmontools` is bundled, matching upstream's own Intel image. Disks are detected automatically:

- SATA/SAS drives (`/dev/sd*`)
- NVMe drives (`/dev/nvme*`)

No manual configuration is required, but Protection mode must be off here too.

## Watchdog and Healthcheck

This add-on uses two internal ports:

- `45876/tcp` for the Beszel agent
- `45877/tcp` for a lightweight HTTP watchdog endpoint

## Installation and Setup

Follow the [Installation and Setup Guide](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md) to install the add-on.

## Need Help?

- [Report issues on GitHub](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues)
- [Check out Beszel docs](https://github.com/henrygd/beszel)

## License

MIT - see [LICENSE](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/LICENSE)

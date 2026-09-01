# Beszel Agent (AMD GPU) for Home Assistant

Monitor your Home Assistant system with Beszel, including **AMD GPU** metrics and **S.M.A.R.T. disk health**. This add-on runs the Beszel agent with the AMD GPU name database, `nvtop`, and smartmontools, and reports stats to your Beszel Hub.

## What it monitors:

- CPU, memory, disk, and network usage
- Home Assistant Add-ons (via Docker API)
- AMD GPU utilisation, temperature, power, and memory
- S.M.A.R.T. disk health data
- Historical data with trends

## Requirements

**Protection mode is only needed for S.M.A.R.T. here.** AMD GPU stats come from sysfs, and the `/dev/dri` device rules that `video: true` requests are applied whether or not protection is on - so GPU monitoring generally works with protection left enabled. Disk access for S.M.A.R.T. needs `full_access`, which Home Assistant honours only for an unprotected add-on.

Unlike the Intel variant, AMD monitoring needs no vendor tooling: Beszel's `amd_sysfs` collector reads the numbers straight out of `/sys/class/drm`. This add-on bundles `amdgpu.ids` so those cards get readable names instead of raw PCI ids, plus `nvtop` as an alternative collector.

The add-on log lists the `amdgpu` cards it can see at startup, which is the quickest way to tell whether the container has GPU access.

## AMD GPU Monitoring

Beszel picks a GPU collector automatically. To pin one, set `GPU_COLLECTOR` under `environment_vars`:

```yaml
environment_vars:
  - name: "GPU_COLLECTOR"
    value: "amd_sysfs"
```

Supported values on this add-on are `amd_sysfs` (recommended) and `nvtop`. Upstream also lists `rocm-smi`, but it is deprecated and is not bundled here.

To turn GPU monitoring off entirely, set `SKIP_GPU` to `true`.

See the [upstream GPU guide](https://beszel.dev/guide/gpu) for the full collector reference.

## S.M.A.R.T. Monitoring

`smartmontools` is bundled, matching upstream's own GPU images. Disks are detected automatically:

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

# Beszel Agent (NVIDIA GPU)

Monitor system metrics, NVIDIA GPU usage and S.M.A.R.T. disk health in Beszel.

## Requirements

Supports 64-bit Intel/AMD (**amd64**) and ARM (**aarch64**). Disable **Protection mode** for disk health and Home Assistant app statistics. NVIDIA GPU access also requires the host configuration below.

### NVIDIA host requirements

**NVIDIA GPU monitoring does not work on Home Assistant OS or a default Supervised installation.** It requires a Supervised host with NVIDIA drivers, the NVIDIA Container Toolkit and NVIDIA configured as Docker's default runtime.

Merge this into the host's `/etc/docker/daemon.json`, preserving its existing settings, then restart Docker:

```json
{
  "default-runtime": "nvidia",
  "runtimes": {
    "nvidia": {
      "path": "nvidia-container-runtime",
      "runtimeArgs": []
    }
  }
}
```

This changes the runtime for every container and is outside Home Assistant's supported setup. Alternatively, run upstream's `henrygd/beszel-agent-nvidia` container on the GPU host. Without GPU access, this app can still report its other metrics. Disabling Protection mode alone does not enable NVIDIA access.

## Setup

1. [Add this repository](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md#1-add-the-repository) and install **Beszel Agent (NVIDIA GPU)**. Use an existing Beszel Hub or install [Beszel Hub](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/beszel_hub/DOCS.md) first.
2. In the Hub, choose **Add System**. Copy its public key and token into the app's **Configuration**, then set `hub_url` to the Hub's address. If asked for the monitored host, use Home Assistant's IP address and port `45876`.
3. Set Protection mode as described above, save and start the app. Confirm live readings in the Hub. Save and restart after configuration changes.

## Configuration

Replace the public key and token with the complete values from **Add System**. Replace `192.168.1.100` with your Hub's IP address.

```yaml
key: "ssh-ed25519 REPLACE_WITH_YOUR_HUB_PUBLIC_KEY"
hub_url: "http://192.168.1.100:8090"
token: "replace-with-system-token"
environment_vars: []
custom_volumes: []
```

| Option | Default | Purpose |
| --- | --- | --- |
| `key` | `""`, required | Hub's complete SSH public key, including its `ssh-` prefix. |
| `hub_url` | `""`, required | Direct HTTP or HTTPS Hub URL, including its port and any base path. Home Assistant Ingress URLs do not work here. |
| `token` | `""`, required | Secret registration token from **Add System**. |
| `environment_vars` | `[]` | Optional Beszel settings, supplied as `name` and `value` pairs. |
| `custom_volumes` | `[]` | Optional checks for paths already visible inside the app. Leave empty for normal use. |

## Advanced options

### Environment variables

Add [Beszel Agent variables](https://www.beszel.dev/guide/environment-variables#agent) to one `environment_vars` list. Each entry needs a `name` and a nonempty `value`. Quote numbers and booleans. Invalid entries are skipped with a warning.

```yaml
environment_vars:
  - name: LOG_LEVEL
    value: "debug"
```

### Private HTTPS certificates

For a Hub with a private or self-signed CA, copy its PEM CA certificate into Home Assistant's `/ssl` folder and add this entry to `environment_vars`:

```yaml
environment_vars:
  - name: CA_CERT_FILE
    value: "/ssl/beszel-ca.crt"
```

Keep the Hub's HTTPS URL, key and token configured. Certificate and hostname verification remain enabled. An unreadable or invalid CA file stops startup. Leave `CA_CERT_FILE` unset for HTTP or publicly trusted HTTPS.

### Custom volume checks

**`custom_volumes` does not mount directories or grant disk access.** It only reports whether `container_path` exists inside the app. `host_path` is a label in the startup log. Both fields must be nonempty. The optional `:ro` or `:rw` suffix is ignored during the check.

```yaml
custom_volumes:
  - host_path: "/ssl"
    container_path: "/ssl:ro"
```

## NVIDIA GPU configuration

GPU detection is automatic. To override it, add these variables to `environment_vars`:

| Variable | Value |
| --- | --- |
| `GPU_COLLECTOR` | `nvidia-smi`, `nvml` or `nvtop`. `nvml` is amd64-only. Use `nvidia-smi` on aarch64. |
| `SKIP_GPU` | `"true"` to disable GPU monitoring. |

```yaml
environment_vars:
  - name: GPU_COLLECTOR
    value: "nvidia-smi"
```

See the [GPU guide](https://beszel.dev/guide/gpu) for collector details.

## S.M.A.R.T. configuration

SATA/SAS and NVMe disks are detected automatically. Check the startup log for detected drives. To select disks or specify device types, add this to your existing `environment_vars` list, using your own device paths:

```yaml
environment_vars:
  - name: SMART_DEVICES
    value: "/dev/nvme0:nvme,/dev/sda:sat"
```

Leave `SMART_DEVICES` unset for automatic detection. See the [S.M.A.R.T. guide](https://beszel.dev/guide/smart-data) for device types.

## Ports and backups

| Port | Purpose |
| --- | --- |
| `45876/tcp` | Agent connection from the Hub. |
| `45877/tcp` | Home Assistant Watchdog, when enabled. |

Run only one Agent variant at a time because they share ports. Include the app in Home Assistant backups. Keep credentials private when sharing configuration or logs.

## Troubleshooting

| Problem | Check |
| --- | --- |
| App will not start | Fill in `key`, `hub_url` and `token`, then check the app's Log tab. |
| No readings in the Hub | Check the Hub URL, public key, token and network connection. |
| HTTPS or `CA_CERT_FILE` error | Check the Hub hostname, PEM CA file and its `/ssl` path. |
| Missing app statistics | Disable Protection mode. |
| Missing temperatures | Check whether the host exposes sensors under `/sys/class/hwmon` or `/sys/class/thermal`. |
| Missing GPU readings | Check the NVIDIA runtime requirements and whether the startup log can query a GPU. |
| Missing disk health | Disable Protection mode and check detected drives and any `SMART_DEVICES` override. |
| Repeated Watchdog restarts | Check startup errors and port `45877`. If another service uses that port, disable **Watchdog** on the **Info** tab and restart. |

[Report unresolved problems](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) with app/HA versions and logs with credentials removed.

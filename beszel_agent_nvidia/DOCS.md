# Beszel Agent (NVIDIA GPU)

Monitor CPU, memory, disk, network and Home Assistant app usage, plus NVIDIA GPU metrics and S.M.A.R.T. disk health, in your Beszel Hub. GPU monitoring requires the host setup described below.

## Requirements

Supports **amd64 and aarch64**. Use an existing Beszel Hub on this Home Assistant host or another machine. **Disable Protection mode** for S.M.A.R.T. disk access and Docker app statistics. NVIDIA GPU access depends on the host's container runtime.

### NVIDIA host requirements

**NVIDIA GPU monitoring is unavailable on Home Assistant OS and default Supervised installations.** Home Assistant apps cannot request the NVIDIA container runtime.

GPU monitoring requires a Supervised host with working NVIDIA drivers, the NVIDIA Container Toolkit installed and its runtime configured as Docker's default. This fragment belongs in the host's `/etc/docker/daemon.json`, merged with its existing settings:

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

Restart Docker after changing it. This host-wide configuration affects every container and is outside Home Assistant's supported setup. Alternatively, run upstream's `henrygd/beszel-agent-nvidia` container directly on the GPU host and connect it to your Hub.

Without GPU access, this app can still report its other supported metrics.

## Setup

1. Add this repository and install **Beszel Agent (NVIDIA GPU)** from the Home Assistant app store. The [installation guide](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md) includes screenshots.
2. Open your existing Hub, or install and start **Beszel Hub** and create its first account at `http://<home-assistant-host>:8090`.
3. In the Hub, choose **Add System**. Copy its public key and token into this app's **Configuration**, then set `hub_url` to the Hub's reachable URL. Use this Home Assistant host's address and Agent port `45876` if the Hub asks for the monitored system's host and port.
4. Apply the Protection mode requirements above. Save the configuration and start the app. Save and restart after later configuration changes.
5. Check the app's **Log** for errors and confirm that the system reports live readings in the Hub.

## Configuration

Every option available in the app configuration is listed below. Replace the three connection values in this example. Leave the optional lists empty until you need them.

```yaml
key: "ssh-ed25519 REPLACE_WITH_YOUR_HUB_PUBLIC_KEY"
hub_url: "http://192.168.1.100:8090"
token: "replace-with-system-token"
environment_vars: []
custom_volumes: []
```

| Option | Default | Purpose |
| --- | --- | --- |
| `key` | Empty, required | The complete SSH public key from the Hub's **Add System** dialog. Keep the `ssh-` prefix and key contents. This is a public key, even though Home Assistant masks the field. |
| `hub_url` | Empty, required | The Hub's HTTP or HTTPS address, including its port and any configured base path. For the Hub app, use `http://<home-assistant-host>:8090`. Use the direct Hub address rather than the Home Assistant UI or an Ingress URL. |
| `token` | Empty, required | The Agent registration token from the Hub's **Add System** dialog. This is a secret and is separate from the SSH public key or a Home Assistant access token. |
| `environment_vars` | `[]` | Optional list of additional Beszel Agent environment variables. Each entry requires `name` and `value`, described below. |
| `custom_volumes` | `[]` | Optional list of path checks. Each entry requires `host_path` and `container_path`. This option cannot create mounts. |

The app refuses to start if `key`, `hub_url` or `token` is empty. Keep these connection settings in their dedicated fields.

## Shared options

### `environment_vars`

Use this list for supported [Beszel Agent environment variables](https://www.beszel.dev/guide/environment-variables#agent). Merge all optional entries into one list rather than repeating the `environment_vars` key.

| Entry field | Requirement |
| --- | --- |
| `name` | A nonempty variable name matching `[A-Za-z_][A-Za-z0-9_]*`, such as `LOG_LEVEL`. |
| `value` | A nonempty string. Quote numbers and booleans, such as `"30"` and `"true"`. |

```yaml
environment_vars:
  - name: LOG_LEVEL
    value: "debug"
```

Entries with empty fields or invalid names are skipped with a warning. Values are stored in app options and may be included in Home Assistant backups, so handle credentials accordingly.

### Private CA certificates

For an HTTPS Hub with a private or self-signed CA, put its PEM CA certificate in Home Assistant's `/ssl` folder, which this app mounts read-only. Add `CA_CERT_FILE` to your existing `environment_vars` list:

```yaml
hub_url: "https://beszel.example.com"
environment_vars:
  - name: CA_CERT_FILE
    value: "/ssl/beszel-ca.crt"
```

Keep your `key` and `token` configured. The certificate must match the Hub's certificate chain and hostname. An unreadable file or a file without a valid PEM certificate stops startup with a `CA_CERT_FILE` error. Leave the variable unset for plain HTTP or publicly trusted HTTPS.

### `custom_volumes`

This option checks whether paths are already visible inside the app. **It does not mount host directories or grant disk access.** Home Assistant controls mounts through the app's declared mappings and device permissions.

| Entry field | Requirement |
| --- | --- |
| `host_path` | A nonempty descriptive source path used in the startup log. The app does not mount or check this host path. |
| `container_path` | The nonempty path to check inside the app. A final `:ro` or `:rw` is stripped for the check and does not change access permissions. |

This example checks the `/ssl` mapping that the app already provides:

```yaml
custom_volumes:
  - host_path: "/ssl"
    container_path: "/ssl:ro"
```

Missing paths produce warnings. Leave `custom_volumes: []` for normal operation.

## NVIDIA GPU configuration

Collector selection is automatic. Add these optional variables to the same `environment_vars` list when you need an override:

| Variable | Setting |
| --- | --- |
| `GPU_COLLECTOR` | `nvidia-smi`, `nvml` or `nvtop`. Omit to use automatic selection. `nvml` is available only on amd64. Use `nvidia-smi` on aarch64. |
| `SKIP_GPU` | Set to `"true"` to disable GPU monitoring. Omit for normal GPU detection. |

```yaml
environment_vars:
  - name: GPU_COLLECTOR
    value: "nvidia-smi"
```

The Debian base supplies the glibc environment required by NVIDIA's injected tools. Check the startup log for `nvidia-smi available` followed by a detected GPU name. A missing tool or a failed GPU query indicates that the host runtime or driver access needs attention. See the [upstream GPU guide](https://beszel.dev/guide/gpu) for collector details.

## S.M.A.R.T. configuration

SATA/SAS (`/dev/sd*`) and NVMe (`/dev/nvme*`) disks are detected automatically. **Protection mode must be disabled** for disk access. Check the startup log for `smartctl` and the detected drives.

To select devices explicitly or supply a device type, add `SMART_DEVICES` to the existing `environment_vars` list. Replace these paths and types with those for your disks:

```yaml
environment_vars:
  - name: SMART_DEVICES
    value: "/dev/nvme0:nvme,/dev/sda:sat"
```

Leave this variable unset for automatic detection. See the [upstream S.M.A.R.T. guide](https://beszel.dev/guide/smart-data) for device-type details.

## Ports, watchdog and storage

| Port | Purpose |
| --- | --- |
| `45876/tcp` | Agent SSH listener, used when the Hub connects to this Agent. |
| `45877/tcp` | Home Assistant Watchdog endpoint at `/cgi-bin/health`. The helper starts only when Watchdog is enabled. |

The app uses host networking. Stop another Agent variant before starting this one so their listeners do not conflict. The watchdog helper always binds port `45877` and cannot be moved through app options. If that port is occupied, disable **Watchdog** on the app's **Info** tab and restart the app.

Home Assistant provides persistent app storage at `/var/lib/beszel-agent`. Include the app in Home Assistant backups to preserve its state.

## Troubleshooting

| Problem | Check |
| --- | --- |
| Required configuration error | Fill in all three connection fields, then save and start again. |
| Hub does not receive readings | Check the Hub URL, public key, token, network reachability and both the Agent and Hub logs. |
| HTTPS or `CA_CERT_FILE` errors | Check the Hub hostname, PEM CA file and its `/ssl` path. |
| Missing Home Assistant app statistics | Disable Protection mode to allow Docker API access. |
| Missing NVIDIA GPU readings | Confirm the host meets the NVIDIA runtime requirements and the startup log can query a GPU. Protection mode alone cannot provide NVIDIA access. |
| Missing disk health | Disable Protection mode, check detected drives and verify any `SMART_DEVICES` paths and types. |
| Repeated Watchdog restarts | Check the log for a port `45877` conflict or startup error. Disable Watchdog and restart if another service needs that port. |

See the [installation and troubleshooting guide](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md) for more detail, or [report an issue](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) with versions and redacted logs.

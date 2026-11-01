# Beszel Agent (Intel GPU)

Monitor system metrics, Intel GPU usage and S.M.A.R.T. disk health in Beszel.

## Requirements

Requires a 64-bit Intel/AMD host (**amd64**). Disable **Protection mode** for Intel Arc/Xe GPUs, disk health and Home Assistant app statistics. Older i915 GPUs may work with protection enabled.

## Setup

1. [Add this repository](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md#1-add-the-repository) and install **Beszel Agent (Intel GPU)**. Use an existing Beszel Hub or install [Beszel Hub](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/beszel_hub/DOCS.md) first.
2. In the Hub, choose **Add System**. Copy its public key and token into the app's **Configuration**, then set `hub_url` to the Hub's address. If asked for the monitored host, use Home Assistant's IP address and port `45876`.
3. Set Protection mode as described above, save and start the app. Confirm live readings in the Hub. Save and restart after configuration changes.

## Configuration

Replace the public key and token with the complete values from **Add System**. Replace `192.168.1.100` with your Hub's IP address.

```yaml
key: "ssh-ed25519 REPLACE_WITH_YOUR_HUB_PUBLIC_KEY"
key_file: ""
hub_url: "http://192.168.1.100:8090"
token: "replace-with-system-token"
environment_vars: []
custom_volumes: []
```

| Option | Default | Purpose |
| --- | --- | --- |
| `key` | `""` | Hub's complete SSH public key. For multiple Hubs, use one key per line. Required unless a key file is configured. |
| `key_file` | `""` | File of public keys inside the app, such as `/ssl/beszel_authorized_keys`. Takes precedence over `key`. |
| `hub_url` | `""`, required | Direct HTTP or HTTPS Hub URL, including its port and any base path. Home Assistant Ingress URLs do not work here. |
| `token` | `""`, required | Secret registration token from **Add System**. |
| `environment_vars` | `[]` | Optional Beszel settings, supplied as `name` and `value` pairs. |
| `custom_volumes` | `[]` | Optional checks for paths already visible inside the app. Leave empty for normal use. |

## Advanced options

### Multiple Hubs and key files

To authorize more than one Hub, use the app's YAML configuration editor and put each Hub's complete public key on its own line:

```yaml
key: |
  ssh-ed25519 REPLACE_WITH_PRIMARY_HUB_PUBLIC_KEY
  ssh-ed25519 REPLACE_WITH_SECOND_HUB_PUBLIC_KEY
```

Alternatively, copy the public keys into Home Assistant's `/ssl/beszel_authorized_keys` file, one key per line, and configure:

```yaml
key: ""
key_file: "/ssl/beszel_authorized_keys"
```

Both formats allow blank lines and comments starting with `#`. A configured `key_file` takes precedence over `key`. The file must be readable and contain public keys. Missing, empty or comment-only files stop startup. Save and restart the app after changing keys or file contents.

You can also set `KEY` or `KEY_FILE` through `environment_vars` to override the corresponding option. The upstream `BESZEL_AGENT_KEY` and `BESZEL_AGENT_KEY_FILE` names are supported too.

Keep `hub_url` and `token` configured for the primary Hub's outbound WebSocket connection. In each additional Hub, add the system using Home Assistant's IP address and port `45876`. That Hub must be able to reach the Agent over SSH.

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

Keep the Hub's HTTPS URL, public keys and token configured. Certificate and hostname verification remain enabled. An unreadable or invalid CA file stops startup. Leave `CA_CERT_FILE` unset for HTTP or publicly trusted HTTPS.

### Custom volume checks

**`custom_volumes` does not mount directories or grant disk access.** It only reports whether `container_path` exists inside the app. `host_path` is a label in the startup log. Both fields must be nonempty. The optional `:ro` or `:rw` suffix is ignored during the check.

```yaml
custom_volumes:
  - host_path: "/ssl"
    container_path: "/ssl:ro"
```

## Intel GPU configuration

GPU detection is automatic. To override it, add these variables to `environment_vars`:

| Variable | Value |
| --- | --- |
| `GPU_COLLECTOR` | `intel_gpu_top` or `nvtop`. |
| `INTEL_GPU_DEVICE` | Device for `intel_gpu_top`, such as `drm:/dev/dri/card0`. Choose a `/dev/dri` device listed in the startup log. |
| `SKIP_GPU` | `"true"` to disable GPU monitoring. |

```yaml
environment_vars:
  - name: GPU_COLLECTOR
    value: "intel_gpu_top"
  - name: INTEL_GPU_DEVICE
    value: "drm:/dev/dri/card0"
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
| App will not start | Configure `key` or `key_file`, plus `hub_url` and `token`, then check the app's Log tab. |
| No readings in the Hub | Check the Hub URL, public key, token and network connection. |
| HTTPS or `CA_CERT_FILE` error | Check the Hub hostname, PEM CA file and its `/ssl` path. |
| Missing app statistics | Disable Protection mode. |
| Missing temperatures | Check whether the host exposes sensors under `/sys/class/hwmon` or `/sys/class/thermal`. |
| Missing GPU readings | Check `/dev/dri` devices in the startup log and any collector/device overrides. Disable Protection mode for Arc/Xe GPUs. |
| Missing disk health | Disable Protection mode and check detected drives and any `SMART_DEVICES` override. |
| Repeated Watchdog restarts | Check startup errors and port `45877`. If another service uses that port, disable **Watchdog** on the **Info** tab and restart. |

[Report unresolved problems](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) with app/HA versions and logs with credentials removed.

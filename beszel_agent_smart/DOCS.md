# Beszel Agent (S.M.A.R.T.) for Home Assistant

Monitor your Home Assistant system with Beszel, including **S.M.A.R.T. disk health monitoring**. This add-on runs the Beszel agent with smartmontools and reports stats to your Beszel Hub.

## What it monitors:

- CPU, memory, disk, and network usage
- Home Assistant Add-ons (via Docker API)
- S.M.A.R.T. disk health data
- Historical data with trends

Lightweight design with built-in smartmontools support.

## S.M.A.R.T. Monitoring

This variant includes `smartmontools` for disk health monitoring. The add-on uses `full_access` mode to provide necessary permissions for accessing disk S.M.A.R.T. data. All disk devices are automatically detected and monitored:
- SATA/SAS drives (`/dev/sd*`)
- NVMe drives (`/dev/nvme*`)

No manual configuration required. You need to **disable the Protection mode** to allow the add-on to access disk devices for S.M.A.R.T. data. The add-on automatically provides access to all disk devices and Beszel Agent monitors all drives with S.M.A.R.T. capabilities.

### Verify S.M.A.R.T. Detection

After starting the add-on, check the logs to see detected drives:

```
S.M.A.R.T. Monitoring Status
✓ smartctl available for S.M.A.R.T. monitoring
Available drives detected:
  - /dev/sda
```

Your Beszel Hub will automatically display S.M.A.R.T. data for all detected drives. Learn more: https://www.beszel.dev/guide/smart-data

## Setup

1. Add repository: **Supervisor** → **Add-on Store** → **⋮** → **Repositories** → paste `https://github.com/vineetchoudhary/home-assistant-beszel-agent`
2. Install "Beszel Agent (S.M.A.R.T.)" from the store
3. Configure (see below)
4. Start the add-on

## Configuration Options

### Required Settings

**key** - SSH public key

Grab this from your Beszel Hub when you're adding a new system to monitor.

```yaml
key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample..."
```

**hub_url** - Beszel Hub URL

Where your Beszel Hub is running.

```yaml
hub_url: "http://192.168.1.100:8090"
```

**token** - Authentication token

Grab this from your Beszel Hub when you're adding a new system to monitor.

```yaml
token: "your-secret-token"
```

### Optional Settings

**environment_vars** - Extra environment variables

Need to tweak something? Set custom environment variables.

```yaml
environment_vars:
  - name: MY_VAR
    value: some_value
```

**custom_volumes** - Mount additional paths (including disk devices for S.M.A.R.T.)

Mount host directories or devices into the container:

```yaml
custom_volumes:
  - host_path: /media/external_drive
    container_path: /mnt/external
  - host_path: /dev/sda
    container_path: /dev/sda
```

Add `:ro` for read-only access:

```yaml
custom_volumes:
  - host_path: /some/path
    container_path: /mnt/data:ro
```

## Complete Example with S.M.A.R.T.

```yaml
key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample..."
hub_url: "http://192.168.1.100:8090"
token: "your-secret-token"
environment_vars:
  - name: LOG_LEVEL
    value: debug
custom_volumes:
  - host_path: /dev/sda
    container_path: /dev/sda
  - host_path: /dev/sdb
    container_path: /dev/sdb
  - host_path: /media/backup
    container_path: /mnt/backup:ro
```

### Protection Mode

You nneed to disable Protection mode to allow the add-on to access Docker stats, system info, and disk devices for S.M.A.R.T. data.

1. Open the add-on's **Configuration** tab
2. Turn off "Protection mode"
3. Restart the add-on

## Permissions

This add-on needs some special permissions to work:
- **Docker API** - to see container stats
- **Host Network** - for accurate network monitoring
- **Host D-Bus** - to get system info
- **Full Access** - to access disk devices for S.M.A.R.T. data

Don't worry, these are set up automatically when you install.

## Need Help?

- [Report issues on GitHub](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues)
- [Check out Beszel docs](https://github.com/henrygd/beszel)

## License

MIT - see [LICENSE](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/LICENSE)

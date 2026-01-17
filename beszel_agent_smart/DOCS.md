# Beszel Agent (S.M.A.R.T.) for Home Assistant

Monitor your Home Assistant system with Beszel, including **S.M.A.R.T. disk health monitoring**. This add-on runs the Beszel agent with smartmontools and reports stats to your Beszel Hub.

## What it monitors:

- CPU, memory, disk, and network usage
- Home Assistant Add-ons (via Docker API)
- S.M.A.R.T. disk health data (temperature, wear level, errors, etc.)
- Historical data with trends

Lightweight design with built-in smartmontools support.

## S.M.A.R.T. Monitoring

This variant includes `smartmontools` for disk health monitoring with AppArmor disabled and the required system capabilities (`SYS_RAWIO` and `SYS_ADMIN`) for accessing drive S.M.A.R.T. data.

### Configure Devices to Monitor

**You must specify which devices to monitor** in your add-on configuration:

```yaml
monitored_devices:
  - /dev/sda
  - /dev/nvme0n1
```

**Find your devices:**

Run this in Home Assistant SSH to list available drives:

```bash
lsblk
```

Or check specific device details:

```bash
ls /dev/sd* /dev/nvme* 2>/dev/null
```

**Leave empty to monitor all drives automatically.**

### Verify S.M.A.R.T. Detection

After starting the add-on, check the logs:

```
S.M.A.R.T. Monitoring Status
✓ smartctl available for S.M.A.R.T. monitoring
Auto-detected drives (all will be monitored):
  - /dev/sda
  - /dev/nvme0n1
```

Or with selective monitoring:

```
Monitoring configured devices:
  - /dev/sda
```

Your Beszel Hub will automatically display S.M.A.R.T. data for monitored drives.

**Learn more:** https://www.beszel.dev/guide/smart-data

## Setup

1. Add repository: **Supervisor** → **Add-on Store** → **⋮** → **Repositories** → paste `https://github.com/vineetchoudhary/home-assistant-beszel-agent`
2. Install "Beszel Agent (S.M.A.R.T.)" from the store
3. Configure (see below)
4. Start the add-on

## Configuration Options

### Required Settings

**key** - Your SSH public key

Grab this from your Beszel Hub when you're adding a new system to monitor.

```yaml
key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample..."
```

**hub_url** - Your Beszel Hub URL

Where your Beszel Hub is running.

```yaml
hub_url: "http://192.168.1.100:8090"
```

**token** - Authentication token

Get this from your Beszel Hub settings.

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

## Troubleshooting

### S.M.A.R.T. data not showing

1. Check add-on logs for detected drives
2. Verify device paths with `lsblk` command
3. Ensure devices are mounted in custom_volumes
4. Some drives may not support S.M.A.R.T. (check with `smartctl -a /dev/sda`)

### Agent won't connect

- Double-check your hub URL (include http:// or https://)
- Verify the SSH key matches what's in Beszel Hub
- Check that port 45876 isn't blocked
- Look at the add-on logs for clues

### Permission errors

If you get permission errors with mounted volumes:
- Ensure the host paths exist
- Check file/directory permissions on the host

## Regular vs S.M.A.R.T. Version

**Use this S.M.A.R.T. version if:**
- You want S.M.A.R.T. disk health monitoring
- You need smartmontools functionality

**Use the regular version if:**
- You don't need S.M.A.R.T. monitoring
- You want the smallest possible image size

## Links

- [Beszel Documentation](https://www.beszel.dev/)
- [S.M.A.R.T. Monitoring Guide](https://www.beszel.dev/guide/smart-data)
- [GitHub Repository](https://github.com/vineetchoudhary/home-assistant-beszel-agent)
- [Report Issues](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues)

## License

This project follows the same license as Beszel.

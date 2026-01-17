# Beszel Agent Dev - Development Version

⚠️ **This is a DEVELOPMENT version** - Use for testing new features and changes only. For production use, install the stable "Beszel Agent" add-on instead.

Monitor your Home Assistant system with Beszel. This add-on runs the Beszel agent and reports stats to your Beszel Hub.

## What makes this Dev version different:

- **Automatic builds** on every code change
- **Multiple image tags**: version tag, `:latest-dev`, and commit SHA
- **Bleeding edge features** that may not be fully tested
- **May have bugs** - not recommended for critical systems

## What it monitors:

- CPU, memory, disk, and network usage
- Home Assistant Add-ons (via Docker API)
- Historical data with trends

Lightweight design that won't slow down your system.

## Setup

1. Add repository: **Supervisor** → **Add-on Store** → **⋮** → **Repositories** → paste `https://github.com/vineetchoudhary/home-assistant-beszel-agent`
2. Install "Beszel Agent Dev" from the store
3. Configure (see below)
4. Start the add-on

## Image Tags

Dev images are tagged with:
- **Version tag**: e.g., `0.17.0-dev`
- **`:latest-dev`**: Always the most recent dev build
- **Commit SHA**: e.g., `:abc1234` - specific to each commit for rollback capability

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

### Optional Stuff

**environment_vars** - Extra environment variables

Need to pass custom environment variables? Add them here:

```yaml
environment_vars:
  - name: "LOG_LEVEL"
    value: "debug"
```

**custom_volumes** - Mount additional paths

Want to monitor extra directories or mount the system bus socket?

```yaml
custom_volumes:
  - host_path: "/var/run/dbus/system_bus_socket"
    container_path: "/var/run/dbus/system_bus_socket:ro"
  - host_path: "/mnt/data"
    container_path: "/mnt/data"
```

Add `:ro` for read-only, `:rw` (or nothing) for read-write.

### Full Example

Here's what a complete config might look like:

```yaml
key: "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExample..."
hub_url: "http://192.168.1.100:8090"
token: "my-secret-token"
environment_vars:
  - name: "LOG_LEVEL"
    value: "info"
custom_volumes:
  - host_path: "/var/run/dbus/system_bus_socket"
    container_path: "/var/run/dbus/system_bus_socket:ro"
```

### Protection Mode

You nneed to disable Protection mode to allow the add-on to access Docker stats and system info.

1. Open the add-on's **Configuration** tab
2. Turn off "Protection mode"
3. Restart the add-on

## Permissions

This add-on needs some special permissions to work:
- **Docker API** - to see container stats
- **Host Network** - for accurate network monitoring
- **Host D-Bus** - to get system info

Don't worry, these are set up automatically when you install.

## Need Help?

- [Report issues on GitHub](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues)
- [Check out Beszel docs](https://github.com/henrygd/beszel)

## License

MIT - see [LICENSE](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/LICENSE)


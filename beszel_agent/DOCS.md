# Beszel Agent for Home Assistant

Monitor your Home Assistant system with Beszel. This add-on runs the Beszel agent and reports stats to your Beszel Hub.

## What it monitors:

- CPU, memory, disk, and network usage
- Home Assistant Add-ons (via Docker API)
- Historical data with trends

Lightweight design that won't slow down your system.

## Setup

1. Add repository: **Supervisor** → **Add-on Store** → **⋮** → **Repositories** → paste `https://github.com/vineetchoudhary/ha-beszel-agent`
2. Install "Beszel Agent" from the store
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

## Permissions

This add-on needs some special permissions to work:
- **Docker API** - to see container stats
- **Host Network** - for accurate network monitoring
- **Host D-Bus** - to get system info

Don't worry, these are set up automatically when you install.

### Protection Mode

Try it first with default settings. If you see errors or missing container stats:

1. Open the add-on's **Configuration** tab
2. Turn off "Protection mode"
3. Restart the add-on

Most users need to disable Protection mode for Docker and D-Bus access.

## Need Help?

- [Report issues on GitHub](https://github.com/vineetchoudhary/ha-beszel-agent/issues)
- [Check out Beszel docs](https://github.com/henrygd/beszel)

## License

MIT - see [LICENSE](https://github.com/vineetchoudhary/ha-beszel-agent/blob/main/LICENSE)


# Beszel Agent for Home Assistant

Monitor your Home Assistant system with Beszel. This app runs the Beszel agent and reports stats to your Beszel Hub.

## What it monitors:

- CPU, memory, disk, and network usage
- Home Assistant Apps (via Docker API)
- Historical data with trends

## Watchdog and Healthcheck

This app uses two internal ports:

- `45876/tcp` for the Beszel agent
- `45877/tcp` for a lightweight HTTP watchdog endpoint

## Connecting to a Hub with a self-signed certificate

Beszel 0.19.0 made agents verify HTTPS certificates. If `hub_url` is an `https://` address served by a self-signed or otherwise untrusted certificate, the agent rejects the connection until you point it at the signing CA.

This app mounts Home Assistant's `/ssl` folder read-only, so put the CA certificate there and name it with `CA_CERT_FILE`:

```yaml
hub_url: "https://beszel.example.com"
environment_vars:
  - name: CA_CERT_FILE
    value: "/ssl/beszel-ca.crt"
```

The file must be PEM encoded. If `CA_CERT_FILE` is set but the file cannot be read or contains no valid certificate, the agent stops at startup rather than falling back to an unverified connection - the log records a `CA_CERT_FILE` error. Leave the setting unset if your Hub uses plain HTTP or a publicly trusted certificate.

## Installation and Setup

Follow the [Installation and Setup Guide](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md) to install the app.


## Need Help?

- [Report issues on GitHub](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues)
- [Check out Beszel docs](https://github.com/henrygd/beszel)

## License

MIT - see [LICENSE](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/LICENSE)

# Beszel Hub

Run the Beszel web UI, API and database inside Home Assistant. The Hub stores readings from Agents on this host or other machines. Supports **amd64 and aarch64**.

## Setup

1. Add this repository and install **Beszel Hub** from the Home Assistant app store. The [installation guide](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md) includes screenshots.
2. Open **Configuration** and set `app_url` to the Hub address that your Agents and notification recipients can reach, for example `http://192.168.1.100:8090`. Leave `environment_vars` empty unless you need an advanced setting.
3. Save and start the app. Open `http://<home-assistant-host>:8090` and create the first admin account when prompted.
4. In the Hub, choose **Add System**. Configure each Agent with the Hub's reachable URL, public key and token, then confirm live readings.
5. Use **Open Web UI** or add Beszel to the Home Assistant sidebar for Ingress access. Set up `ingress_user` below if you want automatic login through that panel.

Save and restart the app after configuration changes. Agents connect to the direct Hub API, so give them its own URL rather than a Home Assistant or Ingress URL.

## Configuration

Every option available in the app configuration is listed below. This example uses normal password login:

```yaml
app_url: "http://192.168.1.100:8090"
environment_vars: []
```

| Option | Default | Purpose |
| --- | --- | --- |
| `app_url` | `""` | The Hub's reachable HTTP or HTTPS URL. Used for notification links and Agent commands generated in the web UI. A URL path also sets the Hub's base path. |
| `ingress_user` | Unset | Optional email address of an existing Beszel account. Automatically signs Ingress visitors in as that account. Omit for normal login. |
| `environment_vars` | `[]` | Optional list of supported Beszel Hub environment variables. Each entry requires `name` and `value`. |

### `app_url`

Set this when you use notifications, a reverse proxy or Ingress. When left empty, the Hub uses its default URL behavior. Ingress can otherwise cause the **Add System** dialog to generate commands using the Home Assistant address, which Agents cannot use as a Hub API URL.

```yaml
app_url: "https://beszel.example.com"
```

**A path in this URL becomes the Hub's base path.** For example, `https://example.com/beszel` makes direct access use `http://<home-assistant-host>:8090/beszel/` rather than the root URL. Include that path in Agent and Bridge URLs too. Only set a path if your Hub is actually served from that subpath.

If you supply `APP_URL` or `BESZEL_HUB_APP_URL` through `environment_vars`, that environment setting overrides the `app_url` option. Prefer one place for this setting.

### `ingress_user`

Create the account in the Hub first, then add its email address in **Configuration**. You can use the YAML editor if the optional field is not shown:

```yaml
app_url: "http://192.168.1.100:8090"
ingress_user: "you@example.com"
environment_vars: []
```

Save, restart and reopen the Beszel panel. To restore normal login, remove `ingress_user` or clear its field, then save and restart. Unknown or invalid addresses fall back to the login screen.

**Anyone who can open the Ingress panel receives this account's access.** Treat the boundary as any logged-in Home Assistant user. A `readonly` account is useful for shared viewing. Use your admin account through the direct Hub URL when you need to make changes.

The internal Ingress proxy supplies a trusted authentication header that the Hub accepts only from its local proxy. Direct access on port `8090` and your own reverse proxies still require authentication unless you separately enable `AUTO_LOGIN`.

### `environment_vars`

Use this list for supported [Beszel Hub environment variables](https://www.beszel.dev/guide/environment-variables). Merge all entries into one list rather than repeating the `environment_vars` key.

| Entry field | Requirement |
| --- | --- |
| `name` | A nonempty variable name matching `[A-Za-z_][A-Za-z0-9_]*`, such as `SHARE_ALL_SYSTEMS`. |
| `value` | A nonempty string. Quote numbers and booleans, such as `"30"` and `"true"`. |

```yaml
environment_vars:
  - name: SHARE_ALL_SYSTEMS
    value: "true"
```

`SHARE_ALL_SYSTEMS` makes systems visible to all users, so add it only if that is your intended sharing policy. Variable names accept the `BESZEL_HUB_` prefix. The prefixed form takes precedence if both forms are set. Empty fields and invalid names are skipped with a warning.

Values are stored in plain text in app options and may be included in Home Assistant backups. Keep credentials private when sharing configuration or logs.

### Creating the first user with environment variables

The browser setup is the simplest way to create the first account. For an unattended first start, set both variables before the Hub initializes its database:

```yaml
environment_vars:
  - name: USER_EMAIL
    value: "admin@example.com"
  - name: USER_PASSWORD
    value: "replace-with-a-strong-password"
```

These variables are read by the first-run migration. Adding or changing them after initialization does not create another account or reset a password. Manage existing accounts in the Hub. Remove the password entry from app options after initialization if it is no longer needed.

## Automatic login on every access path

`ingress_user` limits automatic login to the Home Assistant panel. The upstream `AUTO_LOGIN` environment variable instead signs **every request** in as the chosen account, including direct access on published port `8090` and access through your reverse proxy:

```yaml
environment_vars:
  - name: AUTO_LOGIN
    value: "you@example.com"
```

The account must already exist. Save and restart to apply the setting, or remove the entry and restart to restore authentication. `AUTO_LOGIN` takes precedence over `ingress_user`, and setting both produces a startup warning.

For a separately managed authentication proxy, read the upstream reference before using `TRUSTED_AUTH_HEADER` and `TRUSTED_PROXY_IPS`. A trusted header without restricted proxy IPs allows clients on the direct port to impersonate users. When `ingress_user` is active, the app sets its own trusted header and restricts it to the local Ingress proxy.

## Ports, Ingress and backups

| Port | Purpose |
| --- | --- |
| `8090/tcp` | Hub web UI, API and `/api/health`. Published on host port `8090` by default. Used by Agents, direct browsers and the Home Assistant Watchdog. |
| Internal `8099` | Ingress proxy for the Home Assistant panel. This is not the Hub API port and is not published for external clients. |

If you change the published host port in **Configuration > Network**, use the new port in `app_url` and external Agent/Bridge URLs. The Hub continues listening on `8090` inside its app network. Clearing the published port prevents the Hub's Watchdog from reaching it, so disable Watchdog if you intentionally remove that mapping. Remote Agents still need a reachable API endpoint.

Ingress rewrites the Hub's page paths to Home Assistant's session-specific prefix. If the proxy cannot start, the app logs a warning and the Hub remains available through its direct port.

The database, SSH keys and uploads persist under `/var/lib/beszel-hub`, with the database directory at `/var/lib/beszel-hub/beszel_data`. Include the app in Home Assistant backups. Home Assistant stops it during backups for database consistency, so it is briefly unavailable. Preserve the existing data when restoring or moving a Hub.

## Troubleshooting

| Problem | Check |
| --- | --- |
| Ingress panel fails to load | Check the startup log and `app_url` base path, then try the direct Hub URL. |
| Generated Agent commands contain the Home Assistant URL | Set `app_url` to the Hub's reachable URL, save and restart. |
| Automatic Ingress login does not work | Confirm the email belongs to an existing Hub account and restart after changing `ingress_user`. |
| Direct port no longer asks for a password | Remove `AUTO_LOGIN` and its prefixed form from `environment_vars`, then restart. |
| First-user variables have no effect | The database is already initialized. Use the Hub's account management instead. |
| Watchdog restarts after changing network settings | Restore the published API port or disable Watchdog if intentionally removing it. |
| Agent cannot connect | Check the reachable API URL, base path, public key, token and logs on both sides. |

See the [installation guide](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md) for more detail, or [report an issue](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) with versions and redacted logs.

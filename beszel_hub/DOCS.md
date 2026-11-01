# Beszel Hub

Run the Beszel web interface and store readings from Agents on Home Assistant or other machines. Supports 64-bit Intel/AMD (**amd64**) and ARM (**aarch64**).

## Setup

1. [Add this repository](https://github.com/vineetchoudhary/home-assistant-beszel-agent/blob/main/docs/INSTALLATION.md#1-add-the-repository) and install **Beszel Hub**.
2. Set `app_url` in **Configuration** to the Hub's reachable address, such as `http://192.168.1.100:8090`. Replace the IP address with your Home Assistant host's IP.
3. Save and start the app. Open that address and create the first account.
4. In the Hub, choose **Add System** and use its public key and token to configure an Agent. Confirm live readings in the Hub.

Use **Open Web UI** or the Home Assistant sidebar for access through Ingress. Agents and the Bridge need the direct Hub URL. Save and restart after configuration changes.

## Configuration

```yaml
app_url: "http://192.168.1.100:8090"
environment_vars: []
```

| Option | Default | Purpose |
| --- | --- | --- |
| `app_url` | `""` | Reachable Hub URL for notification links and generated Agent commands. Include any base path. |
| `ingress_user` | Unset | Email of an existing Beszel account for automatic login through Home Assistant. Omit for normal login. |
| `environment_vars` | `[]` | Additional Beszel Hub settings as `name` and `value` pairs. |

### Hub URL and base paths

Set `app_url` when using Ingress, notifications or a reverse proxy so generated links use the Hub's address. For a Hub served at `https://example.com/beszel`, direct access becomes `http://<home-assistant-host>:8090/beszel/`. Include `/beszel` in Agent and Bridge URLs too.

An `APP_URL` or `BESZEL_HUB_APP_URL` entry in `environment_vars` overrides `app_url`. Use one place for this setting.

### Automatic login through Home Assistant

Create a Beszel account first, then set its email in `ingress_user`. Use the YAML editor if the optional field is not shown:

```yaml
app_url: "http://192.168.1.100:8090"
ingress_user: "you@example.com"
environment_vars: []
```

Save, restart and reopen the Beszel panel. Remove or clear `ingress_user` and restart to restore normal login. An unknown account falls back to the login screen.

**Anyone who can open the Home Assistant panel gets this account's access.** Use a `readonly` account for shared viewing. Direct access on port `8090` still requires login.

### Environment variables

Add [Beszel Hub variables](https://www.beszel.dev/guide/environment-variables) to one `environment_vars` list. Each entry needs a valid `name` and a nonempty string `value`. Quote numbers and booleans. Invalid entries are skipped with a warning.

```yaml
environment_vars:
  - name: SHARE_ALL_SYSTEMS
    value: "true"
```

`SHARE_ALL_SYSTEMS` makes every system visible to every user. Variables also accept a `BESZEL_HUB_` prefix, which takes precedence over the unprefixed form. Keep credentials private when sharing app options, logs or backups.

For an unattended first start, set both variables before the database is initialized:

```yaml
environment_vars:
  - name: USER_EMAIL
    value: "admin@example.com"
  - name: USER_PASSWORD
    value: "replace-with-a-strong-password"
```

These do not create accounts or reset passwords after initialization. Manage existing accounts in the Hub and remove the password entry from app options when it is no longer needed.

### Automatic login for all connections

The upstream `AUTO_LOGIN` variable signs every request in as an existing account, including direct port and reverse-proxy access:

```yaml
environment_vars:
  - name: AUTO_LOGIN
    value: "you@example.com"
```

This overrides `ingress_user` and removes the login prompt on every access path. Remove `AUTO_LOGIN` and any `BESZEL_HUB_AUTO_LOGIN` entry, then restart to restore authentication.

If using `TRUSTED_AUTH_HEADER` with a separate authentication proxy, also restrict `TRUSTED_PROXY_IPS` as described in the upstream reference. Without that restriction, clients on the direct port can impersonate users.

## Ports and backups

| Port | Purpose |
| --- | --- |
| `8090/tcp` | Web interface, Agent/Bridge API and Watchdog. Published as host port `8090` by default. |
| Internal `8099` | Home Assistant Ingress. Do not use it as an Agent or Bridge URL. |

If you change the host port under **Configuration > Network**, update `app_url` and external Agent/Bridge URLs. Internal app connections still use `8090`. If you remove the host port, disable **Watchdog**, which needs that mapping.

Include the app in Home Assistant backups to preserve its database, keys and uploads. Backups briefly stop the Hub. Restore the existing data when moving it to preserve history and Agent connections.

## Troubleshooting

| Problem | Check |
| --- | --- |
| Panel will not load | Check the app log and `app_url` base path, then try the direct Hub URL. |
| Generated commands use the Home Assistant URL | Set `app_url` to the Hub's address and restart. |
| Automatic panel login fails | Confirm `ingress_user` matches an existing account and restart. |
| Direct access no longer asks for a password | Remove `AUTO_LOGIN` and `BESZEL_HUB_AUTO_LOGIN`, then restart. |
| First-user variables have no effect | The database is already initialized. Use account management in the Hub. |
| Watchdog restarts after network changes | Restore the host port or disable Watchdog if you removed it intentionally. |
| Agent cannot connect | Check the direct Hub URL, base path, public key, token and both apps' logs. |

[Report unresolved problems](https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues) with app/HA versions and logs with credentials removed.

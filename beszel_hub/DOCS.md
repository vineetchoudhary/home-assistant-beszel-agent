# Beszel Hub for Home Assistant

Run the Beszel Hub directly inside Home Assistant. This app hosts the Beszel web UI and API so smaller setups do not need a separate Docker or Kubernetes deployment for the hub.

## What this app does

- Hosts the Beszel web UI and API on port `8090`
- Persists the hub database, SSH keys, and other runtime data in the app data directory
- Supports optional `APP_URL` configuration and additional Beszel Hub environment variables

## Configuration

### `app_url` (optional)

The public URL of this hub. Beszel uses it to build the links it puts in alert and notification emails, so set it if you use notifications - not only when you are behind a reverse proxy.

```yaml
app_url: "https://beszel.example.com"
```

**If the URL contains a path, that path becomes the hub's base path.** Setting `app_url: "https://example.com/beszel"` makes the hub serve its assets under `/beszel/`, and `http://<home-assistant-host>:8090/` will no longer load - you would have to browse to `http://<home-assistant-host>:8090/beszel/` instead. Only use a path when the hub really is served from a subpath.

Leave it empty for plain direct access on `http://<home-assistant-host>:8090`.

### `environment_vars` (optional)

Pass extra Beszel Hub environment variables when you need advanced features such as OAuth or trusted auth headers.

```yaml
environment_vars:
  - name: "SHARE_ALL_SYSTEMS"
    value: "true"
```

See the upstream Beszel environment variable reference for supported Hub options: https://www.beszel.dev/guide/environment-variables

Names may be given with or without Beszel's `BESZEL_HUB_` prefix; the prefixed form wins if you set both.

> **Values are stored in plain text** in the app options and are included in Home Assistant backups. Avoid putting long-lived secrets here where you can.

### Creating the first user

On its very first start - and only then - the hub reads `USER_EMAIL` and `USER_PASSWORD` to create the initial admin account:

```yaml
environment_vars:
  - name: "USER_EMAIL"
    value: "admin@example.com"
  - name: "USER_PASSWORD"
    value: "change-me"
```

These are read by a one-time database migration, so **adding them later has no effect** - once the database exists, change credentials from the web UI instead. If you leave them unset, the hub starts with no usable account and prompts you to create the first user in the browser, which is the simpler route.

## Getting started

1. Install and start the app.
2. Open the web UI from the Home Assistant sidebar, or directly at `http://<home-assistant-host>:8090`.
3. Create the first admin user when prompted.
4. Add systems from the Beszel UI and point Agents at this Hub.

## Notes

- The Home Assistant watchdog checks `http://[HOST]:8090/api/health`. It uses the same published port as the web UI, so clearing that port in the app's network settings also breaks the watchdog.
- The hub database, SSH key, and uploads live in the app data directory, which Home Assistant includes in backups. The app is set to `backup: cold`, so Home Assistant stops it for the duration of a backup - the hub is briefly unreachable, but the SQLite database is captured in a consistent state.
- The app is served through Home Assistant Ingress, so it appears in the sidebar and needs no port to be opened for the UI. An nginx front-end inside the app rewrites the paths Beszel bakes into its HTML to the per-session Ingress prefix on each request, so the changing Ingress token is handled automatically.
- Port `8090` stays published and is still needed: Agents connect to the Hub on it, and it remains available for direct access and reverse proxies. Ingress covers the browser UI only.
- **Set `app_url` if you use Ingress.** The "add system" dialog builds the Agent install commands from `app_url`, falling back to the address in your browser's URL bar. Viewed through Ingress that fallback is the Home Assistant address, which Agents cannot use to reach the Hub, so the generated commands would be wrong. Set `app_url` to the Hub's own reachable URL, for example `http://<home-assistant-host>:8090`.
- If the Ingress panel does not load, check the app log. Ingress is never fatal: when the proxy cannot start - for example because the path in `app_url` cannot be mapped to an Ingress prefix - the app logs a warning and carries on serving the Hub on port `8090`.
- Agents can run either as the apps in this repository or on external machines.

## Need Help?

- Report app issues on GitHub: https://github.com/vineetchoudhary/home-assistant-beszel-agent/issues
- Check the upstream Beszel docs: https://github.com/henrygd/beszel

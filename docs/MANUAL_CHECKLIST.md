# HA OS acceptance checklist

For contributors testing on a disposable or backed-up Home Assistant system. Record the app, HA OS, Supervisor, Core, Hub and architecture versions with each result. These manual checks track platform acceptance separately from the automated publishing checks.

- [ ] Install and configure the Bridge using the [setup guide](../beszel_bridge/DOCS.md#setup), with Protection mode enabled.
- [ ] Connect a local Hub by its internal hostname on `8090`, with its published port enabled and disabled. Verify base paths and access independent of Ingress.
- [ ] Connect a remote Hub over LAN and HTTPS, including a base path and a private CA in read-only `/ssl`. Reject untrusted certificates.
- [ ] Verify recorded Hub versions, readonly sharing, no shared systems, reduced permissions, session expiry and password changes. Check that newer version metadata is accepted and malformed fields leave other readings and servers working.
- [ ] Test Supervisor and manual MQTT settings, TLS, custom discovery prefixes, rejected credentials, delayed startup and HA rediscovery.
- [ ] Restart HA, Hub, broker and Bridge in different orders. Check recovery with fresh data, no duplicate devices and no Watchdog restart loop during upstream outages.
- [ ] Verify device grouping, metric units, source timestamps, valid zero values, missing sensors, system status and system/metric filters. Test pagination and partial or malformed responses.
- [ ] Rename servers and HA entities, change the same Hub's URL and restore a backup. Existing identity and customization must survive.
- [ ] Test Hub replacement, exclusions, inaccessible-system removal and cleanup with an offline old broker. Preserve unrelated MQTT topics.
- [ ] Import both dashboard templates on desktop and mobile using actual entity IDs. Check missing metrics, unavailable readings, newly collected history and coexistence with other integrations.
- [ ] Test native amd64 and aarch64 installations. Record emulation separately. Verify GPU and S.M.A.R.T. behavior on physical hardware before making hardware claims.
- [ ] Confirm existing Hub and Agent apps still install, start, connect and restore from backups.
- [ ] Resolve failures and include results in the release change. See [Bridge release checks](RELEASE.md#bridge-release-checks) for the automated publishing checks.

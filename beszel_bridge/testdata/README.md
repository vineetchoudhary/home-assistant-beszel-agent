# Hub API fixtures

Sanitized responses captured from disposable Hub/Agent containers using a `readonly` account with one shared system.

| Version | Upstream source commit |
| --- | --- |
| 0.20.0 | `a0dc19eacf6902d2f6934a3efd7ea7277198c40b` |
| 0.21.0 | `ec96c134660c998f7eb044e349e3715b65733fe1` |
| 0.21.1 | `e2ed147a5aa6472e6462521632ccc7101668132c` |
| 0.21.2 | `c97759f0ca4badc824ab0ff43f1571cc3c8f3a72` |

Each version includes `info.json`, `systems.json` and `system_stats.json`. Credentials and tokens are excluded. The 0.21.1 and 0.21.2 responses were captured from real Hub and Agent containers through a readonly account. The 0.21.2 baseline Agent uses `SENSORS=` to keep the absent-temperature fixture independent of host sensors. See [fixture format and coverage](../../docs/TESTING.md#bridge-fixtures) for sanitization, timestamps and derived test cases.

`expanded/monitoring.json` is a separate synthetic capability fixture for multiple filesystems, SMART, hardware sensors, containers, services, storage pools, probes, packages and account-owned alerts. It is not a capture from any Hub version or physical hardware. The Go tests and isolated HTTPS proxy use it to verify mapping and Home Assistant discovery.

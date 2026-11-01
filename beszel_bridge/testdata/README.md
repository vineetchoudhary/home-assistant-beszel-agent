# Hub API fixtures

Sanitized responses captured from disposable Hub/Agent containers using a `readonly` account with one shared system.

| Version | Upstream source commit |
| --- | --- |
| 0.20.0 | `a0dc19eacf6902d2f6934a3efd7ea7277198c40b` |
| 0.21.0 | `ec96c134660c998f7eb044e349e3715b65733fe1` |
| 0.21.1 | `e2ed147a5aa6472e6462521632ccc7101668132c` |

Each version includes `info.json`, `systems.json` and `system_stats.json`. Credentials and tokens are excluded. The 0.21.1 responses were captured from a real Hub and Agent through a readonly account. See [fixture format and coverage](../../docs/TESTING.md#bridge-fixtures) for sanitization, timestamps and derived test cases.

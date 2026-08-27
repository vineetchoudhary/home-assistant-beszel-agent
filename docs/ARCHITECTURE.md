# How It Works

Since you're here, you probably want to know what's going on under the hood.

This repository ships two kinds of add-on:

- **Agent add-ons** collect metrics and report to a Beszel Hub
- **Hub add-ons** run the Beszel server itself - the web UI, the API, and the database

They are independent. You can run only agents and point them at a hub elsewhere, run only the hub and feed it from agents on other machines, or run both on the same Home Assistant box.

## Agent add-on

```
┌──────────────────────────────────────────────┐
│         Home Assistant Supervisor            │
│  ┌────────────────────────────────────────┐  │
│  │      Beszel Agent Add-on Container     │  │
│  │  ┌──────────────────────────────────┐  │  │
│  │  │      bashio (config reader)      │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │       run.sh (entrypoint)        │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │       Beszel Agent Binary        │  │  │
│  │  │  (monitors system/containers)    │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │    HTTP Healthcheck Helper       │  │  │
│  │  │    (watchdog only, optional)     │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  └───────────────────┼────────────────────┘  │
│     Ports 45876 (agent) / 45877 (health)     │
└───────────────────────┼──────────────────────┘
                        │
                        ▼
               ┌──────────────────┐
               │    Beszel Hub    │
               │ (this add-on, or │
               │    elsewhere)    │
               └──────────────────┘
```

Variants: `beszel_agent` (standard), `beszel_agent_smart` (adds S.M.A.R.T. disk health), `beszel_agent_dev` (test builds).

## Hub add-on

```
┌──────────────────────────────────────────────┐
│         Home Assistant Supervisor            │
│  ┌────────────────────────────────────────┐  │
│  │       Beszel Hub Add-on Container      │  │
│  │  ┌──────────────────────────────────┐  │  │
│  │  │      bashio (config reader)      │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │       run.sh (entrypoint)        │  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │        Beszel Hub Binary         │  │  │
│  │  │   serve --http 0.0.0.0:8090      │  │  │
│  │  │   web UI + REST API + /api/health│  │  │
│  │  └────────────────┬─────────────────┘  │  │
│  │                   │                    │  │
│  │  ┌────────────────▼─────────────────┐  │  │
│  │  │  /var/lib/beszel-hub/beszel_data │  │  │
│  │  │  SQLite DB + hub SSH key         │  │  │
│  │  │  (add-on data dir, backed up)    │  │  │
│  │  └──────────────────────────────────┘  │  │
│  └────────────────────────────────────────┘  │
│                  Port 8090                   │
└───────────────────────┼──────────────────────┘
                        │
        ┌───────────────┼───────────────┐
        ▼                               ▼
┌────────────────┐             ┌────────────────┐
│  Agent add-on  │             │  Agents on     │
│  (same HA box) │             │  other hosts   │
└────────────────┘             └────────────────┘
```

Variants: `beszel_hub` (standard), `beszel_hub_dev` (test builds).

Port `8090` serves the web UI, the REST API, and the `/api/health` endpoint the Home Assistant watchdog polls. The add-on is marked `backup: cold` so Home Assistant stops it while taking a backup, which keeps the SQLite database consistent in the snapshot.

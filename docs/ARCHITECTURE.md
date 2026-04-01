# How It Works

Since you're here, you probably want to know what's going on under the hood.

```
┌──────────────────────────────────────────────┐
│         Home Assistant Supervisor           │
│  ┌──────────────────────────────────────┐  │
│  │      Beszel Agent Add-on Container    │  │
│  │  ┌──────────────────────────────┐  │  │
│  │  │      bashio (config reader)     │  │  │
│  │  └───────────────┬──────────────┘  │  │
│  │                 │                     │  │
│  │  ┌───────────────▼──────────────┐  │  │
│  │  │       run.sh (entrypoint)       │  │  │
│  │  └───────────────┬──────────────┘  │  │
│  │                 │                     │  │
│  │  ┌───────────────▼──────────────┐  │  │
│  │  │      Beszel Agent Binary        │  │  │
│  │  │  (monitors system/containers)   │  │  │
│  │  └───────────────┬──────────────┘  │  │
│  │                 │                     │  │
│  │  ┌───────────────▼──────────────┐  │  │
│  │  │   HTTP Healthcheck Helper      │  │  │
│  │  │   (watchdog only, optional)    │  │  │
│  │  └───────────────┬──────────────┘  │  │
│  │                 │                     │  │
│  └─────────────────┼─────────────────────┘  │
│      Ports 45876 (agent) / 45877 (health)  │
└─────────────────┼───────────────────────────┘
                 │
                 ▼
        ┌───────────────┐
        │   Beszel Hub   │
        │   (External)   │
        └───────────────┘
```

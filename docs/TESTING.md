# Testing

Use Docker and Git for container checks. Bridge tests also need Python 3/OpenSSL and Go 1.26+ or the script's Docker toolchain fallback. Real Supervisor and hardware checks use the [manual checklist](MANUAL_CHECKLIST.md).

## Test every app

```bash
./scripts/test-apps.sh --static
./scripts/test-apps.sh --supervisor
./scripts/test-apps.sh beszel_hub
./scripts/test-apps.sh --no-smoke
./scripts/test-apps.sh --keep
```

The script discovers app directories, architectures and base images. With no flags it runs static checks, builds and startup smoke tests. `--supervisor` also verifies bashio option loading and malformed environment handling through a mocked Supervisor. Run it for launcher changes. `--keep` retains built images.

Static checks cover YAML, versions, pins, images, shell/Dockerfile lint and Bridge Go tests/vet/formatting/builds. Package-pin lint exceptions are documented in Dockerfiles because the base tracks `latest`.

### Bridge entities and dashboards

```bash
./scripts/test-bridge.sh
./scripts/test-bridge.sh --hub-version 0.20.0
python3 scripts/test-bridge-e2e.py --build --hub-version 0.20.0
python3 scripts/test-bridge-e2e.py --build --hub-version 0.21.0
```

Focused tests cover declared schemas, authentication, pagination, TLS, freshness, optional metrics, identity, discovery ownership and Linux amd64/arm64 compilation. End-to-end tests use real Hub, Agent, Mosquitto and HA Core **2026.10.1** containers to verify actual devices/entities, units, customization, outages, restarts, dashboards and cleanup.

Without `--hub-version`, the checkout's Hub pin is used. Unsupported versions fail before setup. `--build` builds the selected Hub without changing its pin. Reports include the Bridge app version.

Use `--report .artifacts/bridge/report.json` for sanitized evidence. `--keep` retains the isolated environment and prints resource names. Its HA login is `bridge_lab` / `bridge-lab-only-password`. Remove those containers with `docker rm -fv`, their network and printed temporary directory when finished. Otherwise cleanup is automatic. Only the disposable HA UI is published on a random loopback port.

Container and mocked checks leave real Supervisor/physical-host acceptance open. Publishing additionally requires the [Bridge release gate](RELEASE.md#bridge-release-gate).

#### Bridge fixtures

[Versioned fixtures and source commits](../beszel_bridge/testdata/README.md) come from disposable Hub/Agent containers captured on 16 October 2026 using a normal `readonly` account with one shared system. IDs, names, hosts, user references and the public SSH key use deterministic replacements. Credentials and auth responses are excluded.

Each supported version needs `info.json` (`/api/beszel/info`), `systems.json` (inventory) and `system_stats.json` (latest `1m` sample). Preserve response shapes, values and timestamps. Freshness follows `system_stats.created`, never `systems.updated`. Zero temperature alone does not prove sensor support. Idle samples can omit `b` while retaining valid zero `ni` rates. Malformed, missing, stale and permission-limited cases are derived tests, not captured responses.

### End-to-end test

```bash
./scripts/test-e2e.sh
./scripts/test-e2e.sh --all-agents
./scripts/test-e2e.sh --all-agents --keep
./scripts/test-e2e.sh --clean
```

This builds a real Hub and Agent(s), creates a normal test user/token and verifies registration plus stored metrics. Each Agent gets a unique fingerprint because containers share the Docker host's ID.

`--keep` prints the Hub URL, login and teardown commands. Otherwise containers, images, network and seeded data are removed automatically. Use `--clean` after retained runs or interrupted cleanup. Physical GPU/disk behavior still needs hardware.

## Quick Docker test: Hub

```bash
docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/base:latest -t beszel-hub-test beszel_hub/
docker run --rm -it -p 8090:8090 -v /tmp/beszel-hub-data:/var/lib/beszel-hub beszel-hub-test
```

Open `http://127.0.0.1:8090` to create the first user, or check `/api/health`. For `APP_URL` base paths and GPU-specific requirements, use the [app guides](APPS.md).

## Building a multi-arch image locally

```bash
docker buildx create --name multiarch --driver docker-container --use
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg BUILD_FROM=ghcr.io/home-assistant/base:latest \
  -t <registry>/home-assistant-beszel-agent:test --push beszel_agent/
docker manifest inspect <registry>/home-assistant-beszel-agent:test
```

Multi-platform manifests require a registry and `docker-container` builder. For a local test, build one platform with `--load`. NVIDIA requires the Debian base, and Intel supports only amd64.

## Continuous integration

[Validation](../.github/workflows/validate.yml) runs static checks for pull requests and offers full build/smoke/mock-Supervisor checks manually. Publishing smoke-tests images again before pushing. Passing CI leaves the [real HA OS release checks](MANUAL_CHECKLIST.md) open.

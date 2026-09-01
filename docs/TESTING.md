# Testing Locally

## Requirements

- Docker and Git
- Home Assistant instance (for full testing)
- Beszel Hub (for end-to-end testing)

## Test in Home Assistant

1. **Clone it:**
```bash
git clone https://github.com/vineetchoudhary/home-assistant-beszel-agent.git
cd home-assistant-beszel-agent
```

2. **Add to Home Assistant:**
- **Supervisor** → **App Store** → **⋮** → **Repositories**
- Add: `file:///path/to/home-assistant-beszel-agent` (full path)

3. **Install it:**
- Refresh the app store page
- Find the app you want under local apps ("Beszel Agent", "Beszel Hub", ...)
- Hit Install and configure it
- Watch the logs for any problems

## Quick Docker Test

Don't have HA handy? Test the container directly:

**Build it:**
```bash
cd beszel_agent

docker build \
  --build-arg BUILD_FROM=ghcr.io/home-assistant/base:latest \
  -t beszel-agent-test .
```

`ghcr.io/home-assistant/base` is multi-arch, so this builds natively on both amd64 and ARM64 - the same base the release workflow uses. Add `--platform=linux/amd64` if you specifically want to test the amd64 image on an ARM machine.

**Run it:**
```bash
docker run --rm -it \
  --name beszel-agent-test \
  --network host \
  -v /tmp/beszel-test:/data \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -e BESZEL_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEVS7RCrzT7kxYP9+bcALqVpvX3apD8u7OOfwlGfYkXR test@github-actions" \
  -e BESZEL_HUB_URL="http://0.0.0.0" \
  -e BESZEL_TOKEN="test-token-for-ci" \
  beszel-agent-test
```


## Quick Docker Test - Hub

The Hub needs no configuration to boot, so a bare `docker run` is enough:

**Build it:**
```bash
docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/base:latest -t beszel-hub-test beszel_hub/
```

**Run it:**
```bash
docker run --rm -it --name beszel-hub-test -p 8090:8090 beszel-hub-test
```

**Check it:**
```bash
curl -fsS http://127.0.0.1:8090/api/health
```

You should get `{"message":"API is healthy.","code":200,"data":{}}`, and the logs should show `Server started at http://0.0.0.0:8090`. Open http://127.0.0.1:8090 and the UI will prompt you to create the first user.

To exercise the `app_url` path without a Supervisor, set the environment variable the Hub itself reads:

```bash
docker run --rm -it -p 8090:8090 -e APP_URL="https://beszel.example.com" beszel-hub-test
```

Note that a URL with a path (`https://example.com/beszel`) makes the Hub serve its assets under that path, so `http://127.0.0.1:8090/` will 404 on its JS bundle - browse to `http://127.0.0.1:8090/beszel/` instead. That is expected Beszel behaviour, not an app bug.

Data lands in `/var/lib/beszel-hub/beszel_data` inside the container. Mount a host directory there if you want it to survive `--rm`:

```bash
docker run --rm -it -p 8090:8090 -v /tmp/beszel-hub-data:/var/lib/beszel-hub beszel-hub-test
```


## Quick Docker Test - GPU variants

All three GPU variants are agents, so they take the same test environment variables as the standard agent. What differs is the base image and the tooling inside.

```bash
# Intel (amd64 only)
docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/base:latest \
  -t beszel-agent-intel-test beszel_agent_intel/

# AMD
docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/base:latest \
  -t beszel-agent-amd-test beszel_agent_amd/

# NVIDIA - Debian base, because the NVIDIA toolkit injects glibc binaries
docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/base-debian:latest \
  -t beszel-agent-nvidia-test beszel_agent_nvidia/
```

Run one the same way as the standard agent:

```bash
docker run --rm -it \
  -e BESZEL_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEVS7RCrzT7kxYP9+bcALqVpvX3apD8u7OOfwlGfYkXR test@github-actions" \
  -e BESZEL_HUB_URL="http://0.0.0.0" \
  -e BESZEL_TOKEN="test-token-for-ci" \
  beszel-agent-intel-test
```

Each variant prints a **GPU Monitoring Status** block at startup listing the tooling it found and the devices it can see. Without a real GPU passed into the container you will get warnings there and `no valid GPU data found` from the agent - that is the expected result of the smoke test, not a failure. The container should stay running.

To check the tooling is actually present:

```bash
docker run --rm beszel-agent-intel-test bash -c 'intel_gpu_top -h; nvtop --version; smartctl --version | head -1'
```

## Building a multi-arch image locally

Production images are single multi-arch manifests. To reproduce one you need a
`docker-container` builder and a registry to push to (a manifest list cannot be
loaded into the local image store):

```bash
docker buildx create --name multiarch --driver docker-container --use
docker buildx build --platform linux/amd64,linux/arm64 \
  --build-arg BUILD_FROM=ghcr.io/home-assistant/base:latest \
  -t <registry>/home-assistant-beszel-agent:test --push beszel_agent/
docker manifest inspect <registry>/home-assistant-beszel-agent:test
```

For a plain local test, build one platform with `--load` instead.


## Test Every App At Once

`scripts/test-apps.sh` builds and tests every app in the repository. It discovers them from the filesystem, reads each one's architectures from its `config.yaml`, and picks the Alpine or Debian base by looking at the Dockerfile - so a new `beszel_*` directory is covered automatically with nothing to update.

```bash
./scripts/test-apps.sh
```

That runs three phases:

| Phase | What it does |
| --- | --- |
| Static | YAML parses, required files present, versions in step with `beszel_agent`, no `{arch}` placeholder, image names unique, `bash -n`, shellcheck, hadolint |
| Build | Builds each image on the right base, natively where possible |
| Smoke | Runs each one: agents must reach port 45876 and stay up, hubs must answer `/api/health` |

The Dockerfiles carry `# hadolint ignore=DL3018` / `DL3008` directives with a comment explaining why: the Home Assistant base image tracks `:latest`, and both Alpine and Debian drop superseded package versions from their repositories, so a pinned `apk add pkg=1.2.3-r0` stops resolving the moment the base moves and the build breaks. Reproducibility comes from pinning the upstream Beszel release in each app's `beszel_version` file instead. Because those known findings are suppressed at the source, hadolint is expected to be clean and any new finding fails the run.

Useful variations:

```bash
./scripts/test-apps.sh --static           # fast, no Docker needed
./scripts/test-apps.sh --supervisor       # add the mock Supervisor test (below)
./scripts/test-apps.sh beszel_hub         # just one app
./scripts/test-apps.sh --keep             # leave the built images behind
```

It exits non-zero if anything fails, and cleans up its containers, images and networks on the way out - including when interrupted.

### The mock Supervisor test

The smoke tests use the `BESZEL_*` environment overrides, which bypass bashio entirely. `--supervisor` covers the path real users actually take: it stands up a fake Supervisor API, points the app at it with only `SUPERVISOR_TOKEN` set, and checks that

- the agent reads `hub_url`, `token` and `key` over the API, and the hub reads `app_url`
- a deliberately malformed `environment_vars` name is skipped with a warning rather than killing the app (`export` rejects it, and `run.sh` runs under `set -e`)
- the app is still running afterwards

This is the phase that catches configuration-reading regressions, so run it before changing anything in `run.sh`.

### End-to-end test

`scripts/test-apps.sh` proves each image starts. `scripts/test-e2e.sh` proves the product actually works - a real Hub app and a real Agent app, talking to each other:

```bash
./scripts/test-e2e.sh                        # hub + the standard agent
./scripts/test-e2e.sh --all-agents           # hub + every agent variant at once
./scripts/test-e2e.sh --all-agents --keep    # leave it all up to browse
./scripts/test-e2e.sh --clean                # remove what a previous run left
```

What it does, using only the app images this repository builds:

1. Starts the Hub with `USER_EMAIL` / `USER_PASSWORD`, which Beszel's first-run migration turns into an admin account
2. Authenticates against the Hub's REST API as that user - note it must be the regular user, because Beszel refuses to issue universal tokens to superusers
3. Reads the Hub's SSH public key from `/api/beszel/info`
4. Creates a universal registration token via `/api/beszel/universal-token`
5. Starts the Agent(s) pointed at the Hub with that key and token
6. Polls the Hub's `systems` collection until each agent appears with `status=up`
7. Confirms the stored record contains real metrics, not just a registration

A passing run ends like this:

```
PASS  e2e-beszel_agent: registered with the hub and is reporting (status=up)
PASS  hub stored real metrics: cpu=15.38% mem=8.5% disk=4.69% agent=0.18.8 threads=12
```

### Browsing the result

`--keep` combines with `--all-agents`, which is the useful way to look at a real multi-agent hub: every agent variant registers against the one Hub, and the run finishes by telling you how to reach it.

```
==> Environment left running (--keep)
  Hub UI       http://127.0.0.1:18301
  Login        e2e@example.com / e2e-test-password

  Systems reporting to this hub:
    e2e-beszel_agent             up
    e2e-beszel_agent_amd         up
    e2e-beszel_agent_intel       up
    e2e-beszel_agent_nvidia      up
    e2e-beszel_agent_smart       up
  ...
  Tear down with:
    docker rm -f ... && docker network rm ... && rm -rf ...
```

Open that URL, log in with those credentials, and you get the real Beszel dashboard with every app variant reporting into it.

The Hub is published on a free host port (starting at 18300) so it will not collide with a Beszel instance you already run. Everything else stays on a throwaway Docker network, and containers, images, the network and the seeded agent data are removed on exit unless you pass `--keep`.

A hard kill cannot run the cleanup trap, and `--keep` deliberately skips it, so `--clean` sweeps up whatever a previous run left behind:

```bash
./scripts/test-e2e.sh --clean
```

### Why each agent needs its own fingerprint

Beszel identifies a machine with `host.HostID()`, which every container on one Docker host shares. Point several agents at one hub unchanged and the hub correctly treats them as the same machine reconnecting, closing all but one connection. The script therefore seeds a unique `fingerprint` file in each agent's data directory, which `GetFingerprint()` prefers over the derived value. This is worth knowing outside the test too: you cannot run several agents on one host and expect several systems.


### Continuous integration

`.github/workflows/validate.yml` calls this same script, so CI and local runs cannot drift apart. Pull requests run the static phase; the full build, smoke and Supervisor run is available from the Actions tab via **Run workflow**, and `publish.yml` smoke tests every image again before pushing it to the registry.

## Resources

- [Home Assistant Add-on Development](https://developers.home-assistant.io/docs/add-ons)
- [Beszel Documentation](https://github.com/henrygd/beszel)
- [Docker Documentation](https://docs.docker.com/)

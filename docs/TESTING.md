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
- **Supervisor** → **Add-on Store** → **⋮** → **Repositories**
- Add: `file:///path/to/home-assistant-beszel-agent` (full path)

3. **Install it:**
- Refresh the add-on store page
- Find the add-on you want under local add-ons ("Beszel Agent", "Beszel Hub", ...)
- Hit Install and configure it
- Watch the logs for any problems

## Quick Docker Test

Don't have HA handy? Test the container directly:

**Build it:**
```bash
cd beszel_agent

docker build \
  --build-arg BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest \
  -t beszel-agent-test .
```

If your local Docker engine is running on Apple Silicon or another ARM64 machine, either use `ghcr.io/home-assistant/aarch64-base:latest` or add `--platform=linux/amd64` to the `docker build` command.

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
docker build --build-arg BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest -t beszel-hub-test beszel_hub/
```

On Apple Silicon or another ARM64 machine, use `ghcr.io/home-assistant/aarch64-base:latest` instead.

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

Note that a URL with a path (`https://example.com/beszel`) makes the Hub serve its assets under that path, so `http://127.0.0.1:8090/` will 404 on its JS bundle - browse to `http://127.0.0.1:8090/beszel/` instead. That is expected Beszel behaviour, not an add-on bug.

Data lands in `/var/lib/beszel-hub/beszel_data` inside the container. Mount a host directory there if you want it to survive `--rm`:

```bash
docker run --rm -it -p 8090:8090 -v /tmp/beszel-hub-data:/var/lib/beszel-hub beszel-hub-test
```


## Quick Test Script

Save this as `test-addon.sh`:

```bash
#!/bin/bash

ADDONS="beszel_agent beszel_agent_dev beszel_agent_smart beszel_hub beszel_hub_dev"

for addon in $ADDONS; do
  echo "🪪 Testing ${addon}..."

  echo "✓ Checking YAML..."
  python3 -c "import yaml; yaml.safe_load(open('${addon}/config.yaml'))" || exit 1

  echo "✓ Checking shell script..."
  bash -n "${addon}/run.sh" || exit 1
  if [ -f "${addon}/healthcheck-http.sh" ]; then
    sh -n "${addon}/healthcheck-http.sh" || exit 1
  fi

  echo "✓ Looking for required files..."
  for file in config.yaml Dockerfile run.sh DOCS.md CHANGELOG.md beszel_version icon.png logo.png; do
    if [ ! -f "${addon}/$file" ]; then
      echo "❌ Missing: ${addon}/$file"
      exit 1
    fi
    echo "  - ${addon}/$file"
  done

  echo "✓ Building Docker image..."
  docker build \
    --build-arg BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest \
    -t "${addon//_/-}-test" \
    "${addon}/" || exit 1

  echo "✓ Checking image size..."
  docker images "${addon//_/-}-test" --format "{{.Size}}"
  echo ""
done

echo "✅ All tests passed!"
echo ""
echo "To test interactively:"
echo "  docker run --rm -it --entrypoint /bin/bash beszel-agent-test"
echo "To test the agent watchdog endpoint in the container:"
echo "  wget -S -O- http://127.0.0.1:45877/cgi-bin/health"
echo "To test the hub health endpoint:"
echo "  curl -fsS http://127.0.0.1:8090/api/health"
```

Make it executable and run:
```bash
chmod +x test-addon.sh
./test-addon.sh
```

## Resources

- [Home Assistant Add-on Development](https://developers.home-assistant.io/docs/add-ons)
- [Beszel Documentation](https://github.com/henrygd/beszel)
- [Docker Documentation](https://docs.docker.com/)

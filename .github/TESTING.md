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
- Find "Beszel Agent" under local add-ons
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

**Make a fake config:**
```bash
mkdir -p /tmp/beszel-test

cat > /tmp/beszel-test/options.json << 'EOF'
{
  "key": "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAITestKeyHere",
  "hub_url": "http://localhost:8090",
  "token": "",
  "environment_vars": [],
  "custom_volumes": []
}
EOF
```

**Run it:**
```bash
docker run --rm -it \
  --name beszel-agent-test \
  --network host \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v /tmp/beszel-test:/data \
  beszel-agent-test
```


## Quick Test Script

Save this as `test-addon.sh`:

```bash
#!/bin/bash

echo \"🧪 Testing Beszel Agent...\"

echo \"✓ Checking YAML...\"
python3 -c \"import yaml; yaml.safe_load(open('beszel_agent/config.yaml'))\" || exit 1

echo \"✓ Checking shell script...\"
bash -n beszel_agent/run.sh || exit 1

echo \"✓ Looking for required files...\"
for file in config.yaml Dockerfile run.sh DOCS.md; do
  if [ ! -f \"beszel_agent/$file\" ]; then
    echo \"❌ Missing: $file\"
    exit 1
  fi
  echo "  - beszel_agent/$file"
done

echo "✓ Building Docker image..."
docker build \
  --build-arg BUILD_FROM=ghcr.io/home-assistant/amd64-base:latest \
  -t beszel-agent-test \
  beszel_agent/ || exit 1

echo "✓ Checking image size..."
docker images beszel-agent-test --format "{{.Size}}"

echo ""
echo "✅ All tests passed!"
echo ""
echo "To test interactively:"
echo "  docker run --rm -it --entrypoint /bin/bash beszel-agent-test"
```

Make it executable and run:
```bash
chmod +x test-addon.sh
./test-addon.sh
```


## Resources

- [Home Assistant Add-on Development](https://developers.home-assistant.io/docs/add-ons)
- [Home Assistant Builder](https://github.com/home-assistant/builder)
- [Beszel Documentation](https://github.com/henrygd/beszel)
- [Docker Documentation](https://docs.docker.com/)
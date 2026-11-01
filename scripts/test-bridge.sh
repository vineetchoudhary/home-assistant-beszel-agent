#!/bin/sh
# Check the Bridge, its captured Hub schemas, and both Linux build targets.
set -eu

bridge_repo_root="$(cd "$(dirname "$0")/.." && pwd)"

if [ "${1:-}" = "--module" ]; then
    cd "${bridge_repo_root}/beszel_bridge"
    bridge_unformatted="$(gofmt -l ./*.go)"
    if [ -n "${bridge_unformatted}" ]; then
        printf 'Go files need formatting:\n%s\n' "${bridge_unformatted}" >&2
        exit 1
    fi
    go test -mod=readonly -count=1 ./...
    go vet -mod=readonly ./...
    bridge_build_tmp="$(mktemp -d)"
    trap 'rm -rf "${bridge_build_tmp}"' EXIT HUP INT TERM
    for bridge_arch in amd64 arm64; do
        CGO_ENABLED=0 GOOS=linux GOARCH="${bridge_arch}" \
            go build -mod=readonly -trimpath -o "${bridge_build_tmp}/bridge-${bridge_arch}" .
    done
    printf 'Bridge tests, vet, formatting, and amd64/arm64 compilation passed.\n'
    exit 0
fi

if [ "${1:-}" = "--hub-version" ]; then
    [ "$#" -eq 2 ] || { echo 'Usage: test-bridge.sh [--hub-version VERSION]' >&2; exit 2; }
    python3 - "${bridge_repo_root}/beszel_bridge/compatibility.json" "$2" <<'PY'
import json, re, sys
policy = json.load(open(sys.argv[1]))
if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:[-+][A-Za-z0-9.-]+)?', sys.argv[2]):
    sys.exit('Expected a Hub release version such as 0.21.1')
coverage = 'recorded' if sys.argv[2] in policy['tested_hub_versions'] else 'not yet recorded'
print('Hub ' + sys.argv[2] + ': test coverage ' + coverage + '. Versions are not runtime-gated.')
PY
elif [ "$#" -gt 0 ]; then
    echo 'Usage: test-bridge.sh [--hub-version VERSION]' >&2
    exit 2
fi

if command -v go >/dev/null 2>&1; then
    exec sh "$0" --module
fi
if ! docker info >/dev/null 2>&1; then
    echo 'Bridge checks need Go 1.26+ or a running Docker daemon.' >&2
    exit 2
fi
exec docker run --rm \
    -v "${bridge_repo_root}:/work:ro" \
    -v "${TMPDIR:-/tmp}/beszel-bridge-go-mod:/go/pkg/mod" \
    -v "${TMPDIR:-/tmp}/beszel-bridge-go-build:/root/.cache/go-build" \
    -w /work \
    golang:1.26-alpine@sha256:3082400e369fa24d5fc60bca20edab3f6d604e0c5a690ec66b295eff4dd87ade \
    sh /work/scripts/test-bridge.sh --module

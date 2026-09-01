#!/usr/bin/env bash
#
# Build and test every Beszel add-on in this repository.
#
# Everything is derived from the add-on directories themselves - the arch list
# from config.yaml, the base image from whether the Dockerfile uses apt-get - so
# adding a new beszel_* directory is picked up automatically with no edits here.
#
#   ./scripts/test-addons.sh                 # static checks + build + smoke test
#   ./scripts/test-addons.sh --static        # no Docker, just the fast checks
#   ./scripts/test-addons.sh --supervisor    # also run the mock Supervisor test
#   ./scripts/test-addons.sh beszel_hub      # only these add-ons
#
# Written for bash 3.2 so it runs on stock macOS as well as CI.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}" || exit 1

# ---------------------------------------------------------------------------
# Options
# ---------------------------------------------------------------------------
DO_STATIC=1
DO_BUILD=1
DO_SMOKE=1
DO_SUPERVISOR=0
KEEP_IMAGES=0
SELECTED=""

usage() {
    sed -n '2,18p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    cat <<'EOF'

Options:
  --static        Static checks only (no Docker build or run)
  --no-smoke      Build images but do not run them
  --supervisor    Additionally run the mock Supervisor integration test
  --keep          Keep the built test images instead of removing them
  -h, --help      Show this help
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --static)     DO_BUILD=0; DO_SMOKE=0 ;;
        --no-smoke)   DO_SMOKE=0 ;;
        --supervisor) DO_SUPERVISOR=1 ;;
        --keep)       KEEP_IMAGES=1 ;;
        -h|--help)    usage; exit 0 ;;
        -*)           echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
        *)            SELECTED="${SELECTED} ${1%/}" ;;
    esac
    shift
done

# ---------------------------------------------------------------------------
# Output helpers
# ---------------------------------------------------------------------------
if [ -t 1 ]; then
    C_RED=$'\033[31m'; C_GRN=$'\033[32m'; C_YEL=$'\033[33m'
    C_BLU=$'\033[34m'; C_DIM=$'\033[2m';  C_OFF=$'\033[0m'
else
    C_RED=""; C_GRN=""; C_YEL=""; C_BLU=""; C_DIM=""; C_OFF=""
fi

FAILURES=0
SKIPPED=0
RESULTS=""

pass()  { printf '  %sPASS%s  %s\n' "${C_GRN}" "${C_OFF}" "$*"; }
fail()  { printf '  %sFAIL%s  %s\n' "${C_RED}" "${C_OFF}" "$*"; FAILURES=$((FAILURES + 1)); }
skip()  { printf '  %sSKIP%s  %s\n' "${C_YEL}" "${C_OFF}" "$*"; SKIPPED=$((SKIPPED + 1)); }
info()  { printf '  %s%s%s\n' "${C_DIM}" "$*" "${C_OFF}"; }
head1() { printf '\n%s==> %s%s\n' "${C_BLU}" "$*" "${C_OFF}"; }

record() { RESULTS="${RESULTS}$1|$2|$3
"; }

# ---------------------------------------------------------------------------
# Discovery
# ---------------------------------------------------------------------------
ADDONS=""
while IFS= read -r dir; do
    [ -n "${dir}" ] || continue
    if [ -n "${SELECTED## }" ]; then
        case " ${SELECTED} " in *" ${dir} "*) ;; *) continue ;; esac
    fi
    ADDONS="${ADDONS} ${dir}"
done <<EOF
$(find . -maxdepth 2 -name config.yaml -path './beszel_*' -exec dirname {} \; | sed 's|^\./||' | sort)
EOF

if [ -z "${ADDONS## }" ]; then
    echo "No add-ons matched." >&2
    exit 2
fi

# The value of `arch:` in config.yaml, space separated.
addon_arches() {
    python3 -c "
import yaml,sys
print(' '.join(yaml.safe_load(open(sys.argv[1]+'/config.yaml'))['arch']))" "$1"
}

# Debian base if the Dockerfile installs with apt-get, Alpine otherwise. Derived
# rather than configured so the two cannot drift apart.
addon_base() {
    if grep -q 'apt-get' "$1/Dockerfile"; then
        echo "ghcr.io/home-assistant/base-debian:latest"
    else
        echo "ghcr.io/home-assistant/base:latest"
    fi
}

addon_kind() {
    case "$1" in
        beszel_hub*) echo "hub" ;;
        *)           echo "agent" ;;
    esac
}

HOST_ARCH="$(uname -m)"
case "${HOST_ARCH}" in
    x86_64|amd64)  HOST_HA_ARCH="amd64" ;;
    arm64|aarch64) HOST_HA_ARCH="aarch64" ;;
    *)             HOST_HA_ARCH="" ;;
esac

ha_arch_to_platform() {
    case "$1" in
        amd64)   echo "linux/amd64" ;;
        aarch64) echo "linux/arm64" ;;
        *)       echo "" ;;
    esac
}

# Prefer the host architecture so the build is native; fall back to the first
# architecture the add-on supports (emulated, and noted as such).
addon_platform() {
    _arches="$(addon_arches "$1")"
    for a in ${_arches}; do
        if [ "${a}" = "${HOST_HA_ARCH}" ]; then
            ha_arch_to_platform "${a}"
            return 0
        fi
    done
    # shellcheck disable=SC2086  # deliberate word splitting
    set -- ${_arches}
    ha_arch_to_platform "$1"
}

TEST_KEY="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIEVS7RCrzT7kxYP9+bcALqVpvX3apD8u7OOfwlGfYkXR test@test-addons"

# ---------------------------------------------------------------------------
# Cleanup
# ---------------------------------------------------------------------------
CREATED_CONTAINERS=""
CREATED_IMAGES=""
CREATED_NETWORK=""
TMPDIR_TEST=""

# shellcheck disable=SC2329  # invoked via the EXIT/INT/TERM trap
cleanup() {
    for c in ${CREATED_CONTAINERS}; do docker rm -f "${c}" >/dev/null 2>&1; done
    if [ "${KEEP_IMAGES}" -eq 0 ]; then
        for i in ${CREATED_IMAGES}; do docker rmi "${i}" >/dev/null 2>&1; done
    fi
    [ -n "${CREATED_NETWORK}" ] && docker network rm "${CREATED_NETWORK}" >/dev/null 2>&1
    [ -n "${TMPDIR_TEST}" ] && rm -rf "${TMPDIR_TEST}"
    return 0
}
trap cleanup EXIT INT TERM

have_docker() { docker info >/dev/null 2>&1; }

free_port() {
    _p="$1"
    while nc -z 127.0.0.1 "${_p}" >/dev/null 2>&1; do _p=$((_p + 1)); done
    echo "${_p}"
}

# ---------------------------------------------------------------------------
# Static checks
# ---------------------------------------------------------------------------
if [ "${DO_STATIC}" -eq 1 ]; then
    head1 "Static checks"

    REFERENCE_VERSION="$(grep '^version:' beszel_agent/config.yaml | sed -E 's/version: "(.*)"/\1/')"
    info "reference version (beszel_agent): ${REFERENCE_VERSION}"

    for addon in ${ADDONS}; do
        ok=1

        if ! python3 -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))" "${addon}/config.yaml" 2>/dev/null; then
            fail "${addon}: config.yaml is not valid YAML"; ok=0
        fi

        for f in config.yaml Dockerfile run.sh DOCS.md CHANGELOG.md beszel_version icon.png logo.png; do
            if [ ! -f "${addon}/${f}" ]; then
                fail "${addon}: missing ${f}"; ok=0
            fi
        done

        if ! bash -n "${addon}/run.sh" 2>/dev/null; then
            fail "${addon}: run.sh has a syntax error"; ok=0
        fi

        v="$(grep '^version:' "${addon}/config.yaml" | sed -E 's/version: "(.*)"/\1/')"
        if [ "${v%-dev}" != "${REFERENCE_VERSION}" ]; then
            fail "${addon}: version ${v} does not match beszel_agent ${REFERENCE_VERSION}"; ok=0
        fi

        bv="$(tr -d '[:space:]' < "${addon}/beszel_version")"
        rbv="$(tr -d '[:space:]' < beszel_agent/beszel_version)"
        if [ "${bv}" != "${rbv}" ]; then
            fail "${addon}: beszel_version ${bv} does not match beszel_agent ${rbv}"; ok=0
        fi

        if grep -q '{arch}' "${addon}/config.yaml"; then
            fail "${addon}: config.yaml still uses the {arch} placeholder"; ok=0
        fi

        [ "${ok}" -eq 1 ] && pass "${addon}: config, files, version, image name"
    done

    # Image names must be unique, or two add-ons would overwrite each other.
    dupes="$(grep -h '^image:' beszel_*/config.yaml | sort | uniq -d)"
    if [ -n "${dupes}" ]; then
        fail "duplicate image names: ${dupes}"
    else
        pass "all image names are unique"
    fi

    if command -v shellcheck >/dev/null 2>&1; then
        if shellcheck -e SC1008 beszel_*/run.sh beszel_*/healthcheck-http.sh; then
            pass "shellcheck"
        else
            fail "shellcheck reported problems"
        fi
    elif have_docker; then
        if docker run --rm -v "${REPO_ROOT}:/mnt" -w /mnt koalaman/shellcheck:stable \
                -e SC1008 beszel_*/run.sh beszel_*/healthcheck-http.sh; then
            pass "shellcheck (via docker)"
        else
            fail "shellcheck reported problems"
        fi
    else
        skip "shellcheck (not installed, Docker unavailable)"
    fi

    # The Dockerfiles carry documented `hadolint ignore=` directives for the
    # version-pinning rules, so a clean run is the expected state and any new
    # finding is a real one worth failing on.
    HADOLINT=""
    if command -v hadolint >/dev/null 2>&1; then
        HADOLINT="hadolint"
    elif have_docker; then
        HL_IMAGE="hadolint/hadolint"
        if docker image inspect "${HL_IMAGE}" >/dev/null 2>&1 ||
                docker pull "${HL_IMAGE}" >/dev/null 2>&1; then
            HADOLINT="docker run --rm -i ${HL_IMAGE}"
        fi
    fi
    if [ -n "${HADOLINT}" ]; then
        hl_out=""
        for addon in ${ADDONS}; do
            out="$(${HADOLINT} < "${addon}/Dockerfile" 2>&1)"
            [ -n "${out}" ] && hl_out="${hl_out}${addon}: ${out}
"
        done
        if [ -z "${hl_out}" ]; then
            pass "hadolint"
        else
            fail "hadolint reported findings"
            printf '%s' "${hl_out}" | sed 's/^/        /'
        fi
    else
        skip "hadolint (not installed, unavailable via Docker)"
    fi
fi

# ---------------------------------------------------------------------------
# Build and smoke test
# ---------------------------------------------------------------------------
if [ "${DO_BUILD}" -eq 1 ]; then
    if ! have_docker; then
        head1 "Build"
        skip "Docker is not running - skipping build and smoke tests"
        DO_BUILD=0; DO_SMOKE=0; DO_SUPERVISOR=0
    fi
fi

if [ "${DO_BUILD}" -eq 1 ]; then
    head1 "Build images"
    for addon in ${ADDONS}; do
        base="$(addon_base "${addon}")"
        platform="$(addon_platform "${addon}")"
        image="beszel-addon-test/${addon}:local"
        note=""
        case " $(addon_arches "${addon}") " in
            *" ${HOST_HA_ARCH} "*) ;;
            *) note=" ${C_YEL}(emulated: add-on does not support ${HOST_HA_ARCH})${C_OFF}" ;;
        esac

        info "${addon}: ${platform} from $(basename "${base}")${note}"
        if docker build --platform "${platform}" --build-arg "BUILD_FROM=${base}" \
                -t "${image}" "${addon}/" >/tmp/beszel-build-$$.log 2>&1; then
            CREATED_IMAGES="${CREATED_IMAGES} ${image}"
            size="$(docker images "${image}" --format '{{.Size}}')"
            pass "${addon}: built (${size})"
            record "${addon}" build ok
        else
            fail "${addon}: build failed"
            tail -20 /tmp/beszel-build-$$.log | sed 's/^/        /'
            record "${addon}" build FAIL
        fi
        rm -f /tmp/beszel-build-$$.log
    done
fi

if [ "${DO_SMOKE}" -eq 1 ]; then
    head1 "Smoke test"
    port="$(free_port 18090)"
    for addon in ${ADDONS}; do
        image="beszel-addon-test/${addon}:local"
        docker image inspect "${image}" >/dev/null 2>&1 || { skip "${addon}: no image to run"; continue; }

        platform="$(addon_platform "${addon}")"
        kind="$(addon_kind "${addon}")"
        cname="beszel-test-$$-${addon}"
        CREATED_CONTAINERS="${CREATED_CONTAINERS} ${cname}"

        if [ "${kind}" = "hub" ]; then
            port="$(free_port "${port}")"
            docker run -d --name "${cname}" --platform "${platform}" \
                -p "${port}:8090" "${image}" >/dev/null 2>&1
        else
            docker run -d --name "${cname}" --platform "${platform}" \
                -e BESZEL_KEY="${TEST_KEY}" \
                -e BESZEL_HUB_URL="http://0.0.0.0" \
                -e BESZEL_TOKEN="test-token" \
                "${image}" >/dev/null 2>&1
        fi

        ok=1
        if [ "${kind}" = "hub" ]; then
            healthy=0
            for _ in $(seq 1 30); do
                if curl -fsS "http://127.0.0.1:${port}/api/health" >/dev/null 2>&1; then
                    healthy=1; break
                fi
                sleep 1
            done
            [ "${healthy}" -eq 1 ] || { fail "${addon}: /api/health never responded"; ok=0; }
        else
            sleep 8
        fi

        logs="$(docker logs "${cname}" 2>&1)"
        status="$(docker inspect "${cname}" --format '{{.State.Status}}' 2>/dev/null)"

        if [ "${kind}" = "hub" ]; then
            echo "${logs}" | grep -q "Starting Beszel Hub" || { fail "${addon}: hub did not start"; ok=0; }
        else
            echo "${logs}" | grep -q "Starting Beszel Agent" || { fail "${addon}: agent did not start"; ok=0; }
            echo "${logs}" | grep -q "Starting Beszel Agent on port 45876" || { fail "${addon}: agent never reached its listen port"; ok=0; }
        fi

        if echo "${logs}" | grep -q "cannot execute: required file not found"; then
            fail "${addon}: binary will not execute (architecture or libc mismatch)"; ok=0
        fi
        [ "${status}" = "running" ] || { fail "${addon}: container is ${status:-gone}, expected running"; ok=0; }

        if [ "${ok}" -eq 1 ]; then
            pass "${addon}: starts and stays running"
            record "${addon}" smoke ok
        else
            echo "${logs}" | tail -15 | sed 's/^/        /'
            record "${addon}" smoke FAIL
        fi

        docker rm -f "${cname}" >/dev/null 2>&1
    done
fi

# ---------------------------------------------------------------------------
# Mock Supervisor integration
#
# Everything above uses the BESZEL_* environment overrides, which bypass bashio
# entirely. This phase stands up a fake Supervisor API so the add-ons take the
# path real users take: bashio reading options over HTTP.
# ---------------------------------------------------------------------------
if [ "${DO_SUPERVISOR}" -eq 1 ]; then
    head1 "Mock Supervisor integration"

    TMPDIR_TEST="$(mktemp -d)"
    cat > "${TMPDIR_TEST}/server.py" <<'PYEOF'
import json, os
from http.server import BaseHTTPRequestHandler, HTTPServer

AGENT = {
    "key": os.environ["TEST_KEY"],
    "hub_url": "http://hub.invalid:8090",
    "token": "mock-token-123",
    "environment_vars": [
        {"name": "LOG_LEVEL", "value": "debug"},
        {"name": "BAD NAME", "value": "must-be-skipped"},
    ],
    "custom_volumes": [{"host_path": "/etc", "container_path": "/etc"}],
}
HUB = {
    "app_url": "https://beszel.example.com",
    "environment_vars": [
        {"name": "SHARE_ALL_SYSTEMS", "value": "true"},
        {"name": "BAD NAME", "value": "must-be-skipped"},
    ],
}
INFO = {"watchdog": True, "state": "started"}

class H(BaseHTTPRequestHandler):
    def _send(self, obj):
        b = json.dumps(obj).encode()
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(b)))
        self.end_headers()
        self.wfile.write(b)
    def do_GET(self):
        if self.path == "/addons/self/options/config":
            self._send({"result": "ok", "data": HUB if os.environ.get("MODE") == "hub" else AGENT})
        elif self.path == "/addons/self/info":
            self._send({"result": "ok", "data": INFO})
        else:
            self._send({"result": "ok", "data": {}})
    def log_message(self, *a):
        pass

HTTPServer(("0.0.0.0", 80), H).serve_forever()
PYEOF
    cat > "${TMPDIR_TEST}/Dockerfile" <<'EOF'
FROM python:3-alpine
COPY server.py /server.py
CMD ["python3","/server.py"]
EOF

    if ! docker build -t "beszel-mock-supervisor:$$" "${TMPDIR_TEST}" >/dev/null 2>&1; then
        fail "could not build the mock Supervisor image"
    else
        CREATED_IMAGES="${CREATED_IMAGES} beszel-mock-supervisor:$$"
        CREATED_NETWORK="beszel-test-net-$$"
        docker network create "${CREATED_NETWORK}" >/dev/null 2>&1

        for addon in ${ADDONS}; do
            image="beszel-addon-test/${addon}:local"
            docker image inspect "${image}" >/dev/null 2>&1 || { skip "${addon}: no image to run"; continue; }

            kind="$(addon_kind "${addon}")"
            platform="$(addon_platform "${addon}")"
            mock="beszel-test-$$-mock-${addon}"
            cname="beszel-test-$$-sv-${addon}"
            CREATED_CONTAINERS="${CREATED_CONTAINERS} ${mock} ${cname}"

            docker run -d --name "${mock}" --network "${CREATED_NETWORK}" \
                --network-alias supervisor \
                -e MODE="${kind}" -e TEST_KEY="${TEST_KEY}" \
                "beszel-mock-supervisor:$$" >/dev/null 2>&1
            sleep 2

            docker run -d --name "${cname}" --platform "${platform}" \
                --network "${CREATED_NETWORK}" \
                -e SUPERVISOR_TOKEN=mock-token "${image}" >/dev/null 2>&1
            sleep 8

            logs="$(docker logs "${cname}" 2>&1)"
            status="$(docker inspect "${cname}" --format '{{.State.Status}}' 2>/dev/null)"
            ok=1

            if [ "${kind}" = "hub" ]; then
                echo "${logs}" | grep -q "App URL: https://beszel.example.com" \
                    || { fail "${addon}: app_url was not read from the Supervisor API"; ok=0; }
            else
                echo "${logs}" | grep -q "Hub URL: http://hub.invalid:8090" \
                    || { fail "${addon}: hub_url was not read from the Supervisor API"; ok=0; }
                echo "${logs}" | grep -q "Token configured" \
                    || { fail "${addon}: token was not read from the Supervisor API"; ok=0; }
            fi

            # A malformed name must be skipped with a warning, never abort the
            # add-on - `export` fails on it and run.sh runs under `set -e`.
            echo "${logs}" | grep -q "is not a valid variable name" \
                || { fail "${addon}: malformed environment_vars name was not rejected"; ok=0; }
            [ "${status}" = "running" ] \
                || { fail "${addon}: container is ${status:-gone} after reading options"; ok=0; }

            if [ "${ok}" -eq 1 ]; then
                pass "${addon}: reads options via bashio, survives a bad env var name"
                record "${addon}" supervisor ok
            else
                echo "${logs}" | tail -15 | sed 's/^/        /'
                record "${addon}" supervisor FAIL
            fi

            docker rm -f "${cname}" "${mock}" >/dev/null 2>&1
        done
    fi
fi

# ---------------------------------------------------------------------------
# Summary
# ---------------------------------------------------------------------------
head1 "Summary"
n=0
for addon in ${ADDONS}; do n=$((n + 1)); done
printf '  %d add-on(s) checked\n' "${n}"
[ "${SKIPPED}" -gt 0 ] && printf '  %s%d check(s) skipped%s\n' "${C_YEL}" "${SKIPPED}" "${C_OFF}"

if [ "${FAILURES}" -eq 0 ]; then
    printf '  %sAll checks passed%s\n\n' "${C_GRN}" "${C_OFF}"
    exit 0
fi
printf '  %s%d check(s) failed%s\n\n' "${C_RED}" "${FAILURES}" "${C_OFF}"
exit 1

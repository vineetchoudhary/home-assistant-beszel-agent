#!/usr/bin/env bash
#
# End-to-end test: a real Hub app and a real Agent app, talking to each
# other.
#
# scripts/test-apps.sh proves each image starts. This proves the product
# works: the Hub serves its API, an Agent registers itself over a universal
# token, and real metrics arrive in the Hub's database.
#
#   ./scripts/test-e2e.sh                  # hub + the standard agent
#   ./scripts/test-e2e.sh --all-agents     # hub + every agent variant at once
#   ./scripts/test-e2e.sh --all-agents --keep   # leave it all up to browse
#   ./scripts/test-e2e.sh --clean               # remove anything a previous run left
#
# Written for bash 3.2 so it runs on stock macOS as well as CI.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "${REPO_ROOT}" || exit 1

HUB_APP="beszel_hub"
AGENT_APPS="beszel_agent"
ALL_AGENTS=0
KEEP=0
CLEAN_ONLY=0

USER_EMAIL="e2e@example.com"
USER_PASSWORD="e2e-test-password"

usage() {
    sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
    cat <<'EOF'

Options:
  --hub <dir>     Hub app to run (default: beszel_hub)
  --agent <dir>   Agent app to run; repeatable
  --all-agents    Run every beszel_agent* variant against the one Hub
  --keep          Leave the hub and agents running, and print how to reach them
  --clean         Remove containers, networks and images left by any previous run
  -h, --help      Show this help
EOF
}

AGENTS_OVERRIDDEN=0
while [ $# -gt 0 ]; do
    case "$1" in
        --hub)        HUB_APP="${2%/}"; shift ;;
        --agent)      if [ "${AGENTS_OVERRIDDEN}" -eq 0 ]; then AGENT_APPS=""; AGENTS_OVERRIDDEN=1; fi
                      AGENT_APPS="${AGENT_APPS} ${2%/}"; shift ;;
        --all-agents) ALL_AGENTS=1 ;;
        --keep)       KEEP=1 ;;
        --clean)      CLEAN_ONLY=1 ;;
        -h|--help)    usage; exit 0 ;;
        *)            echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
    esac
    shift
done

if [ "${ALL_AGENTS}" -eq 1 ]; then
    AGENT_APPS=""
    while IFS= read -r d; do
        [ -n "${d}" ] && AGENT_APPS="${AGENT_APPS} ${d}"
    done <<EOF
$(find . -maxdepth 2 -name config.yaml -path './beszel_agent*' -exec dirname {} \; | sed 's|^\./||' | sort)
EOF
fi

if [ -t 1 ]; then
    C_RED=$'\033[31m'; C_GRN=$'\033[32m'
    C_BLU=$'\033[34m'; C_DIM=$'\033[2m';  C_OFF=$'\033[0m'
else
    C_RED=""; C_GRN=""; C_BLU=""; C_DIM=""; C_OFF=""
fi

FAILURES=0
pass()  { printf '  %sPASS%s  %s\n' "${C_GRN}" "${C_OFF}" "$*"; }
fail()  { printf '  %sFAIL%s  %s\n' "${C_RED}" "${C_OFF}" "$*"; FAILURES=$((FAILURES + 1)); }
info()  { printf '  %s%s%s\n' "${C_DIM}" "$*" "${C_OFF}"; }
head1() { printf '\n%s==> %s%s\n' "${C_BLU}" "$*" "${C_OFF}"; }

RUN_ID="$$"
NETWORK="beszel-e2e-${RUN_ID}"
HUB_NAME="beszel-e2e-hub-${RUN_ID}"
CONTAINERS=""
IMAGES=""
TMPROOT="$(mktemp -d)"

# shellcheck disable=SC2329  # invoked via the EXIT/INT/TERM trap
cleanup() {
    if [ "${KEEP}" -eq 1 ]; then
        head1 "Environment left running (--keep)"
        printf '  %-12s %s\n' "Hub UI" "${HUB_URL_HOST}"
        printf '  %-12s %s / %s\n' "Login" "${USER_EMAIL}" "${USER_PASSWORD}"

        # Ask the hub what it can actually see, rather than listing what we
        # started - that is the interesting part when poking around by hand.
        if [ -n "${AUTH_TOKEN:-}" ]; then
            systems="$(hub_api -H "Authorization: ${AUTH_TOKEN}" \
                "${HUB_URL_HOST}/api/collections/systems/records?perPage=100" \
                | python3 -c "
import json,sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for i in d.get('items', []):
    print('    %-28s %s' % (i.get('name'), i.get('status')))" 2>/dev/null)"
            if [ -n "${systems}" ]; then
                printf '\n  Systems reporting to this hub:\n%s\n' "${systems}"
            fi
        fi

        printf '\n  %-12s %s\n' "Network" "${NETWORK}"
        printf '  %-12s %s\n' "Agent data" "${TMPROOT}"
        printf '  %-12s\n' "Containers"
        for c in ${CONTAINERS}; do printf '    %s\n' "${c}"; done
        printf '\n  Tear down with:\n    docker rm -f%s \\\n      && docker network rm %s \\\n      && rm -rf %s\n\n' \
            "${CONTAINERS}" "${NETWORK}" "${TMPROOT}"
        return 0
    fi
    for c in ${CONTAINERS}; do docker rm -f "${c}" >/dev/null 2>&1; done
    for i in ${IMAGES}; do docker rmi "${i}" >/dev/null 2>&1; done
    docker network rm "${NETWORK}" >/dev/null 2>&1
    rm -rf "${TMPROOT}"
    return 0
}
trap cleanup EXIT INT TERM

if ! docker info >/dev/null 2>&1; then
    echo "Docker is not running." >&2
    exit 2
fi

# Sweep anything a previous run left behind. Normal exits and Ctrl-C are handled
# by the trap above, but a hard kill (or a deliberate --keep) leaves resources
# around, and this is the way to reclaim them.
if [ "${CLEAN_ONLY}" -eq 1 ]; then
    head1 "Removing leftovers from previous runs"
    n=0
    for c in $(docker ps -aq --filter 'name=beszel-e2e-' 2>/dev/null); do
        docker rm -f "${c}" >/dev/null 2>&1 && n=$((n + 1))
    done
    printf '  containers removed: %d\n' "${n}"
    n=0
    for net in $(docker network ls --filter 'name=beszel-e2e-' --format '{{.Name}}' 2>/dev/null); do
        docker network rm "${net}" >/dev/null 2>&1 && n=$((n + 1))
    done
    printf '  networks removed:   %d\n' "${n}"
    n=0
    for img in $(docker images --format '{{.Repository}}:{{.Tag}}' 2>/dev/null | grep '^beszel-e2e/'); do
        docker rmi "${img}" >/dev/null 2>&1 && n=$((n + 1))
    done
    printf '  images removed:     %d\n' "${n}"
    printf '\n  Note: seeded agent data lives in mktemp directories and is removed by\n'
    printf '  the tear-down line that --keep prints.\n\n'
    exit 0
fi

app_base() {
    if grep -q 'apt-get' "$1/Dockerfile"; then
        echo "ghcr.io/home-assistant/base-debian:latest"
    else
        echo "ghcr.io/home-assistant/base:latest"
    fi
}

HOST_ARCH="$(uname -m)"
case "${HOST_ARCH}" in
    x86_64|amd64)  HOST_HA_ARCH="amd64" ;;
    arm64|aarch64) HOST_HA_ARCH="aarch64" ;;
    *)             HOST_HA_ARCH="" ;;
esac

app_platform() {
    _arches="$(python3 -c "
import yaml,sys
print(' '.join(yaml.safe_load(open(sys.argv[1]+'/config.yaml'))['arch']))" "$1")"
    for a in ${_arches}; do
        [ "${a}" = "${HOST_HA_ARCH}" ] && { [ "${a}" = "amd64" ] && echo "linux/amd64" || echo "linux/arm64"; return 0; }
    done
    # shellcheck disable=SC2086  # deliberate word splitting
    set -- ${_arches}
    [ "$1" = "amd64" ] && echo "linux/amd64" || echo "linux/arm64"
}

build_app() {
    _dir="$1"
    _img="beszel-e2e/${_dir}:${RUN_ID}"
    if ! docker build --platform "$(app_platform "${_dir}")" \
            --build-arg "BUILD_FROM=$(app_base "${_dir}")" \
            -t "${_img}" "${_dir}/" >/tmp/e2e-build-${RUN_ID}.log 2>&1; then
        fail "${_dir}: build failed"
        tail -15 /tmp/e2e-build-${RUN_ID}.log | sed 's/^/        /'
        rm -f /tmp/e2e-build-${RUN_ID}.log
        return 1
    fi
    rm -f /tmp/e2e-build-${RUN_ID}.log
    # NB: this runs in a command substitution, so it cannot record the image
    # for cleanup itself - the caller does that.
    echo "${_img}"
}

free_port() {
    _p="$1"
    while nc -z 127.0.0.1 "${_p}" >/dev/null 2>&1; do _p=$((_p + 1)); done
    echo "${_p}"
}
HUB_PORT="$(free_port 18300)"
HUB_URL_HOST="http://127.0.0.1:${HUB_PORT}"

# The hub is published on a free host port so this can use the host's curl.
# Spawning a container per API call made the polling loops an order of
# magnitude slower.
hub_api() {
    curl -s --max-time 15 "$@" 2>/dev/null
}

json_get() { python3 -c "
import json,sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(1)
for k in sys.argv[1].split('.'):
    if isinstance(d, list):
        d = d[int(k)]
    else:
        d = d.get(k)
    if d is None:
        sys.exit(1)
print(d)" "$1"; }

# ---------------------------------------------------------------------------
head1 "Build images"
# ---------------------------------------------------------------------------
docker network create "${NETWORK}" >/dev/null 2>&1

HUB_IMAGE="$(build_app "${HUB_APP}")" || exit 1
IMAGES="${IMAGES} ${HUB_IMAGE}"
pass "${HUB_APP}: built"

AGENT_IMAGES=""
for a in ${AGENT_APPS}; do
    img="$(build_app "${a}")" || exit 1
    IMAGES="${IMAGES} ${img}"
    AGENT_IMAGES="${AGENT_IMAGES} ${a}=${img}"
    pass "${a}: built"
done

# ---------------------------------------------------------------------------
head1 "Start the Hub"
# ---------------------------------------------------------------------------
# USER_EMAIL/USER_PASSWORD are read by Beszel's first-run migration, which
# creates both a superuser and a regular user. The regular user is the one that
# matters here: superusers are not allowed to issue universal tokens.
CONTAINERS="${CONTAINERS} ${HUB_NAME}"
docker run -d --name "${HUB_NAME}" --network "${NETWORK}" --network-alias hub \
    -p "${HUB_PORT}:8090" \
    --platform "$(app_platform "${HUB_APP}")" \
    -e BESZEL_HUB_USER_EMAIL="${USER_EMAIL}" \
    -e BESZEL_HUB_USER_PASSWORD="${USER_PASSWORD}" \
    "${HUB_IMAGE}" >/dev/null 2>&1

healthy=0
for _ in $(seq 1 45); do
    if [ "$(hub_api -o /dev/null -w '%{http_code}' "${HUB_URL_HOST}/api/health")" = "200" ]; then
        healthy=1; break
    fi
    sleep 1
done
if [ "${healthy}" -ne 1 ]; then
    fail "hub never became healthy"
    docker logs "${HUB_NAME}" 2>&1 | tail -20 | sed 's/^/        /'
    exit 1
fi
pass "hub is serving /api/health"

# ---------------------------------------------------------------------------
head1 "Authenticate against the Hub API"
# ---------------------------------------------------------------------------
AUTH_JSON="$(hub_api -X POST -H 'Content-Type: application/json' \
    -d "{\"identity\":\"${USER_EMAIL}\",\"password\":\"${USER_PASSWORD}\"}" \
    "${HUB_URL_HOST}/api/collections/users/auth-with-password")"

AUTH_TOKEN="$(printf '%s' "${AUTH_JSON}" | json_get token)"
if [ -z "${AUTH_TOKEN}" ]; then
    fail "could not authenticate as ${USER_EMAIL}"
    printf '        %s\n' "${AUTH_JSON}"
    exit 1
fi
pass "authenticated as the first user created by USER_EMAIL/USER_PASSWORD"

HUB_KEY="$(hub_api -H "Authorization: ${AUTH_TOKEN}" "${HUB_URL_HOST}/api/beszel/info" | json_get key)"
if [ -z "${HUB_KEY}" ]; then
    fail "could not read the hub public key from /api/beszel/info"
    exit 1
fi
pass "fetched the hub SSH public key (${HUB_KEY%% *} ...)"

TOKEN_JSON="$(hub_api -H "Authorization: ${AUTH_TOKEN}" \
    "${HUB_URL_HOST}/api/beszel/universal-token?enable=1")"
UNIVERSAL_TOKEN="$(printf '%s' "${TOKEN_JSON}" | json_get token)"
if [ -z "${UNIVERSAL_TOKEN}" ]; then
    fail "could not create a universal token"
    printf '        %s\n' "${TOKEN_JSON}"
    exit 1
fi
pass "created a universal registration token"

# ---------------------------------------------------------------------------
head1 "Connect agents"
# ---------------------------------------------------------------------------
EXPECTED=""
for pair in ${AGENT_IMAGES}; do
    app="${pair%%=*}"
    img="${pair##*=}"
    host="e2e-${app}"
    cname="beszel-e2e-${app}-${RUN_ID}"
    CONTAINERS="${CONTAINERS} ${cname}"
    EXPECTED="${EXPECTED} ${host}"

    # Beszel fingerprints a machine from its host id, which every container on
    # this host shares - without a distinct fingerprint the hub treats each
    # agent as the same system reconnecting and closes the duplicate. A saved
    # fingerprint file in the data directory takes precedence, so seed one.
    fpdir="${TMPROOT}/${app}"
    mkdir -p "${fpdir}"
    printf '%s' "$(printf '%s' "e2e-${app}-${RUN_ID}" | shasum -a 256 | cut -c1-48)" \
        > "${fpdir}/fingerprint"

    docker run -d --name "${cname}" --network "${NETWORK}" --hostname "${host}" \
        --platform "$(app_platform "${app}")" \
        -v "${fpdir}:/var/lib/beszel-agent" \
        -e BESZEL_KEY="${HUB_KEY}" \
        -e BESZEL_HUB_URL="http://hub:8090" \
        -e BESZEL_TOKEN="${UNIVERSAL_TOKEN}" \
        "${img}" >/dev/null 2>&1
    info "${app}: started as host '${host}'"
done

# ---------------------------------------------------------------------------
head1 "Verify registration and metrics"
# ---------------------------------------------------------------------------
# The agent dials the hub, registers itself using the universal token, and then
# pushes stats. Poll the systems collection until each expected host shows up.
for host in ${EXPECTED}; do
    found=0
    status=""
    for _ in $(seq 1 60); do
        rec="$(hub_api -H "Authorization: ${AUTH_TOKEN}" \
            "${HUB_URL_HOST}/api/collections/systems/records?filter=(name='${host}')")"
        status="$(printf '%s' "${rec}" | json_get items.0.status 2>/dev/null)"
        if [ "${status}" = "up" ]; then found=1; break; fi
        sleep 2
    done

    if [ "${found}" -eq 1 ]; then
        pass "${host}: registered with the hub and is reporting (status=up)"
    else
        fail "${host}: never reached status=up (last status: ${status:-not registered})"
        for pair in ${AGENT_IMAGES}; do
            [ "e2e-${pair%%=*}" = "${host}" ] || continue
            docker logs "beszel-e2e-${pair%%=*}-${RUN_ID}" 2>&1 | tail -15 | sed 's/^/        /'
        done
    fi
done

# One system record is enough to prove stats really landed in the database.
FIRST_HOST=""
for h in ${EXPECTED}; do FIRST_HOST="${h}"; break; done
if [ -n "${FIRST_HOST}" ]; then
    rec="$(hub_api -H "Authorization: ${AUTH_TOKEN}" \
        "${HUB_URL_HOST}/api/collections/systems/records?filter=(name='${FIRST_HOST}')")"
    summary="$(printf '%s' "${rec}" | python3 -c "
import json,sys
d = json.load(sys.stdin)
if not d.get('items'):
    sys.exit(1)
i = d['items'][0]['info']
print('cpu=%s%% mem=%s%% disk=%s%% agent=%s threads=%s'
      % (i.get('cpu'), i.get('mp'), i.get('dp'), i.get('v'), i.get('t')))
sys.exit(0 if i.get('v') else 1)" 2>/dev/null)"
    if [ -n "${summary}" ]; then
        pass "hub stored real metrics: ${summary}"
    else
        fail "the system record contains no agent metrics"
        printf '        %s\n' "$(printf '%s' "${rec}" | head -c 400)"
    fi
fi

# ---------------------------------------------------------------------------
head1 "Summary"
# ---------------------------------------------------------------------------
n=0
for _ in ${EXPECTED}; do n=$((n + 1)); done
printf '  hub: %s, agents: %d\n' "${HUB_APP}" "${n}"
if [ "${FAILURES}" -eq 0 ]; then
    printf '  %sEnd-to-end test passed%s\n\n' "${C_GRN}" "${C_OFF}"
    exit 0
fi
printf '  %s%d check(s) failed%s\n\n' "${C_RED}" "${FAILURES}" "${C_OFF}"
exit 1

#!/usr/bin/with-contenv bashio
# shellcheck shell=bash

set -euo pipefail

die() {
    bashio::log.error "$1"
    exit 1
}

supervisor_api_available() {
    [ -n "${SUPERVISOR_TOKEN:-}" ]
}

hub_base_path() {
    local app_url="${1:-}" base="/" rest

    case "${app_url}" in
        *://*)
            rest="${app_url#*://}"
            case "${rest}" in
                */*)
                    base="/${rest#*/}"
                    base="${base%%[?]*}"
                    base="${base%%#*}"
                    case "${base}" in
                        */) ;;
                        *) base="${base}/" ;;
                    esac
                    ;;
            esac
            ;;
    esac

    printf '%s' "${base}"
}

start_ingress_proxy() {
    local hub_base test_output line

    hub_base="$(hub_base_path "${1:-}")"

    case "${hub_base}" in
        *[!A-Za-z0-9._~/-]*)
            bashio::log.warning "The app_url path cannot be mapped to an Ingress prefix: ${hub_base}"
            bashio::log.warning "Ingress is unavailable; the hub is still reachable on port 8090"
            return 1
            ;;
    esac

    bashio::log.info "Hub base path: ${hub_base}"
    sed "s|__HUB_BASE__|${hub_base}|g" \
        /etc/nginx/ingress.conf.template > /etc/nginx/http.d/ingress.conf

    mkdir -p /run/nginx

    if ! test_output="$(nginx -t 2>&1)"; then
        bashio::log.warning "nginx config test failed - Ingress will be unavailable"
        while IFS= read -r line; do
            bashio::log.warning "${line}"
        done <<< "${test_output}"
        return 1
    fi

    if ! nginx; then
        bashio::log.warning "nginx failed to start - Ingress will be unavailable"
        return 1
    fi

    for _ in 1 2 3; do
        if [ -s /run/nginx/nginx.pid ]; then
            bashio::log.info "Ingress proxy listening on port 8099"
            return 0
        fi
        sleep 1
    done

    bashio::log.warning "nginx exited immediately after starting - Ingress will be unavailable"
    return 1
}

bashio::log.info "========================================"
bashio::log.info "Starting Beszel Hub..."
bashio::log.info "========================================"

if [ ! -f /usr/local/bin/beszel ]; then
    die "Beszel Hub binary not found at /usr/local/bin/beszel"
fi

# Beszel Hub reads APP_URL (and its BESZEL_HUB_APP_URL alias) from the environment
# itself, so anything already exported is passed through untouched. The app option
# is only consulted when the Supervisor API is reachable.
APP_URL="${BESZEL_HUB_APP_URL:-${APP_URL:-}}"

if [ -z "${APP_URL}" ] && supervisor_api_available; then
    APP_URL="$(bashio::config 'app_url' 2>/dev/null || true)"
    if [ -n "${APP_URL}" ]; then
        export APP_URL
    fi
fi

if [ -n "${APP_URL}" ]; then
    bashio::log.info "App URL: ${APP_URL}"
else
    bashio::log.info "App URL not configured; Beszel Hub will use its default URL"
fi

if supervisor_api_available && bashio::config.has_value 'environment_vars'; then
    bashio::log.info "Processing custom environment variables..."
    index=0
    while bashio::config.exists "environment_vars[${index}]"; do
        NAME=$(bashio::config "environment_vars[${index}].name")
        VALUE=$(bashio::config "environment_vars[${index}].value")

        if [[ -z "$NAME" || -z "$VALUE" ]]; then
            bashio::log.warning "Skipping environment variable at index ${index}: name or value is empty"
        elif [[ ! "$NAME" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]]; then
            # `export` rejects a malformed name, and because this script runs under
            # `set -e` that would stop the app before the agent is ever started.
            # Warn and skip instead, so one typo cannot take the app down.
            bashio::log.warning "Skipping environment variable at index ${index}: '${NAME}' is not a valid variable name"
        else
            export "${NAME}=${VALUE}"
            bashio::log.info "Set environment variable: ${NAME}"
        fi

        index=$((index + 1))
    done
fi

# Mapped to the app data directory by config.yaml, so it survives restarts and
# updates. Passed explicitly rather than relying on the working directory, because
# Beszel resolves its default "beszel_data" relative to the current directory.
DATA_DIR="/var/lib/beszel-hub/beszel_data"
mkdir -p "${DATA_DIR}"

APP_URL="${BESZEL_HUB_APP_URL:-${APP_URL:-}}"
start_ingress_proxy "${APP_URL}" || true

bashio::log.info "Hub data directory: ${DATA_DIR}"
bashio::log.info "Beszel Hub web UI available at http://[HOST]:8090"
bashio::log.info "Create the first admin user from the web UI after the app starts"

exec /usr/local/bin/beszel serve --http "0.0.0.0:8090" --dir "${DATA_DIR}"

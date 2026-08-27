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

bashio::log.info "========================================"
bashio::log.info "Starting Beszel Hub..."
bashio::log.info "========================================"

if [ ! -f /usr/local/bin/beszel ]; then
    die "Beszel Hub binary not found at /usr/local/bin/beszel"
fi

# Beszel Hub reads APP_URL (and its BESZEL_HUB_APP_URL alias) from the environment
# itself, so anything already exported is passed through untouched. The add-on option
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

        if [[ -n "$NAME" && -n "$VALUE" ]]; then
            export "${NAME}=${VALUE}"
            bashio::log.info "Set environment variable: ${NAME}"
        else
            bashio::log.warning "Skipping invalid environment variable at index ${index}"
        fi

        index=$((index + 1))
    done
fi

# Mapped to the add-on data directory by config.yaml, so it survives restarts and
# updates. Passed explicitly rather than relying on the working directory, because
# Beszel resolves its default "beszel_data" relative to the current directory.
DATA_DIR="/var/lib/beszel-hub/beszel_data"
mkdir -p "${DATA_DIR}"

bashio::log.info "Hub data directory: ${DATA_DIR}"
bashio::log.info "Beszel Hub web UI available at http://[HOST]:8090"
bashio::log.info "Create the first admin user from the web UI after the add-on starts"

exec /usr/local/bin/beszel serve --http "0.0.0.0:8090" --dir "${DATA_DIR}"

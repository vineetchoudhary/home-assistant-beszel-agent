#!/usr/bin/with-contenv bashio
# shellcheck shell=bash
set -euo pipefail
umask 077

config_file="${BESZEL_BRIDGE_CONFIG:-/data/options.json}"

# Supervisor normally writes /data/options.json. Reading via Bashio also makes
# development/mock Supervisor launches behave like a managed app installation.
if [ ! -s "${config_file}" ] && [ -n "${SUPERVISOR_TOKEN:-}" ]; then
    mkdir -p "$(dirname "${config_file}")"
    bashio::addon.config > "${config_file}"
fi

exec /usr/local/bin/bridge run --config "${config_file}" --data /data

#!/usr/bin/with-contenv bashio
# shellcheck shell=bash

set -euo pipefail

# Function to log errors and exit
die() {
    bashio::log.error "$1"
    exit 1
}

# Check if Supervisor API is available by checking for SUPERVISOR_TOKEN environment variable
supervisor_api_available() {
    [ -n "${SUPERVISOR_TOKEN:-}" ]
}

fetch_addon_info() {
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL \
            -H "Authorization: Bearer ${SUPERVISOR_TOKEN}" \
            http://supervisor/addons/self/info 2>/dev/null || true
    elif command -v wget >/dev/null 2>&1; then
        wget -qO- \
            --header="Authorization: Bearer ${SUPERVISOR_TOKEN}" \
            http://supervisor/addons/self/info 2>/dev/null || true
    else
        return 1
    fi
}

resolve_healthcheck_port() {
    local addon_info=""
    local health_port=""

    addon_info="$(fetch_addon_info)"
    health_port="$(printf '%s' "${addon_info}" | sed -nE 's/.*\[PORT:([0-9]+)\].*/\1/p' | head -n 1)"

    if [ -z "${health_port}" ]; then
        health_port="$(printf '%s' "${addon_info}" | sed -nE 's|.*http://[^:]+:([0-9]+)/.*|\1|p' | head -n 1)"
    fi

    if [ -z "${health_port}" ]; then
        health_port="45877"
    fi

    printf '%s\n' "${health_port}"
}

# Check if add-on watchdog is enabled by querying the Supervisor API
addon_watchdog_enabled() {
    local addon_info=""

    if ! supervisor_api_available; then
        bashio::log.info "Skipping healthcheck server: SUPERVISOR_TOKEN not available"
        return 1
    fi

    if ! addon_info="$(fetch_addon_info)"; then
        bashio::log.warning "Skipping healthcheck server: neither curl nor wget is available"
        return 1
    fi

    if printf '%s' "${addon_info}" | tr -d '\n' | grep -q '"watchdog":[[:space:]]*true'; then
        return 0
    fi

    if [ -n "${addon_info}" ]; then
        bashio::log.info "Skipping healthcheck server: add-on watchdog is disabled"
    else
        bashio::log.warning "Skipping healthcheck server: unable to query supervisor add-on info"
    fi

    return 1
}

# Start a simple HTTP server to serve the healthcheck endpoint if the add-on watchdog is enabled
start_healthcheck_server() {
    local health_root="/tmp/beszel-health"
    local health_port=""
    local httpd_output=""
    local -a httpd_cmd=()

    if ! addon_watchdog_enabled; then
        return 0
    fi

    health_port="$(resolve_healthcheck_port)"

    if command -v httpd >/dev/null 2>&1; then
        httpd_cmd=("$(command -v httpd)" -f -p "${health_port}" -h "${health_root}")
    elif command -v busybox >/dev/null 2>&1 && busybox --list 2>/dev/null | grep -qx 'httpd'; then
        httpd_cmd=("$(command -v busybox)" httpd -f -p "${health_port}" -h "${health_root}")
    else
        bashio::log.warning "Healthcheck server disabled: no httpd implementation found; Beszel Agent will continue running"
        return 0
    fi

    mkdir -p "${health_root}/cgi-bin"
    ln -sf /healthcheck-http.sh "${health_root}/cgi-bin/health"
    export BESZEL_AGENT_BIN=/usr/local/bin/agent

    # Run the helper independently so healthcheck failures never stop the agent.
    (
        while true; do
            httpd_output="$("${httpd_cmd[@]}" 2>&1 || true)"
            if printf '%s' "${httpd_output}" | grep -qi 'Address in use'; then
                bashio::log.error "Healthcheck server failed to bind port ${health_port}: ${httpd_output}"
                bashio::log.error "Change the watchdog port or disable the watchdog, otherwise Home Assistant may keep restarting this add-on in a loop."
            elif [ -n "${httpd_output}" ]; then
                bashio::log.warning "Healthcheck server exited: ${httpd_output}"
            fi
            bashio::log.warning "Healthcheck server exited; retrying in 5 seconds"
            sleep 5
        done
    ) &

    bashio::log.info "Healthcheck endpoint available at http://[HOST]:${health_port}/cgi-bin/health"
    return 0
}

bashio::log.info "========================================"
bashio::log.info "Starting Beszel Agent (S.M.A.R.T.)..."
bashio::log.info "========================================"

# Get required configuration
# Use environment variables first for local/docker testing to avoid noisy Supervisor API failures.
KEY=""
if [ -n "${BESZEL_KEY:-}" ]; then
    bashio::log.info "Using KEY from environment variable (test mode)"
    KEY="$BESZEL_KEY"
elif supervisor_api_available; then
    KEY="$(bashio::config 'key' 2>/dev/null || true)"
fi
if [ -z "$KEY" ]; then
    die "Configuration error: 'key' is required but not set"
fi

# Validate key format (basic check for SSH key)
if ! [[ "$KEY" =~ ^ssh- ]]; then
    bashio::log.warning "Key does not appear to be a valid SSH public key (should start with 'ssh-')"
fi

HUB_URL=""
if [ -n "${BESZEL_HUB_URL:-}" ]; then
    bashio::log.info "Using HUB_URL from environment variable (test mode)"
    HUB_URL="$BESZEL_HUB_URL"
elif supervisor_api_available; then
    HUB_URL="$(bashio::config 'hub_url' 2>/dev/null || true)"
fi
if [ -z "$HUB_URL" ]; then
    die "Configuration error: 'hub_url' is required but not set"
fi

TOKEN=""
if [ -n "${BESZEL_TOKEN:-}" ]; then
    bashio::log.info "Using TOKEN from environment variable (test mode)"
    TOKEN="$BESZEL_TOKEN"
elif supervisor_api_available; then
    TOKEN="$(bashio::config 'token' 2>/dev/null || true)"
fi
if [ -z "$TOKEN" ]; then
    die "Configuration error: 'token' is required but not set"
fi

# Export environment variables
export HUB_URL
export TOKEN

bashio::log.info "Hub URL: ${HUB_URL}"
bashio::log.info "Token configured"

# Set custom environment variables dynamically
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

# Process custom volumes (for logging/verification purposes)
# Note: Actual mounting is handled by Home Assistant supervisor based on config.yaml
if supervisor_api_available && bashio::config.has_value 'custom_volumes'; then
    bashio::log.info "Custom volumes configured:"
    index=0
    volume_count=0
    while bashio::config.exists "custom_volumes[${index}]"; do
        HOST_PATH=$(bashio::config "custom_volumes[${index}].host_path" || echo "")
        CONTAINER_PATH=$(bashio::config "custom_volumes[${index}].container_path" || echo "")
        
        if [[ -n "$HOST_PATH" && -n "$CONTAINER_PATH" ]]; then
            # Parse :ro suffix if present
            ACTUAL_PATH="${CONTAINER_PATH%:ro}"
            ACTUAL_PATH="${ACTUAL_PATH%:rw}"
            RO_FLAG=""
            [[ "$CONTAINER_PATH" == *":ro" ]] && RO_FLAG=" (read-only)"
            [[ "$CONTAINER_PATH" == *":rw" ]] && RO_FLAG=" (read-write)"
            
            bashio::log.info "  [${volume_count}] ${HOST_PATH} -> ${ACTUAL_PATH}${RO_FLAG}"
            
            # Verify the mount exists in the container
            if [[ -e "$ACTUAL_PATH" ]]; then
                bashio::log.info "      ✓ Mount point verified"
            else
                bashio::log.warning "      ✗ Mount point not accessible: ${ACTUAL_PATH}"
            fi
            volume_count=$((volume_count + 1))
        else
            bashio::log.warning "Skipping invalid volume configuration at index ${index} (missing host_path or container_path)"
        fi
        
        index=$((index + 1))
    done
    
    if [ $volume_count -eq 0 ]; then
        bashio::log.info "  No valid custom volumes configured"
    fi
fi

# Check for S.M.A.R.T. monitoring support
bashio::log.info "========================================"
bashio::log.info "S.M.A.R.T. Monitoring Status"
bashio::log.info "========================================"
if command -v smartctl >/dev/null 2>&1; then
    bashio::log.info "✓ smartctl available for S.M.A.R.T. monitoring"
    
    # Auto-detect available drives
    DRIVES=$(smartctl --scan 2>/dev/null | awk '{print $1}' || true)
    if [ -n "$DRIVES" ]; then
        bashio::log.info "Available drives detected:"
        echo "$DRIVES" | while read -r drive; do
            bashio::log.info "  - $drive"
        done
    else
        bashio::log.warning "No drives detected"
    fi
else
    bashio::log.error "✗ smartctl not found"
fi

# Verify agent binary exists
if [ ! -f /usr/local/bin/agent ]; then
    die "Beszel Agent binary not found at /usr/local/bin/agent"
fi

start_healthcheck_server

# Start the Beszel Agent
bashio::log.info "========================================"
bashio::log.info "Starting Beszel Agent on port 45876..."
bashio::log.info "========================================"
exec /usr/local/bin/agent -key "$KEY"

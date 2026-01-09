#!/usr/bin/with-contenv bashio
# shellcheck shell=bash

set -euo pipefail

# Function to log errors and exit
die() {
    bashio::log.error "$1"
    exit 1
}

bashio::log.info "========================================"
bashio::log.info "Starting Beszel Agent..."
bashio::log.info "========================================"

# Get required configuration
KEY="$(bashio::config 'key')"
if [ -z "$KEY" ]; then
    die "Configuration error: 'key' is required but not set"
fi

# Validate key format (basic check for SSH key)
if ! [[ "$KEY" =~ ^ssh- ]]; then
    bashio::log.warning "Key does not appear to be a valid SSH public key (should start with 'ssh-')"
fi

HUB_URL="$(bashio::config 'hub_url')"
if [ -z "$HUB_URL" ]; then
    die "Configuration error: 'hub_url' is required but not set"
fi

TOKEN="$(bashio::config 'token')"
if [ -z "$TOKEN" ]; then
    die "Configuration error: 'token' is required but not set"
fi

# Export environment variables
export HUB_URL
export TOKEN

bashio::log.info "Hub URL: ${HUB_URL}"
bashio::log.info "Token configured"

# Set custom environment variables dynamically
if bashio::config.has_value 'environment_vars'; then
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
if bashio::config.has_value 'custom_volumes'; then
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

# Verify agent binary exists
if [ ! -f /usr/local/bin/agent ]; then
    die "Beszel Agent binary not found at /usr/local/bin/agent"
fi

# Display agent version
AGENT_VERSION=$(/usr/local/bin/agent --version 2>&1 || echo "unknown")
bashio::log.info "Agent version: ${AGENT_VERSION}"

# Start the Beszel Agent
bashio::log.info "========================================"
bashio::log.info "Starting Beszel Agent on port 45876..."
bashio::log.info "========================================"
exec /usr/local/bin/agent -key "$KEY"

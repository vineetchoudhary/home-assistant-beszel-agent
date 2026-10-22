ARG BUILD_FROM
FROM $BUILD_FROM

# Set shell
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Copy configuration and run script
COPY config.yaml /tmp/config.yaml
COPY beszel_version /tmp/beszel_version
COPY beszel_sha256 /tmp/beszel_sha256
COPY run.sh /
COPY healthcheck-http.sh /

# Install Beszel Agent
# hadolint ignore=DL3018
RUN set -euo pipefail && \
    # Install a tiny HTTP server for Home Assistant watchdog probes
    apk add --no-cache busybox-extras && \
    \
    # Extract upstream Beszel version
    BESZEL_VERSION=$(tr -d '[:space:]' < /tmp/beszel_version) && \
    if [ -z "${BESZEL_VERSION}" ]; then \
        echo "[ERROR] Failed to extract version from beszel_version" && exit 1; \
    fi && \
    echo "[INFO] Installing Beszel Agent version: ${BESZEL_VERSION}" && \
    \
    # Detect architecture
    ARCH=$(uname -m) && \
    case "$ARCH" in \
        x86_64) BESZEL_ARCH="amd64" ;; \
        aarch64) BESZEL_ARCH="arm64" ;; \
        *) echo "[ERROR] Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    echo "[INFO] Detected architecture: ${ARCH} -> ${BESZEL_ARCH}" && \
    \
    # Download and install Beszel Agent with retries
    BESZEL_ASSET="beszel-agent_linux_${BESZEL_ARCH}.tar.gz" && \
    DOWNLOAD_URL="https://github.com/henrygd/beszel/releases/download/v${BESZEL_VERSION}/${BESZEL_ASSET}" && \
    echo "[INFO] Downloading from: ${DOWNLOAD_URL}" && \
    for i in 1 2 3; do \
        if curl --fail --location --silent --show-error --connect-timeout 30 --max-time 120 \
            --output /tmp/beszel-agent.tar.gz "${DOWNLOAD_URL}"; then \
            break; \
        elif [ $i -eq 3 ]; then \
            echo "[ERROR] Failed to download after 3 attempts" && exit 1; \
        else \
            echo "[WARN] Download attempt $i failed, retrying..." && sleep 2; \
        fi; \
    done && \
    \
    # Verify the download before unpacking it. beszel_sha256 carries the lines
    # for this app's assets, copied verbatim from the upstream release's
    # checksums file. A mismatch, or an asset with no pinned digest, fails the
    # build rather than installing an unverified binary.
    EXPECTED_SHA=$(awk -v asset="${BESZEL_ASSET}" '$2 == asset { print $1 }' /tmp/beszel_sha256) && \
    if [ -z "${EXPECTED_SHA}" ]; then \
        echo "[ERROR] No pinned SHA-256 for ${BESZEL_ASSET} in beszel_sha256" && exit 1; \
    fi && \
    if ! echo "${EXPECTED_SHA}  /tmp/beszel-agent.tar.gz" | sha256sum -c - > /dev/null 2>&1; then \
        echo "[ERROR] Checksum mismatch for ${BESZEL_ASSET}" && exit 1; \
    fi && \
    echo "[INFO] Checksum verified for ${BESZEL_ASSET}" && \
    \
    # Extract and install
    tar -xzf /tmp/beszel-agent.tar.gz -C /tmp && \
    if [ ! -f /tmp/beszel-agent ]; then \
        echo "[ERROR] beszel-agent binary not found after extraction" && exit 1; \
    fi && \
    mv /tmp/beszel-agent /usr/local/bin/agent && \
    chmod +x /usr/local/bin/agent /run.sh /healthcheck-http.sh && \
    \
    # Cleanup
    rm -rf /tmp/beszel-agent* /tmp/config.yaml /tmp/beszel_version /tmp/beszel_sha256 && \
    echo "[INFO] Beszel Agent installed successfully"

# Run the agent
CMD ["/run.sh"]

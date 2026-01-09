ARG BUILD_FROM
FROM $BUILD_FROM

# Set shell
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Copy configuration and run script
COPY config.yaml /tmp/config.yaml
COPY run.sh /

# Install Beszel Agent
RUN set -euo pipefail && \
    # Extract version from config.yaml
    BESZEL_VERSION=$(awk -F': ' '/^version:/ {print $2}' /tmp/config.yaml | tr -d '"' | tr -d ' ') && \
    if [ -z "${BESZEL_VERSION}" ]; then \
        echo "[ERROR] Failed to extract version from config.yaml" && exit 1; \
    fi && \
    echo "[INFO] Installing Beszel Agent version: ${BESZEL_VERSION}" && \
    \
    # Detect architecture
    ARCH=$(uname -m) && \
    case "$ARCH" in \
        x86_64) BESZEL_ARCH="amd64" ;; \
        armv7l) BESZEL_ARCH="arm" ;; \
        aarch64) BESZEL_ARCH="arm64" ;; \
        i386|i686) BESZEL_ARCH="386" ;; \
        armhf) BESZEL_ARCH="arm" ;; \
        *) echo "[ERROR] Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    echo "[INFO] Detected architecture: ${ARCH} -> ${BESZEL_ARCH}" && \
    \
    # Download and install Beszel Agent with retries
    DOWNLOAD_URL="https://github.com/henrygd/beszel/releases/download/v${BESZEL_VERSION}/beszel-agent_linux_${BESZEL_ARCH}.tar.gz" && \
    echo "[INFO] Downloading from: ${DOWNLOAD_URL}" && \
    for i in 1 2 3; do \
        if wget --timeout=30 -q -O /tmp/beszel-agent.tar.gz "${DOWNLOAD_URL}"; then \
            break; \
        elif [ $i -eq 3 ]; then \
            echo "[ERROR] Failed to download after 3 attempts" && exit 1; \
        else \
            echo "[WARN] Download attempt $i failed, retrying..." && sleep 2; \
        fi; \
    done && \
    \
    # Extract and install
    tar -xzf /tmp/beszel-agent.tar.gz -C /tmp && \
    if [ ! -f /tmp/beszel-agent ]; then \
        echo "[ERROR] beszel-agent binary not found after extraction" && exit 1; \
    fi && \
    mv /tmp/beszel-agent /usr/local/bin/agent && \
    chmod +x /usr/local/bin/agent /run.sh && \
    \
    # Cleanup
    rm -rf /tmp/beszel-agent* /tmp/config.yaml && \
    echo "[INFO] Beszel Agent installed successfully"

# Verify installation
RUN /usr/local/bin/agent --version || echo "[WARN] Could not verify agent version"

# Run the agent
CMD ["/run.sh"]

ARG BUILD_FROM
FROM $BUILD_FROM

# Set shell
SHELL ["/bin/bash", "-o", "pipefail", "-c"]

# Copy configuration and run script
COPY beszel_version /tmp/beszel_version
COPY run.sh /
COPY healthcheck-http.sh /

# hadolint ignore=DL3008
RUN set -euo pipefail && \
    export DEBIAN_FRONTEND=noninteractive && \
    apt-get update && \
    # busybox provides the httpd used for the watchdog health endpoint,
    # libdrm-amdgpu1 carries amdgpu.ids for naming AMD GPUs on hybrid systems.
    apt-get install -y --no-install-recommends \
        busybox \
        ca-certificates \
        libdrm-amdgpu1 \
        nvtop \
        smartmontools \
        wget && \
    rm -rf /var/lib/apt/lists/* && \
    echo "[INFO] Installed nvtop for GPU monitoring" && \
    echo "[INFO] Installed smartmontools for S.M.A.R.T. monitoring" && \
    echo "[INFO] Installed busybox for watchdog health endpoint" && \
    \
    # Extract upstream Beszel version
    BESZEL_VERSION=$(tr -d '[:space:]' < /tmp/beszel_version) && \
    if [ -z "${BESZEL_VERSION}" ]; then \
        echo "[ERROR] Failed to extract version from beszel_version" && exit 1; \
    fi && \
    echo "[INFO] Installing Beszel Agent version: ${BESZEL_VERSION}" && \
    \
    # Detect architecture. Beszel's NVML collector is built only for linux/amd64
    # with glibc, so amd64 uses the _glibc build; arm64 has no glibc asset and
    # falls back to the portable build with the nvidia-smi collector.
    ARCH=$(uname -m) && \
    case "$ARCH" in \
        x86_64) BESZEL_ASSET="beszel-agent_linux_amd64_glibc.tar.gz" ;; \
        aarch64) BESZEL_ASSET="beszel-agent_linux_arm64.tar.gz" ;; \
        *) echo "[ERROR] Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    echo "[INFO] Detected architecture: ${ARCH} -> ${BESZEL_ASSET}" && \
    \
    # Download and install Beszel Agent with retries
    DOWNLOAD_URL="https://github.com/henrygd/beszel/releases/download/v${BESZEL_VERSION}/${BESZEL_ASSET}" && \
    echo "[INFO] Downloading from: ${DOWNLOAD_URL}" && \
    for i in 1 2 3; do \
        if wget --timeout=30 -q -O /tmp/beszel-agent.tar.gz "${DOWNLOAD_URL}"; then \
            break; \
        elif [ "$i" -eq 3 ]; then \
            echo "[ERROR] Failed to download after 3 attempts" && exit 1; \
        else \
            echo "[WARN] Download attempt $i failed, retrying..." && sleep 2; \
        fi; \
    done && \
    \
    # Extract and install
    tar -xzf /tmp/beszel-agent.tar.gz -C /tmp beszel-agent && \
    if [ ! -f /tmp/beszel-agent ]; then \
        echo "[ERROR] beszel-agent binary not found after extraction" && exit 1; \
    fi && \
    mv /tmp/beszel-agent /usr/local/bin/agent && \
    chmod +x /usr/local/bin/agent /run.sh /healthcheck-http.sh && \
    \
    # Cleanup
    apt-get purge -y wget && \
    apt-get autoremove -y && \
    rm -f /tmp/beszel-agent.tar.gz /tmp/beszel_version && \
    echo "[INFO] Beszel Agent installed successfully with NVIDIA GPU and S.M.A.R.T. support"

# Run the agent
CMD ["/run.sh"]

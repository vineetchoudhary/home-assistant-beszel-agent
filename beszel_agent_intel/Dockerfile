ARG BUILD_FROM
FROM $BUILD_FROM

# Set shell
SHELL ["/bin/sh", "-o", "pipefail", "-c"]

# Copy configuration and run script
COPY beszel_version /tmp/beszel_version
COPY run.sh /
COPY healthcheck-http.sh /

# Install Beszel Agent with Intel GPU and S.M.A.R.T. support
# hadolint ignore=DL3018
RUN set -eu && \
    # igt-gpu-tools (intel_gpu_top) and nvtop only live in edge/testing.
    # libdrm carries amdgpu.ids, which the agent reads to name hybrid AMD GPUs.
    apk add --no-cache libdrm smartmontools busybox-extras && \
    apk add --no-cache -X https://dl-cdn.alpinelinux.org/alpine/edge/testing \
        igt-gpu-tools nvtop && \
    echo "[INFO] Installed intel_gpu_top and nvtop for GPU monitoring" && \
    echo "[INFO] Installed smartmontools for S.M.A.R.T. monitoring" && \
    echo "[INFO] Installed busybox-extras for watchdog health endpoint" && \
    \
    # Extract upstream Beszel version
    BESZEL_VERSION=$(tr -d '[:space:]' < /tmp/beszel_version) && \
    if [ -z "${BESZEL_VERSION}" ]; then \
        echo "[ERROR] Failed to extract version from beszel_version" && exit 1; \
    fi && \
    echo "[INFO] Installing Beszel Agent version: ${BESZEL_VERSION}" && \
    \
    # Detect architecture. igt-gpu-tools is x86_64 only, so this add-on is amd64 only.
    ARCH=$(uname -m) && \
    case "$ARCH" in \
        x86_64) BESZEL_ARCH="amd64" ;; \
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
    rm -f /tmp/beszel-agent.tar.gz /tmp/beszel_version && \
    echo "[INFO] Beszel Agent installed successfully with Intel GPU and S.M.A.R.T. support"

# Run the agent
CMD ["/run.sh"]

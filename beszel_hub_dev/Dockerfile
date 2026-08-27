ARG BUILD_FROM
FROM $BUILD_FROM

SHELL ["/bin/bash", "-o", "pipefail", "-c"]

COPY beszel_version /tmp/beszel_version
COPY run.sh /

RUN set -euo pipefail && \
    BESZEL_VERSION=$(tr -d '[:space:]' < /tmp/beszel_version) && \
    if [ -z "${BESZEL_VERSION}" ]; then \
        echo "[ERROR] Failed to extract version from beszel_version" && exit 1; \
    fi && \
    echo "[INFO] Installing Beszel Hub version: ${BESZEL_VERSION}" && \
    ARCH=$(uname -m) && \
    case "$ARCH" in \
        x86_64) BESZEL_ARCH="amd64" ;; \
        aarch64) BESZEL_ARCH="arm64" ;; \
        *) echo "[ERROR] Unsupported architecture: $ARCH" && exit 1 ;; \
    esac && \
    echo "[INFO] Detected architecture: ${ARCH} -> ${BESZEL_ARCH}" && \
    DOWNLOAD_URL="https://github.com/henrygd/beszel/releases/download/v${BESZEL_VERSION}/beszel_linux_${BESZEL_ARCH}.tar.gz" && \
    echo "[INFO] Downloading from: ${DOWNLOAD_URL}" && \
    for i in 1 2 3; do \
        if wget --timeout=30 -q -O /tmp/beszel-hub.tar.gz "${DOWNLOAD_URL}"; then \
            break; \
        elif [ "$i" -eq 3 ]; then \
            echo "[ERROR] Failed to download after 3 attempts" && exit 1; \
        else \
            echo "[WARN] Download attempt $i failed, retrying..." && sleep 2; \
        fi; \
    done && \
    tar -xzf /tmp/beszel-hub.tar.gz -C /tmp beszel && \
    if [ ! -f /tmp/beszel ]; then \
        echo "[ERROR] beszel binary not found after extraction" && exit 1; \
    fi && \
    mv /tmp/beszel /usr/local/bin/beszel && \
    chmod +x /usr/local/bin/beszel /run.sh && \
    rm -f /tmp/beszel-hub.tar.gz /tmp/beszel_version && \
    echo "[INFO] Beszel Hub installed successfully"

CMD ["/run.sh"]

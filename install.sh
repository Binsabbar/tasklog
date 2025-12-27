#!/bin/bash

# tasklog installation script
# Usage: curl ... | sudo bash -s <VERSION>

set -e

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "Error: Version argument is required."
    echo "Usage: curl ... | sudo bash -s <VERSION>"
    exit 1
fi

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Linux)
        OS="linux"
        ;;
    Darwin)
        OS="darwin"
        ;;
    *)
        echo "Error: Unsupported operating system: $OS"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo "Error: Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

BINARY_NAME="tasklog_${VERSION}_${OS}_${ARCH}"
DOWNLOAD_URL="https://github.com/Binsabbar/tasklog/releases/download/v${VERSION}/${BINARY_NAME}"
INSTALL_DIR="/usr/local/bin"
TARGET_PATH="${INSTALL_DIR}/tasklog"

echo "Installing tasklog version ${VERSION} for ${OS}/${ARCH}..."

# Check if we have write permissions to INSTALL_DIR
if [ ! -w "$INSTALL_DIR" ]; then
    echo "Error: You do not have write permissions for ${INSTALL_DIR}."
    echo "Please run this script with sudo."
    exit 1
fi

TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

echo "Downloading ${DOWNLOAD_URL}..."
if ! curl -fsSL -o "${TMP_DIR}/${BINARY_NAME}" "$DOWNLOAD_URL"; then
    echo "Error: Failed to download binary. Please check the version and internet connection."
    exit 1
fi

chmod +x "${TMP_DIR}/${BINARY_NAME}"
mv "${TMP_DIR}/${BINARY_NAME}" "$TARGET_PATH"

echo "Successfully installed tasklog to ${TARGET_PATH}"
tasklog --version

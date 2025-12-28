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

# Validate VERSION format to prevent injection attacks
# Allows: 1.0.0, 1.0.0-alpha.1, 1.0.0-beta.2, 1.0.0-rc.1, etc.
if ! echo "$VERSION" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[0-9]+)?)?$'; then
    echo "Error: Invalid version format: $VERSION"
    echo "Expected format: X.Y.Z or X.Y.Z-prerelease (e.g., 1.0.0, 1.0.0-alpha.1)"
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

# Check if INSTALL_DIR exists and we have write permissions
if [ ! -d "$INSTALL_DIR" ]; then
    echo "Error: Installation directory ${INSTALL_DIR} does not exist."
    echo "Please create the directory or choose a different installation path."
    exit 1
fi

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

# Verify installation (non-fatal if version check fails)
if tasklog --version 2>/dev/null; then
    echo "Installation verified successfully."
else
    echo "Note: Installation completed, but version check could not be verified."
    echo "You may need to restart your shell or check if ${TARGET_PATH} is in your PATH."
fi

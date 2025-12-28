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

ARCHIVE_NAME="tasklog_${VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/Binsabbar/tasklog/releases/download/v${VERSION}/${ARCHIVE_NAME}"
CHECKSUM_URL="https://github.com/Binsabbar/tasklog/releases/download/v${VERSION}/checksums.txt"
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

# Download the archive
echo "Downloading ${ARCHIVE_NAME}..."
if ! curl -fsSL -o "${TMP_DIR}/${ARCHIVE_NAME}" "$DOWNLOAD_URL"; then
    echo "Error: Failed to download binary. Please check the version and internet connection."
    exit 1
fi

# Download checksums file
echo "Downloading checksums..."
if ! curl -fsSL -o "${TMP_DIR}/checksums.txt" "$CHECKSUM_URL"; then
    echo "Error: Failed to download checksums file."
    exit 1
fi

# Verify checksum
echo "Verifying checksum..."
EXPECTED_CHECKSUM=$(grep "${ARCHIVE_NAME}" "${TMP_DIR}/checksums.txt" | awk '{print $1}')
if [ -z "$EXPECTED_CHECKSUM" ]; then
    echo "Error: Could not find checksum for ${ARCHIVE_NAME} in checksums.txt"
    exit 1
fi

# Calculate checksum (use shasum on macOS, sha256sum on Linux)
if command -v sha256sum &> /dev/null; then
    ACTUAL_CHECKSUM=$(sha256sum "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $1}')
elif command -v shasum &> /dev/null; then
    ACTUAL_CHECKSUM=$(shasum -a 256 "${TMP_DIR}/${ARCHIVE_NAME}" | awk '{print $1}')
else
    echo "Error: Neither sha256sum nor shasum found. Cannot verify checksum."
    exit 1
fi

if [ "$EXPECTED_CHECKSUM" != "$ACTUAL_CHECKSUM" ]; then
    echo "Error: Checksum verification failed!"
    echo "Expected: ${EXPECTED_CHECKSUM}"
    echo "Actual:   ${ACTUAL_CHECKSUM}"
    echo "The downloaded file may be corrupted or tampered with."
    exit 1
fi
echo "Checksum verified successfully."

# Extract the archive
echo "Extracting archive..."
if ! tar -xzf "${TMP_DIR}/${ARCHIVE_NAME}" -C "${TMP_DIR}"; then
    echo "Error: Failed to extract archive."
    exit 1
fi

# Find and install the binary
chmod +x "${TMP_DIR}/tasklog"
mv "${TMP_DIR}/tasklog" "$TARGET_PATH"

echo "Successfully installed tasklog to ${TARGET_PATH}"

# Verify installation (non-fatal if version check fails)
if tasklog --version 2>/dev/null; then
    echo "Installation verified successfully."
else
    echo "Note: Installation completed, but version check could not be verified."
    echo "You may need to restart your shell or check if ${TARGET_PATH} is in your PATH."
fi

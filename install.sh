#!/bin/sh
# muninndb-lite installer
#
# Interactive usage:
#   curl -fsSL https://raw.githubusercontent.com/Aperrix/muninndb-lite/develop/install.sh | sh
#
# Scripted / hook usage:
#   MUNINN_VERSION=v0.4.1-alpha-lite MUNINN_INSTALL_DIR=./bin sh install.sh
#
# Environment variables:
#   MUNINN_VERSION      Pin a specific release tag (default: latest)
#   MUNINN_INSTALL_DIR  Where to install the binary (default: /usr/local/bin or ~/.local/bin)
#   MUNINN_QUIET        Set to 1 to suppress non-error output (for hooks)
set -e

REPO="Aperrix/muninndb-lite"
BIN_NAME="muninndb-lite"

# ── Configuration ────────────────────────────────────────────────────────────
QUIET="${MUNINN_QUIET:-0}"
DESIRED_VERSION="${MUNINN_VERSION:-}"
INSTALL_DIR="${MUNINN_INSTALL_DIR:-}"

log() {
  if [ "${QUIET}" != "1" ]; then
    echo "$@"
  fi
}

# ── Detect platform ─────────────────────────────────────────────────────────
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "${ARCH}" in
  x86_64)          ARCH="amd64" ;;
  arm64|aarch64)   ARCH="arm64" ;;
  *)
    echo "muninndb-lite: unsupported architecture: ${ARCH}" >&2
    exit 1
    ;;
esac

case "${OS}" in
  darwin|linux) ;;
  *)
    echo "muninndb-lite: unsupported OS: ${OS}" >&2
    echo "  Download manually: https://github.com/${REPO}/releases/latest" >&2
    exit 1
    ;;
esac

PLATFORM="${OS}-${ARCH}"

# ── Resolve install directory ────────────────────────────────────────────────
if [ -z "${INSTALL_DIR}" ]; then
  if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
  else
    INSTALL_DIR="${HOME}/.local/bin"
  fi
fi
mkdir -p "${INSTALL_DIR}"

TARGET="${INSTALL_DIR}/${BIN_NAME}"

# ── Resolve version ─────────────────────────────────────────────────────────
if [ -z "${DESIRED_VERSION}" ]; then
  log "  Checking latest release..."
  DESIRED_VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
    | sed -n 's/.*"tag_name" *: *"\([^"]*\)".*/\1/p' | head -1)

  if [ -z "${DESIRED_VERSION}" ]; then
    echo "muninndb-lite: could not determine latest version (GitHub API rate limit?)" >&2
    echo "  Try again in a minute, or download from: https://github.com/${REPO}/releases/latest" >&2
    exit 1
  fi
fi

# ── Check if already installed with correct version ──────────────────────────
if [ -x "${TARGET}" ]; then
  CURRENT_VERSION=$("${TARGET}" version 2>/dev/null || echo "")
  if [ "${CURRENT_VERSION}" = "${DESIRED_VERSION}" ]; then
    log "  ${BIN_NAME} ${DESIRED_VERSION} already installed at ${TARGET}"
    exit 0
  fi
  log "  Upgrading ${BIN_NAME} ${CURRENT_VERSION} → ${DESIRED_VERSION}..."
fi

# ── Download binary ──────────────────────────────────────────────────────────
URL="https://github.com/${REPO}/releases/download/${DESIRED_VERSION}/${BIN_NAME}-${PLATFORM}"
CHECKSUM_URL="${URL%/*}/${BIN_NAME}-${PLATFORM}.sha256"
TMP=$(mktemp)

log "  Downloading ${BIN_NAME} ${DESIRED_VERSION} for ${OS}/${ARCH}..."
HTTP_CODE=$(curl -sSL --progress-bar -w "%{http_code}" -o "${TMP}" "${URL}")
if [ "${HTTP_CODE}" != "200" ]; then
  rm -f "${TMP}"
  echo "" >&2
  echo "muninndb-lite: download failed (HTTP ${HTTP_CODE})" >&2
  echo "  URL: ${URL}" >&2
  echo "" >&2
  echo "  This may mean the release asset for ${PLATFORM} is not yet available." >&2
  echo "  Download manually: https://github.com/${REPO}/releases/tag/${DESIRED_VERSION}" >&2
  exit 1
fi

# ── Verify checksum ──────────────────────────────────────────────────────────
EXPECTED_CHECKSUM=$(curl -fsSL "${CHECKSUM_URL}" 2>/dev/null | awk '{print $1}')
if [ -n "${EXPECTED_CHECKSUM}" ]; then
  if command -v sha256sum > /dev/null 2>&1; then
    ACTUAL_CHECKSUM=$(sha256sum "${TMP}" | awk '{print $1}')
  elif command -v shasum > /dev/null 2>&1; then
    ACTUAL_CHECKSUM=$(shasum -a 256 "${TMP}" | awk '{print $1}')
  else
    ACTUAL_CHECKSUM=""
  fi

  if [ -n "${ACTUAL_CHECKSUM}" ] && [ "${ACTUAL_CHECKSUM}" != "${EXPECTED_CHECKSUM}" ]; then
    rm -f "${TMP}"
    echo "muninndb-lite: checksum verification failed" >&2
    echo "  expected: ${EXPECTED_CHECKSUM}" >&2
    echo "  got:      ${ACTUAL_CHECKSUM}" >&2
    exit 1
  fi
  log "  Checksum verified."
fi

# ── Install ──────────────────────────────────────────────────────────────────
chmod +x "${TMP}"
mv "${TMP}" "${TARGET}"

# ── PATH warning (interactive only) ──────────────────────────────────────────
if [ "${QUIET}" != "1" ]; then
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      echo ""
      echo "  Warning: ${INSTALL_DIR} is not in your PATH."
      echo "  Add this to your shell profile (~/.zshrc or ~/.bashrc):"
      echo ""
      echo "    export PATH=\"${INSTALL_DIR}:\$PATH\""
      echo ""
      ;;
  esac
fi

# ── Done ─────────────────────────────────────────────────────────────────────
log ""
log "  ${BIN_NAME} ${DESIRED_VERSION} installed to ${TARGET}"
log ""
if [ "${QUIET}" != "1" ]; then
  echo "  Quick start (Claude Code):"
  echo "    claude mcp add --transport stdio muninn -- ${BIN_NAME} mcp"
  echo ""
fi

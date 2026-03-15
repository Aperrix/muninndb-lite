#!/bin/sh
# muninndb-lite installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Aperrix/muninndb-lite/develop/install.sh | sh
set -e

REPO="Aperrix/muninndb-lite"
BIN_NAME="muninndb-lite"
INSTALL_DIR="/usr/local/bin"

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

# ── Resolve latest release tag ───────────────────────────────────────────────
echo "  Checking latest release..."
TAG=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | sed -n 's/.*"tag_name" *: *"\([^"]*\)".*/\1/p' | head -1)

if [ -z "${TAG}" ]; then
  echo "muninndb-lite: could not determine latest version (GitHub API rate limit?)" >&2
  echo "  Try again in a minute, or download from: https://github.com/${REPO}/releases/latest" >&2
  exit 1
fi

# ── Download binary ──────────────────────────────────────────────────────────
URL="https://github.com/${REPO}/releases/download/${TAG}/${BIN_NAME}-${PLATFORM}"
TMP=$(mktemp)

echo "  Downloading ${BIN_NAME} ${TAG} for ${OS}/${ARCH}..."
HTTP_CODE=$(curl -sSL --progress-bar -w "%{http_code}" -o "${TMP}" "${URL}")
if [ "${HTTP_CODE}" != "200" ]; then
  rm -f "${TMP}"
  echo "" >&2
  echo "muninndb-lite: download failed (HTTP ${HTTP_CODE})" >&2
  echo "  URL: ${URL}" >&2
  echo "" >&2
  echo "  This may mean the release asset for ${PLATFORM} is not yet available." >&2
  echo "  Download manually: https://github.com/${REPO}/releases/tag/${TAG}" >&2
  exit 1
fi
chmod +x "${TMP}"

# ── Install ──────────────────────────────────────────────────────────────────
if [ -w "${INSTALL_DIR}" ]; then
  mv "${TMP}" "${INSTALL_DIR}/${BIN_NAME}"
else
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "${INSTALL_DIR}"
  mv "${TMP}" "${INSTALL_DIR}/${BIN_NAME}"

  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) ;;
    *)
      echo ""
      echo "  ⚠  ${INSTALL_DIR} is not in your PATH."
      echo "     Add this to your shell profile (~/.zshrc or ~/.bashrc):"
      echo ""
      echo "       export PATH=\"\$HOME/.local/bin:\$PATH\""
      echo ""
      ;;
  esac
fi

# ── Done ─────────────────────────────────────────────────────────────────────
echo ""
echo "  ${BIN_NAME} ${TAG} installed to ${INSTALL_DIR}/${BIN_NAME}"
echo ""
echo "  Quick start (Claude Code):"
echo "    claude mcp add --transport stdio muninn -- ${BIN_NAME} mcp"
echo ""
echo "  Other MCP clients — add to your config:"
echo "    { \"mcpServers\": { \"muninn\": { \"command\": \"${BIN_NAME}\", \"args\": [\"mcp\"] } } }"
echo ""

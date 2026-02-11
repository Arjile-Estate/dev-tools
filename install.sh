#!/usr/bin/env bash
set -euo pipefail

# dev-tools installer
# Usage: curl -fsSL https://raw.githubusercontent.com/slaanesh/dev-tools/main/install.sh | bash

REPO="slaanesh/dev-tools"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"

log() { echo "==> $*"; }
error() { echo "ERROR: $*" >&2; exit 1; }

detect_os() {
    case "$(uname -s)" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       error "Unsupported OS: $(uname -s). Only linux and darwin are supported." ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)  echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *)             error "Unsupported architecture: $(uname -m). Only amd64 and arm64 are supported." ;;
    esac
}

get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"v([^"]+)".*/\1/')
    if [ -z "$version" ]; then
        error "Failed to fetch latest version from GitHub"
    fi
    echo "$version"
}

main() {
    local os arch version archive_name url tmp_dir

    os=$(detect_os)
    arch=$(detect_arch)
    version=$(get_latest_version)

    log "Installing dev-tools v${version} (${os}/${arch})"

    archive_name="dev-tools_${version}_${os}_${arch}.tar.gz"
    url="https://github.com/${REPO}/releases/download/v${version}/${archive_name}"

    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    log "Downloading ${url}"
    curl -fsSL "$url" -o "${tmp_dir}/${archive_name}"

    log "Extracting to ${INSTALL_DIR}"
    mkdir -p "$INSTALL_DIR"
    tar -xzf "${tmp_dir}/${archive_name}" -C "$tmp_dir"
    cp "${tmp_dir}/dev-tools" "$INSTALL_DIR/dev-tools"
    chmod +x "${INSTALL_DIR}/dev-tools"

    log "Installed dev-tools v${version} to ${INSTALL_DIR}/dev-tools"

    # Check if INSTALL_DIR is in PATH
    case ":${PATH}:" in
        *":${INSTALL_DIR}:"*) ;;
        *)
            echo ""
            echo "NOTE: ${INSTALL_DIR} is not in your PATH."
            echo "Add it by running:"
            echo ""
            echo "  export PATH=\"${INSTALL_DIR}:\$PATH\""
            echo ""
            echo "Or add the line above to your shell profile (~/.bashrc, ~/.zshrc, etc.)"
            ;;
    esac
}

main

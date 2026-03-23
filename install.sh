#!/bin/sh
# Lintree Installer
# Usage: curl -fsSL https://get.lintree.sh | sh
#
# Environment variables:
#   LINTREE_INSTALL_DIR - Override the installation directory (default: /usr/local/bin)

set -e

# --- Colors and formatting ---

setup_colors() {
    if [ -t 1 ] && [ -n "$(tput colors 2>/dev/null)" ]; then
        RED='\033[0;31m'
        GREEN='\033[0;32m'
        YELLOW='\033[0;33m'
        BLUE='\033[0;34m'
        MAGENTA='\033[0;35m'
        CYAN='\033[0;36m'
        BOLD='\033[1m'
        RESET='\033[0m'
    else
        RED=''
        GREEN=''
        YELLOW=''
        BLUE=''
        MAGENTA=''
        CYAN=''
        BOLD=''
        RESET=''
    fi
}

info()    { printf "${BLUE}  ->  %s${RESET}\n" "$1"; }
success() { printf "${GREEN}  ✓   %s${RESET}\n" "$1"; }
warn()    { printf "${YELLOW}  !   %s${RESET}\n" "$1"; }
error()   { printf "${RED}  ✗   %s${RESET}\n" "$1" >&2; }

fatal() {
    error "$1"
    exit 1
}

# --- Banner ---

banner() {
    printf "\n"
    printf "${MAGENTA}${BOLD}"
    printf "  ╭─────────────────────────────────╮\n"
    printf "  │                                 │\n"
    printf "  │        lintree installer        │\n"
    printf "  │                                 │\n"
    printf "  ╰─────────────────────────────────╯\n"
    printf "${RESET}"
    printf "\n"
}

# --- Platform detection ---

detect_os() {
    os="$(uname -s | tr '[:upper:]' '[:lower:]')"
    case "$os" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        mingw*|msys*|cygwin*|win*)
            printf "\n"
            warn "Windows is not supported by this installer."
            warn "Please download the binary directly from:"
            printf "\n"
            info "https://github.com/PatchMon/lintree/releases"
            printf "\n"
            exit 1
            ;;
        *)
            fatal "Unsupported operating system: $os"
            ;;
    esac
    success "Detected OS: ${OS}"
}

detect_arch() {
    arch="$(uname -m)"
    case "$arch" in
        x86_64|amd64)   ARCH="amd64" ;;
        aarch64|arm64)   ARCH="arm64" ;;
        *)
            fatal "Unsupported architecture: $arch"
            ;;
    esac
    success "Detected architecture: ${ARCH}"
}

# --- HTTP client detection ---

detect_http_client() {
    if command -v curl >/dev/null 2>&1; then
        HTTP_CLIENT="curl"
    elif command -v wget >/dev/null 2>&1; then
        HTTP_CLIENT="wget"
    else
        fatal "Neither curl nor wget found. Please install one and try again."
    fi
    success "Using HTTP client: ${HTTP_CLIENT}"
}

http_get() {
    url="$1"
    output="$2"
    if [ "$HTTP_CLIENT" = "curl" ]; then
        if [ -n "$output" ]; then
            curl -fsSL -o "$output" "$url"
        else
            curl -fsSL "$url"
        fi
    else
        if [ -n "$output" ]; then
            wget -qO "$output" "$url"
        else
            wget -qO- "$url"
        fi
    fi
}

# --- Fetch latest release ---

fetch_latest_version() {
    info "Fetching latest release..."
    RELEASE_JSON="$(http_get "https://api.github.com/repos/PatchMon/lintree/releases/latest")" || \
        fatal "Failed to fetch latest release from GitHub API."

    # Extract tag_name without jq — works with grep + sed on POSIX
    TAG="$(printf '%s' "$RELEASE_JSON" | grep '"tag_name"' | sed 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/')"

    if [ -z "$TAG" ]; then
        fatal "Could not determine the latest release tag."
    fi
    success "Latest version: ${TAG}"
}

# --- Download and install ---

download_and_install() {
    ARCHIVE="lintree-${OS}-${ARCH}.tar.gz"
    DOWNLOAD_URL="https://github.com/PatchMon/lintree/releases/download/${TAG}/${ARCHIVE}"

    TMPDIR="$(mktemp -d)" || fatal "Failed to create temporary directory."
    # Ensure cleanup on exit
    trap 'rm -rf "$TMPDIR"' EXIT INT TERM

    info "Downloading ${ARCHIVE}..."
    http_get "$DOWNLOAD_URL" "${TMPDIR}/${ARCHIVE}" || \
        fatal "Failed to download ${DOWNLOAD_URL}"
    success "Downloaded successfully."

    # --- SHA256 checksum verification ---
    CHECKSUMS_URL="https://github.com/PatchMon/lintree/releases/download/${TAG}/checksums.txt"
    info "Downloading checksums.txt..."
    if http_get "$CHECKSUMS_URL" "${TMPDIR}/checksums.txt"; then
        success "Downloaded checksums.txt."

        # Compute actual checksum of the downloaded archive
        if [ "$OS" = "darwin" ]; then
            ACTUAL_SUM="$(shasum -a 256 "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')"
        else
            ACTUAL_SUM="$(sha256sum "${TMPDIR}/${ARCHIVE}" | awk '{print $1}')"
        fi

        # Extract expected checksum for this archive from checksums.txt
        EXPECTED_SUM="$(grep "${ARCHIVE}" "${TMPDIR}/checksums.txt" | awk '{print $1}')"

        if [ -z "$EXPECTED_SUM" ]; then
            warn "No checksum entry found for ${ARCHIVE} in checksums.txt. Skipping verification."
        elif [ "$ACTUAL_SUM" != "$EXPECTED_SUM" ]; then
            fatal "SHA256 checksum mismatch for ${ARCHIVE}. Expected: ${EXPECTED_SUM}, got: ${ACTUAL_SUM}. The download may be corrupted or tampered with."
        else
            success "SHA256 checksum verified."
        fi
    else
        warn "Could not download checksums.txt. Skipping checksum verification."
    fi

    info "Extracting archive..."
    tar -xzf "${TMPDIR}/${ARCHIVE}" -C "$TMPDIR" || \
        fatal "Failed to extract archive."
    success "Extracted."

    # Locate the binary (may be named lintree or lintree-{os}-{arch})
    if [ -f "${TMPDIR}/lintree-${OS}-${ARCH}" ]; then
        BINARY_PATH="${TMPDIR}/lintree-${OS}-${ARCH}"
    elif [ -f "${TMPDIR}/lintree" ]; then
        BINARY_PATH="${TMPDIR}/lintree"
    else
        BINARY_PATH="$(find "$TMPDIR" -name 'lintree*' -type f ! -name '*.tar.gz' | head -1)"
        if [ -z "$BINARY_PATH" ]; then
            fatal "Could not find the lintree binary in the extracted archive."
        fi
    fi

    chmod +x "$BINARY_PATH"

    INSTALL_DIR="${LINTREE_INSTALL_DIR:-/usr/local/bin}"

    info "Installing to ${INSTALL_DIR}..."

    # Create install directory if it does not exist
    if [ ! -d "$INSTALL_DIR" ]; then
        if [ "$(id -u)" -eq 0 ]; then
            mkdir -p "$INSTALL_DIR" || fatal "Failed to create directory ${INSTALL_DIR}."
        else
            sudo mkdir -p "$INSTALL_DIR" || fatal "Failed to create directory ${INSTALL_DIR}. Try running with sudo."
        fi
    fi

    # Install the binary
    if [ "$(id -u)" -eq 0 ]; then
        mv "$BINARY_PATH" "${INSTALL_DIR}/lintree" || \
            fatal "Failed to install binary to ${INSTALL_DIR}."
    else
        sudo mv "$BINARY_PATH" "${INSTALL_DIR}/lintree" || \
            fatal "Failed to install binary to ${INSTALL_DIR}. Try running with sudo."
    fi

    success "Installed lintree to ${INSTALL_DIR}/lintree"
}

# --- Verify installation ---

verify() {
    info "Verifying installation..."
    if command -v lintree >/dev/null 2>&1; then
        VERSION_OUTPUT="$(lintree --version 2>&1)" || true
        success "lintree is ready: ${VERSION_OUTPUT}"
    elif [ -x "${INSTALL_DIR}/lintree" ]; then
        VERSION_OUTPUT="$("${INSTALL_DIR}/lintree" --version 2>&1)" || true
        success "lintree is ready: ${VERSION_OUTPUT}"
        warn "${INSTALL_DIR} is not in your PATH. Add it with:"
        printf "\n"
        info "export PATH=\"${INSTALL_DIR}:\$PATH\""
        printf "\n"
    else
        fatal "Installation could not be verified."
    fi
}

# --- Main ---

main() {
    setup_colors
    banner

    detect_os
    detect_arch
    detect_http_client

    printf "\n"

    fetch_latest_version
    download_and_install

    printf "\n"

    verify

    printf "\n"
    printf "${GREEN}${BOLD}  Done! Run 'lintree --help' to get started.${RESET}\n"
    printf "\n"
}

main

#!/bin/sh
# probaci installer — downloads the right release binary for your OS/arch,
# verifies its checksum, and installs it onto your PATH.
#
#   curl -fsSL https://raw.githubusercontent.com/simtabi/probaci/main/install.sh | sh
#
# Environment overrides:
#   PROBACI_VERSION       version to install (default: latest release)
#   PROBACI_INSTALL_DIR   install directory (default: /usr/local/bin if writable
#                         or running as root, else ~/.local/bin)
#   PROBACI_BASE_URL      base URL for the archive + checksums.txt
#                         (default: the GitHub release download dir; set to a
#                          local/file:// dir to install a locally-built bundle)
set -eu

REPO="simtabi/probaci"
BIN="probaci"

info() { printf '%s\n' "==> $*"; }
warn() { printf '%s\n' "warning: $*" >&2; }
die()  { printf '%s\n' "error: $*" >&2; exit 1; }

have() { command -v "$1" >/dev/null 2>&1; }

# download URL FILE
download() {
	if have curl; then curl -fsSL "$1" -o "$2"
	elif have wget; then wget -qO "$2" "$1"
	else die "need curl or wget to download"; fi
}

# fetch URL -> stdout
fetch() {
	if have curl; then curl -fsSL "$1"
	elif have wget; then wget -qO- "$1"
	else die "need curl or wget"; fi
}

detect_platform() {
	os=$(uname -s)
	case "$os" in
		Linux) OS=linux ;;
		Darwin) OS=darwin ;;
		MINGW* | MSYS* | CYGWIN*) OS=windows ;;
		*) die "unsupported OS: $os (use the manual download)" ;;
	esac
	arch=$(uname -m)
	case "$arch" in
		x86_64 | amd64) ARCH=amd64 ;;
		aarch64 | arm64) ARCH=arm64 ;;
		armv7l) ARCH=armv7 ;;
		armv6l) ARCH=armv6 ;;
		i386 | i686) ARCH=386 ;;
		*) die "unsupported arch: $arch (use the manual download)" ;;
	esac
	EXT=tar.gz
	if [ "$OS" = windows ]; then EXT=zip; fi
}

resolve_version() {
	if [ -n "${PROBACI_VERSION:-}" ]; then
		VER=${PROBACI_VERSION#v}
		TAG="v${VER}"
		return
	fi
	[ -n "${PROBACI_BASE_URL:-}" ] && die "set PROBACI_VERSION when PROBACI_BASE_URL is overridden"
	info "resolving latest release"
	TAG=$(fetch "https://api.github.com/repos/${REPO}/releases/latest" \
		| grep -m1 '"tag_name"' | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
	[ -n "$TAG" ] || die "could not resolve the latest release tag"
	VER=${TAG#v}
}

resolve_base() {
	if [ -n "${PROBACI_BASE_URL:-}" ]; then
		BASE=${PROBACI_BASE_URL%/}
	else
		BASE="https://github.com/${REPO}/releases/download/${TAG}"
	fi
}

resolve_install_dir() {
	if [ -n "${PROBACI_INSTALL_DIR:-}" ]; then
		DIR=$PROBACI_INSTALL_DIR
	elif [ -w /usr/local/bin ] 2>/dev/null || [ "$(id -u)" = 0 ]; then
		DIR=/usr/local/bin
	else
		DIR="${HOME}/.local/bin"
	fi
}

# sha256 of a file -> stdout (portable)
sha256() {
	if have sha256sum; then sha256sum "$1" | awk '{print $1}'
	elif have shasum; then shasum -a 256 "$1" | awk '{print $1}'
	else echo ""; fi
}

verify() {
	work=$1 asset=$2
	[ -f "$work/checksums.txt" ] || { warn "no checksums.txt — skipping verification"; return 0; }
	want=$(grep " ${asset}\$" "$work/checksums.txt" | awk '{print $1}' | head -n1)
	[ -n "$want" ] || { warn "checksum for $asset not listed — skipping"; return 0; }
	got=$(sha256 "$work/$asset")
	[ -n "$got" ] || { warn "no sha256 tool — skipping verification"; return 0; }
	[ "$want" = "$got" ] || die "checksum mismatch for $asset (want $want, got $got)"
	info "checksum verified"
}

main() {
	detect_platform
	resolve_version
	resolve_base
	asset="${BIN}_${VER}_${OS}_${ARCH}.${EXT}"

	work=$(mktemp -d)
	# shellcheck disable=SC2064
	trap "rm -rf '$work'" EXIT INT TERM

	info "downloading ${asset}"
	download "${BASE}/${asset}" "${work}/${asset}" || die "download failed: ${BASE}/${asset}"
	download "${BASE}/checksums.txt" "${work}/checksums.txt" 2>/dev/null || true
	verify "$work" "$asset"

	info "extracting"
	( cd "$work" && tar -xzf "$asset" )
	[ -f "${work}/${BIN}" ] || die "archive did not contain ${BIN}"

	resolve_install_dir
	mkdir -p "$DIR" || die "cannot create $DIR"
	install -m 0755 "${work}/${BIN}" "${DIR}/${BIN}" 2>/dev/null \
		|| { cp "${work}/${BIN}" "${DIR}/${BIN}" && chmod 0755 "${DIR}/${BIN}"; } \
		|| die "cannot write ${DIR}/${BIN} (set PROBACI_INSTALL_DIR or run with sudo)"

	info "installed ${BIN} ${VER} to ${DIR}/${BIN}"
	case ":${PATH}:" in
		*":${DIR}:"*) "${DIR}/${BIN}" version 2>/dev/null || true ;;
		*) warn "${DIR} is not on your PATH — add it, e.g.:"
		   # shellcheck disable=SC2016
		   printf '  export PATH="%s:$PATH"\n' "$DIR" >&2 ;;
	esac
}

main "$@"

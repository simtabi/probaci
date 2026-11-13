#!/usr/bin/env bash
#
# Cross-compile every target in build/targets.txt into build/, one folder per OS.
# It builds on this machine and publishes nothing; GoReleaser
# (src/.goreleaser.yaml) is the release path, run by release.yml on a pushed
# vX.Y.Z tag or on request (docs/release.md).
#
# build/ holds one folder per OS and nothing else:
#
#   build/targets.txt                         tracked input, "GOOS GOARCH [GOARM]"
#   build/checksums.txt                       SHA-256 over every binary, as {os}/{file},
#                                             so `cd build && shasum -a 256 -c checksums.txt`
#   build/macos/probaci_macos_arm64           ... and _amd64, and _universal on a macOS host
#   build/linux/probaci_linux_amd64           ... _386, _arm64, _armv6, _armv7
#   build/windows/probaci_windows_amd64.exe   ... _386, _arm64
#   build/archives/                           only with ARCHIVES=1: .tar.gz / .zip per
#                                             binary (plus LICENSE/README/CHANGELOG), and
#                                             their own bare-name checksums.txt
#
# The OS folder and the name use `macos`, never `darwin`; arm is armvN; Windows
# binaries end in .exe. Static (CGO_ENABLED=0), reproducible (-trimpath),
# stripped (-s -w), with version metadata embedded.
#
# Every generated file under build/ is removed first, so nothing stale from a
# removed target survives. targets.txt (and nfpm.yaml, if one is added) is input
# and is kept; when targets.txt is missing it is written from the default matrix
# below.
#
#   VERSION=v1.2.3 ./src/scripts/build-all.sh       # binaries and archives
#   ARCHIVES=0 ./src/scripts/build-all.sh           # binaries only (`make cross`)
#   TARGETS=path/to/list ./src/scripts/build-all.sh # use an external target list
#
# Portable to bash 3.2 (the macOS system bash): no mapfile / wait -n / assoc arrays.
set -euo pipefail

# The script lives in src/scripts/; build/ and the files it bundles are at the
# repository root, and the module it compiles is src/.
ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
cd "$ROOT"

BINARY="probaci"
PKG="github.com/simtabi/probaci/src/internal/version"
BUILD_DIR="build"
ARCHIVE_DIR="${BUILD_DIR}/archives"
TARGETS_FILE="${BUILD_DIR}/targets.txt"
WITH_ARCHIVES="${ARCHIVES:-1}"

# The default target matrix, written to build/targets.txt only when that file is
# missing.
DEFAULT_TARGETS='# Release targets for src/scripts/build-all.sh -- "GOOS GOARCH [GOARM]".
# GoReleaser (src/.goreleaser.yaml) is the release path; this drives `make cross`
# and `make dist`. Output goes to build/<os>/ with names {binary}_{os}_{arch},
# darwin written as macos, arm as armvN, and .exe on windows; a macos_universal
# (Intel + Apple Silicon) binary is produced on macOS.
linux amd64
linux 386
linux arm64
linux arm 6
linux arm 7
darwin amd64
darwin arm64
windows amd64
windows 386
windows arm64'

# --- preflight ---------------------------------------------------------------
command -v go >/dev/null 2>&1 || { echo "error: 'go' is not on PATH" >&2; exit 1; }

mkdir -p "$BUILD_DIR"
if [ -n "${TARGETS:-}" ]; then
  [ -f "$TARGETS" ] || { echo "error: no target list at ${TARGETS}" >&2; exit 1; }
  MATRIX_FILE="$TARGETS"
else
  if [ ! -f "$TARGETS_FILE" ]; then
    printf '%s\n' "$DEFAULT_TARGETS" > "$TARGETS_FILE"
    echo "==> wrote the default matrix to ${TARGETS_FILE}"
  fi
  MATRIX_FILE="$TARGETS_FILE"
fi

# Portable SHA-256: Linux ships sha256sum, macOS/BSD ship shasum.
sha256() {
  if command -v sha256sum >/dev/null 2>&1; then sha256sum "$@"
  elif command -v shasum   >/dev/null 2>&1; then shasum -a 256 "$@"
  else echo "error: no sha256sum or shasum found" >&2; return 1; fi
}

VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo none)}"
DATE="${DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
LDFLAGS="-s -w -X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.Date=${DATE}"

# Extra files bundled into each archive (whichever exist).
EXTRAS=""
for f in LICENSE README.md CHANGELOG.md third-party-notices.md; do
  [ -f "$f" ] && EXTRAS="${EXTRAS} ${f}"
done

# --- clear generated output, keep the inputs ---------------------------------
echo "==> ${BINARY} ${VERSION} (${COMMIT}) into ${BUILD_DIR}/"
for entry in "$BUILD_DIR"/* "$BUILD_DIR"/.[!.]*; do
  [ -e "$entry" ] || continue
  case "$(basename "$entry")" in
    targets.txt|nfpm.yaml) ;;
    *) rm -rf "$entry" ;;
  esac
done
[ "$WITH_ARCHIVES" = "1" ] && mkdir -p "$ARCHIVE_DIR"

# make_archive <os-folder> <bare-binary-filename> <goos>
make_archive() {
  [ "$WITH_ARCHIVES" = "1" ] || return 0
  folder="$1"; bin="$2"; goos="$3"
  base="${bin%.exe}"                      # probaci_macos_arm64
  stage="$(mktemp -d "${TMPDIR:-/tmp}/build-all.XXXXXX")"
  cp "${BUILD_DIR}/${folder}/${bin}" "${stage}/${bin}"
  # shellcheck disable=SC2086
  [ -n "$EXTRAS" ] && cp $EXTRAS "${stage}/"
  if [ "$goos" = "windows" ]; then
    if command -v zip >/dev/null 2>&1; then
      ( cd "$stage" && zip -qr "${base}.zip" . )
      mv "${stage}/${base}.zip" "${ARCHIVE_DIR}/"
    else
      echo "  ! 'zip' not found; skipping ${base}.zip" >&2
    fi
  else
    tar -czf "${ARCHIVE_DIR}/${base}.tar.gz" -C "$stage" .
  fi
  rm -rf "$stage"
}

# --- build every target ------------------------------------------------------
count=0
have_macos_amd64=0
have_macos_arm64=0

# `|| [ -n "$goos" ]` processes a final line with no trailing newline; the \r in
# IFS tolerates CRLF line endings.
while IFS=$' \t\r' read -r goos goarch goarm || [ -n "${goos:-}" ]; do
  case "$goos" in ''|\#*) continue ;; esac   # skip blanks and comments

  os_token="$goos";   [ "$goos" = "darwin" ] && os_token="macos"
  arch_token="$goarch"; [ "$goarch" = "arm" ] && arch_token="armv${goarm}"
  ext="";             [ "$goos" = "windows" ] && ext=".exe"

  name="${BINARY}_${os_token}_${arch_token}${ext}"
  mkdir -p "${BUILD_DIR}/${os_token}"
  printf '  -> %-40s' "${os_token}/${name}"

  GOOS="$goos" GOARCH="$goarch" GOARM="${goarm:-}" CGO_ENABLED=0 \
    go -C src build -trimpath -ldflags "$LDFLAGS" \
      -o "${ROOT}/${BUILD_DIR}/${os_token}/${name}" ./cmd/probaci
  echo "ok"

  make_archive "$os_token" "$name" "$goos"
  count=$((count + 1))
  [ "$name" = "${BINARY}_macos_amd64" ] && have_macos_amd64=1
  [ "$name" = "${BINARY}_macos_arm64" ] && have_macos_arm64=1
done < "$MATRIX_FILE"

[ "$count" -gt 0 ] || { echo "error: no targets built (empty matrix?)" >&2; exit 1; }

# --- macOS universal (Intel + Apple Silicon in one binary) -------------------
if [ "$have_macos_amd64" = 1 ] && [ "$have_macos_arm64" = 1 ]; then
  if command -v lipo >/dev/null 2>&1; then
    uni="${BINARY}_macos_universal"
    printf '  -> %-40s' "macos/${uni}"
    lipo -create -output "${BUILD_DIR}/macos/${uni}" \
      "${BUILD_DIR}/macos/${BINARY}_macos_amd64" "${BUILD_DIR}/macos/${BINARY}_macos_arm64"
    echo "ok"
    make_archive macos "$uni" "darwin"
  else
    echo "  ! 'lipo' not available (non-macOS host); skipping macos_universal" >&2
  fi
fi

# --- checksums ---------------------------------------------------------------
# build/checksums.txt names {os}/{file}, so it verifies from inside build/ as-is.
( cd "$BUILD_DIR" && find . -mindepth 2 -maxdepth 2 -type f -name "${BINARY}_*" -not -path './archives/*' \
    | sed 's|^\./||' | LC_ALL=C sort | while IFS= read -r f; do sha256 "$f"; done > checksums.txt )
echo "==> wrote ${BUILD_DIR}/checksums.txt"
# The archives' checksums keep bare names, the form release assets use.
if [ "$WITH_ARCHIVES" = "1" ] && ls "${ARCHIVE_DIR}/${BINARY}_"* >/dev/null 2>&1; then
  ( cd "$ARCHIVE_DIR" && sha256 "${BINARY}"_* > checksums.txt )
  echo "==> wrote ${ARCHIVE_DIR}/checksums.txt"
fi

echo "==> done: ${count} binaries in ${BUILD_DIR}/<os>/$( [ "$WITH_ARCHIVES" = "1" ] && echo ", archives in ${ARCHIVE_DIR}" )"

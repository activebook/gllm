#!/bin/sh
set -eu

# -------- defaults (override via flags or env) --------
OWNER="${OWNER:-activebook}"
REPO="${REPO:-gllm}"
BIN="${BIN:-gllm}"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
ASSET="${ASSET:-}"          # if empty, computed from arch
VERSION="${VERSION:-latest}" # "latest" or a tag like "v1.2.3"
SKIP_VERIFY="${SKIP_VERIFY:-0}"
REQUIRE_CHECKSUM="${REQUIRE_CHECKSUM:-0}" # set to 1 to fail hard if checksum file missing

# -------- pretty errors --------
die() { printf >&2 "ERROR: %s\n" "$*"; exit 1; }
warn(){ printf >&2 "WARN: %s\n" "$*"; }

need_cmd() { command -v "$1" >/dev/null 2>&1 || die "Missing required command: $1"; }
either_cmd() { for cmd in "$@"; do command -v "$cmd" >/dev/null 2>&1 && return 0; done; die "Missing at least one of: $@"; }

usage() {
  cat <<EOF
Usage:
  $0 [--version vX.Y.Z|latest] [--dir /path] [--asset name.tar.gz] [--skip-verify]

Environment overrides:
  OWNER, REPO, BIN, INSTALL_DIR, ASSET, VERSION, SKIP_VERIFY, REQUIRE_CHECKSUM

Examples:
  curl -fsSL https://raw.githubusercontent.com/$OWNER/$REPO/main/build/install.sh | sh
  sh install.sh --version v1.2.3
  sh install.sh --dir ~/.local/bin
EOF
}

# -------- args --------
while [ "${1:-}" ]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --version) VERSION="${2:-}"; [ -n "$VERSION" ] || die "--version needs a value"; shift 2 ;;
    --dir) INSTALL_DIR="${2:-}"; [ -n "$INSTALL_DIR" ] || die "--dir needs a value"; shift 2 ;;
    --asset) ASSET="${2:-}"; [ -n "$ASSET" ] || die "--asset needs a value"; shift 2 ;;
    --skip-verify) SKIP_VERIFY=1; shift ;;
    *) die "Unknown argument: $1 (use --help)" ;;
  esac
done

need_cmd uname
need_cmd tar
need_cmd mktemp
need_cmd grep
need_cmd awk
need_cmd sed
either_cmd curl wget
either_cmd doas sudo

# -------- downloader (curl preferred, wget fallback) --------
download() {
  url="$1"
  out="$2"

  if command -v curl >/dev/null 2>&1; then
    curl --progress-bar -fL --retry 3 --retry-delay 1 --connect-timeout 10 -o "$out" "$url" \
      || die "Failed to download: $url"
  elif command -v wget >/dev/null 2>&1; then
    wget -qO "$out" "$url" || die "Failed to download: $url"
  else
    die "Need curl or wget to download files"
  fi
}

# -------- sha256 verifier (tries sha256sum, shasum, openssl) --------
sha256_check() {
  file="$1"
  expected="$2"

  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$file" | awk '{print $1}')"
  elif command -v shasum >/dev/null 2>&1; then
    actual="$(shasum -a 256 "$file" | awk '{print $1}')"
  elif command -v openssl >/dev/null 2>&1; then
    actual="$(openssl dgst -sha256 "$file" | awk '{print $NF}')"
  else
    die "No SHA-256 tool found (install coreutils or openssl), or rerun with --skip-verify"
  fi

  [ "$actual" = "$expected" ] || die "Checksum mismatch for $file
  expected: $expected
  actual:   $actual"
}

# -------- detect OS and arch --------
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin)
    case "$ARCH" in
      x86_64) GOOS="Darwin"; GOARCH="x86_64" ;;
      arm64|aarch64) GOOS="Darwin"; GOARCH="arm64" ;;
      *) die "Unsupported architecture: $ARCH" ;;
    esac
    ;;
  Linux)
    case "$ARCH" in
      x86_64|amd64) GOOS="Linux"; GOARCH="x86_64" ;;
      aarch64|arm64) GOOS="Linux"; GOARCH="arm64" ;;
      *) die "Unsupported architecture: $ARCH" ;;
    esac
    ;;
  *)
    die "Unsupported operating system: $OS"
    ;;
esac

# Default asset naming
if [ -z "$ASSET" ]; then
  ASSET="${BIN}_${GOOS}_${GOARCH}.tar.gz"
fi

tmpdir="$(mktemp -d)"
cleanup() { rm -rf "$tmpdir"; }
trap cleanup EXIT INT TERM

# -------- resolve tag --------
TAG="$VERSION"
if [ "$VERSION" = "latest" ]; then
  # GitHub documents /releases/latest as a stable "latest release" link. :contentReference[oaicite:0]{index=0}
  # We use it to learn the real tag via redirect header.
  if command -v curl >/dev/null 2>&1; then
    TAG="$(curl -fsSLI "https://github.com/$OWNER/$REPO/releases/latest" \
      | grep -i '^location:' | awk -F'/' '{print $NF}' \
      | tr -d '\r' )" || true
  elif command -v wget >/dev/null 2>&1; then
    TAG="$(wget -S -O /dev/null "https://github.com/$OWNER/$REPO/releases/latest" 2>&1 \
      | grep -i '^  location:' | awk -F'/' '{print $NF}' \
      | tr -d '\r' )" || true
  fi
  [ -n "$TAG" ] || die "Could not resolve latest tag from GitHub"
fi

BASE="https://github.com/$OWNER/$REPO/releases/download/$TAG"
ASSET_URL="$BASE/$ASSET"

printf "Installing %s from %s/%s ...\n" "$BIN" "$OWNER" "$REPO"
printf "  tag:    %s\n" "$TAG"
printf "  asset:  %s\n" "$ASSET"
printf "  dir:    %s\n" "$INSTALL_DIR"

# -------- download asset --------
asset_path="$tmpdir/$ASSET"
download "$ASSET_URL" "$asset_path"

# -------- checksum verify (GoReleaser default is project_version_checksums.txt) :contentReference[oaicite:1]{index=1} --------
if [ "$SKIP_VERIFY" -eq 1 ]; then
  warn "Skipping checksum verification (--skip-verify)."
else
  # Try a few common GoReleaser checksum filenames:
  # Default: <project>_<version>_checksums.txt (version often without leading 'v'). :contentReference[oaicite:2]{index=2}
  V_NOPREFIX="$(printf "%s" "$TAG" | sed 's/^v//')"

  candidates="
${REPO}_${V_NOPREFIX}_checksums.txt
${REPO}_${TAG}_checksums.txt
${BIN}_${V_NOPREFIX}_checksums.txt
checksums.txt
"

  checksum_file=""
  for f in $candidates; do
    url="$BASE/$f"
    out="$tmpdir/$f"
    if ( download "$url" "$out" ) 2>/dev/null; then
      checksum_file="$out"
      break
    fi
  done

  if [ -z "$checksum_file" ]; then
    msg="No checksums file found in the release (tried common names)."
    if [ "$REQUIRE_CHECKSUM" -eq 1 ]; then
      die "$msg Set REQUIRE_CHECKSUM=0 to allow install without verification."
    else
      warn "$msg Proceeding without verification."
    fi
  else
    # checksums file format is: "<sha256>  <filename>"
    expected="$(grep "  $ASSET\$" "$checksum_file" | awk '{print $1}' || true)"
    [ -n "$expected" ] || die "Checksums file downloaded but no entry for $ASSET in $(basename "$checksum_file")"
    sha256_check "$asset_path" "$expected"
    printf "  verified: sha256 OK\n"
  fi
fi

# -------- extract & find binary --------
tar -xzf "$asset_path" -C "$tmpdir"

# Try common layouts: either binary at root, or inside a folder
bin_path=""
if [ -f "$tmpdir/$BIN" ]; then
  bin_path="$tmpdir/$BIN"
else
  # find first matching file named BIN
  bin_path="$(find "$tmpdir" -maxdepth 3 -type f -name "$BIN" 2>/dev/null | head -n 1 || true)"
fi
[ -n "$bin_path" ] || die "After extracting, could not find '$BIN' inside the tarball."

chmod +x "$bin_path" || true

# -------- install (use privilege escalation if needed) --------
mkdir -p "$INSTALL_DIR" 2>/dev/null || true

install_cmd="install -m 0755 \"$bin_path\" \"$INSTALL_DIR/$BIN\""
if [ -w "$INSTALL_DIR" ] 2>/dev/null; then
  sh -c "$install_cmd" || die "Failed to install into $INSTALL_DIR"
else
  if command -v doas >/dev/null 2>&1; then
    doas sh -c "$install_cmd" || die "doas install failed"
  elif command -v sudo >/dev/null 2>&1; then
    sudo sh -c "$install_cmd" || die "sudo install failed (do you have sudo rights?)"
  else
    die "No write permission to $INSTALL_DIR and no privilege escalation tool (doas/sudo) available. Re-run as root or use --dir \$HOME/.local/bin"
  fi
fi

printf "Done!\n"
printf "Run: %s --help\n" "$BIN"

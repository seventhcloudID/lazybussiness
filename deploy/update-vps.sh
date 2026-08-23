#!/usr/bin/env bash
# Update app di VPS (user ubuntu — folder aaPanel biasanya root:www).
#   curl -fsSL https://raw.githubusercontent.com/seventhcloudID/lazybussiness/main/deploy/update-vps.sh | sudo bash
set -euo pipefail

APP_DIR="${APP_DIR:-/www/wwwroot/flowa.tigaawan.com}"
REPO="${REPO:-https://github.com/seventhcloudID/lazybussiness.git}"
BRANCH="${BRANCH:-main}"
TMP="/tmp/lazybussiness-src-$$"

SUDO=""
if [[ $(id -u) -ne 0 ]]; then
  SUDO="sudo"
  if [[ ! -w "$APP_DIR" ]]; then
    echo "==> $APP_DIR tidak writable — pakai sudo untuk sync, binary & restart"
  fi
fi

echo "==> Clone $REPO ($BRANCH) ke $TMP"
rm -rf "$TMP"
git clone --depth 1 --branch "$BRANCH" "$REPO" "$TMP"

echo "==> Build di $TMP (hindari permission + VCS error di APP_DIR)"
export PATH="/usr/local/go/bin:${PATH:-}"
(
  cd "$TMP"
  CGO_ENABLED=0 go build -buildvcs=false -o lazybussiness .
)

echo "==> Sync source ke $APP_DIR"
mkdir -p "$APP_DIR"
$SUDO rsync -a \
  --exclude '.env' \
  --exclude '.data' \
  --exclude 'lazybussiness' \
  "$TMP"/ "$APP_DIR"/

echo "==> Install binary"
$SUDO install -m 755 "$TMP/lazybussiness" "$APP_DIR/lazybussiness"
rm -rf "$TMP"

echo "==> Restart service"
$SUDO systemctl restart lazybussiness
$SUDO systemctl --no-pager status lazybussiness || true

echo ""
echo "Selesai — https://flowa.tigaawan.com"

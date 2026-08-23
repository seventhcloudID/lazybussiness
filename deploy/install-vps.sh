#!/usr/bin/env bash
# Jalankan di VPS:
#   curl -fsSL https://raw.githubusercontent.com/seventhcloudID/lazybussiness/main/deploy/install-vps.sh | bash
set -euo pipefail

APP_DIR="${APP_DIR:-/www/wwwroot/flowa.tigaawan.com}"
REPO="https://github.com/seventhcloudID/lazybussiness.git"
TMP="/tmp/lazybussiness-src-$$"

echo "==> Clone ke $TMP"
rm -rf "$TMP"
git clone --depth 1 "$REPO" "$TMP"

echo "==> Sync ke $APP_DIR (file lama aaPanel tidak dihapus total, ditimpa oleh repo)"
mkdir -p "$APP_DIR"
SUDO=""
if [[ ! -w "$APP_DIR" ]]; then
  SUDO="sudo"
fi
$SUDO rsync -a --exclude '.env' --exclude '.data' --exclude 'lazybussiness' "$TMP"/ "$APP_DIR"/
rm -rf "$TMP"

export PATH="/usr/local/go/bin:${PATH:-}"

if ! command -v go >/dev/null 2>&1; then
  echo "==> Install Go 1.22"
  curl -fsSL https://go.dev/dl/go1.22.12.linux-amd64.tar.gz -o /tmp/go.tgz
  rm -rf /usr/local/go
  tar -C /usr/local -xzf /tmp/go.tgz
  export PATH="/usr/local/go/bin:$PATH"
  if ! grep -q '/usr/local/go/bin' /etc/profile; then
    echo 'export PATH=/usr/local/go/bin:$PATH' >> /etc/profile
  fi
fi
export PATH="/usr/local/go/bin:$PATH"

echo "==> Build"
(
  cd "$APP_DIR"
  CGO_ENABLED=0 go build -buildvcs=false -o lazybussiness .
) || {
  echo "==> Build gagal di APP_DIR (permission?) — build di /tmp lalu install"
  BUILD_TMP="/tmp/lazybussiness-build-$$"
  rm -rf "$BUILD_TMP"
  git clone --depth 1 "$REPO" "$BUILD_TMP"
  ( cd "$BUILD_TMP"; CGO_ENABLED=0 go build -buildvcs=false -o lazybussiness . )
  $SUDO install -m 755 "$BUILD_TMP/lazybussiness" "$APP_DIR/lazybussiness"
  rm -rf "$BUILD_TMP"
}

cd "$APP_DIR"
if [[ ! -f "$APP_DIR/.env" ]]; then
  $SUDO cp "$APP_DIR/.env.example" "$APP_DIR/.env"
  echo "==> .env dibuat dari contoh — WAJIB edit: nano $APP_DIR/.env"
fi

echo "==> systemd"
$SUDO cp -f deploy/lazybussiness.service /etc/systemd/system/lazybussiness.service
$SUDO systemctl daemon-reload
$SUDO systemctl enable lazybussiness
$SUDO systemctl restart lazybussiness
$SUDO systemctl --no-pager status lazybussiness || true

echo ""
echo "Selesai. Edit kredensial lalu restart:"
echo "  nano $APP_DIR/.env"
echo "  systemctl restart lazybussiness"
echo "Nginx harus proxy_pass ke http://127.0.0.1:8080 (lihat deploy/nginx-flowa.conf)"
echo "Untuk avoid 504 thumbnail: proxy_read_timeout 600s di location /api/ai/ — lalu nginx -s reload"

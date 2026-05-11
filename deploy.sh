#!/bin/bash
# deploy.sh - Script deploy otomatis untuk BuyaClaw
# Usage: ./deploy.sh

set -e  # Stop jika ada error

DIR="/www/wwwroot/claw.elvisyam.id"
PM2_NAME="picoclaw-web"

# Warna output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

log()  { echo -e "${GREEN}[✓]${NC} $1"; }
warn() { echo -e "${YELLOW}[!]${NC} $1"; }
fail() { echo -e "${RED}[✗]${NC} $1"; exit 1; }

echo ""
echo "================================================"
echo "   BuyaClaw Deploy Script"
echo "================================================"
echo ""

cd "$DIR" || fail "Direktori $DIR tidak ditemukan"

# 1. Pull update terbaru
log "Pulling update dari GitHub..."
git checkout -- web/frontend/src/routeTree.gen.ts 2>/dev/null || true
git pull origin main || fail "Git pull gagal"

# 2. Build frontend
log "Building frontend..."
cd "$DIR/web/frontend"
npm install --silent || fail "npm install gagal"
npm run build || fail "npm run build gagal"

# 3. Copy dist ke backend untuk di-embed
log "Menyalin frontend dist ke backend..."
rm -rf "$DIR/web/backend/dist"
cp -r "$DIR/web/frontend/dist" "$DIR/web/backend/dist" || fail "Copy dist gagal"

# 4. Build binary Go
cd "$DIR"
log "Building picoclaw binary..."
go build -o picoclaw ./cmd/picoclaw/ || fail "Build picoclaw gagal"

log "Building picoclaw-launcher binary..."
go build -o picoclaw-launcher ./web/backend/ || fail "Build picoclaw-launcher gagal"

# 5. Restart via PM2
log "Restarting $PM2_NAME via PM2..."
pm2 restart "$PM2_NAME" || fail "PM2 restart gagal"

# 6. Simpan state PM2
pm2 save --force > /dev/null 2>&1

# 7. Verifikasi
sleep 3
STATUS=$(pm2 jlist | python3 -c "
import sys, json
procs = json.load(sys.stdin)
for p in procs:
    if p['name'] == '$PM2_NAME':
        print(p['pm2_env']['status'])
" 2>/dev/null || echo "unknown")

echo ""
echo "================================================"
if [ "$STATUS" = "online" ]; then
    log "Deploy berhasil! $PM2_NAME status: online ✅"
else
    warn "Deploy selesai tapi status PM2: $STATUS"
    warn "Cek log: pm2 logs $PM2_NAME"
fi
echo "================================================"
echo ""

#!/bin/bash
# Ganti proses panel di launchd dengan biner ./panel yang baru dibangun.
# Jebakan: JANGAN `launchctl kickstart -k` setelah rebuild (EX_CONFIG 78); pakai bootout lalu bootstrap, lalu tunggu.
set -euo pipefail
cd "$(dirname "$0")/.."
LABEL=com.macminiops.panel
PLIST="$HOME/Library/LaunchAgents/$LABEL.plist"
U=$(id -u)
[ -x ./panel ] || { echo "biner ./panel tidak ada (jalankan make build)"; exit 1; }
[ -f "$PLIST" ] || { echo "LaunchAgent tidak ada: $PLIST (jalankan install.sh)"; exit 1; }
launchctl bootout "gui/$U/$LABEL" 2>/dev/null || true
sleep 3
launchctl bootstrap "gui/$U" "$PLIST"
echo "menunggu panel naik (maks 120 dtk)..."
for i in $(seq 1 60); do
  code=$(curl -s -o /dev/null -m 3 -w '%{http_code}' http://127.0.0.1:9100/api/settings/public || true)
  [ "$code" = 200 ] && { echo "OK: panel menjawab (200) setelah ~$((i*2)) dtk"; exit 0; }
  sleep 2
done
echo "GAGAL: panel tidak menjawab dalam 120 dtk (lihat ~/Library/Logs/panel*.log)"; exit 1

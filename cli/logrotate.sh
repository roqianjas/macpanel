#!/bin/bash
# Rotasi log tanpa sudo. Dijalankan harian oleh launchd (com.macminiops.logrotate).
# Aturan: bila file > LIMIT_KB -> salin ke <file>.<timestamp>.gz lalu kosongkan (copytruncate, aman untuk proses yang masih menulis).
# Simpan KEEP salinan terbaru. Nginx: setelah rotasi kirim 'nginx -s reopen'.
set -u
export PATH=/opt/homebrew/bin:$PATH
LIMIT_KB=${LIMIT_KB:-5120}; KEEP=${KEEP:-5}
LOGS=(
  /opt/homebrew/var/log/nginx/access.log
  /opt/homebrew/var/log/nginx/error.log
  /opt/homebrew/var/log/netdata/collector.log
  /opt/homebrew/var/log/netdata/daemon.log
  /opt/homebrew/var/log/netdata/access.log
  /opt/homebrew/var/log/redis.log
  /opt/homebrew/var/log/postgresql@17.log
  /opt/homebrew/var/log/php-fpm.log
  "$HOME/Library/Logs/uptime-kuma.log"
  "$HOME/Library/Logs/hermes-dashboard.log"
  "$HOME/Library/Logs/portal-build.log"
  "$HOME/Library/Logs/panel.log"
  "$HOME/Library/Logs/macminiops-backup.log"
)
# glob: log worker/job lab dan pool PHP-FPM dev (kosong bila belum ada)
for g in "$HOME"/Library/Logs/lab/*.log /opt/homebrew/var/log/php-fpm-dev-*.log; do [ -f "$g" ] && LOGS+=("$g"); done
ts=$(date +%Y%m%d-%H%M%S); rotated=0
for f in "${LOGS[@]}"; do
  [ -f "$f" ] || continue
  kb=$(( $(stat -f %z "$f") / 1024 ))
  [ "$kb" -gt "$LIMIT_KB" ] || continue
  gzip -c "$f" > "$f.$ts.gz" && : > "$f" && { echo "rotated $f (${kb} KB)"; rotated=$((rotated+1)); }
  ls -1t "$f".*.gz 2>/dev/null | tail -n +$((KEEP+1)) | while read old; do rm -f "$old"; done
done
if [ "$rotated" -gt 0 ] && pgrep -f 'nginx: master' >/dev/null; then nginx -s reopen 2>/dev/null; fi
echo "$(date '+%F %T') logrotate selesai: $rotated file dirotasi"

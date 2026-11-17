#!/bin/bash
# Backup staging: ~/Developer + DB dumps + dotfiles -> ~/Backups/staging/<timestamp>/
# Bukan backup sungguhan sampai disalin ke media lain (disk eksternal / cloud). Lihat DEST di bawah.
# Private key SSH SENGAJA tidak ikut. Rahasia DB tidak pernah keluar dari container.
set -euo pipefail
export PATH=/opt/homebrew/bin:$PATH
TS=$(date +%Y%m%d-%H%M%S)
OUT="$HOME/Backups/staging/$TS"
KEEP=7
mkdir -p "$OUT"; chmod 700 "$HOME/Backups" "$HOME/Backups/staging" "$OUT"

echo "[1/5] PostgreSQL dump"
set -a; . "$HOME/.config/macminiops/db.env"; set +a
PGPASSWORD="$POSTGRES_PASSWORD" /opt/homebrew/opt/postgresql@17/bin/pg_dumpall --no-role-passwords -h 127.0.0.1 -U dev | gzip > "$OUT/postgres-all.sql.gz"

echo "[2/5] MySQL dump"
MYSQL_PWD="$MYSQL_ROOT_PASSWORD" /opt/homebrew/opt/mysql@8.4/bin/mysqldump -h127.0.0.1 -uroot --all-databases --single-transaction --no-tablespaces --routines --events | gzip > "$OUT/mysql-all.sql.gz"

echo "[3/5] ~/Developer (tanpa node_modules/.venv/build cache, tanpa .env)"
tar -czf "$OUT/developer.tar.gz" -C "$HOME" \
  --exclude='node_modules' --exclude='.venv' --exclude='__pycache__' --exclude='dist' --exclude='.next' \
  --exclude='.env' --exclude='.env.*' --exclude='Developer/_ops/stack/.env' \
  Developer

echo "[4/5] Dotfiles"
cd "$HOME"
FILES=""
for f in .zshrc .zprofile .gitconfig .gitignore_global .ssh/config .config/mise/config.toml; do [ -e "$f" ] && FILES="$FILES $f"; done
tar -czf "$OUT/dotfiles.tar.gz" $FILES
ls "$HOME"/.ssh/*.pub >/dev/null 2>&1 && tar -czf "$OUT/ssh-public-keys.tar.gz" .ssh/*.pub .ssh/authorized_keys 2>/dev/null || true

echo "[5/5] DB panel (snapshot konsisten; kunci enkripsi TOTP ada di Keychain dan SENGAJA tidak ikut)"
PANEL_DB="$HOME/.config/macminiops/panel/panel.db"
if [ -f "$PANEL_DB" ]; then
  sqlite3 "$PANEL_DB" ".backup '$OUT/panel.db'" && chmod 600 "$OUT/panel.db"
  [ "$(sqlite3 "$OUT/panel.db" 'PRAGMA integrity_check;')" = "ok" ] || { echo "snapshot DB panel rusak"; exit 1; }
  sqlite3 "$OUT/panel.db" 'PRAGMA journal_mode=DELETE;' >/dev/null; rm -f "$OUT/panel.db-shm" "$OUT/panel.db-wal"   # snapshot satu berkas, tanpa berkas samping WAL
fi

# Checksum + validasi arsip
( cd "$OUT" && shasum -a 256 * > SHA256SUMS && for f in *.gz; do gzip -t "$f"; done )
echo "OK: $OUT"; du -sh "$OUT"

# Retensi lokal
ls -1dt "$HOME"/Backups/staging/*/ | tail -n +$((KEEP+1)) | xargs -I{} rm -rf {}

# DEST (isi nanti, jangan simpan kredensial di skrip):
#   disk eksternal : rsync -a "$OUT/" /Volumes/<NAMA>/macmini-backup/$TS/
#   cloud (restic) : restic -r <repo> backup "$OUT"

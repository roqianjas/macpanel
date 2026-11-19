#!/bin/bash
MP="${MACPANEL_HOME:-$HOME/MacPanel}"
# Backup staging: ~/MacPanel/wwwroot + DB dumps + dotfiles -> ~/MacPanel/backup/staging/<timestamp>/
# Bukan backup sungguhan sampai disalin ke media lain (disk eksternal / cloud). Lihat DEST di bawah.
# Private key SSH SENGAJA tidak ikut. Kata sandi DB tidak ikut dicetak ke log.
set -euo pipefail
export PATH=/opt/homebrew/bin:$PATH
TS=$(date +%Y%m%d-%H%M%S)
OUT="$MP/backup/staging/$TS"
KEEP=7
CONF="$HOME/.config/macminiops/backup.conf"
if [ -f "$CONF" ]; then K=$(sed -n 's/^KEEP=\([0-9]\{1,3\}\)$/\1/p' "$CONF" | head -1); [ -n "$K" ] && [ "$K" -ge 1 ] && KEEP=$K; fi
# Jadwal otomatis (bukan manual `lab backup run`, yang mengirim BACKUP_FORCE=1): hormati ENABLED dan EVERY_DAYS
if [ -z "${BACKUP_FORCE:-}" ] && [ -f "$CONF" ]; then
  EN=$(sed -n 's/^ENABLED=\([01]\)$/\1/p' "$CONF" | head -1); EV=$(sed -n 's/^EVERY_DAYS=\([0-9]\{1,2\}\)$/\1/p' "$CONF" | head -1)
  [ "${EN:-1}" = 0 ] && { echo "backup otomatis dimatikan; dilewati"; exit 0; }
  if [ "${EV:-1}" -gt 1 ] 2>/dev/null; then
    LAST=$(ls -1d "$MP"/backup/staging/20* 2>/dev/null | tail -1)
    if [ -n "$LAST" ] && [ $(( $(date +%s) - $(stat -f %m "$LAST") )) -lt $(( EV * 86400 - 7200 )) ]; then echo "backup terakhir kurang dari $EV hari; dilewati"; exit 0; fi
  fi
fi
mkdir -p "$OUT"; chmod 700 "$MP/backup" "$MP/backup/staging" "$OUT"

echo "[1/5] PostgreSQL dump"
set -a; . "$HOME/.config/macminiops/db.env"; set +a
PGPASSWORD="$POSTGRES_PASSWORD" /opt/homebrew/opt/postgresql@17/bin/pg_dumpall --no-role-passwords -h 127.0.0.1 -U dev | gzip > "$OUT/postgres-all.sql.gz"

echo "[2/5] MySQL dump"
MYSQL_PWD="$MYSQL_ROOT_PASSWORD" /opt/homebrew/opt/mysql@8.4/bin/mysqldump -h127.0.0.1 -P3307 -uroot --all-databases --single-transaction --no-tablespaces --routines --events | gzip > "$OUT/mysql-all.sql.gz"
if [ -n "${MARIADB_ROOT_PASSWORD:-}" ]; then
  MYSQL_PWD="$MARIADB_ROOT_PASSWORD" /opt/homebrew/opt/mariadb@11.8/bin/mariadb-dump -h127.0.0.1 -P3306 -uroot --all-databases --single-transaction --routines --events | gzip > "$OUT/mariadb-all.sql.gz"
fi

echo "[3/5] ~/MacPanel/wwwroot (tanpa node_modules/.venv/build cache, tanpa .env)"
# bagian ini bernama 'developer' demi kompatibilitas arsip lama; isinya MacPanel (situs, server, docs) tanpa folder backup
MPREL="${MP#$HOME/}"
tar -czf "$OUT/developer.tar.gz" -C "$HOME" \
  --exclude='node_modules' --exclude='.venv' --exclude='__pycache__' --exclude='dist' --exclude='.next' \
  --exclude='.env' --exclude='.env.*' --exclude="$MPREL/backup" \
  "$MPREL" Developer 2>/dev/null || [ $? -eq 1 ]

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
"$MP/server/scripts/lab" backup prune --auto || echo "peringatan: pembersihan otomatis gagal"

# DEST (isi nanti, jangan simpan kredensial di skrip):
#   disk eksternal : rsync -a "$OUT/" /Volumes/<NAMA>/macmini-backup/$TS/
#   cloud (restic) : restic -r <repo> backup "$OUT"

#!/bin/bash
# Tambah/hapus website dev di nginx. Domain <nama>.<TLD> (TLD dari ~/Developer/_ops/web/tld).
#   addsite.sh api 3000                  -> https://api.lab     -> reverse proxy 127.0.0.1:3000 (websocket OK)
#   addsite.sh landing ~/Developer/x/y   -> https://landing.lab -> file statis dari direktori
#   addsite.sh app ~/Developer/x/y/public --php 8.4  -> situs PHP/Laravel (FPM versi 8.3|8.4|8.5, front controller index.php)
#   addsite.sh --remove <nama>           -> hapus site + sertifikatnya
#   addsite.sh --list                    -> daftar site
# Sertifikat per site dibuat mkcert. Config diuji (nginx -t); gagal = otomatis dibatalkan.
set -euo pipefail
export PATH=/opt/homebrew/bin:$PATH
W="$HOME/Developer/_ops/web"; N="$W/nginx"; C=/opt/homebrew/etc/nginx/certs
TLD=$(tr -d '[:space:]' < "$W/tld")

reload() {
  if pgrep -f 'nginx: master' >/dev/null; then nginx -s reload && echo "OK, nginx reload"; else echo "catatan: nginx belum jalan (brew services start nginx)"; fi
}

case "${1:-}" in
  --list) ls "$N/sites" | sed "s/\.conf$/.$TLD/"; exit 0;;
  --remove)
    [ -n "${2:-}" ] || { echo "nama?"; exit 1; }
    rm -f "$N/sites/$2.conf" "$C/$2.$TLD.pem" "$C/$2.$TLD-key.pem"
    nginx -t -c "$N/nginx.conf" >/dev/null 2>&1 || { echo "config tidak valid setelah hapus, cek manual"; exit 1; }
    echo "dihapus: $2.$TLD"; reload; exit 0;;
esac

PHPV=""; ARGS=()
while [ $# -gt 0 ]; do case "$1" in --php) PHPV="${2:-}"; shift 2;; *) ARGS+=("$1"); shift;; esac; done
set -- "${ARGS[@]}"
if [ -n "$PHPV" ]; then
  [[ "$PHPV" =~ ^8\.[2-5]$ ]] || { echo "--php harus 8.3, 8.4, atau 8.5"; exit 1; }
  [ -S "/opt/homebrew/var/run/php-fpm-dev-$PHPV.sock" ] || { echo "FPM $PHPV belum jalan (brew services start php@$PHPV)"; exit 1; }
fi
NAME="${1:-}"; TARGET="${2:-}"
[[ "$NAME" =~ ^[a-z0-9]([a-z0-9-]*[a-z0-9])?$ ]] || { echo "nama harus huruf kecil/angka/-"; exit 1; }
[ -n "$TARGET" ] || { echo "pakai: addsite.sh <nama> <port|direktori>"; exit 1; }
HOST="$NAME.$TLD"; F="$N/sites/$NAME.conf"
[ -e "$F" ] && { echo "sudah ada: $F (hapus dulu dengan --remove $NAME)"; exit 1; }

if [ -n "$PHPV" ]; then
  [[ ! "$TARGET" =~ ^[0-9]+$ ]] || { echo "--php butuh direktori, bukan port"; exit 1; }
  DIR=$(cd "${TARGET/#\~/$HOME}" && pwd)
  LOC=$(printf 'root %s;\n    index index.php index.html;\n    client_max_body_size 64m;\n    location / { try_files $uri $uri/ /index.php?$query_string; }\n    location ~ /\\.(?!well-known) { return 404; }\n    location ~ ^/index\\.php(/|$) {\n        fastcgi_split_path_info ^(.+\\.php)(/.+)$;\n        fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-%s.sock;\n        include /opt/homebrew/etc/nginx/fastcgi_params;\n        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;\n        fastcgi_param PATH_INFO $fastcgi_path_info;\n        fastcgi_param HTTPS on;\n        fastcgi_read_timeout 120s;\n    }\n    location ~ \\.php$ { return 404; }' "$DIR" "$PHPV")
elif [[ "$TARGET" =~ ^[0-9]+$ ]]; then
  LOC=$(printf 'location / {\n        include %s/snippets/proxy.conf;\n        proxy_pass http://127.0.0.1:%s;\n    }' "$N" "$TARGET")
else
  DIR=$(cd "${TARGET/#\~/$HOME}" && pwd)
  LOC=$(printf 'root %s;\n    index index.html;\n    location / { try_files $uri $uri/ =404; }' "$DIR")
fi

rollback() { rm -f "$F" "$C/$HOST.pem" "$C/$HOST-key.pem"; }
( cd "$C" && mkcert -cert-file "$HOST.pem" -key-file "$HOST-key.pem" "$HOST" >/dev/null 2>&1 ) || { echo "mkcert gagal"; rollback; exit 1; }
chmod 600 "$C/$HOST-key.pem"

cat > "$F" <<EOF
server {
    listen 80; listen [::]:80;
    server_name $HOST;
    include $N/snippets/tailnet_only.conf;
    return 301 https://\$host\$request_uri;
}
server {
    listen 443 ssl; listen [::]:443 ssl;
    http2 on;
    server_name $HOST;
    include $N/snippets/tailnet_only.conf;
    ssl_certificate     $C/$HOST.pem;
    ssl_certificate_key $C/$HOST-key.pem;
    $LOC
}
EOF

if nginx -t -c "$N/nginx.conf" >/dev/null 2>&1; then
  reload; echo "https://$HOST"
else
  echo "config tidak valid, dibatalkan:"; nginx -t -c "$N/nginx.conf" 2>&1 | grep -iE 'emerg|error' | head -3; rollback; exit 1
fi

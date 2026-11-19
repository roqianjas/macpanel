#!/bin/bash
MP="${MACPANEL_HOME:-$HOME/MacPanel}"
# Tambah/hapus website dev di nginx. Domain <nama>.<TLD> (TLD dari ~/MacPanel/server/tld).
#   addsite.sh api 3000                  -> https://api.lab     -> reverse proxy 127.0.0.1:3000 (websocket OK)
#   addsite.sh landing ~/MacPanel/wwwroot/x/y   -> https://landing.lab -> file statis dari direktori
#   addsite.sh app ~/MacPanel/wwwroot/x/y/public --php 8.4  -> situs PHP/Laravel (FPM versi 8.3|8.4|8.5, front controller index.php)
#   addsite.sh blog ~/MacPanel/wwwroot/x --apache [--php 8.4]  -> dilayani Apache (.htaccess) di 127.0.0.1:8080, nginx hanya proxy TLS
#   ... --wildcard                       -> server_name <nama>.lab dan *.<nama>.lab (multi-tenant); sertifikat mkcert untuk keduanya
#   addsite.sh --remove <nama>           -> hapus site + sertifikatnya (wildcard ikut, satu conf satu sertifikat)
#   addsite.sh --list                    -> daftar site
# Sertifikat per site dibuat mkcert. Config diuji (nginx -t); gagal = otomatis dibatalkan.
set -euo pipefail
export PATH=/opt/homebrew/bin:$PATH
W="$MP/server"; N="$W/nginx"; C=/opt/homebrew/etc/nginx/certs
TLD=$(tr -d '[:space:]' < "$W/tld")

reload() {
  if pgrep -f 'nginx: master' >/dev/null; then nginx -s reload && echo "OK, nginx reload"; else echo "catatan: nginx belum jalan (brew services start nginx)"; fi
}

case "${1:-}" in
  --list) ls "$N/sites" | sed "s/\.conf$/.$TLD/"; exit 0;;
  --remove)
    [ -n "${2:-}" ] || { echo "nama?"; exit 1; }
    rm -f "$N/sites/$2.conf" "$C/$2.$TLD.pem" "$C/$2.$TLD-key.pem"
    "$W/scripts/lab" apache site-remove "$2" >/dev/null 2>&1 || true
    nginx -t -c "$N/nginx.conf" >/dev/null 2>&1 || { echo "config tidak valid setelah hapus, cek manual"; exit 1; }
    echo "dihapus: $2.$TLD"; reload; exit 0;;
esac

PHPV=""; WP=0; WC=0; AP=0; ARGS=()
while [ $# -gt 0 ]; do case "$1" in --php) PHPV="${2:-}"; shift 2;; --wordpress) WP=1; shift;; --wildcard) WC=1; shift;; --apache) AP=1; shift;; *) ARGS+=("$1"); shift;; esac; done
set -- "${ARGS[@]}"
if [ -n "$PHPV" ]; then
  [[ "$PHPV" =~ ^8\.[2-5]$ ]] || { echo "--php harus 8.3, 8.4, atau 8.5"; exit 1; }
  [ -S "/opt/homebrew/var/run/php-fpm-dev-$PHPV.sock" ] || { echo "FPM $PHPV belum jalan (brew services start php@$PHPV)"; exit 1; }
fi
NAME="${1:-}"; TARGET="${2:-}"
[[ "$NAME" =~ ^[a-z0-9]([a-z0-9-]{0,38}[a-z0-9])?$ ]] || { echo "nama harus huruf kecil/angka/-, 1-40 karakter"; exit 1; }
[ -n "$TARGET" ] || { echo "pakai: addsite.sh <nama> <port|direktori>"; exit 1; }
HOST="$NAME.$TLD"; F="$N/sites/$NAME.conf"
SN="$HOST"; MKNAMES=("$HOST")
if [ "$WC" = 1 ]; then SN="$HOST *.$HOST"; MKNAMES=("$HOST" "*.$HOST"); fi
[ -e "$F" ] && { echo "sudah ada: $F (hapus dulu dengan --remove $NAME)"; exit 1; }

if [ "$AP" = 1 ]; then
  [[ ! "$TARGET" =~ ^[0-9]+$ ]] || { echo "--apache butuh direktori, bukan port"; exit 1; }
  [ "$WP" = 0 ] || { echo "--apache tidak digabung dengan --wordpress (WordPress memakai nginx)"; exit 1; }
  [ -x /opt/homebrew/opt/httpd/bin/httpd ] || { echo "Apache belum terpasang (lab app install httpd)"; exit 1; }
  DIR=$(cd "${TARGET/#\~/$HOME}" && pwd)
  "$W/scripts/lab" apache site-add "$NAME" "$HOST" "$DIR" ${PHPV:+--php "$PHPV"} || { echo "vhost Apache gagal"; exit 1; }
  LOC=$(printf 'client_max_body_size 64m;\n    location / {\n        include %s/snippets/proxy.conf;\n        proxy_pass http://127.0.0.1:8080;\n    }' "$N")
  pgrep -f 'httpd -f' >/dev/null || echo "catatan: Apache belum jalan (nyalakan dari Aplikasi atau: lab service start httpd)"
elif [ -n "$PHPV" ]; then
  [[ ! "$TARGET" =~ ^[0-9]+$ ]] || { echo "--php butuh direktori, bukan port"; exit 1; }
  DIR=$(cd "${TARGET/#\~/$HOME}" && pwd)
  if [ "$WP" = 1 ]; then
    LOC=$(printf 'root %s;\n    index index.php index.html;\n    client_max_body_size 64m;\n    location / { try_files $uri $uri/ /index.php?$args; }\n    location ~ /\\.(?!well-known) { return 404; }\n    location ~ \\.php$ {\n        try_files $uri =404;\n        fastcgi_split_path_info ^(.+\\.php)(/.+)$;\n        fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-%s.sock;\n        include /opt/homebrew/etc/nginx/fastcgi_params;\n        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;\n        fastcgi_param PATH_INFO $fastcgi_path_info;\n        fastcgi_param HTTPS on;\n        fastcgi_read_timeout 120s;\n    }' "$DIR" "$PHPV")
  else
  LOC=$(printf 'root %s;\n    index index.php index.html;\n    client_max_body_size 64m;\n    location / { try_files $uri $uri/ /index.php?$query_string; }\n    location ~ /\\.(?!well-known) { return 404; }\n    location ~ ^/index\\.php(/|$) {\n        fastcgi_split_path_info ^(.+\\.php)(/.+)$;\n        fastcgi_pass unix:/opt/homebrew/var/run/php-fpm-dev-%s.sock;\n        include /opt/homebrew/etc/nginx/fastcgi_params;\n        fastcgi_param SCRIPT_FILENAME $realpath_root$fastcgi_script_name;\n        fastcgi_param PATH_INFO $fastcgi_path_info;\n        fastcgi_param HTTPS on;\n        fastcgi_read_timeout 120s;\n    }\n    location ~ \\.php$ { return 404; }' "$DIR" "$PHPV")
  fi
elif [[ "$TARGET" =~ ^[0-9]+$ ]]; then
  LOC=$(printf 'location / {\n        include %s/snippets/proxy.conf;\n        proxy_pass http://127.0.0.1:%s;\n    }' "$N" "$TARGET")
else
  DIR=$(cd "${TARGET/#\~/$HOME}" && pwd)
  LOC=$(printf 'root %s;\n    index index.html;\n    location / { try_files $uri $uri/ =404; }' "$DIR")
fi

rollback() { rm -f "$F" "$C/$HOST.pem" "$C/$HOST-key.pem"; [ "$AP" = 1 ] && "$W/scripts/lab" apache site-remove "$NAME" >/dev/null 2>&1; true; }
( cd "$C" && mkcert -cert-file "$HOST.pem" -key-file "$HOST-key.pem" "${MKNAMES[@]}" >/dev/null 2>&1 ) || { echo "mkcert gagal"; rollback; exit 1; }
chmod 600 "$C/$HOST-key.pem"

cat > "$F" <<EOF
server {
    listen 80; listen [::]:80;
    server_name $SN;
    include $N/snippets/tailnet_only.conf;
    return 301 https://\$host\$request_uri;
}
server {
    listen 443 ssl; listen [::]:443 ssl;
    http2 on;
    server_name $SN;
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

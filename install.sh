#!/bin/bash
MP="${MACPANEL_HOME:-$HOME/MacPanel}"
# MacPanel installer (macOS, Apple Silicon). Idempotent: aman dijalankan ulang.
#   ./install.sh             -> tampilkan rencana lalu minta konfirmasi
#   ./install.sh --yes       -> jalankan tanpa bertanya
#   ./install.sh --dry-run   -> hanya tampilkan rencana
#   --no-brew     lewati `brew install`
#   --no-system   lewati semua yang menyentuh sistem (mkcert, LaunchAgent, nginx reload, symlink ke /opt/homebrew)
#   --tld NAME    domain lokal (bawaan: lab)
# Tidak pernah memakai sudo dan tidak pernah meminta kata sandi. Langkah yang butuh sudo dicetak di akhir untuk Anda jalankan sendiri.
set -euo pipefail

YES=0; DRY=0; BREW=1; SYSTEM=1; TLD=lab
while [ $# -gt 0 ]; do case "$1" in
  --yes|-y) YES=1;; --dry-run) DRY=1;; --no-brew) BREW=0;; --no-system) SYSTEM=0;;
  --tld) TLD="${2:?--tld butuh nilai}"; shift;;
  -h|--help) sed -n 2,11p "$0"; exit 0;;
  *) echo "opsi tidak dikenal: $1"; exit 2;; esac; shift; done
[[ "$TLD" =~ ^[a-z]{2,10}$ ]] || { echo "TLD harus huruf kecil 2-10 karakter"; exit 2; }

say() { printf '\n==> %s\n' "$*"; }
run() { if [ "$DRY" = 1 ]; then echo "  [dry-run] $*"; else "$@"; fi; }
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OPS="$MP/server"; PANEL="$OPS/panel"; WEB="$OPS"; BREWP=/opt/homebrew
LOGS="$HOME/Library/Logs"; AGENT="$HOME/Library/LaunchAgents/com.macminiops.panel.plist"
NEXT=()

say "Pemeriksaan awal"
[ "$(uname -s)" = Darwin ] || { echo "Hanya macOS."; exit 1; }
[ "$(uname -m)" = arm64 ] || echo "peringatan: dirancang untuk Apple Silicon (arm64); jalur /opt/homebrew dipakai tetap."
[ -x "$BREWP/bin/brew" ] || { echo "Homebrew belum ada di $BREWP. Pasang dari https://brew.sh lalu jalankan ulang."; exit 1; }
export PATH="$BREWP/bin:$BREWP/sbin:/usr/bin:/bin:/usr/sbin:/sbin"
[ "$(id -u)" != 0 ] || { echo "Jangan jalankan sebagai root."; exit 1; }

NEED=(nginx mkcert dnsmasq sqlite)
have_tool() { command -v "$1" >/dev/null 2>&1 || { command -v mise >/dev/null 2>&1 && mise which "$1" >/dev/null 2>&1; }; }
for t in go node pnpm; do have_tool "$t" || NEED+=("$t"); done
MISSING=(); for f in "${NEED[@]}"; do "$BREWP/bin/brew" list --versions "$f" >/dev/null 2>&1 || have_tool "$f" || MISSING+=("$f"); done

say "Rencana"
cat <<EOF
  Folder kerja : $OPS  (panel di $PANEL)
  Domain       : panel.$TLD  (hanya loopback + Tailscale; akses LAN ditolak 403)
  Paket brew   : ${MISSING[*]:-(semua sudah ada)}
  Dibuat       : symlink CLI 'lab' dan 'panelctl' ke $BREWP/bin, nginx.conf + snippets, sertifikat mkcert, LaunchAgent panel (127.0.0.1:9100)
  Tidak dilakukan: sudo, kata sandi, DNS (dicetak di akhir), pembuatan akun (Anda sendiri yang mengisinya)
EOF
[ "$DRY" = 1 ] && exit 0
if [ "$YES" != 1 ]; then read -r -p "Lanjut? [y/N] " a; [[ "$a" =~ ^[Yy]$ ]] || { echo "Dibatalkan."; exit 1; }; fi

if [ "$BREW" = 1 ] && [ ${#MISSING[@]} -gt 0 ]; then
  say "Memasang paket Homebrew: ${MISSING[*]}"
  HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_CURL_RETRIES=5 brew install "${MISSING[@]}"
fi

say "Tata letak folder"
mkdir -p "$MP/wwwroot" "$MP/docs" "$OPS/scripts" "$WEB/nginx/sites" "$WEB/nginx/snippets" "$LOGS" "$MP/backup" "$HOME/.config/macminiops/projects"
chmod 700 "$HOME/.config/macminiops" "$HOME/.config/macminiops/projects"
if [ "$SRC" != "$PANEL" ]; then
  if [ -e "$PANEL" ] && [ ! -L "$PANEL" ]; then echo "$PANEL sudah ada dan bukan symlink; pindahkan dulu atau jalankan installer dari sana."; exit 1; fi
  ln -sfn "$SRC" "$PANEL"; echo "  $PANEL -> $SRC"
fi
for f in lab panelctl lab-svc lab-sys lab-dev addsite.sh backup.sh logrotate.sh; do ln -sfn "../panel/cli/$f" "$OPS/scripts/$f"; done
chmod +x "$SRC"/cli/*
echo "$TLD" > "$WEB/tld"
for f in "$SRC"/nginx/snippets/*; do sed "s#__HOME__#$HOME#g" "$f" > "$WEB/nginx/snippets/$(basename "$f")"; done
sed "s#__HOME__#$HOME#g" "$SRC/nginx/nginx.conf" > "$WEB/nginx/nginx.conf.new"
if [ ! -f "$WEB/nginx/nginx.conf" ]; then mv "$WEB/nginx/nginx.conf.new" "$WEB/nginx/nginx.conf"; else
  cmp -s "$WEB/nginx/nginx.conf" "$WEB/nginx/nginx.conf.new" && rm -f "$WEB/nginx/nginx.conf.new" || echo "  nginx.conf Anda sudah ada dan berbeda; versi baru disimpan di $WEB/nginx/nginx.conf.new (tidak ditimpa)"; fi

say "Membangun panel"
tool() { if command -v "$1" >/dev/null 2>&1; then "$@"; else mise exec -- "$@"; fi; }
( cd "$PANEL/ui" && tool pnpm install --frozen-lockfile && tool pnpm run build )
( cd "$PANEL" && tool go build -o panel ./cmd/panel )
[ -x "$PANEL/panel" ] && echo "  biner: $PANEL/panel"

if [ "$SYSTEM" = 1 ]; then
  say "Integrasi sistem"
  CLI="$BREWP/bin/lab"
  if [ -e "$CLI" ] && [ ! -L "$CLI" ]; then echo "  $CLI sudah ada dan bukan symlink; dilewati"; else ln -sfn "$OPS/scripts/lab" "$CLI"; echo "  $CLI -> $OPS/scripts/lab"; fi
  PCTL="$BREWP/bin/panelctl"
  if [ -e "$PCTL" ] && [ ! -L "$PCTL" ]; then echo "  $PCTL sudah ada dan bukan symlink; dilewati"; else ln -sfn "$OPS/scripts/panelctl" "$PCTL"; echo "  $PCTL -> $OPS/scripts/panelctl"; fi
  NGX="$BREWP/etc/nginx/nginx.conf"
  if [ -L "$NGX" ] && [ "$(readlink "$NGX")" = "$WEB/nginx/nginx.conf" ]; then echo "  nginx.conf sudah terhubung"
  elif [ ! -e "$NGX" ] || cmp -s "$NGX" "$BREWP/etc/nginx/nginx.conf.default" 2>/dev/null; then
    [ -e "$NGX" ] && cp -p "$NGX" "$NGX.bak.$(date +%Y%m%d-%H%M%S)"; ln -sfn "$WEB/nginx/nginx.conf" "$NGX"; echo "  $NGX -> $WEB/nginx/nginx.conf"
  else echo "  nginx.conf Anda sudah dikustom; TIDAK diubah. Tambahkan baris ini ke blok http {} Anda: include $WEB/nginx/sites/*.conf;"; NEXT+=("Tambahkan 'include $WEB/nginx/sites/*.conf;' ke blok http di $NGX"); fi
  say "Sertifikat lokal (mkcert)"
  mkcert -install || NEXT+=("Jalankan 'mkcert -install' di Terminal (butuh kata sandi login Mac Anda)")
  say "Kunci enkripsi TOTP di Keychain"
  "$PANEL/panel" init-key 2>&1 | tail -1 || true
  say "LaunchAgent panel"
  cat > "$AGENT" <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict>
  <key>Label</key><string>com.macminiops.panel</string>
  <key>ProgramArguments</key><array>
    <string>$PANEL/panel</string><string>serve</string><string>--addr</string><string>127.0.0.1:9100</string>
    <string>--hook-addr</string><string>127.0.0.1:9101</string><string>--origin</string><string>https://panel.$TLD</string><string>--ui</string><string>$PANEL/ui/dist</string>
  </array>
  <key>WorkingDirectory</key><string>$PANEL</string>
  <key>EnvironmentVariables</key><dict><key>PATH</key><string>$BREWP/bin:/usr/bin:/bin</string><key>HOME</key><string>$HOME</string></dict>
  <key>RunAtLoad</key><true/><key>KeepAlive</key><true/><key>ThrottleInterval</key><integer>30</integer>
  <key>ProcessType</key><string>Background</string>
  <key>StandardOutPath</key><string>$LOGS/panel.log</string><key>StandardErrorPath</key><string>$LOGS/panel.log</string>
</dict></plist>
EOF
  U=$(id -u); launchctl bootout "gui/$U/com.macminiops.panel" 2>/dev/null || true; sleep 1
  launchctl bootstrap "gui/$U" "$AGENT" && echo "  LaunchAgent dimuat"
  say "Situs panel.$TLD di nginx"
  "$OPS/scripts/addsite.sh" panel 9100 2>&1 | tail -2 || true
  nginx -t 2>&1 | tail -1
else
  echo; echo "(--no-system: langkah sistem dilewati)"
fi

say "Selesai. Langkah yang perlu Anda lakukan sendiri"
NEXT+=("Buat akun admin: $PANEL/panel user add <nama>   (kata sandi dan kunci TOTP diisi/dicatat oleh Anda; masukkan kunci ke Google Authenticator)")
NEXT+=("DNS *.$TLD (butuh sudo): echo 'address=/$TLD/<IP-Tailscale-atau-127.0.0.1>' ke $BREWP/etc/dnsmasq.conf, lalu 'sudo brew services start dnsmasq' dan 'echo nameserver 127.0.0.1 | sudo tee /etc/resolver/$TLD'")
if ! command -v tailscale >/dev/null 2>&1 && [ ! -d /Applications/Tailscale.app ]; then
  NEXT+=("Tailscale belum ada. Disarankan versi Homebrew (jalan sebelum login, cocok untuk reboot jarak jauh): brew install tailscale && sudo brew services start tailscale && sudo tailscale up --hostname=<nama-mesin> --accept-routes --operator=\$USER")
fi
NEXT+=("Setelah IP tailnet diketahui ('tailscale ip -4'): pasang di dnsmasq.conf (listen-address dan address=/$TLD/), lalu tambahkan split DNS '$TLD' ke IP itu di admin console Tailscale. IP berubah bila Tailscale dipasang ulang.")
NEXT+=("Jalankan nginx: brew services start nginx    (bila belum)")
NEXT+=("Buka https://panel.$TLD dari perangkat di tailnet Anda")
i=1; for n in "${NEXT[@]}"; do echo "  $i. $n"; i=$((i+1)); done

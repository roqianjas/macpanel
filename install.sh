#!/bin/bash
MP="${MACPANEL_HOME:-$HOME/MacPanel}"
# MacPanel installer (macOS, Apple Silicon). Idempotent: safe to run again.
#   ./install.sh             -> show the plan, then ask for confirmation
#   ./install.sh --yes       -> run without asking
#   ./install.sh --dry-run   -> only show the plan
#   --no-brew     skip `brew install`
#   --no-system   skip everything that touches the system (mkcert, LaunchAgent, nginx reload, symlinks into /opt/homebrew)
#   --tld NAME    local domain (default: lab)
# Never uses sudo and never asks for passwords. Steps that need sudo are printed at the end for you to run yourself.
set -euo pipefail

YES=0; DRY=0; BREW=1; SYSTEM=1; TLD=lab
while [ $# -gt 0 ]; do case "$1" in
  --yes|-y) YES=1;; --dry-run) DRY=1;; --no-brew) BREW=0;; --no-system) SYSTEM=0;;
  --tld) TLD="${2:?--tld butuh nilai}"; shift;;
  -h|--help) sed -n 2,11p "$0"; exit 0;;
  *) echo "unknown option: $1"; exit 2;; esac; shift; done
[[ "$TLD" =~ ^[a-z]{2,10}$ ]] || { echo "TLD must be 2-10 lowercase letters"; exit 2; }

say() { printf '\n==> %s\n' "$*"; }
run() { if [ "$DRY" = 1 ]; then echo "  [dry-run] $*"; else "$@"; fi; }
SRC="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OPS="$MP/server"; PANEL="$OPS/panel"; WEB="$OPS"; BREWP=/opt/homebrew
LOGS="$HOME/Library/Logs"; AGENT="$HOME/Library/LaunchAgents/com.macminiops.panel.plist"
NEXT=()

say "Preflight checks"
[ "$(uname -s)" = Darwin ] || { echo "macOS only."; exit 1; }
[ "$(uname -m)" = arm64 ] || echo "warning: designed for Apple Silicon (arm64); the /opt/homebrew path is used as-is."
[ -x "$BREWP/bin/brew" ] || { echo "Homebrew not found in $BREWP. Install it from https://brew.sh and run again."; exit 1; }
export PATH="$BREWP/bin:$BREWP/sbin:/usr/bin:/bin:/usr/sbin:/sbin"
[ "$(id -u)" != 0 ] || { echo "Do not run as root."; exit 1; }

NEED=(nginx mkcert dnsmasq sqlite)
[ "$(uname -m)" = arm64 ] && NEED+=(macmon)  # sensor suhu tanpa sudo; hanya Apple Silicon
have_tool() { command -v "$1" >/dev/null 2>&1 || { command -v mise >/dev/null 2>&1 && mise which "$1" >/dev/null 2>&1; }; }
for t in go node pnpm; do have_tool "$t" || NEED+=("$t"); done
MISSING=(); for f in "${NEED[@]}"; do "$BREWP/bin/brew" list --versions "$f" >/dev/null 2>&1 || have_tool "$f" || MISSING+=("$f"); done

say "Plan"
cat <<EOF
  Work folder  : $OPS  (panel in $PANEL)
  Domain       : panel.$TLD  (loopback + Tailscale only; LAN access is refused with 403)
  Brew packages: ${MISSING[*]:-(all present)}
  Creates      : symlinks for the 'lab' and 'panelctl' CLIs in $BREWP/bin, nginx.conf + snippets, mkcert certificates, the panel LaunchAgent (127.0.0.1:9100)
  Never does   : sudo, passwords, DNS (printed at the end); you type the admin password yourself
EOF
[ "$DRY" = 1 ] && exit 0
if [ "$YES" != 1 ]; then read -r -p "Continue? [y/N] " a; [[ "$a" =~ ^[Yy]$ ]] || { echo "Cancelled."; exit 1; }; fi

if [ "$BREW" = 1 ] && [ ${#MISSING[@]} -gt 0 ]; then
  say "Installing Homebrew packages: ${MISSING[*]}"
  HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_CURL_RETRIES=5 brew install "${MISSING[@]}"
fi

say "Folder layout"
mkdir -p "$MP/wwwroot" "$MP/docs" "$OPS/scripts" "$WEB/nginx/sites" "$WEB/nginx/snippets" "$LOGS" "$MP/backup" "$HOME/.config/macminiops/projects"
chmod 700 "$HOME/.config/macminiops" "$HOME/.config/macminiops/projects"
if [ "$SRC" != "$PANEL" ]; then
  if [ -e "$PANEL" ] && [ ! -L "$PANEL" ]; then echo "$PANEL already exists and is not a symlink; move it away or run the installer from there."; exit 1; fi
  ln -sfn "$SRC" "$PANEL"; echo "  $PANEL -> $SRC"
fi
for f in lab panelctl addsite.sh backup.sh logrotate.sh; do ln -sfn "../panel/cli/$f" "$OPS/scripts/$f"; done
chmod +x "$SRC"/cli/*
echo "$TLD" > "$WEB/tld"
bash "$SRC/scripts/guide.sh" install  # panduan pengguna untuk halaman Dokumentasi
for f in "$SRC"/nginx/snippets/*; do sed "s#__HOME__#$HOME#g" "$f" > "$WEB/nginx/snippets/$(basename "$f")"; done
sed "s#__HOME__#$HOME#g" "$SRC/nginx/nginx.conf" > "$WEB/nginx/nginx.conf.new"
if [ ! -f "$WEB/nginx/nginx.conf" ]; then mv "$WEB/nginx/nginx.conf.new" "$WEB/nginx/nginx.conf"; else
  cmp -s "$WEB/nginx/nginx.conf" "$WEB/nginx/nginx.conf.new" && rm -f "$WEB/nginx/nginx.conf.new" || echo "  your nginx.conf exists and differs; the new version is saved at $WEB/nginx/nginx.conf.new (not overwritten)"; fi

say "Building the panel"
tool() { if command -v "$1" >/dev/null 2>&1; then "$@"; else mise exec -- "$@"; fi; }
( cd "$PANEL/ui" && tool pnpm install --frozen-lockfile && tool pnpm run build )
( cd "$PANEL" && tool go build -o panel ./cmd/panel )
[ -x "$PANEL/panel" ] && echo "  binary: $PANEL/panel"

if [ "$SYSTEM" = 1 ]; then
  say "System integration"
  CLI="$BREWP/bin/lab"
  if [ -e "$CLI" ] && [ ! -L "$CLI" ]; then echo "  $CLI exists and is not a symlink; skipped"; else ln -sfn "$OPS/scripts/lab" "$CLI"; echo "  $CLI -> $OPS/scripts/lab"; fi
  PCTL="$BREWP/bin/panelctl"
  if [ -e "$PCTL" ] && [ ! -L "$PCTL" ]; then echo "  $PCTL exists and is not a symlink; skipped"; else ln -sfn "$OPS/scripts/panelctl" "$PCTL"; echo "  $PCTL -> $OPS/scripts/panelctl"; fi
  NGX="$BREWP/etc/nginx/nginx.conf"
  if [ -L "$NGX" ] && [ "$(readlink "$NGX")" = "$WEB/nginx/nginx.conf" ]; then echo "  nginx.conf already linked"
  elif [ ! -e "$NGX" ] || cmp -s "$NGX" "$BREWP/etc/nginx/nginx.conf.default" 2>/dev/null; then
    [ -e "$NGX" ] && cp -p "$NGX" "$NGX.bak.$(date +%Y%m%d-%H%M%S)"; ln -sfn "$WEB/nginx/nginx.conf" "$NGX"; echo "  $NGX -> $WEB/nginx/nginx.conf"
  else echo "  your nginx.conf is customised; NOT changed. Add this line to your http {} block: include $WEB/nginx/sites/*.conf;"; NEXT+=("Add 'include $WEB/nginx/sites/*.conf;' to the http block in $NGX"); fi
  say "Local certificates (mkcert)"
  mkcert -install || NEXT+=("Run 'mkcert -install' in Terminal (needs your Mac login password)")
  say "TOTP encryption key in the Keychain"
  "$PANEL/panel" init-key 2>&1 | tail -1 || true
  say "Panel LaunchAgent"
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
  launchctl bootstrap "gui/$U" "$AGENT" && echo "  LaunchAgent loaded"
  say "panel.$TLD site in nginx"
  "$OPS/scripts/addsite.sh" panel 9100 2>&1 | tail -2 || true
  nginx -t 2>&1 | tail -1
else
  echo; echo "(--no-system: system steps skipped)"
fi

say "Done. Steps left for you"
NEXT+=("Create the admin account: $PANEL/panel user add <name>   (you type the password and note the TOTP key; add the key to your authenticator app)")
NEXT+=("DNS for *.$TLD (needs sudo): add 'address=/$TLD/<Tailscale-IP-or-127.0.0.1>' to $BREWP/etc/dnsmasq.conf, then 'sudo brew services start dnsmasq' and 'echo nameserver 127.0.0.1 | sudo -S -p '' tee /etc/resolver/$TLD'")
if ! command -v tailscale >/dev/null 2>&1 && [ ! -d /Applications/Tailscale.app ]; then
  NEXT+=("Tailscale is not installed. The Homebrew version is recommended (runs before login, good for remote reboots): brew install tailscale && sudo brew services start tailscale && sudo tailscale up --hostname=<machine-name> --accept-routes --operator=\$USER")
fi
NEXT+=("Once the tailnet IP is known ('tailscale ip -4'): put it in dnsmasq.conf (listen-address and address=/$TLD/), then add a split-DNS entry for '$TLD' pointing to that IP in the Tailscale admin console. The IP changes if Tailscale is reinstalled.")
NEXT+=("Start nginx: brew services start nginx    (if not running)")
NEXT+=("Open https://panel.$TLD from a device on your tailnet")
i=1; for n in "${NEXT[@]}"; do echo "  $i. $n"; i=$((i+1)); done

# Guided finish: offer to create the admin now. The password is typed by you into the panel's own prompt; this script never sees it.
if [ "$SYSTEM" = 1 ] && [ -t 0 ] && [ -x "$PANEL/panel" ]; then
  if ! "$PANEL/panel" user list 2>/dev/null | grep -q .; then
    echo
    read -r -p "Create the admin account now? [Y/n] " a
    if [[ ! "$a" =~ ^[Nn]$ ]]; then
      read -r -p "Admin username (lowercase letters, digits, - _): " u
      [[ "$u" =~ ^[a-z0-9_-]{3,32}$ ]] && "$PANEL/panel" user add "$u" || echo "Skipped: run '$PANEL/panel user add <name>' when ready."
    fi
  fi
fi

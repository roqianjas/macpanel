#!/bin/bash
# Gagalkan bila berkas rahasia atau data pribadi ikut ter-track di git. Jalan di make check, pre-commit, dan CI.
cd "$(dirname "$0")/.."
bad=0
if git rev-parse --git-dir >/dev/null 2>&1; then
  files=$(git ls-files --cached --others --exclude-standard)  # termasuk berkas baru yang belum di-add
else
  files=$(find . -type f -not -path './ui/node_modules/*' -not -path './.git/*' | sed 's|^\./||')
fi
hit=$(echo "$files" | grep -E '(^|/)(db\.env|\.env|panel\.db(-wal|-shm)?|.*\.pem|.*\.key|id_(rsa|ed25519)[^/]*)$|\.bak\.' || true)
[ -n "$hit" ] && { echo "berkas terlarang ter-track:"; echo "$hit"; bad=1; }
pat='/Users/[a-z0-9_]+/|tail[0-9a-f]{4,}\.ts\.net|\b100\.(6[4-9]|[7-9][0-9]|1[01][0-9]|12[0-7])\.[0-9]{1,3}\.[0-9]{1,3}\b'
# yang dikecualikan adalah COCOKANNYA (IP contoh 100.64.0.x, path contoh), bukan seluruh barisnya
hit=$(echo "$files" | grep -vE '^(ui/node_modules|ui/pnpm-lock|go\.sum|docs/screenshots/|scripts/check-secrets\.sh|\.pre-commit)' | xargs grep -noIE "$pat" 2>/dev/null | grep -vE ':(100\.64\.0\.[0-9]+|100\.100\.[0-9]+\.[0-9]+|/Users/(someuser|admin|example|user)/)$' | head -10 || true)
[ -n "$hit" ] && { echo "kemungkinan data pribadi (path rumah / host tailnet / IP tailnet):"; echo "$hit" | cut -c1-160; bad=1; }
[ $bad = 0 ] && echo "secrets: bersih"
exit $bad

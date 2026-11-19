#!/bin/bash
# User guide: the source is docs/guide/*.md (in the repo). This script:
#   guide.sh install   -> copy to $MACPANEL_HOME/docs (read by the panel's Documentation page)
#   guide.sh usage     -> rebuild docs/USAGE.md (all chapters joined)
#   guide.sh check     -> fail when docs/USAGE.md differs from the joined chapters (used by make check)
set -euo pipefail
cd "$(dirname "$0")/.."
MP="${MACPANEL_HOME:-$HOME/MacPanel}"
build() {
  printf '# Usage\n\nPanel user guide (also shown inside the panel under Dokumentasi, in Indonesian).\n\n---\n'
  for f in docs/guide/*.md; do printf '\n'; cat "$f"; done
}
case "${1:-}" in
  install) mkdir -p "$MP/docs"; cp docs/guide/*.md "$MP/docs/"; echo "guide: $(ls docs/guide/*.md | wc -l | tr -d ' ') chapters -> $MP/docs";;
  usage) build > docs/USAGE.md; echo "docs/USAGE.md rebuilt";;
  check) build | cmp -s - docs/USAGE.md || { echo "docs/USAGE.md is stale: run 'make usage'"; exit 1; }; echo "usage: in sync";;
  *) sed -n 2,5p "$0"; exit 2;;
esac

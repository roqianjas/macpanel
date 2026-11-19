# Contributing

Thanks for helping. MacPanel is a small, security-sensitive project: it controls services on someone's Mac, so changes are reviewed with that in mind.

## Ground rules
- **No shell strings, no `sudo` from the web.** The panel only calls fixed `lab` subcommands with validated list arguments. Anything that needs root is printed as a command for the user to run. The one controlled exception is the Shell task type (see `docs/DESIGN.md`).
- **Every user-facing feature needs a terminal or API equivalent** (`lab ...` or `panelctl`).
- **Never commit secrets or personal data** (`.env`, `*.pem`, `panel.db`, real hostnames or tailnet IPs). `scripts/check-secrets.sh` enforces this and runs in `make check`.
- **Docs are generic.** `docs/guide/` is the user guide: no author names, hostnames, IPs or history.

## Setup
Requirements: macOS (Apple Silicon), Homebrew, `mise` (Go, Node, pnpm), `uv`.

```sh
git clone https://github.com/roqianjas/macpanel && cd macpanel
make check        # fmt, vet, lint, race tests, API drift, UI tests + build, CLI tests, guide sync, secrets
make quick        # faster loop
```

## Before sending a PR
1. `make check` passes (CI is not available; this is the gate).
2. New logic has a test, and for security-relevant code you broke it on purpose once to confirm a test fails (mutation check).
3. UI: all text goes through `t()` (`ui/src/i18n`), every input has a `placeholder`, light and dark themes work, touch targets are at least 44 px, no layout overflow at 1280 / 390 / 320 px.
4. If the OpenAPI surface changed: `make gen-api`. If `docs/guide/` changed: `make usage`.
5. Note user-visible changes in `CHANGELOG.md`.

## Adding a language
Copy `ui/src/i18n/locales/id.ts` to `locales/<code>.ts`, translate the values (keep keys and `{placeholders}`), and register one line in `LOCALES` in `ui/src/i18n/index.ts`. `i18n.test.ts` checks every language has the same keys and placeholders. Keys ending in `.one` are the singular form where a language distinguishes it.

## Reporting security problems
See `SECURITY.md`. Please do not open public issues for vulnerabilities.

# Security

MacPanel can run privileged-looking actions on your Mac (restart services, edit nginx config, install packages). Treat it like an admin tool.

## Design (see docs/DESIGN.md)
- Binds to `127.0.0.1` only; expose it through nginx restricted to a private network (Tailscale/VPN). Never port-forward it.
- Login = password + TOTP (Google Authenticator compatible). Destructive actions need the password again (step-up, valid 15 minutes; TOTP is login-only) and retyping the target name.
- No shell, no terminal, no file manager, no `sudo` from the web. The panel calls a fixed set of `lab` subcommands with validated arguments (list args, no shell strings).
- Hash-chained audit log. TOTP key is stored in the macOS Keychain, not on disk.
- Roles: `admin` and read-only `viewer`. Accounts are created from the local terminal (`panel user add`) or by an admin in the panel (temporary password, shown to nobody afterwards, forced change at first login). 2FA is per account.
- Responses carry a strict CSP (`default-src 'self'`, no inline script, `img-src 'self' data:`); uploaded logos and avatars are raster images only (SVG is rejected because it can carry script).
- Optional Apache listens on `127.0.0.1:8080` only, behind nginx; its generated config denies everything by default.

## Known limits
- Anyone who already controls the macOS account running the panel can alter its database and audit log.
- Early project: expect rough edges. Review `docs/DESIGN.md` before exposing it to a network you do not fully trust.

## Supported versions
Only the latest release receives fixes.

## Git history note
Early commits contain a private Tailscale (CGNAT, `100.x`) address of the author's own machine. It is not routable from the internet and grants nothing, but it is public. History was deliberately not rewritten to avoid breaking clones. Later commits use documentation addresses (`100.64.0.x`), enforced by `scripts/check-secrets.sh`.

## Reporting
Open a private security advisory on GitHub (Security tab) rather than a public issue.

## API tokens (agents and automation)
- `Authorization: Bearer mpt_...`, three scopes: `read` (GET only), `operate` (writes without re-verification), `full` (also step-up actions; deletions still require the typed target name). Viewer accounts are capped at `read`.
- Only the SHA-256 of a token is stored; shown once at creation; 1-365 days; max 10 per account; revocable; last use (time, IP) recorded; every write is audited as `api_call` with the token name. Failed attempts per IP are rate-limited.
- Never reachable with a token: `/api/auth/*`, `/api/me/*` (password, 2FA, devices, tokens), `*/creds`, `*/env`. Creating a token needs the password (step-up) from a human session.
- 2FA can be switched off per account (password + current code; revokes other sessions and trusted devices). Treat that as a deliberate reduction in security.

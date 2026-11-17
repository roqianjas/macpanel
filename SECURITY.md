# Security

MacPanel can run privileged-looking actions on your Mac (restart services, edit nginx config, install packages). Treat it like an admin tool.

## Design (see docs/DESIGN.md)
- Binds to `127.0.0.1` only; expose it through nginx restricted to a private network (Tailscale/VPN). Never port-forward it.
- Login = password + TOTP (Google Authenticator compatible). Destructive actions need the password again (step-up, valid 15 minutes; TOTP is login-only) and retyping the target name.
- No shell, no terminal, no file manager, no `sudo` from the web. The panel calls a fixed set of `lab` subcommands with validated arguments (list args, no shell strings).
- Hash-chained audit log. TOTP key is stored in the macOS Keychain, not on disk.
- Roles: `admin` and read-only `viewer`. Users are created only from the local terminal.

## Known limits
- Anyone who already controls the macOS account running the panel can alter its database and audit log.
- Early project: expect rough edges. Review `docs/DESIGN.md` before exposing it to a network you do not fully trust.

## Reporting
Open a private security advisory on GitHub (Security tab) rather than a public issue.

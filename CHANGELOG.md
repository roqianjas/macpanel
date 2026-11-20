# Changelog

## Unreleased

- Temporary public access per site (Public tab, `lab public`) through Cloudflare Quick Tunnel: one loopback listener per site, explicit confirmation + step-up, time limit with automatic shutdown, Activity log, never restarts after reboot. Listener files live in `nginx/public/`, outside `sites/`, so they never appear as sites. Cloudflare Tunnel is an optional app (install/stop/remove from Apps). The Apps page can now install and remove Apache too.
- Extra domains accept any host name (e.g. a Tailscale Funnel name) with an explicit public-exposure confirmation; only `.lab` names get an mkcert certificate.

## 0.1.0 (first public release, macOS only)

MacPanel is a self-hosted web control panel for a Mac (Apple Silicon) used as a development server: the aaPanel/cPanel experience, native, reachable only over a private network such as Tailscale.

**Sites and runtimes:** nginx sites (PHP, static, reverse proxy), per-site PHP 8.2-8.5 with PECL extensions, local HTTPS (mkcert), rewrite rules, config history with automatic rollback, optional Apache (`.htaccess`) behind nginx, Go/Python/Next.js projects, Composer, deploy and GitHub webhook.

**Data:** MariaDB, MySQL and PostgreSQL (create, dump, restore, extra users with privileges, root password rotation), Redis with required password, Adminer.

**Operations:** services and optional apps, workers and cron on launchd (Shell, PHP, Node, Python and URL task types, full cron schedules, `@reboot`), scheduled local backups, disk and memory cleanup (`lab clean`, `lab mem`), staged uninstall (`lab uninstall`), Tailscale status, exposure checker, `pf` firewall helper (prints the `sudo` commands for you), optional Docker via Colima.

**Monitoring:** CPU, memory, disk, bandwidth and hardware health with history, alerts (in panel and optional ntfy push).

**Security:** loopback bind, password + TOTP (per account), step-up for destructive actions, hash-chained audit log, admin/viewer roles, scoped API tokens with `panelctl`, strict CSP, no web shell, no `sudo` from the web.

**Interface:** four styles (Kertas Teknis, Bengkel, Ruang Mesin, Kaca Dingin) with light/dark and accent colour, global search (Ctrl K), profile menu with avatar or photo, per-account language (Indonesian and English, more can be added with one file), skeleton loading, responsive down to 320 px, accessible (keyboard, 44 px targets, reduced motion), configurable panel logo and favicon.

**Install:** `./install.sh` (never uses `sudo`; `--dry-run` shows the plan), with a guided finish for the admin account.

### Known limits
- macOS on Apple Silicon with Homebrew only. No Linux support yet.
- The interface is fully Indonesian/English, but the in-panel user guide, hardware/network/disk-forecast alert texts, the disk-cleanup catalog descriptions and server error messages are Indonesian only.
- Backups are local only (no offsite upload). No ACME/Let's Encrypt (local mkcert only), no email/Telegram/webhook notifications (ntfy push exists).
- The installer's system steps were verified on the author's machine, not on a freshly provisioned Mac.

# MacPanel

**A self-hosted web control panel for macOS: the aaPanel / cPanel experience for a Mac mini or MacBook dev server.**

Manage websites (nginx), multiple PHP versions and extensions, MySQL/PostgreSQL databases, Node/Go/Python runtimes, workers and cron (launchd), deploys, backups and monitoring from one browser UI, protected by password + TOTP and meant to be reachable only over a private network (e.g. Tailscale).

> Status: early, used daily on one Mac mini (macOS, Apple Silicon). Works, tested, but not yet a one-command install. See "Install" and "Known gaps".

Keywords: macOS server panel, Mac mini home server, nginx panel for Mac, aaPanel alternative for macOS, cPanel alternative macOS, Homebrew PHP-FPM manager, launchd GUI, self-hosted dev server panel.

## Features
- **Websites:** add/remove/pause sites (PHP, static, reverse proxy), per-site PHP version, rewrite/redirect rules, nginx config editor with version history and automatic rollback on `nginx -t` failure, local HTTPS (mkcert), logs, HTTP health + certificate expiry.
- **PHP:** install 8.2-8.5 side by side, PECL extensions with on/off switches (redis, apcu, xdebug, imagick, igbinary, msgpack, pcov, yaml, memcached, mongodb), FPM pool settings, restart + logs, Composer.
- **Databases:** MySQL and PostgreSQL create / dump / restore / upload dump (restore refuses dangerous client commands and runs as the project user).
- **Runtimes:** Node, Go, Python, pnpm via mise.
- **Workers and cron:** supervisor/crontab equivalent on launchd.
- **Deploy:** per-site git pull, composer, npm/pnpm, artisan, FPM restart.
- **Monitoring:** CPU, memory, disk, load, per-interface bandwidth with 7-day history; alerts for disk, memory, services, site errors, SSL expiry, stale backups.
- **Security:** loopback bind, TOTP, step-up for destructive actions, hash-chained audit log, admin/viewer roles, no web shell.
- UI: React + Vite, light/dark, responsive, accessible (keyboard, 44px targets, reduced motion). Backend: one Go binary with SQLite.

## How it is built
```
browser -> nginx (TLS, private-network only) -> panel (Go, 127.0.0.1:9100)
                                                   |
                                                   v   fixed subcommands, validated args, no shell
                                                  lab  (cli/: Python wrappers around brew, nginx, launchd, mise, mysql, psql)
```
- `*.go`, `ui/` panel. `cli/` the `lab` command. `nginx/` reference config. `docs/` design and usage.
- Docs: [docs/DESIGN.md](docs/DESIGN.md) (threat model), [docs/USAGE.md](docs/USAGE.md).

## Install
Requirements: macOS (Apple Silicon), [Homebrew](https://brew.sh), and a private network such as [Tailscale](https://tailscale.com).

```sh
git clone https://github.com/roqianjas/macpanel ~/MacPanel/server/panel
cd ~/MacPanel/server/panel
./install.sh --dry-run     # show the plan, change nothing
./install.sh               # asks for confirmation (use --yes to skip)
```
The installer installs missing Homebrew packages (nginx, mkcert, dnsmasq, go, node, pnpm, sqlite), lays out `~/MacPanel/server`, builds the UI and the Go binary, creates the Keychain key and the `panel` LaunchAgent (127.0.0.1:9100), and adds `https://panel.lab` to nginx. It never uses `sudo` or asks for passwords. At the end it prints the few steps you do yourself: create the admin (`panel user add <name>`, you enter the password and scan the TOTP key), and DNS for `*.lab` (needs `sudo`).
Options: `--no-brew`, `--no-system` (skip anything outside your home folder and the build), `--tld NAME`. It will not overwrite a customised nginx.conf (prints one `include` line to add instead).

Tested: a clean copy of this repo installed into an empty HOME (`--no-brew --no-system`): UI built from scratch, Go binary built, layout correct, re-run is a no-op. Not tested: the system steps on a fresh Mac (mkcert, LaunchAgent, nginx link), since the author's machine already has them.

## Known gaps
- The `~/MacPanel/server` layout is fixed (the installer creates it).
- macOS Homebrew PHP compiles bundled extensions statically, so only PECL extensions can be toggled.
- No file manager, web terminal, Docker manager or firewall/`sudo` actions by design.
- Backups are local; no offsite upload.

## Contributing
Issues and PRs welcome. Run `go vet ./... && go test ./... -race` and `cd ui && pnpm run test` before sending.

## License
MIT

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

## Install (manual, v0)
Requirements: macOS on Apple Silicon, [Homebrew](https://brew.sh), `nginx`, `mkcert`, `go`, `node`+`pnpm` (mise recommended), PHP/MySQL/PostgreSQL as you need, and a private network such as Tailscale.

```sh
git clone https://github.com/roqianjas/macpanel ~/Developer/_ops/web/panel
cd ~/Developer/_ops/web/panel
(cd ui && pnpm install && pnpm run build)
go build -o panel .
./panel init-key                       # creates the TOTP encryption key in the macOS Keychain
./panel user add <name>                # prints the TOTP setup key ONCE; add --role viewer for read-only
./panel serve --origin https://panel.lab
```
Then put nginx in front (see `nginx/` and `cli/addsite.sh panel 9100`) and run it with a LaunchAgent (KeepAlive). `cli/` scripts currently expect the layout `~/Developer/_ops/{scripts,web}`; `__HOME__` in `nginx/` must be replaced with your home directory.

## Known gaps
- Hard-coded `~/Developer/_ops` layout; no installer yet.
- macOS Homebrew PHP compiles bundled extensions statically, so only PECL extensions can be toggled.
- No file manager, web terminal, Docker manager or firewall/`sudo` actions by design.
- Backups are local; no offsite upload.

## Contributing
Issues and PRs welcome. Run `go vet ./... && go test ./... -race` and `cd ui && pnpm run test` before sending.

## License
MIT

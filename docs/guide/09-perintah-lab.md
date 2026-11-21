# Perintah lab

`lab` adalah perintah di Terminal yang dipakai panel di belakang layar. Anda juga dapat memakainya langsung. Semua perintah menolak argumen yang tidak valid dan bersifat idempoten.

## Uninstall

`lab uninstall` melepas MacPanel dan, bila diminta, aplikasi bundelnya. Tanpa `--apply` hanya mencetak rencana dan tidak mengubah apa pun, jadi selalu lihat rencananya dulu.

```
lab uninstall --level 1                      # rencana: panel saja
lab uninstall --level 2                      # rencana: + aplikasi dihentikan (paket dan data tetap)
lab uninstall --level 3 --backup-dir ~/Backups-macpanel   # rencana: semuanya
lab uninstall --level 1 --apply              # jalankan
```

- **Tingkat 1**: LaunchAgent panel, proxy npm, backup harian, logrotate, firewall dari `lab firewall`, kunci enkripsi panel di Keychain, tautan nginx.conf, cache dan log panel. Situs, database, dan aplikasi tidak berubah.
- **Tingkat 2**: ditambah menghentikan dan melepas dari login nginx, PHP-FPM, database, Redis, Mailpit, Netdata, dnsmasq, Colima, 9router, dan Kuma, serta kunci firewall RustDesk. Paket dan data tetap ada.
- **Tingkat 3**: ditambah backup akhir ke folder **di luar** `~/MacPanel` yang diverifikasi (hash dan keterbacaan arsip) sebelum apa pun dihapus, lalu paket Homebrew bundel, data database, CA mkcert, `~/.config/macminiops`, dan `~/MacPanel` dihapus. Anda harus mengetik ulang nama mesin (atau `--confirm-host NAMA`); `--yes` tidak melewati konfirmasi ini. Dibatalkan bila ruang kosong kurang atau backup gagal diverifikasi.
- Alat Docker (`colima`, `docker`, `docker-compose`, `lima`) **tidak ikut** dihapus di tingkat 3 kecuali Anda menambahkan `--with-docker-tools` atau menjawab ya saat ditanya; Anda mungkin memakainya di luar panel.
- Tingkat 1 sampai 3 tidak pernah menyentuh Tailscale (lihat tingkat 4 dan `lab tailscale` di bawah), aplikasi RustDesk, Hermes, Homebrew sendiri, dan toolchain (mise, go, node, composer).
- Jalankan sebagai pengguna biasa; `sudo` dipanggil per langkah dan meminta password Anda. Panel hanya menampilkan perintahnya (Sistem → Uninstall) dan tidak menjalankannya.
- Folder backup akhir berisi `config.tar.gz` yang memuat kredensial; simpan dengan hati-hati (izin 0600).

## Menutup port dari jaringan rumah

`lab firewall` memakai firewall bawaan macOS (pf) untuk menolak sambungan masuk ke satu port dari perangkat lain di LAN. Layanannya tetap berjalan; yang ditutup hanya jalur masuknya.

```
lab firewall block 8080 --dry-run   # lihat langkahnya, tanpa mengubah apa pun
lab firewall block 8080             # tutup (loopback dan tailnet tetap boleh)
lab firewall block 8080 --allow none  # tutup, hanya loopback yang boleh
lab firewall status                 # daftar port yang diblokir (tanpa sudo)
lab firewall verify                 # bukti aturan benar-benar aktif di kernel (sudo)
lab firewall unblock 8080           # buka kembali
```

- Jalankan sebagai pengguna biasa di terminal; `sudo` dipanggil per langkah dan meminta password Anda. Panel tidak pernah menjalankannya.
- Aturan dimuat ulang otomatis saat boot oleh LaunchDaemon. Saat boot, port ditutup lebih dulu baru menunggu Tailscale, jadi tidak ada celah terbuka.
- Konfigurasi pf yang ada dicadangkan (`.bak.<waktu>`) dan sintaks aturan diuji sebelum diganti.
- Port 22 (SSH) ditolak karena memblokirnya bisa mengunci Anda dari mesin sendiri.
- Menguji dari mesin ini lewat IP LAN tidak membuktikan port tertutup (sambungan lokal lewat loopback). Ujilah dari perangkat lain di jaringan yang sama.

## Website
```
lab site add <nama> <folder> [--php 8.4] [--wordpress] [--apache]
lab site list | remove <nama>
lab site enable|disable <nama>               # aktifkan / jeda
lab site cert <nama>                         # perbarui sertifikat
lab siteclone <asal> <tujuan> [--with-db]

### Apache (opsional, untuk `.htaccess`)

nginx tetap menjadi pintu depan (HTTPS dan pembatasan tailnet). Situs bertipe Apache dilayani Apache di `127.0.0.1:8080`, yang tidak bisa dijangkau dari jaringan.

```
lab app install httpd              # pasang Apache dari Homebrew (belum menyala)
lab service start httpd            # nyalakan (stop | restart juga ada)
lab site add blog ~/MacPanel/wwwroot/blog --apache [--php 8.4]
lab apache sites                   # daftar vhost Apache
lab apache config                  # bangkitkan ulang config dan uji sintaks
```

## Versi PHP CLI
```
lab php shims                       # pasang pemilih versi php (blok PATH di ~/.zshrc, dengan cadangan)
lab php default [8.3]               # tampilkan atau atur default global CLI
lab php which [folder]              # versi yang terpakai di folder dan alasannya
lab php status [--json]             # default, shim, dan versi web per situs
```

## Akses publik sementara
```
lab app install cloudflared        # pasang Cloudflare Tunnel (tanpa proses sendiri)
lab public on blog --ttl 120       # buka situs ke publik 120 menit (5-1440), mencetak alamat acak
lab public status [--json]         # tunnel aktif dan sisa waktunya
lab public off blog                # tutup satu situs (atau: lab public off --all)
```
Listener loopback per situs dibuat di `nginx/public/public-NAMA.conf` (di luar `sites/`, jadi tidak tampil sebagai situs; `nginx.conf` ditambah satu baris include dengan cadangan) dan dihapus otomatis saat tunnel berhenti. Situs sistem, proxy ke layanan internal, dan situs Apache ditolak.

Config Apache dibangkitkan oleh `lab` (jangan diedit tangan, akan ditimpa). `--apache` tidak digabung dengan `--wordpress` atau port proxy. Di panel: Website, Tambah situs, centang "Layani dengan Apache".
lab appinstall laravel|wordpress <nama> [--php 8.4] [--db mariadb|mysql]
```

## Database
```
lab db create <nama> [--engine mariadb|mysql|pgsql]
lab db list
lab db dump <nama> [--engine ...]
lab db restore <nama> [--engine ...] [--file <dump>]
lab db users <nama> [--engine ...]
lab db passwd <nama> [--engine ...]
lab db drop <nama> [--engine ...]
lab db passwd <nama> [--engine ...] [--password-stdin]   # sandi user proyek
lab db user list <db> [--engine ...]
lab db user add <db> <user> --privs ro|rw|all [--engine ...] [--password-stdin]
lab db user passwd|privs|remove <db> <user> [--engine ...]
lab db root-passwd --engine mariadb|mysql|pgsql [--password-stdin]
lab dbcfg get|set|slow mariadb|mysql|redis ...
lab redis password status|enable|disable
lab sys exposure
lab proxy status|enable|disable|purge
lab firewall status
lab firewall block <port> [--allow tailnet|none] [--dry-run]
lab firewall unblock <port>
lab firewall verify
lab uninstall [--level 1|2|3] [--backup-dir DIR] [--with-docker-tools] [--apply]
```
Kata sandi pilihan sendiri dibaca dari stdin (`--password-stdin`), tidak pernah lewat argumen. Tanpa itu, kata sandi acak dibuat dan dicetak sekali. Bawaan `--engine` adalah `mariadb`. Port: MariaDB 3306, MySQL 3307, PostgreSQL 5432.

## PHP dan runtime
```
lab php list | install <ver> | remove <ver>
lab ext list <ver> | install|remove|enable|disable <ver> <nama>
lab ini get <ver> | set <ver> kunci=nilai
lab composer status | install | update | run <situs> <langkah>
lab runtime list | install | use | remove <alat> <versi>
```

## Worker dan cron

Pengganti Supervisor dan crontab, dijalankan launchd (berjalan setelah Anda login di Mac mini).

```
lab worker add <nama> --type php|node|python|cmd --dir <folder> --cmd "<perintah>" [--php 8.4] [--procs 2] [--env KUNCI=nilai]
lab worker add <nama> --type shell --dir <folder> --script-stdin     # isi skrip dibaca dari stdin
lab job add <nama> ... <salah satu jadwal>
lab job add <nama> --type url --url https://situs.lab/schedule/run --cron @hourly
lab worker|job list | status | restart | remove | logs
```

**Tipe tugas**

| Tipe | Isi | Batasan |
|---|---|---|
| `shell` | Skrip zsh beberapa baris, disimpan di `~/MacPanel/server/tasks/<nama>.sh` (hanya bisa dibaca Anda), dijalankan di folder kerja | Bebas (pipa, pengalihan, variabel). Maks 20000 byte. Lewat panel butuh kata sandi ulang dan tercatat |
| `php`, `node`, `python` | Satu perintah, mis. `php artisan schedule:run` | Program pertama harus sesuai tipe; tanpa kutip, pipa, atau `$`; flag kode sebaris (`-r`, `-e`, `-c`) ditolak |
| `url` | Meminta alamat `http(s)` lewat curl (batas 120 detik) | Tanpa spasi atau kutip |

**Jadwal cron job** (pilih satu): `--every 5m` atau `2h` (tiap N menit atau jam), `--at 03:00` (harian), atau `--cron "<5 kolom>"`.

`--cron` memakai format `menit jam tanggal bulan hari`, dengan `*`, daftar `1,15`, rentang `8-17`, langkah `*/10` atau `8-17/2`, dan nama singkat (`mon-fri`, `jan`). Alias: `@hourly`, `@daily`, `@weekly`, `@monthly`, `@yearly`, dan `@reboot` (saat login). Contoh: `*/15 8-17 * * 1-5` (tiap 15 menit pada jam kerja), `0 3 1 * *` (tanggal 1 pukul 03:00).

Dua batas yang sengaja ditolak, bukan diterjemahkan diam-diam, karena launchd berbeda dari cron:
- Tanggal **dan** hari-dalam-minggu tidak boleh sama-sama dibatasi (cron memakai OR, launchd memakai AND).
- Ekspresi yang meluas lebih dari 500 entri jadwal ditolak; perjarang menit atau jam.

Zona waktu mengikuti Mac mini. Daftar `lab job list` memisahkan jadwal dan perintah dengan tab.

## Aplikasi dan Docker

```
lab appinstall go|python|nextjs <nama> [--runtime <versi>] [--port <port>]
lab docker status|overview|stats
lab docker container start|stop|restart|remove|logs <nama>
lab docker image pull|remove <ref> | prune
lab docker compose list | up|down|ps|logs <proyek>
```

## Layanan dan aplikasi
```
lab service list
lab service start|stop|restart|logs <nama>
lab app install|remove <nama>
lab nginx test | reload
```

## Backup dan pembaruan
```
lab backup run | list | prune | delete | ls | extract
lab backup config                            # jadwal dan retensi
lab update check | apply
```

## Sistem
```
lab sys procs | disk | security | docker | dns | stats | kill | hw | ssd | net [--dns-name nama]
```


## Ruang disk dan memori (`lab clean`, `lab mem`)

Tersedia juga di panel: **Sistem → Ruang dan memori**.

- `lab clean scan`: mengukur katalog lokasi yang dikenal dan menyimpan hasilnya. Hanya membaca.
- `lab clean last`: menampilkan hasil analisis terakhir.
- `lab clean run <id> [<id> ...]`: **pratinjau** apa yang akan dibersihkan. Tambahkan `--apply` untuk benar-benar menghapus. Entri tingkat Hati-hati juga butuh `--confirm-careful`.
- `lab mem top`: ringkasan memori, pemakai terbesar per layanan, dan saran penghematan.

Tiga tingkat, ditetapkan oleh katalog di kode (bukan jalur bebas dari masukan):

| Tingkat | Isi | Perlakuan |
|---|---|---|
| Aman | cache yang dibuat ulang otomatis (Go, Homebrew, npm, pnpm, pip/uv, Composer, node-gyp, golangci-lint, image Colima, cache proxy npm, Tempat Sampah, log lama berputar) | bisa dipilih langsung; yang dihapus isinya, foldernya tetap ada |
| Hati-hati | folder sekali pakai di `backup/` (`migrasi-*`, `unduh-mac`, `pre-macpanel-*`) | satu per satu; di panel nama folder harus diketik ulang |
| Hanya ditampilkan | data Hermes, Dokumen, situs, data database, backup rutin, data Docker, Aplikasi | tidak pernah dihapus |

Aturan keamanan:
- Entri dilewati bila proses terkait sedang berjalan (mis. `go`, `brew`, `npm`).
- Symlink tidak pernah diikuti keluar dari jalur entri.
- Ukuran yang tidak terbaca (izin macOS) ditandai "sebagian", tidak ditebak.
- Bagian sistem macOS yang butuh Full Disk Access tidak ikut dihitung.
- Menghapus dari panel butuh verifikasi kata sandi ulang dan tercatat di Aktivitas. Token API tidak boleh memicu penghapusan.

Memori: macOS memakai RAM kosong sebagai cache file, jadi angka "terpakai" yang tinggi belum berarti masalah. Lihat **tekanan memori** dan swap. Saran penghematan hanya menyebut layanan opsional yang berjalan dan versi PHP-FPM yang tidak dipakai nginx (socket situs dan snippet seperti Adminer dipetakan ke versinya). `sudo purge` hanya mengosongkan cache file dan membuat aplikasi sedikit lebih lambat setelahnya; panel hanya menampilkan perintahnya untuk disalin.


## Tailscale (`lab tailscale`, `lab uninstall --level 4`)

Panel dan SSH dari jauh terjangkau lewat tailnet, jadi Tailscale diperlakukan terpisah dari tingkat 1-3.

- `lab tailscale status`: terpasang atau tidak, daemon sistem, dan status tailnet. Hanya membaca.
- `lab tailscale install`: rencana pemasangan (paket Homebrew + daemon sistem yang hidup sejak boot). Tambahkan `--apply` untuk menjalankan. Langkah `sudo` dipanggil per langkah dengan perintah tetap. Login ke tailnet (`tailscale up`) membuka browser dan **harus Anda lakukan sendiri**; tidak pernah dijalankan otomatis.
- `lab tailscale start|stop|restart --apply`: menjalankan, menghentikan, atau memulai ulang daemon (layanan sistem, butuh sudo). `lab tailscale down --apply`: keluar dari tailnet tanpa menghentikan daemon. Tanpa `--apply` hanya rencana. `stop` dan `down` memutus akses jarak jauh, jadi menolak berjalan dari sesi SSH yang lewat tailnet kecuali dengan `--allow-remote-cut`.
- Login ke tailnet (`tailscale up`) membuka browser dan tidak pernah dijalankan otomatis.
- `lab uninstall --level 4`: melepas **hanya** Tailscale (hentikan daemon, hapus paket). Panel, situs, database, backup, dan `/etc/resolver` tidak diubah. Tanpa `--apply` hanya rencana.

Pengaman tingkat 4: akses jarak jauh berakhir setelahnya, jadi perintah menolak berjalan dari sesi SSH yang masuk lewat tailnet (sesi akan putus di tengah jalan) kecuali `--allow-remote-cut`, dan meminta Anda mengetik ulang nama mesin. Perangkat tetap tercatat di admin console Tailscale sampai Anda menghapusnya di sana. Versi aplikasi (bukan Homebrew) dilepas lewat Finder.

## Filter daftar di panel

Halaman dengan daftar punya tab filter di atas daftar dengan jumlah di tiap tab; tab yang kosong disembunyikan. Website: jenis (PHP, Proxy, Statis, Kustom) serta Sistem dan Dijeda. Aplikasi: grup layanan, Tidak berjalan, Opsional. Database: mesin. Worker dan cron: status. Backup: umur. Docker: status container. Aktivitas: pintasan pencarian ke server (Gagal, Berjalan, Login, dan sejenisnya). Filter dan kolom cari bekerja bersamaan.

# Panel

Panel web untuk mengelola environment lokal seperti aaPanel/cPanel: situs, database, versi PHP dan runtime, worker, cron, pembaruan. Alamat: **https://panel.lab** (hanya dari tailnet). Kode: `~/Developer/_ops/web/panel` (Go + SQLite, UI React). Desain, model ancaman, dan status per fase: `web/panel/docs/DESIGN.md`.

## Status (2026-11-16)
| Fase | Isi | Status |
|---|---|---|
| P1 | Login argon2id, TOTP (Google Authenticator), recovery code, sesi, CSRF, lockout, audit berantai | selesai, teruji |
| P2a | Eksekutor aman, Website (tambah/hapus), Database (buat/dump/drop) | selesai, teruji |
| P2b | Pengaturan situs, rewrite, SSL, editor nginx berpagar, versi config, log | selesai, teruji |
| P3 | PHP, Runtime (node/go/python/pnpm), Worker, Cron, Pembaruan, job asinkron | selesai, teruji |
| P4 | UI web, LaunchAgent, `panel.lab` | selesai, teruji |
| P5 | Pantau (CPU, memori, disk, beban, bandwidth), Aplikasi (layanan + aplikasi opsional), jeda situs, tampilan Konsol Ramah | selesai, teruji |
| P4c | Menggantikan `home.lab` | **belum, sengaja** (lihat bawah) |

## Cara pakai
1. Buka https://panel.lab. Masuk dengan nama pengguna dan kata sandi, lalu kode 6 digit dari aplikasi autentikator (atau satu recovery code).
2. Menu (dikelompokkan di rail kiri; tombol Ciutkan menyisakan ikon dengan label):
   - **Gambaran:** Ringkasan (status keseluruhan, kartu CPU/memori/disk/jaringan, grafik, status layanan) dan Pantau (grafik CPU+memori, beban, disk, bandwidth per interface, total lalu lintas, proses teratas; rentang 1 jam, 24 jam, 7 hari).
   - **Kelola:** Website, Aplikasi, Database, PHP dan runtime, Worker dan cron.
   - **Sistem:** Pembaruan, Aktivitas.
3. Aksi merusak atau berisiko (hapus, upgrade, edit config, tambah worker/cron) meminta **kode TOTP ulang** dan, untuk penghapusan, mengetik ulang nama. Kode TOTP yang sama tidak bisa dipakai dua kali; bila baru dipakai untuk masuk, tunggu kode berikutnya (maks 30 detik).
4. Pemasangan PHP/runtime dan upgrade berjalan di latar belakang. Progres tampil di pojok kanan bawah dan daftar menyegarkan diri saat selesai.
5. Yang sengaja **tidak ada**: terminal web, pengelola berkas, aksi `sudo`. Kata sandi database tidak pernah ditampilkan.

## Pantau dan bandwidth
Panel mencatat satu sampel tiap 30 detik ke database panel dan menyimpannya 7 hari. Isinya pengukuran nyata; bila sebuah nilai gagal dibaca, disimpan kosong (bukan 0) dan grafik tidak menggambar titik itu.
- CPU: `top -l 2` (sampel kedua). Memori: `memory_pressure` (terpakai = 100 - bebas, sama dengan portal). Disk: `statfs` volume Data. Beban: `sysctl vm.loadavg`.
- Bandwidth: selisih penghitung byte `netstat -ibn` antar sampel, dibagi selang waktu, untuk interface ber-IPv4 non-loopback (LAN `en1` dan Tailscale `utun4` di mesin ini). Penghitung yang turun (reboot, interface baru) tidak menghasilkan laju. Total periode adalah jumlah selisih; lalu lintas saat panel mati tidak ikut terhitung.
  - `en1` (label "LAN") adalah kartu jaringan fisik ke router: semua lalu lintas Mac mini lewat sini, termasuk internet dan paket Tailscale terenkripsi. Ia juga menghitung lalu lintas lokal (laptop ke Mac mini); memisahkan internet dari lokal butuh pencatatan per koneksi, belum ada.
  - `utun4` (Tailscale) hanya lalu lintas di dalam terowongan; paketnya juga terhitung di `en1`. Karena itu kartu Jaringan di Ringkasan hanya menjumlahkan kartu fisik (tanpa Tailscale) agar tidak dobel.
- Panel hanya memanggil program sistem baca-saja dengan path mutlak dan argumen tetap (tanpa masukan pengguna). Daftar proses hanya memuat nama proses, tidak ada argumen perintah.
- Grafik pertama setelah panel dinyalakan butuh dua sampel (sekitar 30 detik).

## Aplikasi (layanan)
Halaman Aplikasi memeriksa tiap layanan langsung ke port atau socketnya, bukan dari catatan. Katalog: nginx, PHP-FPM 8.3/8.4/8.5, PostgreSQL, MySQL, Redis, Netdata, Uptime Kuma, Nginx UI, dnsmasq, panel, serta opsional Memcached dan Mailpit (bisa dipasang dan dihapus dari halaman).
- Mulai tidak butuh TOTP ulang; **hentikan, mulai ulang, dan hapus** meminta TOTP ulang dan mengetik id layanan.
- dnsmasq (berjalan sebagai root) dan panel itu sendiri tidak dikendalikan dari panel.
- Pemasangan dan penghapusan berjalan sebagai job asinkron (pojok kanan bawah; yang sukses hilang sendiri setelah 8 detik).
- Perintah di baliknya: `lab service list|start|stop|restart|logs`, `lab app list|install|remove` (lihat docs/06).

## Database: dump dan pulihkan
Tombol **Dump** membuat `~/Backups/db/<nama>-<engine>-<waktu>.sql.gz`. Tombol **Pulihkan** menampilkan dump milik database itu (30 terbaru) dan menjalankan `lab db restore` sebagai job asinkron dengan TOTP ulang. Pemulihan hanya ke database yang **kosong**; database yang berisi ditolak dengan pesan jelas. Database sistem (`dev`, `postgres`, ...) tidak bisa di-dump atau dipulihkan dari panel.

## PHP: ekstensi, pengaturan, Composer
Halaman PHP dan runtime, tombol **Kelola** per versi:
- **Ekstensi:** satu daftar lengkap (bawaan + tambahan) dengan pencarian dan filter. Ekstensi **Tambahan** punya saklar aktif/nonaktif dan tombol hapus; yang belum terpasang punya tombol Pasang. Katalog (semua teruji pasang, termuat, dan hapus di PHP 8.4 dan 8.5): redis, apcu, xdebug, imagick, igbinary, msgpack, pcov, yaml, memcached, mongodb. Tiap aksi memulai ulang FPM lalu memverifikasi lewat `php -m`; bila tidak termuat, perubahan dibatalkan.
- **Ekstensi bawaan (66 di PHP 8.5)** hanya ditampilkan dengan saklar terkunci: PHP Homebrew mengompilasinya permanen (`--enable-mbstring`, `--with-curl`, dst., folder modul kosong), jadi tidak bisa dimatikan satu per satu. Ini beda dengan cPanel/aaPanel yang memakai modul terpisah. Satu-satunya jalan agar bawaan bisa diatur adalah mengompilasi PHP sendiri; belum dikerjakan.
- **Pengaturan:** memory_limit, upload_max_filesize, post_max_size, max_execution_time, max_input_time, max_input_vars, display_errors, date.timezone. Hanya berlaku untuk situs web (pool FPM `dev`); CLI memakai php.ini. Bila `php-fpm -t` atau restart gagal, dikembalikan otomatis. Cadangan `dev.conf.bak.<waktu>` dibuat tiap simpan.
- **FPM dan log:** mulai ulang FPM versi itu dan baca log master (satu berkas bersama semua versi).
- **Composer:** kartu status, pasang, dan perbarui (Homebrew). Menjalankan composer per proyek ada di tab Deploy.

## Deploy per situs
Tab Deploy (situs non-sistem): git pull --ff-only, composer install (atau --no-dev -o, update, dump-autoload), npm ci atau pnpm install, build, artisan migrate dan optimize, restart FPM. Langkah yang muncul menyesuaikan isi proyek; dijalankan berurutan, berhenti di langkah gagal, tanpa shell, TOTP ulang. Git berjalan tanpa prompt (SSH BatchMode): key ber-passphrase harus ditarik manual di terminal.

## Backup, unggah dump, peringatan, peran
- **Backup:** halaman Backup menampilkan 14 backup terakhir dan tombol jalankan sekarang. Status offsite jujur: selama belum ada tujuan di luar disk ini, banner peringatan tetap tampil.
- **Unggah dump:** di dialog Pulihkan (maks 256 MB, .sql atau .sql.gz). `lab db restore` memindai isi dump dan menolak perintah klien (`\!`, `system`, `source`, `\copy`, `\i`, `\o`, dll.) sebelum menyentuh database, menghapus DEFINER dan OWNER/GRANT, dan menjalankan restore sebagai user proyek (bukan root).
- **Peringatan:** disk >=85% (penting >=92%), memori >=95%, layanan wajib mati, situs HTTP 5xx atau tidak menjawab, SSL <14 hari, backup >36 jam. Hanya tampil di panel (chip di header dan kartu di Ringkasan); tidak ada pesan keluar.
- **Peran:** `admin` dan `viewer` (hanya-baca; semua POST ditolak server kecuali keluar). Dikelola dari terminal: `panel user add <nama> --role viewer`, `panel user role <nama> admin|viewer` (sesi dicabut), `panel user list`. DB lama otomatis dimigrasi (semua user menjadi admin).
- **Tambah situs + database:** pilihan database di dialog Tambah situs (nama sama dengan situs, kata sandi acak, tidak ditampilkan).

## Website: jeda dan aktifkan
Tombol Jeda mengganti nama `<situs>.conf` menjadi `<situs>.conf.disabled` (nginx hanya memuat `*.conf`), menguji `nginx -t`, lalu reload; gagal di tahap mana pun dikembalikan. Folder proyek dan config tidak dihapus. Situs sistem tidak bisa dijeda. Daftar Website juga menampilkan status HTTP nyata (GET / lewat loopback ke nginx, dengan latensi), versi PHP, dan sisa umur sertifikat. Versi PHP situs bisa diganti dari tab Umum.

## Fitur lanjutan (P7)
- **Website per situs:** tab Domain (hingga 10 domain tambahan `nama.lab`, sertifikat mkcert dibuat ulang memuat semua nama), Akses (password dasar htpasswd apr1 hingga 20 pengguna, batas IP yang hanya boleh mempersempit tailnet, aturan allow/deny wajib diakhiri `deny all`), `.env` (baca butuh step-up, admin saja, cadangan 3 terakhir, izin 0600), Trafik (dari log akses nginx, bukan analitik pengunjung), Kloning (salin proyek + database terpisah dengan `.env` baru), dan Pasang cepat Laravel/WordPress (rollback otomatis bila gagal).
- **Database:** Pengaturan server MySQL (`max_connections`, buffer pool, slow log) dan Redis (`maxmemory`, policy, AOF, save) lewat berkas milik panel (`my.cnf.d/20-macpanel.cnf`, `redis.d/macpanel.conf`), restart otomatis dan dikembalikan bila server tidak naik. Tombol Akses per database: lihat kredensial dan ganti kata sandi (step-up, audit tanpa isi). Penelusur Redis (cari, lihat, hapus kunci; admin saja).
- **PHP:** pool FPM (`pm`, `pm.max_children`, dst.) di tab Pengaturan dengan validasi konsistensi mode dynamic.
- **Backup:** jadwal harian dan retensi bisa diatur (launchd + `backup.conf`), jelajahi arsip dan ekstrak berkas ke `~/Backups/restore/` (berkas asli tidak pernah ditimpa).
- **Pembersihan backup:** retensi otomatis berdasarkan jumlah (`KEEP`) dan umur (`DAYS`) di `~/.config/macminiops/backup.conf`; backup harian memanggil `lab backup prune --auto` setelah selesai. Backup terbaru TIDAK PERNAH dihapus (aturan di CLI). Di halaman Backup: hapus per pilihan, pratinjau lalu bersihkan (melebihi jumlah/umur), dan bersihkan dump DB (`~/Backups/db`) serta hasil ekstrak (`~/Backups/restore`) per umur, otomatis bila `EXTRA_DAYS` > 0. Hanya berkas bernama pola dump/ekstrak yang dihapus; symlink dan berkas lain dilewati. `~/Backups/retired` hanya ditampilkan ukurannya (hapus manual).
- **Webhook deploy (GitHub):** tab Deploy per situs. Penerima berjalan di listener terpisah `127.0.0.1:9101` (`--hook-addr`) yang hanya melayani `POST /hook/{situs}`; panel (9100) tidak ikut terbuka. Keamanan: HMAC-SHA256 (`X-Hub-Signature-256`), secret 64 hex disimpan terenkripsi dan tampil sekali, hanya branch terpilih, langkah deploy dari konfigurasi (bukan payload), tolak replay (`X-GitHub-Delivery`), jeda 15 dtk, satu deploy sekaligus per situs, kunci IP setelah tanda tangan salah berulang, semua penolakan 404 seragam. Agar GitHub menjangkaunya, ekspos HANYA port 9101 (Tailscale Funnel atau Cloudflare Tunnel); itu keputusan pemilik dan tidak dilakukan otomatis. Teruji nyata dengan repo git: push sah men-deploy (v1 -> v2), tanpa/salah tanda tangan 404, branch lain diabaikan, replay ditolak, LAN ke 9101 tertutup; mutasi 21/21.
- **DNS (baca saja):** tab DNS di Sistem: dnsmasq menjawab, wildcard vs IP Tailscale, `/etc/resolver`, resolusi tiap situs. Tidak ada tulis (butuh sudo).
- **Installer:** `install.sh` (lihat README repo).
- **Sistem:** halaman Keamanan (firewall, SSH, port yang mendengarkan), Proses (hentikan hanya proses pengembangan, SIGTERM), Disk per folder, status Docker (baca saja).
- **Akun:** ganti kata sandi sendiri (step-up, sesi lain dicabut).
- **Temuan nyata:** Mailpit dari Homebrew mendengarkan di semua antarmuka (`*:8025`, `*:1025`, tanpa login). Diperbaiki: dijalankan dengan plist milik panel yang terikat 127.0.0.1; diverifikasi LAN dan tailnet tertutup.
- **Sengaja tidak ada:** file manager, terminal web, backup S3, firewall/SSH dari web (butuh sudo).
- **Teruji:** 50 mutan keamanan, 44 tertangkap langsung; 3 ditutup dengan tes tambahan (batas 20 IP, 20 pengguna, 3 cadangan); 3 sisanya setara karena pengaman ganda (viewer diblok global di server.go, `deny all` dicek dua tempat). Uji mesin nyata: pool FPM/MySQL/Redis, rotasi sandi DB (lama ditolak, baru jalan), jadwal launchd, ekstrak arsip + path jahat, penginstal Laravel/WordPress, kloning dengan data.
- **Uji browser (Chromium, mesin asli):** Domain tambahan (3 nama menjawab HTTPS), password dasar + batas IP (401/200/403 sesuai), `.env`, Trafik (angka cocok dengan permintaan uji), Akses DB + ganti sandi (login dengan sandi baru berhasil), penelusur Redis, pengaturan Redis (nilai aktif berubah), jelajah+ekstrak backup, ganti sandi akun, kloning situs. Audit 390px dan tema terang: tanpa overflow halaman, target >=44px, kontras lulus.
- **Bug yang ditemukan lewat uji ini dan diperbaiki:** (1) salah kata sandi saat ganti sandi membalas 401 sehingga UI mengeluarkan sesi (kini 400); (2) tombol Hentikan proses bisa menyasar python milik Hermes (kini dilindungi lewat baris perintah); (3) tabel Sistem memakai kelas CSS yang tidak ada; (4) dua teks usang. Kloning tidak membawa alias/password dasar/batas IP (situs baru dari template bersih).
- **Belum diuji:** Pasang cepat lewat tombol UI (hanya via `lab appinstall`), Safari/Firefox/pembaca layar.

## Operasional
- LaunchAgent `com.macminiops.panel` (`~/Library/LaunchAgents`), `KeepAlive`, bind `127.0.0.1:9100`, log `~/Library/Logs/panel.log` (ikut `logrotate.sh`).
- Nginx: `web/nginx/sites/panel.conf`, `tailnet_only`, proxy ke 9100.
- Mulai ulang: `launchctl kickstart -k gui/$(id -u)/com.macminiops.panel`.
- Setelah mengubah kode:
  - UI: `cd ~/Developer/_ops/web/panel/ui && mise exec -- pnpm run build` (cukup muat ulang browser, tanpa restart).
  - Server: `cd ~/Developer/_ops/web/panel && mise exec -- go build -o panel . && launchctl kickstart -k gui/$(id -u)/com.macminiops.panel`.
- Tes: `mise exec -- go test ./... -race` (sekitar 100 detik; tes yang menyentuh mesin asli diaktifkan dengan `PANEL_REAL=1|2|3`).
- CLI: `panel init-key | user add <nama> | user remove <nama> | user list | audit verify | serve`. Lewat SSH, Keychain login terkunci: jalankan dulu `security unlock-keychain ~/Library/Keychains/login.keychain-db`.
- Jangan tempel setup key atau recovery code ke chat. Bila bocor: `panel user remove <nama>` lalu buat ulang (pernah terjadi sekali; sudah dirotasi).

## Setelah reboot atau mati listrik
FileVault berhenti di layar unlock, jadi LaunchAgent (termasuk panel, worker, cron) baru jalan **setelah Anda login** di Mac mini. Panel membaca kunci enkripsi dari Keychain login; bila Keychain belum terbuka, panel menolak start dan launchd mencoba lagi tiap 30 detik (perilaku aman, bukan galat). Reboot terencana: `sudo fdesetup authrestart` (dijalankan Anda sendiri). **Belum diuji**: perilaku nyata setelah reboot penuh.

## Backup dan restore
- `backup.sh` (02:30) menyalin DB panel `~/.config/macminiops/panel/panel.db` sebagai satu snapshot (`sqlite3 .backup`, `integrity_check` ok) ke `~/Backups/staging/<waktu>/panel.db`, masuk checksum.
- Kunci enkripsi TOTP ada di Keychain dan **sengaja tidak ikut backup**. Akibatnya, pada Mac baru snapshot tidak bisa membuka secret TOTP lama: buat ulang pengguna (`panel init-key`, `panel user add`) dan daftarkan ulang ke aplikasi autentikator.
- Backup masih lokal (disk yang sama), belum offsite. Restore snapshot diuji: terbuka dan datanya terbaca (1 user, 4 baris audit).

## Keamanan singkat
Panel hanya memanggil `lab` (argv list, tanpa shell, env bersih, timeout), semua keputusan akses di server, aksi merusak butuh TOTP ulang dan ketik nama, situs dan database sistem tidak bisa diubah/dihapus dari panel, perintah worker/cron dibatasi (program dan flag yang diizinkan, path di bawah `~/Developer`), editor config memakai allowlist direktif dan wajib mempertahankan pembatas tailnet. Cookie `__Host-` HttpOnly/Secure/SameSite=Strict, CSP ketat, UI disajikan dari daftar berkas yang dikenal (tanpa listing folder).

Terverifikasi dari luar (2026-11-16): akses lewat IP LAN 403, port 9100 tertutup dari LAN, API tanpa sesi 401, `/.env`, `/panel.db`, traversal, dan listing folder tidak bocor.

## Yang diuji dan yang belum
Teruji: 87 tes Go (`-race`, 0 gagal; 3 tes mesin-asli opsional dilewati) dan 9 tes UI (format, sumbu, geometri), uji mutasi (sengaja merusak penjaga lalu memastikan tes gagal), dan uji UI di browser asli dengan mesin asli: login dua langkah, step-up TOTP, situs (tambah, rewrite, edit config, restore, hapus), database (buat, dump, hapus), worker dan cron (tambah, restart, hapus), pasang dan hapus PHP 8.2 sebagai job asinkron. Responsif 390px dan 1280px tanpa overflow, target sentuh ≥44px, kontras ≥4,5:1 di dua tema, navigasi keyboard.

Uji tambahan P5 (browser asli, mesin asli): login, Ringkasan, Pantau (grafik dengan data nyata), pasang-nyalakan-mulai ulang-hentikan-hapus Memcached, jeda dan aktifkan situs (nginx menjawab 200 lalu menolak lalu 200 lagi), ganti PHP 8.4 ke 8.3 (nginx menjawab 8.3.36), hapus situs; 390px dan 1440px tanpa overflow, target sentuh >=44px di ponsel, tema terang dan gelap.

Belum teruji atau diketahui:
- Bandwidth hanya mencakup interface yang punya IPv4; interface yang baru muncul tercatat mulai sampel berikutnya. Laju memakai selisih penghitung 30 detik, jadi lonjakan lebih pendek dari itu terrata-rata.
- Memori memakai angka `memory_pressure` (sama dengan portal), bukan "terpakai" versi Activity Monitor; keduanya bisa berbeda.
- Perilaku setelah reboot penuh (Keychain di bawah launchd).
- `lab update apply` pada paket nyata lewat panel (saat diuji belum ada paket yang perlu di-upgrade). Jadwal cron `--at` sudah teruji: dijadwalkan 21:33, jalan 21:33:05, exit 0.
- `govulncheck`: dipindai 2026-11-16 (v1.9.0, basis data 11 Nov): tidak ada kerentanan. Percobaan ulang kadang timeout jaringan.
- Pemindaian parser nginx terhadap tafsir nginx pada kasus langka; `root` situs bisa menyajikan folder lain di bawah `~/Developer` oleh pemilik sesi penuh + TOTP.
- Tampilan di Safari/Firefox dan pembaca layar; hanya Chromium yang diuji.

## Kenapa `home.lab` belum diganti
Portal `home.lab` memuat grafik sumber daya, status layanan, log tersaring, dan dokumentasi; panel belum punya itu. Mengganti sekarang berarti kehilangan fitur. `home.lab` tetap hidup sampai panel setara.

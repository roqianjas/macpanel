# Usage

Panel user guide (also shown inside the panel under Dokumentasi, in Indonesian).


---

# Pengenalan

MacPanel adalah panel web untuk mengelola Mac sebagai server pengembangan: website (nginx), versi PHP, database, runtime, worker dan cron, deploy, backup, dan pemantauan. Cara kerjanya meniru aaPanel/cPanel, tetapi berjalan native di macOS tanpa Docker.

## Cara mengakses
- Buka `https://panel.lab` dari perangkat yang tersambung ke jaringan privat (Tailscale). Dari jaringan lain alamat ini ditolak (403).
- Masuk dengan nama pengguna dan kata sandi, lalu kode 6 digit dari aplikasi autentikator (atau satu recovery code).
- Centang **Percayai perangkat ini** bila ingin perangkat tersebut cukup memakai nama pengguna dan kata sandi selama 30 hari.

## Peta menu
| Grup | Menu | Fungsi |
|---|---|---|
| Gambaran | Ringkasan | Status keseluruhan: CPU, memori, disk, jaringan, layanan |
| Gambaran | Pantau | Grafik 1 jam, 24 jam, 7 hari, proses teratas, bandwidth |
| Kelola | Website | Tambah, jeda, hapus, dan atur situs |
| Kelola | Aplikasi | Layanan dan aplikasi opsional |
| Kelola | Database | MariaDB, MySQL, PostgreSQL, Redis |
| Kelola | PHP dan runtime | Versi PHP, ekstensi, Node, Go, Python |
| Kelola | Worker dan cron | Proses latar dan tugas terjadwal |
| Sistem | Sistem | Keamanan, proses, disk, DNS, Docker (lihat saja) |
| Sistem | Backup | Jadwal, daftar arsip, pemulihan, pembersihan |
| Sistem | Pembaruan | Pembaruan paket Homebrew |
| Sistem | Aktivitas | Riwayat aksi, perangkat tepercaya |
| Sistem | Dokumentasi | Halaman ini |

## Tata letak folder
Semua berada di bawah satu folder akar (bawaan `~/MacPanel`, dapat diganti dengan variabel lingkungan `MACPANEL_HOME`).

| Folder | Isi |
|---|---|
| `wwwroot/<situs>` | Berkas setiap situs |
| `server/` | Konfigurasi nginx, panel, skrip `lab`, aplikasi pendukung |
| `backup/` | Dump database dan arsip backup |
| `docs/` | Dokumentasi yang tampil di menu ini |

## Yang sengaja tidak ada
Terminal web, pengelola berkas, dan aksi yang butuh `sudo`. Untuk hal-hal itu panel menampilkan perintahnya dan Anda menjalankannya sendiri di Terminal.

---

# Website

Setiap situs dilayani nginx dan dapat diakses di `https://<nama>.lab` dengan sertifikat lokal. Berkasnya ada di `wwwroot/<nama>`.

## Menambah situs
Menu **Website** lalu **Tambah situs**. Pilih jenis:
- **PHP**: Laravel, WordPress, atau PHP biasa; pilih versi PHP.
- **Statis**: HTML, CSS, JS.
- **Proxy**: meneruskan ke aplikasi lokal (Node, Go, Python) pada port tertentu.

Nama situs hanya huruf kecil, angka, dan tanda hubung. Nama sistem (misalnya `panel`, `db`) tidak dapat dipakai.

## Pasang cepat
Tombol **Pasang cepat** membuat proyek Laravel atau WordPress lengkap: folder, database baru, berkas `.env` atau `wp-config.php`, dan situsnya. Anda dapat memilih MariaDB atau MySQL. Bila gagal di tengah, semua yang baru dibuat dibersihkan otomatis.

## Pengaturan per situs
Klik nama situs untuk membuka tab:
| Tab | Fungsi |
|---|---|
| Umum | Versi PHP, jeda atau aktifkan, kloning |
| Domain | Domain tambahan (sertifikat dibuat ulang agar memuat semua nama) |
| Akses | Kata sandi dasar (basic auth) dan batas IP |
| .env | Lihat dan ubah berkas `.env` (membutuhkan verifikasi ulang) |
| Trafik | Ringkasan log akses: status, halaman, dan IP teratas |
| Deploy | Tarik kode dari git, composer, npm/pnpm, migrasi; juga webhook GitHub |
| Rewrite | Aturan rewrite dan redirect |
| SSL | Status dan perpanjangan sertifikat |
| Config nginx | Editor konfigurasi dengan riwayat versi dan uji sebelum diterapkan |
| Log | Log akses dan error |

Konfigurasi yang gagal pada `nginx -t` tidak diterapkan.

## Menjeda dan menghapus
- **Jeda** menonaktifkan situs tanpa menghapus berkas.
- **Hapus** menghapus konfigurasi dan sertifikat; folder proyek tidak dihapus. Anda harus mengetik ulang nama situs.
- Situs sistem tidak dapat dijeda atau dihapus.

## Webhook deploy (GitHub)
Di tab Deploy, aktifkan webhook, pilih branch dan langkah deploy yang diizinkan. Panel menampilkan secret satu kali; simpan di pengaturan webhook GitHub. Setiap permintaan diverifikasi tanda tangannya, dan permintaan tidak sah dijawab 404. Penerima webhook mendengarkan hanya di loopback; mengeksposnya ke internet (misalnya lewat tunnel) adalah keputusan Anda dan tidak dilakukan otomatis.

---

# Database

Semua server database mendengarkan hanya di `127.0.0.1`; tidak ada yang terbuka ke jaringan.

| Mesin | Port | Catatan |
|---|---|---|
| MariaDB | 3306 | Port bawaan MySQL/MariaDB; mesin bawaan saat membuat database baru |
| MySQL | 3307 | Berjalan berdampingan dengan MariaDB |
| PostgreSQL | 5432 | |
| Redis | 6379 | Tanpa kata sandi, hanya loopback |

Klien yang terhubung lewat `localhost` memakai socket `/tmp/mysql.sock` yang menunjuk ke MariaDB. Untuk MySQL gunakan `127.0.0.1` dengan port 3307.

## Membuat database
Menu **Database** lalu **Buat database**. Panel membuat database dan satu pengguna dengan nama sama, serta kata sandi acak. Kata sandi disimpan di berkas kredensial proyek dan tidak pernah ditampilkan di layar; opsional ditulis ke kunci `DB_*` di `.env` proyek.

Pada Laravel isi `.env` seperti berikut (ganti `DB_PORT` sesuai mesin):
```
DB_CONNECTION=mysql
DB_HOST=127.0.0.1
DB_PORT=3306
```
Driver `mysql` dipakai untuk MariaDB maupun MySQL.

## Aksi per database
- **Dump**: menyimpan `<nama>-<mesin>-<waktu>.sql.gz` di `backup/db`.
- **Pulihkan**: memulihkan dari dump database itu atau dari berkas yang Anda unggah. Perintah klien berbahaya di dalam dump ditolak.
- **Akses**: melihat nama pengguna, mengganti kata sandi, atau menampilkan kredensial (membutuhkan verifikasi ulang).
- **Hapus**: menghapus database dan penggunanya; dump terakhir dibuat dulu. Anda harus mengetik ulang nama.

## Pengaturan server
Bagian **Pengaturan server** mengubah nilai penting MariaDB, MySQL (`max_connections`, `innodb_buffer_pool_size`, `max_allowed_packet`, slow query log) dan Redis (`maxmemory`, kebijakan eviction, AOF, save). Nilai disimpan di berkas khusus panel, server dimulai ulang sebentar, dan bila gagal naik, perubahan dikembalikan otomatis.

## Adminer
`https://db.lab` menyediakan antarmuka web untuk MariaDB, MySQL, dan PostgreSQL. Pilih sistem yang sesuai dan isi server serta port:
| Sistem | Server |
|---|---|
| MySQL / MariaDB | `127.0.0.1:3306` (MariaDB) atau `127.0.0.1:3307` (MySQL) |
| PostgreSQL | `127.0.0.1:5432` |

Gunakan kredensial proyek dari menu Database. Kata sandi akun administrator server ada di berkas rahasia milik pemilik mesin dan tidak dibagikan lewat panel.

## Redis
Penelusur Redis di halaman Database memungkinkan mencari dan melihat kunci, serta menghapusnya (hanya admin).

## Klien lain
Aplikasi seperti DBeaver dapat terhubung langsung ke `127.0.0.1` pada port di atas. Dari komputer lain gunakan SSH tunnel ke mesin ini.

---

# PHP dan runtime

## PHP
Beberapa versi PHP (8.2 sampai 8.5) dapat dipasang berdampingan. Setiap situs memilih versinya sendiri, dan tiap versi memiliki pool PHP-FPM sendiri.

Menu **PHP dan runtime** menyediakan:
- Pasang atau hapus versi PHP.
- **Ekstensi**: saklar untuk ekstensi PECL (redis, apcu, xdebug, imagick, dan lain-lain). Ekstensi bawaan PHP Homebrew sudah terkompilasi dan tidak dapat dimatikan.
- **php.ini**: ubah nilai umum seperti `memory_limit`, `upload_max_filesize`, `max_execution_time`.
- **Pool FPM**: mode proses dan `pm.max_children` dengan validasi konsistensi.
- **Composer**: lihat dan perbarui.

## Runtime
Node, pnpm, Go, dan Python dikelola oleh `mise`. Menu ini memperlihatkan versi yang terpasang dan memungkinkan memasang versi lain atau mengubah versi bawaan. Pemasangan berjalan di latar belakang; progresnya tampil di pojok kanan bawah.

## Worker dan cron
Menu **Worker dan cron** adalah pengganti Supervisor dan crontab, berjalan di atas launchd.
- **Worker**: proses yang selalu hidup dan dihidupkan ulang otomatis (queue Laravel, bot, dan sejenisnya). Tentukan folder, perintah, versi PHP, dan jumlah proses.
- **Cron**: tugas terjadwal, per interval (misalnya tiap 1 menit) atau pada jam tertentu.
- Log tiap worker dan cron dapat dilihat dari halaman yang sama.

---

# Backup

## Isi backup
Setiap backup berisi: berkas semua situs (`wwwroot`), dump seluruh database (MariaDB, MySQL, PostgreSQL), berkas konfigurasi pribadi (dotfiles), kunci publik SSH, dan snapshot database panel, ditambah daftar kontrol `SHA256SUMS` untuk memeriksa keutuhan. Kunci privat SSH dan kunci enkripsi TOTP sengaja tidak ikut. Arsip disimpan di folder `backup/`.

## Jadwal dan retensi
- Jadwal harian dijalankan oleh launchd; jam dan retensi dapat diatur di halaman **Backup**.
- Retensi menurut jumlah arsip (`KEEP`) dan umur (`DAYS`). Arsip terbaru tidak pernah dihapus.

## Halaman Backup
- Lihat daftar arsip dan ukurannya.
- **Jelajahi** isi arsip dan ekstrak berkas ke `backup/restore/`; berkas aslinya tidak ditimpa.
- **Bersihkan**: hapus per pilihan atau otomatis menurut retensi, dengan pratinjau sebelum dijalankan. Hanya berkas berpola dump atau ekstrak yang dihapus.

## Penting
Backup disimpan di disk yang sama dengan data. Itu melindungi dari salah hapus, tetapi **bukan** dari kerusakan disk atau kehilangan perangkat. Salin folder `backup/` secara berkala ke disk eksternal atau layanan lain, dan uji pemulihan secara berkala.

---

# Aplikasi dan layanan

Menu **Aplikasi** memeriksa setiap layanan langsung ke port atau socketnya, bukan dari catatan, sehingga statusnya mencerminkan keadaan nyata.

## Layanan inti
nginx, PHP-FPM (tiap versi), MariaDB, MySQL, PostgreSQL, Redis, dan dnsmasq (DNS untuk domain `.lab`). Tersedia tombol mulai, hentikan, mulai ulang, dan lihat log. Panel sendiri tidak dapat dihentikan dari panel, dan dnsmasq (berjalan sebagai root) hanya ditampilkan; kendalinya lewat `sudo` di Terminal.

## Aplikasi bawaan
Dipasang saat penyiapan mesin dan dikelola dari tombol mulai/hentikan; semuanya hanya mendengarkan `127.0.0.1` dan disajikan lewat nginx pada jaringan privat:
| Aplikasi | Alamat | Fungsi |
|---|---|---|
| Adminer | `db.lab` | Antarmuka web database |
| Uptime Kuma | `kuma.lab` | Pemantau uptime |
| Netdata | `stats.lab` | Metrik sistem terperinci |

## Aplikasi opsional
Tombol **Pasang** memasang aplikasi dan membuat alamat `<nama>.lab`:
| Aplikasi | Fungsi |
|---|---|
| Mailpit | Menangkap email uji coba (SMTP lokal) |
| Memcached | Cache memori |
| 9Router | Router dan proxy model AI |

Catatan 9Router: dashboard memakai kata sandi bawaan dari pembuatnya. 9Router menolak login jarak jauh sampai kata sandi itu diganti, jadi ganti terlebih dulu dari mesin itu sendiri di `http://127.0.0.1:20128` (dari komputer lain: `ssh -L 20128:127.0.0.1:20128 <mesin>`, lalu buka `http://localhost:20128`). Data akun disimpan di `~/.9router` dan tidak ikut backup panel.

## Menghapus aplikasi opsional
Hapus memerlukan verifikasi ulang dan mengetik nama aplikasi. Data pengguna aplikasi tidak dihapus.

## Domain `.lab`
dnsmasq menjawab semua `*.lab` dengan alamat jaringan privat mesin ini, dan nginx menyajikan HTTPS memakai sertifikat lokal (mkcert). Perangkat klien perlu memercayai CA lokal tersebut dan memakai mesin ini sebagai DNS untuk domain `.lab` (di Tailscale: Split DNS).

---

# Pemantauan dan sistem

## Ringkasan dan Pantau
Panel mencatat satu sampel tiap 30 detik dan menyimpannya 7 hari. Grafik hanya berisi pengukuran nyata; bila satu nilai gagal dibaca, titiknya kosong (bukan 0).

- **CPU**: pemakaian total.
- **Memori**: App Memory + Wired + Compressed, sama dengan definisi Activity Monitor macOS (memori yang dipakai cache tidak dihitung terpakai).
- **Disk**: volume data.
- **Beban**: load average.
- **Bandwidth**: selisih byte per antarmuka jaringan; total periode adalah jumlah selisih, sehingga reboot tidak menurunkan angka.

## Peringatan
Panel menandai disk hampir penuh, memori tinggi, layanan mati, situs yang menjawab error, sertifikat yang akan kedaluwarsa, dan backup yang sudah lama.

## Halaman Sistem
| Tab | Isi |
|---|---|
| Keamanan | Status firewall, SSH, enkripsi disk, dan port yang sedang mendengarkan |
| Proses | Proses pengembangan yang berjalan; dapat dihentikan dengan SIGTERM (proses sistem dilindungi) |
| Disk | Pemakaian per folder |
| DNS | Status dnsmasq dan resolusi tiap situs (hanya baca) |
| Docker | Status Docker bila terpasang (hanya baca) |

Perubahan yang membutuhkan `sudo` (firewall, DNS sistem, sharing) tidak dilakukan panel. Panel hanya menampilkan perintahnya.

## Pembaruan
Menu **Pembaruan** menampilkan paket Homebrew yang usang. Pembaruan berjalan di latar belakang dan progresnya tampil di pojok kanan bawah. Versi database ditetapkan (MariaDB 11.8, MySQL 8.4, PostgreSQL 17) sehingga tidak naik ke versi mayor tanpa Anda sadari.

---

# Akun dan keamanan

## Login
- Kata sandi disimpan dengan argon2id. Percobaan salah berulang mengunci akun dan alamat IP sementara.
- Kode TOTP (Google Authenticator atau sejenisnya) diminta saat login. Setiap kode hanya dapat dipakai sekali.
- Saat pengaturan awal, kunci TOTP dan 10 recovery code ditampilkan **sekali**. Simpan di tempat aman (pengelola kata sandi). Satu recovery code hanya berlaku sekali.

## Perangkat tepercaya
Pada login lengkap, centang **Percayai perangkat ini**. Perangkat itu lalu cukup memakai nama pengguna dan kata sandi selama 30 hari.
- Setiap akun memiliki maksimum 10 perangkat tepercaya.
- Sesi baru dari perangkat tepercaya tetap meminta verifikasi ulang kata sandi untuk aksi berbahaya.
- Cabut satu atau semua perangkat di **Aktivitas**. Mengganti kata sandi atau menghapus akun mencabut semuanya.
- Risiko: orang yang memegang perangkat tepercaya sekaligus kata sandi dapat masuk tanpa kode 2FA sampai perangkat itu dicabut.

## Verifikasi ulang untuk aksi berbahaya
Aksi seperti menghapus situs atau database, melihat kredensial, mengubah konfigurasi, dan memasang aplikasi meminta **kata sandi ulang**. Setelah benar, aksi lain berlaku tanpa bertanya lagi selama 15 menit. Tebakan salah kena pembatasan yang sama dengan login. Aksi penghapusan juga meminta Anda mengetik ulang nama.

## Peran
- **Admin**: semua aksi.
- **Viewer**: hanya melihat; tidak dapat mengubah apa pun dan tidak melihat kredensial.

## Mengelola akun (dari Terminal mesin ini)
```
panel user add [--role admin|viewer] <nama>
panel user list
panel user role <nama> admin|viewer
panel user rename <lama> <baru>
panel user remove <nama>
panel audit verify
```
Penambahan akun menampilkan kunci TOTP dan recovery code sekali saja. Ganti kata sandi sendiri dari menu akun di panel.

## Log aktivitas
Menu **Aktivitas** mencatat setiap aksi: siapa, apa, kapan, dari IP mana. Catatan saling terkait secara kriptografis sehingga perubahan atau penghapusan diam-diam dapat terdeteksi.

## Model keamanan singkat
- Panel hanya mendengarkan loopback dan hanya terbuka lewat nginx untuk jaringan privat.
- Panel tidak memakai shell dan tidak menjalankan `sudo`; ia hanya memanggil perintah `lab` dengan argumen yang divalidasi.
- Tidak ada terminal web maupun pengelola berkas.
- Kata sandi database tidak pernah dikirim ke layar kecuali Anda meminta lewat aksi yang diverifikasi.

---

# Perintah `lab`

`lab` adalah perintah di Terminal yang dipakai panel di belakang layar. Anda juga dapat memakainya langsung. Semua perintah menolak argumen yang tidak valid dan bersifat idempoten.

## Website
```
lab site add <nama> <folder> [--php 8.4] [--wordpress]
lab site list | remove <nama>
lab site enable|disable <nama>               # aktifkan / jeda
lab site cert <nama>                         # perbarui sertifikat
lab siteclone <asal> <tujuan> [--with-db]
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
lab dbcfg get|set|slow mariadb|mysql|redis ...
```
Bawaan `--engine` adalah `mariadb`. Port: MariaDB 3306, MySQL 3307, PostgreSQL 5432.

## PHP dan runtime
```
lab php list | install <ver> | remove <ver>
lab ext list <ver> | install|remove|enable|disable <ver> <nama>
lab ini get <ver> | set <ver> kunci=nilai
lab composer status | install | update | run <situs> <langkah>
lab runtime list | install | use | remove <alat> <versi>
```

## Worker dan cron
```
lab worker add <nama> --dir <folder> --cmd "<perintah>" [--php 8.4] [--procs 2]
lab job add <nama> --dir <folder> --cmd "<perintah>" (--every 1m | --at 02:30)
lab worker|job list | status | restart | remove | logs
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
lab sys procs | disk | security | docker | dns | stats | kill
```

---

# Pemecahan masalah

## `https://panel.lab` tidak terbuka
1. Pastikan perangkat Anda tersambung ke jaringan privat (Tailscale aktif).
2. Pastikan DNS untuk domain `.lab` mengarah ke mesin ini (Split DNS). Cek dengan `dig panel.lab`.
3. Peringatan sertifikat: pasang CA lokal (mkcert) di perangkat Anda.
4. Respons **403** berarti Anda mengakses dari luar jaringan privat; itu disengaja.
5. Respons **502** berarti panel atau aplikasi di belakang nginx tidak berjalan. Lihat menu Aplikasi.

## Panel tidak naik setelah dibangun ulang
Muat ulang LaunchAgent dengan `launchctl bootout` lalu `launchctl bootstrap`, kemudian tunggu hingga 75 detik. Jangan memakai `kickstart -k`; ia mematikan paksa proses.

## Situs menampilkan 502
Layanan PHP-FPM atau aplikasi yang diproksi tidak berjalan atau versinya tidak terpasang. Periksa menu Aplikasi dan tab Log situs.

## Tidak bisa login ke Adminer
Pilih sistem yang tepat (MySQL untuk MariaDB dan MySQL, PostgreSQL untuk Postgres), isi server `127.0.0.1:<port>`, dan pakai kredensial proyek dari menu Database. Port MariaDB 3306, MySQL 3307, PostgreSQL 5432.

## Aplikasi Laravel tidak terhubung ke database
Cocokkan `DB_PORT` di `.env` dengan mesinnya: 3306 untuk MariaDB, 3307 untuk MySQL. Setelah mengubah `.env`, jalankan `php artisan config:clear`.

## Lupa kata sandi atau kehilangan perangkat 2FA
Gunakan satu recovery code saat login. Bila tidak ada, buat ulang akun dari Terminal mesin ini (`panel user add`) atau minta administrator lain mengganti kata sandinya.

## Disk penuh
Menu Sistem, tab Disk menunjukkan folder terbesar. Bersihkan backup lama dari halaman Backup, dan cache paket dengan `brew cleanup`.

## Mac tidak terjangkau setelah restart
Setelah restart pada disk terenkripsi, layanan penuh hanya naik setelah seseorang masuk di layar. Hindari restart saat jauh dari mesin, kecuali Anda yakin sambungan jaringan privat naik sebelum login.

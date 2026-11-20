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
| Gambaran | Pantau | Grafik 1 jam, 24 jam, 7 hari, suhu dan kipas, tekanan memori, kesehatan SSD, proses teratas, bandwidth |
| Kelola | Website | Tambah, jeda, hapus, dan atur situs |
| Kelola | Aplikasi | Layanan dan aplikasi opsional |
| Kelola | Database | MariaDB, MySQL, PostgreSQL, Redis |
| Kelola | PHP dan runtime | Versi PHP, ekstensi, Node, Go, Python |
| Kelola | Worker dan cron | Proses latar dan tugas terjadwal |
| Sistem | Sistem | Keamanan, proses, disk, DNS, Docker (lihat saja) |
| Sistem | Backup | Jadwal, daftar arsip, pemulihan, pembersihan |
| Sistem | Pembaruan | Pembaruan paket Homebrew |
| Sistem | Aktivitas | Riwayat aksi |
| Kelola | Docker | Container, image, dan proyek Compose (opsional, dipasang dari Aplikasi) |
| Sistem | Pengaturan | Nama dan tema panel, kata sandi, 2FA, perangkat tepercaya, token API |
| Sistem | Pengguna | Akun panel: tambah, peran, nonaktifkan, reset, hapus (admin) |
| Sistem | Dokumentasi | Halaman ini |

## Daftar yang panjang

Daftar website, database, worker/cron, dan backup punya kolom **Cari** serta tombol halaman (10 baris per halaman) begitu isinya cukup banyak (8 item atau lebih). Mengetik beberapa kata mempersempit hasil: setiap kata harus cocok, huruf besar-kecil tidak berpengaruh, dan karakter seperti `*` atau `%` dicari apa adanya. Di **Aktivitas**, audit dan aksi dimuat 25 baris sekali jalan; tombol "Muat ... lebih lama" mengambil berikutnya, dan pencarian dijalankan di server sehingga menjangkau seluruh riwayat, bukan hanya baris yang sudah tampil.

Tombol halaman bisa dipakai dengan keyboard: setelah sampai di halaman pertama atau terakhir, fokus berpindah ke tombol lawannya.

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

# Website

Setiap situs dilayani nginx dan dapat diakses di `https://<nama>.lab` dengan sertifikat lokal. Berkasnya ada di `wwwroot/<nama>`.

## Menambah situs
Menu **Website** lalu **Tambah situs**. Pilih jenis:
- **PHP**: Laravel, WordPress, atau PHP biasa; pilih versi PHP.
- **Statis**: HTML, CSS, JS.
- **Proxy**: meneruskan ke aplikasi lokal (Node, Go, Python, container) pada port tertentu.

Nama situs hanya huruf kecil, angka, dan tanda hubung. Nama sistem (misalnya `panel`, `db`) tidak dapat dipakai.

Untuk situs yang membutuhkan `.htaccess`, centang **Layani dengan Apache** (Apache harus sudah dipasang dan menyala di menu Aplikasi).

## Pasang cepat
Tombol **Pasang cepat** membuat proyek Laravel atau WordPress lengkap: folder, database baru, berkas `.env` atau `wp-config.php`, dan situsnya. Anda dapat memilih MariaDB atau MySQL. Bila gagal di tengah, semua yang baru dibuat dibersihkan otomatis.

## Proyek Go, Python, dan Next.js
Di **Pasang cepat**, pilih **Go**, **Python**, atau **Next.js**. Panel membuat:
1. Folder proyek berisi kerangka aplikasi yang langsung jalan (Go: `net/http`; Python: virtualenv `.venv` dan `app.py`; Next.js: `create-next-app` dengan TypeScript, App Router, dan pnpm).
2. **Worker** (proses latar belakang di launchd) yang menjalankan aplikasi di `127.0.0.1` pada port yang dipilih otomatis (3100-3999) atau port yang Anda tentukan, dan hidup kembali bila mati atau setelah restart.
3. Situs proxy `https://<nama>.lab` yang meneruskan ke port itu.

Versi runtime (Go, Python, Node) dapat dipilih; kosong berarti versi bawaan dari menu **PHP dan runtime**. Aplikasi hanya mendengarkan `127.0.0.1`; satu-satunya jalur masuk adalah nginx. Bila langkah mana pun gagal (mis. build atau aplikasi tidak menjawab), folder, worker, dan situs yang baru dibuat dibersihkan otomatis.

Tab **Deploy** mengenali proyek ini: Go menjalankan `go build`, Python `pip install -r requirements.txt`, Next.js install dan build dengan pnpm atau npm, lalu **Mulai ulang aplikasi** me-restart workernya. Menghapus situs ikut menghentikan dan menghapus workernya. Log aplikasi ada di menu **Worker dan cron**.

Untuk proyek yang sudah ada, gunakan **Tambah situs** jenis Proxy dan buat workernya sendiri di **Worker dan cron**.

## Pengaturan per situs
Klik nama situs untuk membuka tab:
| Tab | Fungsi |
|---|---|
| Umum | Versi PHP, jeda atau aktifkan, kloning |
| Domain | Extra domains: `name.lab` names get a local certificate; any other host name (e.g. a Tailscale Funnel name) is accepted without a certificate and needs an explicit public-exposure confirmation, because the tailnet-only rule does not stop Funnel traffic |
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

## Akses publik sementara (tab Publik)
Untuk menguji bersama orang di luar jaringan privat, tab **Publik** membuka satu situs ke internet lewat Cloudflare Quick Tunnel (tanpa akun dan tanpa domain). Pasang dulu **Cloudflare Tunnel** dari halaman Aplikasi.
- Centang konfirmasi, pilih batas waktu (30 menit sampai 24 jam), lalu **Buka ke publik**. Alamat acak `https://…trycloudflare.com` muncul; salin dan bagikan.
- Setiap situs punya listener lokal sendiri, jadi hanya situs itu yang terbuka. Situs sistem dan proxy ke layanan internal ditolak. Berkas tersembunyi (`.env`, `.git`) selalu ditolak.
- Tidak ada login tambahan, alamat berubah setiap dinyalakan, dan tidak ada jaminan ketersediaan. Pakai data uji.
- Mati otomatis saat waktu habis, atau tekan **Matikan sekarang**. Tidak menyala lagi setelah reboot. Situs Apache belum didukung.
- Jumlah situs yang bisa publik bersamaan tidak dibatasi (masing-masing satu proses `cloudflared`).

## Webhook deploy (GitHub)
Di tab Deploy, aktifkan webhook, pilih branch dan langkah deploy yang diizinkan. Panel menampilkan secret satu kali; simpan di pengaturan webhook GitHub. Setiap permintaan diverifikasi tanda tangannya, dan permintaan tidak sah dijawab 404. Penerima webhook mendengarkan hanya di loopback; mengeksposnya ke internet (misalnya lewat tunnel) adalah keputusan Anda dan tidak dilakukan otomatis.

# Database

Semua server database mendengarkan hanya di `127.0.0.1`; tidak ada yang terbuka ke jaringan.

| Mesin | Port | Catatan |
|---|---|---|
| MariaDB | 3306 | Port bawaan MySQL/MariaDB; mesin bawaan saat membuat database baru |
| MySQL | 3307 | Berjalan berdampingan dengan MariaDB |
| PostgreSQL | 5432 | |
| Redis | 6379 | Hanya loopback; kata sandi opsional (lihat bagian Redis) |

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
- **Akses**: melihat kredensial (membutuhkan verifikasi ulang), mengganti kata sandi user proyek (acak, atau pilihan sendiri), dan mengelola user database tambahan (lihat di bawah).
- **Hapus**: menghapus database dan penggunanya; dump terakhir dibuat dulu. Anda harus mengetik ulang nama.

## User database tambahan
Di dialog **Akses**, bagian *User database*, Anda dapat membuat user lain selain user proyek, misalnya untuk laporan baca-saja atau aplikasi kedua.
- **Hak** per database: *Baca-saja* (SELECT), *Baca-tulis* (SELECT, INSERT, UPDATE, DELETE), atau *Penuh* (termasuk mengubah struktur). Tidak ada hak global.
- **Host** hanya `127.0.0.1` dan `localhost`; tidak ada user yang dapat terhubung dari mesin lain.
- Kata sandi **acak** (tampil sekali) atau **pilihan sendiri** (12-100 karakter dari huruf, angka, dan `. _ + = : , @ % ^ -`).
- Ganti kata sandi, ubah hak, dan hapus butuh verifikasi ulang; hapus juga meminta mengetik nama user. Hanya user yang dibuat dari panel yang dapat diubah atau dihapus di sini; akun lain tidak disentuh.
- Menghapus database ikut menghapus user tambahannya.
- Tidak tersedia lewat token API (hanya membaca daftar).

## Kata sandi root server database
Kartu *Kata sandi root server database* mengganti akun administrator MariaDB, MySQL (`root`), dan PostgreSQL (`dev`).
- Pilih acak atau sendiri, lalu ketik nama mesin untuk konfirmasi (butuh verifikasi ulang).
- Panel memeriksa kata sandi lama masih cocok, mengganti, menyimpan ke berkas rahasia pemilik mesin, lalu memverifikasi login dengan kata sandi baru. Bila verifikasi gagal, berkas dikembalikan.
- Perbarui koneksi di klien yang memakai akun root (mis. DBeaver). Proyek memakai user sendiri sehingga tidak terpengaruh.
- Tidak tersedia lewat token API.

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

Redis hanya mendengar di loopback, tetapi proses lain di mesin yang sama tetap bisa memakainya bila tanpa kata sandi. Untuk mewajibkan kata sandi, jalankan di terminal:

```
lab redis password status     # keadaan sekarang (tidak menampilkan kata sandi)
lab redis password enable     # buat kata sandi acak, pasang di redis.conf, mulai ulang Redis
lab redis password disable    # kembalikan seperti semula
```

- Kata sandi disimpan di `~/.config/macminiops/redis.env` (izin 0600). Lihat dan salin sendiri dari berkas itu untuk aplikasi atau klien seperti DBeaver; jangan dikirim ke chat.
- `lab redis` dan penelusur di panel memakainya otomatis; kata sandi tidak pernah muncul di argumen proses.
- Aplikasi Anda yang memakai Redis harus diberi kata sandi itu (di Laravel: `REDIS_PASSWORD` pada `.env`). Aplikasi yang tidak memakai Redis (cache `array`, antrean `sync`, sesi `file`) tidak terpengaruh.
- `enable` membuat cadangan `redis.conf` bertimestamp dan memverifikasi bahwa koneksi tanpa kata sandi ditolak; bila gagal, konfigurasi dikembalikan otomatis. Bila `redis.conf` sudah memuat `requirepass` yang Anda atur sendiri, perintah berhenti tanpa mengubah apa pun.

## Klien lain
Aplikasi seperti DBeaver dapat terhubung langsung ke `127.0.0.1` pada port di atas. Dari komputer lain gunakan SSH tunnel ke mesin ini.

# PHP dan runtime

## PHP
Beberapa versi PHP (8.2 sampai 8.5) dapat dipasang berdampingan. Setiap situs memilih versinya sendiri, dan tiap versi memiliki pool PHP-FPM sendiri.

Menu **PHP dan runtime** menyediakan:
- Pasang atau hapus versi PHP.
- **Ekstensi**: saklar untuk ekstensi PECL (redis, apcu, xdebug, imagick, dan lain-lain). Ekstensi bawaan PHP Homebrew sudah terkompilasi dan tidak dapat dimatikan.
- **php.ini**: ubah nilai umum seperti `memory_limit`, `upload_max_filesize`, `max_execution_time`.
- **php.ini CLI**: `lab ini` hanya mengubah pool FPM. Untuk batas CLI (mis. `memory_limit` bagi test suite besar), buat file `.ini` di `/opt/homebrew/etc/php/<versi>/conf.d/` (mis. `zz-lab-memory.ini`). Nilai `php_admin_value` pada pool FPM tetap mengalahkannya.
- **Pool FPM**: mode proses dan `pm.max_children` dengan validasi konsistensi.
- **Composer**: lihat dan perbarui.

## Runtime
Node, pnpm, Go, dan Python dikelola oleh `mise`. Menu ini memperlihatkan versi yang terpasang dan memungkinkan memasang versi lain atau mengubah versi bawaan. Pemasangan berjalan di latar belakang; progresnya tampil di pojok kanan bawah.

## Worker dan cron
Menu **Worker dan cron** adalah pengganti Supervisor dan crontab, berjalan di atas launchd.
- **Worker**: proses yang selalu hidup dan dihidupkan ulang otomatis (queue Laravel, bot, dan sejenisnya). Tentukan folder, perintah, versi PHP, dan jumlah proses.
- **Cron**: tugas terjadwal, per interval (misalnya tiap 1 menit) atau pada jam tertentu.
- Log tiap worker dan cron dapat dilihat dari halaman yang sama.

# Backup

## Isi backup
Setiap backup berisi: berkas semua situs (`wwwroot`), dump seluruh database (MariaDB, MySQL, PostgreSQL), berkas konfigurasi pribadi (dotfiles), kunci publik SSH, dan snapshot database panel, ditambah daftar kontrol `SHA256SUMS` untuk memeriksa keutuhan. Kunci privat SSH dan kunci enkripsi TOTP sengaja tidak ikut. Arsip disimpan di folder `backup/`.

## Jadwal dan retensi
- Backup otomatis dijalankan oleh launchd. Di halaman **Backup** Anda bisa mengatur jam, **interval (1 sampai 30 hari)**, retensi, dan **mematikan backup otomatis** sama sekali bila disk terasa penuh. Saat dimatikan tidak ada salinan baru sampai dinyalakan lagi atau "Jalankan sekarang" ditekan; backup manual tetap berjalan.
- Dari terminal: `lab backup config set --time 02:30 --keep 7 --every-days 2` (interval) dan `--enabled no` (matikan). `lab backup run` selalu menjalankan backup walau jadwal mati.
- Retensi menurut jumlah arsip (`KEEP`) dan umur (`DAYS`). Arsip terbaru tidak pernah dihapus.

## Halaman Backup
- Lihat daftar arsip dan ukurannya.
- **Jelajahi** isi arsip dan ekstrak berkas ke `backup/restore/`; berkas aslinya tidak ditimpa.
- **Bersihkan**: hapus per pilihan atau otomatis menurut retensi, dengan pratinjau sebelum dijalankan. Hanya berkas berpola dump atau ekstrak yang dihapus.

## Penting
Backup disimpan di disk yang sama dengan data. Itu melindungi dari salah hapus, tetapi **bukan** dari kerusakan disk atau kehilangan perangkat. Salin folder `backup/` secara berkala ke disk eksternal atau layanan lain, dan uji pemulihan secara berkala.

# Aplikasi dan layanan

Menu **Aplikasi** memeriksa setiap layanan langsung ke port atau socketnya, bukan dari catatan, sehingga statusnya mencerminkan keadaan nyata.

## Unduhan besar yang sering terputus

Bila jaringan sering memutus unduhan besar (tanda: instalasi npm/pnpm gagal dengan `ECONNRESET`, misalnya saat memasang proyek Next.js), nyalakan proxy lokal yang melanjutkan unduhan dari titik putus:

```
lab proxy enable     # nyalakan (LaunchAgent, hanya 127.0.0.1:4873)
lab proxy status     # keadaan dan isi cache
lab proxy disable    # matikan; cache dipertahankan
lab proxy purge      # hapus cache
```

- Hanya dipakai oleh instalasi proyek dari panel/`lab` (Next.js dan sejenisnya) saat proxy aktif dan sehat; bila proxy mati, instalasi langsung ke registry seperti biasa.
- Bukan proxy umum: hanya GET, hanya path paket npm, hanya ke registry npm resmi. Berkas diperiksa terhadap integritas SHA-512 dari registry sebelum dianggap lengkap.
- Unduhan yang terputus tetap berlanjut di latar; percobaan berikutnya langsung memakai hasilnya. Cache disimpan di `~/.cache/macpanel/regproxy` (berkas lama dipangkas otomatis).
- Proyek di luar `lab` (menjalankan `npm install` sendiri) memakainya dengan `npm_config_registry=http://127.0.0.1:4873/`.

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
| Apache | Web server untuk situs yang butuh `.htaccess` (lihat di bawah) |

### Apache untuk situs yang butuh .htaccess
Pasang **Apache** dari tab Web server, lalu nyalakan. Apache hanya mendengarkan di `127.0.0.1:8080` dan tidak bisa dijangkau dari jaringan; nginx tetap memegang HTTPS dan pembatasan jaringan privat. Setelah menyala, centang **Layani dengan Apache** saat menambah situs PHP atau statis (Website > Tambah situs). Situs itu muncul dengan jenis Apache dan tab filternya sendiri.

Batas: situs Apache memakai satu vhost per situs (`.htaccess` berlaku, daftar isi folder dimatikan). Pengaturan lanjutan per situs (aturan rewrite, alias) dari panel belum tersedia untuk jenis ini, dan WordPress tetap dilayani nginx. Perintah terminal: `lab app install httpd`, `lab service start httpd`, `lab site add <nama> <folder> --apache`.

Catatan 9Router: dashboard memakai kata sandi bawaan dari pembuatnya. 9Router menolak login jarak jauh sampai kata sandi itu diganti, jadi ganti terlebih dulu dari mesin itu sendiri di `http://127.0.0.1:20128` (dari komputer lain: `ssh -L 20128:127.0.0.1:20128 <mesin>`, lalu buka `http://localhost:20128`). Data akun disimpan di `~/.9router` dan tidak ikut backup panel.

## Cloudflare Tunnel (opsional)
`cloudflared` dipasang dari Homebrew lewat halaman Aplikasi. Ia tidak punya proses sendiri: akses publik dinyalakan per situs di tab **Publik**. Status "Berjalan" berarti ada tunnel aktif; **Berhenti** mematikan semua tunnel sekaligus. Menghapus aplikasi juga mematikan semua tunnel.

## Menghapus aplikasi opsional
Hapus memerlukan verifikasi ulang dan mengetik nama aplikasi. Data pengguna aplikasi tidak dihapus.

## Domain `.lab`
dnsmasq menjawab semua `*.lab` dengan alamat jaringan privat mesin ini, dan nginx menyajikan HTTPS memakai sertifikat lokal (mkcert). Perangkat klien perlu memercayai CA lokal tersebut dan memakai mesin ini sebagai DNS untuk domain `.lab` (di Tailscale: Split DNS).

# Pemantauan dan sistem

## Ringkasan dan Pantau
Panel mencatat satu sampel tiap 30 detik dan menyimpannya 7 hari. Grafik hanya berisi pengukuran nyata; bila satu nilai gagal dibaca, titiknya kosong (bukan 0).

- **CPU**: pemakaian total.
- **Memori**: App Memory + Wired + Compressed, sama dengan definisi Activity Monitor macOS (memori yang dipakai cache tidak dihitung terpakai).
- **Disk**: volume data.
- **Beban**: load average.
- **Bandwidth**: selisih byte per antarmuka jaringan; total periode adalah jumlah selisih, sehingga reboot tidak menurunkan angka.

## Perangkat keras
Kartu **Kesehatan mesin** di Ringkasan dan bagian **Perangkat keras** di Pantau menampilkan suhu CPU, GPU, dan SSD, kecepatan kipas, daya sistem, tekanan memori, kesehatan SSD, dan perkiraan disk penuh. Satu sampel tiap 30 detik, disimpan 7 hari.

- **Sumber.** `macmon` (Apple Silicon, tanpa sudo) adalah sumber utama untuk suhu CPU/GPU, kipas, daya, dan tekanan memori. Suhu SSD, memori, dan suasana hanya ada di Netdata, jadi dilengkapi dari sana bila Netdata terpasang. Bila macmon tidak ada, suhu diambil dari Netdata dan layar memberi catatan bahwa skalanya berbeda. Tanpa keduanya, layar menyebut "tidak ada sumber suhu" dan tidak menampilkan angka. Pasang macmon dengan `brew install macmon`.
- **Daya** hanya dibaca dari macmon. **Nilai yang mustahil** (suhu di luar 5 sampai 125 derajat, kipas negatif, dan sejenisnya) dibuang, bukan diganti angka lain.
- **Tekanan memori** lebih berarti daripada persen memori terpakai: tekanan tinggi berarti macOS terpaksa memampatkan atau menukar isi memori ke disk.
- **Kesehatan SSD** hanya status SMART (Sehat atau Bermasalah). Persen keausan tidak tersedia tanpa sudo, jadi tidak ditampilkan.
- **Perkiraan disk penuh** memakai tren linear dan baru muncul setelah ada data minimal 24 jam. Bila pemakaian naik-turun tidak beraturan, perkiraan diberi catatan "kasar" dan tidak memicu peringatan.
- **Data basi.** Bila sampel terakhir lebih dari 2 menit, layar menandainya basi dan tidak menampilkan angka lama sebagai kondisi sekarang.

## Konektivitas
Bagian **Konektivitas** di Pantau (dan dua baris di kartu Kesehatan mesin) mengukur satu kali per menit, disimpan 7 hari:

- **Tailscale**: status (Tersambung, Berhenti, Perlu login), apakah mesin ini online di tailnet, dan jumlah perangkat lain yang online. Berhenti atau tidak online berarti akses jarak jauh ke mesin ini terputus.
- **Internet**: waktu sambung TCP ke dua server publik (bukan ping, jadi tidak butuh izin khusus). Internet dianggap terputus hanya bila kedua server gagal.
- **DNS**: apakah nama lokal (domain panel) dan satu nama publik bisa di-resolve.
- Persen ketersediaan dihitung dari sampel yang ada, bukan dari waktu: bila panel sendiri mati, itu tidak dihitung sebagai internet turun. Pengukuran yang gagal dijalankan tidak pernah ditulis sebagai "mati".

## Notifikasi ponsel
Di **Pengaturan > Notifikasi ponsel**, peringatan penting dapat dikirim ke ponsel lewat [ntfy](https://ntfy.sh) (aplikasi gratis untuk Android dan iOS), termasuk pesan "pulih" saat masalah selesai.

1. Di Pengaturan, klik **Buat topik acak**, lalu **Simpan**. Salin topik yang tampil (hanya tampil sekali) dan berlangganan topik itu di aplikasi ntfy di ponsel.
2. Klik **Kirim pesan uji** untuk memastikan sampai di ponsel, lalu **Aktifkan**.
3. Pilih tingkat: hanya yang penting, atau penting dan perhatian.

Yang perlu diketahui:
- **Topik berfungsi seperti kata sandi.** Di ntfy.sh publik, siapa pun yang tahu topik dapat membaca pesannya. Panel menyimpannya terenkripsi dan tidak pernah menampilkannya lagi, mencatatnya di audit, atau mengirimnya lewat API.
- **Pesan melewati server ntfy** yang dipilih. Isinya hanya judul dan detail peringatan, tanpa kata sandi atau jalur file. Untuk kendali penuh, jalankan ntfy sendiri dan isi alamatnya (harus https, bukan alamat internal).
- **Penyalaan pertama diam**: peringatan yang sudah ada saat notifikasi pertama kali diaktifkan tidak dikirim, hanya yang baru. Pesan uji menjawab "apakah jalurnya berfungsi".
- **Tidak berisik**: sebuah peringatan harus terlihat dua menit berturut-turut sebelum dikirim, tidak dikirim ulang selama masih aktif (kecuali yang penting, diingatkan tiap 6 jam), dan paling banyak 12 pesan per jam.
- Hanya admin lewat sesi manusia dengan verifikasi kata sandi ulang yang dapat mengubahnya. Token API tidak pernah boleh.

## Mengatur ambang peringatan

Di **Pengaturan** ada kartu "Ambang peringatan suhu dan memori". Isinya batas "perhatian" dan "penting" untuk suhu CPU/GPU, suhu SSD, tekanan memori, serta batas swap. Angka bawaan hanyalah tebakan beralasan, jadi sebaiknya disesuaikan setelah Anda melihat grafik di Pantau beberapa hari: pasang batas sedikit di atas puncak normal mesin Anda supaya peringatan yang muncul berarti.

- Perubahan langsung dipakai peringatan di Ringkasan dan notifikasi ponsel, tanpa memulai ulang panel.
- Batas "perhatian" harus lebih rendah dari batas "penting". Angka di luar rentang masuk akal (misalnya suhu di bawah 40 °C yang akan membuat peringatan menyala terus) ditolak dengan pesan yang jelas.
- Koma desimal boleh dipakai (72,5).
- Tombol "Kembalikan ke bawaan" mengembalikan semuanya.
- Hanya admin yang dapat mengubah. Pengguna baca-saja dan token API hanya dapat melihat. Setiap perubahan tercatat di Aktivitas.

## Paparan layanan ke jaringan rumah

Kartu **Paparan layanan** di Pantau menjawab satu pertanyaan: adakah layanan di mesin ini yang bisa dijangkau perangkat lain di jaringan rumah padahal seharusnya hanya lokal? Layanan pengembangan (database, cache, penangkap email) sebaiknya hanya mendengar di `127.0.0.1`; akses jarak jauh lewat Tailscale.

- Setiap 5 menit panel mendaftar layanan yang mendengar di luar loopback, lalu **benar-benar mencoba menyambung dari IP LAN mesin ini**. Layanan dianggap terbuka hanya bila sambungan itu berhasil; yang mendengar di semua antarmuka tetapi diblokir firewall ditampilkan sebagai "Diblokir", bukan sebagai bahaya.
- Peringatan muncul bila layanan yang sama terlihat terbuka 2 kali berturut-turut (sekitar 10 menit), supaya layanan yang baru naik tidak langsung memicu alarm. Peringatan ikut dikirim ke notifikasi ponsel bila itu dinyalakan. Port yang menyimpan data (database, Redis, Memcached) dinilai "penting", yang lain "perhatian".
- Layanan bawaan macOS (AirPlay, Continuity, dan sejenisnya) ditandai "Sistem macOS" dan tidak memicu peringatan.
- **Daftar izin**: layanan yang memang sengaja terbuka (misalnya nginx, yang menjawab tetapi menolak perangkat di luar tailnet) dicatat sebagai `proses:port`. Tombol "Tandai disengaja" menambahkan, "Cabut" menghapus. Ini hanya mengubah peringatan, tidak menutup port; untuk menutup, ikat layanannya ke 127.0.0.1. Hanya admin yang dapat mengubah, dan setiap perubahan tercatat di Aktivitas.
- **Batas yang perlu diketahui**: nama proses hanya terlihat untuk layanan milik pengguna ini. Layanan milik sistem (misalnya SSH dan Screen Sharing) ditemukan dari sambungan ke port 1 sampai 1024 serta beberapa port penting, dan ditampilkan sebagai "Proses tidak terlihat" dengan tebakan nama dari nomor port. Port di atas 1024 milik sistem tidak ditemukan. Pemeriksaan ini mengirim sambungan singkat ke port-port itu setiap 5 menit, yang dapat tercatat di log layanan terkait.
- Dari terminal: `lab sys exposure` menampilkan hasil mentah (JSON).
- **Menutup port dari LAN**: tombol "Cara menutup" pada baris yang perlu perhatian menampilkan perintah `lab firewall block <port>` yang siap disalin. Panel sengaja tidak menjalankannya, karena mengubah firewall butuh hak root; Anda menjalankannya di terminal (perintahnya meminta password `sudo`).
- **"Dijaga pf"**: port yang dicakup aturan blokir di anchor `macminiops-*` ditandai ini. Itu bukti aturan *tertulis*, bukan *aktif*; uji sambung dari mesin sendiri lewat loopback dan tidak bisa membuktikannya. Pastikan dengan `lab firewall verify`.

Panel menandai disk hampir penuh, memori tinggi, layanan mati, situs yang menjawab error, sertifikat yang akan kedaluwarsa, dan backup yang sudah lama. Dari perangkat keras: suhu CPU atau GPU 85 (perhatian) dan 95 (penting), suhu SSD 70 dan 80, macOS membatasi kecepatan CPU, tekanan memori 80 dan 90 persen, swap di atas 2 GB, SMART bermasalah, disk diperkirakan penuh dalam 14 hari (perhatian) atau 3 hari (penting), serta pemantauan perangkat keras yang berhenti. Dari konektivitas: Tailscale berhenti atau tidak online selama 2 menit (penting), internet terputus 3 menit, internet lambat (rata-rata di atas 300 ms), DNS lokal gagal 2 menit, dan pemantauan konektivitas yang berhenti.

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

# Akun dan keamanan

## Pengaturan umum
Menu **Pengaturan** (grup Sistem) memuat:
- **Nama panel**: tampil di halaman masuk, menu samping, dan judul tab. Hanya admin yang dapat mengganti.
- **Tema**: Ikuti sistem, Terang, atau Gelap untuk perangkat yang dipakai (disimpan di browser). Admin juga dapat menetapkan tema bawaan untuk perangkat yang belum memilih.
- **Gaya**: empat tampilan siap pakai di Pengaturan > Tampilan: Kertas Teknis (bawaan, editorial), Bengkel (berani, garis tebal), Ruang Mesin (bernuansa instrumen), dan Kaca Dingin (modern, lembut). Setiap gaya punya versi terang dan gelap. Gaya hanya mengubah warna, huruf, dan bentuk; letak menu dan fungsi tidak berubah.
- **Warna aksen**: pilih dari delapan warna atau warna sendiri. Warna disesuaikan otomatis (lebih gelap atau lebih terang) agar teks tetap terbaca di gaya dan mode yang dipilih, jadi warna apa pun aman dipakai. Pilihan gaya, aksen, dan mode disimpan di browser perangkat itu saja.
- **Logo dan favicon**: pilih salah satu dari 17 ikon (mengikuti warna aksen) atau unggah gambar sendiri (PNG, JPG, atau WebP, maksimal 2 MB). Gambar dipotong persegi dan dikecilkan ke 128 px; SVG tidak diterima. Logo tampil di menu samping dan halaman masuk, dan ikon tab browser (favicon) ikut berubah. Disimpan di browser perangkat itu saja.
- **Kata sandi**, **perangkat tepercaya**, **2FA**, dan **Token API** (lihat di bawah).

Semuanya juga dapat dikendalikan dari Terminal mesin ini:
```
panel settings get
panel settings set name "Nama Panel"
panel settings set theme system|light|dark
```

## Login
- Kata sandi disimpan dengan argon2id. Percobaan salah berulang mengunci akun dan alamat IP sementara.
- Kode TOTP (Google Authenticator atau sejenisnya) diminta saat login. Setiap kode hanya dapat dipakai sekali.
- Saat pengaturan awal, kunci TOTP dan 10 recovery code ditampilkan **sekali**. Simpan di tempat aman (pengelola kata sandi). Satu recovery code hanya berlaku sekali.

## Menyalakan dan mematikan 2FA
Di **Pengaturan**, kartu *Verifikasi dua langkah*:
- **Mematikan** membutuhkan kata sandi dan kode 6 digit (atau satu recovery code) saat ini. Semua sesi lain dikeluarkan dan semua perangkat tepercaya dicabut. Setelah itu akun masuk hanya dengan nama pengguna dan kata sandi.
- **Menyalakan** meminta kata sandi ulang, membuat kunci TOTP baru, lalu meminta satu kode untuk memastikan aplikasi autentikator sudah benar. Setelah aktif, 10 recovery code baru tampil **sekali**; kode lama tidak berlaku.
- Mematikan 2FA menurunkan keamanan. Disarankan hanya bila akses panel sudah dibatasi jaringan privat dan Anda menerima risikonya.

Dari Terminal mesin ini:
```
panel user 2fa off <nama>
panel user 2fa on <nama>       # menampilkan kunci TOTP dan recovery code sekali
panel user passwd <nama>       # ganti kata sandi; semua sesi dicabut
```

## Perangkat tepercaya
Pada login lengkap, centang **Percayai perangkat ini**. Perangkat itu lalu cukup memakai nama pengguna dan kata sandi selama 30 hari.
- Setiap akun memiliki maksimum 10 perangkat tepercaya.
- Sesi baru dari perangkat tepercaya tetap meminta verifikasi ulang kata sandi untuk aksi berbahaya.
- Cabut satu atau semua perangkat di **Pengaturan**. Mengganti kata sandi atau menghapus akun mencabut semuanya.
- Risiko: orang yang memegang perangkat tepercaya sekaligus kata sandi dapat masuk tanpa kode 2FA sampai perangkat itu dicabut.

## Verifikasi ulang untuk aksi berbahaya
Aksi seperti menghapus situs atau database, melihat kredensial, mengubah konfigurasi, dan memasang aplikasi meminta **kata sandi ulang**. Setelah benar, aksi lain berlaku tanpa bertanya lagi selama 15 menit. Tebakan salah kena pembatasan yang sama dengan login. Aksi penghapusan juga meminta Anda mengetik ulang nama.

## Peran
- **Admin**: semua aksi.
- **Viewer** (hanya-baca): hanya melihat; tidak dapat mengubah apa pun dan tidak melihat kredensial. Viewer tetap dapat mengelola akunnya sendiri: kata sandi, 2FA, dan perangkat tepercaya.

## Mengelola pengguna
Menu **Pengguna** (khusus admin):
- **Tambah pengguna**: pilih nama dan peran. Panel membuat **kata sandi sementara** (tampil sekali). Berikan lewat jalur aman.
- Saat masuk pertama, akun baru hanya dapat **mengganti kata sandi** (dan menyiapkan 2FA); semua menu lain terkunci sampai kata sandi diganti. Akun baru belum memakai 2FA; pemiliknya menyalakannya sendiri di Pengaturan.
- **Kelola** akun: ubah peran, nonaktifkan atau aktifkan, reset kata sandi (sementara, wajib diganti), matikan 2FA (mis. ponsel hilang), dan hapus (ketik ulang nama).
- Mengubah peran, mereset, atau menonaktifkan mengeluarkan semua sesi dan mencabut token dan perangkat tepercaya akun itu.
- Penjagaan: tidak dapat dilakukan pada akun sendiri, dan harus selalu tersisa minimal satu admin aktif.
- Semua aksi butuh verifikasi ulang, tercatat di Aktivitas (tanpa kata sandi), dan tidak tersedia lewat token API.

## Mengelola akun (dari Terminal mesin ini)
```
panel user add [--role admin|viewer] <nama>
panel user list
panel user role <nama> admin|viewer
panel user rename <lama> <baru>
panel user disable|enable <nama>
panel user reset <nama>        # kata sandi sementara, tampil sekali
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

## Pencarian cepat

Kotak **Cari** di bilah atas membuka pencarian untuk halaman dan tab (mis. "tailscale", "bersihkan cache", "crontab"), situs, layanan, database, tugas, dan judul dokumen. Pintasan: `Ctrl K` (atau `Cmd K` di Mac) dari mana saja, atau `/` bila tidak sedang mengetik. Panah memilih, Enter membuka, Esc menutup. Data situs dan layanan dimuat saat pencarian dibuka dan disimpan sebentar; halaman khusus admin tidak muncul untuk peran hanya-baca.

## Bahasa

Panel tersedia dalam Bahasa Indonesia dan English. Pilih bahasa dari menu profil (pojok kanan atas), halaman Profil, atau layar login. Pilihan disimpan di akun Anda sehingga ikut ke perangkat lain; di layar login hanya diingat di browser ini.

Urutan penentuan: pilihan akun, lalu bahasa browser, lalu Indonesia. Seluruh antarmuka diterjemahkan. Yang masih berbahasa Indonesia: panduan ini, teks peringatan perangkat keras/jaringan/perkiraan disk, deskripsi katalog pembersihan disk, dan pesan galat dari server. Teks yang belum punya terjemahan tidak pernah kosong; panel memakai teks aslinya.

Menambah bahasa (untuk kontributor): salin `ui/src/i18n/locales/id.ts` menjadi `locales/<kode>.ts`, terjemahkan nilainya (jangan ubah kunci atau `{placeholder}`), lalu daftarkan satu baris di `LOCALES` pada `ui/src/i18n/index.ts`. Kunci berakhiran `.one` dipakai untuk bentuk tunggal bila bahasa membedakannya. Tes `i18n.test.ts` memastikan setiap bahasa memiliki kunci dan placeholder yang sama dengan bahasa dasar.

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

## Akses publik sementara
```
lab app install cloudflared        # pasang Cloudflare Tunnel (tanpa proses sendiri)
lab public on blog --ttl 120       # buka situs ke publik 120 menit (5-1440), mencetak alamat acak
lab public status [--json]         # tunnel aktif dan sisa waktunya
lab public off blog                # tutup satu situs (atau: lab public off --all)
```
Listener loopback per situs dibuat di `nginx/sites/public-NAMA.conf` dan dihapus otomatis saat tunnel berhenti. Situs sistem, proxy ke layanan internal, dan situs Apache ditolak.

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
Gunakan satu recovery code saat login. Bila tidak ada, di Terminal mesin ini jalankan `panel user 2fa off <nama>` lalu masuk dengan kata sandi dan nyalakan 2FA lagi di Pengaturan. Kata sandi dapat diganti dengan `panel user passwd <nama>`.

## Akun baru tidak bisa membuka menu
Akun dengan kata sandi sementara hanya dapat mengganti kata sandi dulu. Setelah diganti, semua menu terbuka. Bila kata sandi sementara hilang, admin dapat mereset dari menu Pengguna.

## Kata sandi root database diganti tetapi aplikasi gagal konek
Hanya klien yang memakai akun root yang terpengaruh. Perbarui koneksi di klien tersebut. Proyek memakai user sendiri dan tidak berubah.

## Disk penuh
Menu Sistem, tab Disk menunjukkan folder terbesar. Bersihkan backup lama dari halaman Backup, dan cache paket dengan `brew cleanup`.

## Mac tidak terjangkau setelah restart
Setelah restart pada disk terenkripsi, layanan penuh hanya naik setelah seseorang masuk di layar. Hindari restart saat jauh dari mesin, kecuali Anda yakin sambungan jaringan privat naik sebelum login.

## Agen atau skrip mendapat 401/403 dari API
Periksa token di `panelctl whoami`. 401: token salah, kedaluwarsa, atau dicabut (buat baru di Pengaturan). 403 "butuh token tingkat 'full'": aksi tersebut membutuhkan token `full`. 403 "tidak tersedia lewat token API": rute itu sengaja tertutup untuk token.

# API dan agen AI

Semua yang dapat dilakukan di panel juga tersedia lewat API, sehingga skrip atau agen AI dapat mengendalikan panel. Ada dua jalur: perintah `panelctl` dan HTTP langsung.

## Token API
Buat di **Pengaturan > Token API** (butuh kata sandi ulang), atau di Terminal mesin ini:
```
panel token create <pengguna> --name <nama> --scope read|operate|full [--days 90]
panel token list
panel token revoke <id>
```
Token tampil **sekali** dan hanya hash-nya yang disimpan. Berlaku 1 sampai 365 hari (bawaan 90), maksimal 10 aktif per akun, dapat dicabut kapan saja. Pemakaian terakhir (waktu dan IP) tercatat, dan setiap aksi ubah tercatat di Aktivitas sebagai `api_call` dengan nama token.

| Tingkat | Boleh |
|---|---|
| `read` | Hanya membaca (GET) |
| `operate` | Membaca, menambah, dan mengubah: situs, database, deploy, pengaturan. Tidak termasuk aksi yang butuh verifikasi ulang |
| `full` | Semua di atas, plus aksi yang butuh verifikasi ulang: hapus, ubah config, pasang aplikasi, jalankan backup. Penghapusan wajib menyertakan `confirm` berisi nama target |

Akun hanya-baca selalu mendapat tingkat `read`. **Tidak pernah tersedia lewat token**, apa pun tingkatnya: login dan 2FA, ganti kata sandi, perangkat tepercaya, manajemen token, kredensial database (`creds`), isi `.env` situs, manajemen pengguna panel (`/api/accounts`), kata sandi root database (`/api/dbroot`), serta membuat, mengubah, atau menghapus user database (token hanya boleh membaca daftarnya).

Perlakukan token seperti kata sandi: jangan ditempel ke chat atau repo, simpan di pengelola rahasia, dan cabut bila bocor.

## panelctl
```
panelctl whoami                     # siapa saya, tingkat token
panelctl routes [--filter /api/db]  # daftar endpoint yang tersedia
panelctl sites | dbs | services | backups | alerts | system
panelctl hw [--range 1h|24h|7d]     # suhu, kipas, daya, tekanan memori, SSD, perkiraan disk penuh
panelctl netmon [--range 1h|24h|7d] # Tailscale, internet, DNS
panelctl audit | jobs [--limit N] [--cursor C] [-q teks]   # riwayat berhalaman, terbaru dulu
panelctl site-add <nama> <folder|port> [--php 8.4]
panelctl site-action <nama> enable|disable
panelctl site-remove <nama> --confirm <nama>        # token full
panelctl db-create <nama> [--engine mariadb|mysql|pgsql] [--env <berkas .env>]
panelctl db-dump <nama> [--engine ...]
panelctl db-drop <nama> --confirm <nama> [--engine ...]   # token full
panelctl service <id> start|stop|restart
panelctl backup-run                                  # token full
panelctl settings | set --name "..." --theme dark
panelctl get /api/...       panelctl post /api/... '{"kunci":"nilai"}'
panelctl --wait ...         # tunggu aksi latar belakang selesai
```
Keluaran selalu JSON; kode keluar 0 bila sukses, 1 bila ditolak atau gagal. Konfigurasi:
- `PANEL_URL` (bawaan `https://panel.lab`)
- `PANEL_TOKEN`, atau simpan token di `~/.config/macpanel/token` dengan izin 0600 (izin lebih longgar ditolak)
- `PANEL_CA`: berkas CA lokal bila perangkat belum memercayainya (mkcert terdeteksi otomatis)

Dari komputer lain, perintah ini hanya jalan bila perangkat itu ada di jaringan privat yang sama dan dapat me-resolve `panel.lab`.

## HTTP langsung
Kirim header `Authorization: Bearer <token>` ke `https://panel.lab/api/...`. Tidak perlu cookie atau CSRF. `GET /api/routes` mengembalikan daftar endpoint beserta tingkat minimum. Aksi latar belakang (pasang, backup, deploy) menjawab `202` dengan `job`; pantau di `GET /api/jobs/<id>`.

Contoh:
```
curl --cacert rootCA.pem -H "Authorization: Bearer $PANEL_TOKEN" https://panel.lab/api/sites
```

## Respons galat
| Kode | Arti |
|---|---|
| 401 | Token tidak valid, kedaluwarsa, atau dicabut |
| 403 | Tingkat token kurang, atau rute memang tidak tersedia lewat token |
| 429 | Terlalu banyak percobaan dari satu alamat; tunggu beberapa menit |

# Docker

Docker bersifat opsional dan tidak memakai sumber daya sampai dipasang dan dijalankan. Panel memakai **Colima**: mesin virtual ringan tanpa aplikasi GUI, berjalan di latar belakang lewat launchd.

## Memasang dan menjalankan
1. Menu **Aplikasi**, cari **Docker (Colima)**, lalu **Pasang**. Panel memasang Colima, Docker CLI, dan plugin Compose lewat Homebrew (butuh internet).
2. Klik **Mulai**. Start pertama mengunduh image VM dan bisa beberapa menit.
3. Menu **Docker** menampilkan container, proyek Compose, dan image.

Mesin virtual dibatasi 2 CPU, 4 GB RAM, dan 40 GB disk. Dari komputer Anda, **hanya folder wwwroot** yang dibagikan ke VM, bukan seluruh folder rumah. Mencopot Docker menghapus VM beserta semua container, image, dan volume di dalamnya.

## Container
Tab **Container** menampilkan status, image, port, serta pemakaian CPU dan RAM. Anda dapat menjalankan, menghentikan, memulai ulang, melihat log, dan menghapus container (verifikasi ulang dan ketik ulang nama). Data di volume tidak ikut terhapus saat container dihapus.

## Proyek Compose
Taruh `compose.yaml` (atau `docker-compose.yml`) di folder situs, misalnya `wwwroot/<nama>`. Tab **Proyek Compose** menampilkannya; **Jalankan** menjalankan `docker compose up -d`, **Hentikan** menjalankan `down`.

Agar tetap aman, berkas diperiksa sebelum dijalankan dan **ditolak** bila memuat:
- `privileged`, `network_mode: host`, `pid: host`, `ipc: host`, `cap_add`, atau `devices`;
- `security_opt` dengan `unconfined`;
- mount di luar wwwroot atau yang memuat `docker.sock`;
- port yang tidak diikat ke `127.0.0.1`. Tulis `"127.0.0.1:8080:80"`, bukan `"8080:80"`.

Untuk membuka aplikasi container lewat HTTPS, buat situs **Proxy** ke port tersebut di menu Website.

## Image
Tab **Image**: tarik image (nama huruf kecil, mis. `nginx:1.27`), hapus image, dan bersihkan image yang tidak terpakai.

## Yang sengaja tidak ada
- Menjalankan `docker run` dengan argumen bebas, dan shell di dalam container. Akses ke Docker setara akses administrator di VM, jadi panel hanya menyediakan aksi yang sudah dibatasi di atas.
- Memasang, membuat, atau menghapus volume dan network dari panel.

## Pemecahan masalah
- **Docker belum berjalan**: mulai dari menu Aplikasi; lihat log `~/Library/Logs/colima.log`.
- **Compose ditolak kebijakan**: pesan menyebut layanan dan baris yang melanggar; ubah berkas sesuai daftar di atas.
- **Disk VM penuh**: tab Image, **Bersihkan tak terpakai**.

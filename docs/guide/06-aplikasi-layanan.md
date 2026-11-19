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

## Menghapus aplikasi opsional
Hapus memerlukan verifikasi ulang dan mengetik nama aplikasi. Data pengguna aplikasi tidak dihapus.

## Domain `.lab`
dnsmasq menjawab semua `*.lab` dengan alamat jaringan privat mesin ini, dan nginx menyajikan HTTPS memakai sertifikat lokal (mkcert). Perangkat klien perlu memercayai CA lokal tersebut dan memakai mesin ini sebagai DNS untuk domain `.lab` (di Tailscale: Split DNS).

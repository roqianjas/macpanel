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

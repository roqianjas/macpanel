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

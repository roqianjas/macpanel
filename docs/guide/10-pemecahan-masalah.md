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

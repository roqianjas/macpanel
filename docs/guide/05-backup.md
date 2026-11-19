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

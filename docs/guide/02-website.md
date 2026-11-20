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

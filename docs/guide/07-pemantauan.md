# Pemantauan dan sistem

## Ringkasan dan Pantau
Panel mencatat satu sampel tiap 30 detik dan menyimpannya 7 hari. Grafik hanya berisi pengukuran nyata; bila satu nilai gagal dibaca, titiknya kosong (bukan 0).

- **CPU**: pemakaian total.
- **Memori**: App Memory + Wired + Compressed, sama dengan definisi Activity Monitor macOS (memori yang dipakai cache tidak dihitung terpakai).
- **Disk**: volume data. Ukuran memakai basis 1000 (GB) seperti Pengaturan Sistem macOS; memori memakai basis 1024 seperti Tentang Mac Ini. Kartu juga menampilkan ruang "dapat dibersihkan" yang bisa dibebaskan macOS sendiri; Pengaturan Sistem menjumlahkannya ke angka "tersedia", sedangkan angka "bebas" di panel adalah ruang yang benar-benar kosong.
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

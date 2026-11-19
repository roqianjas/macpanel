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

Urutan penentuan: pilihan akun, lalu bahasa browser, lalu Indonesia. Bagian panel yang belum diterjemahkan tetap tampil dalam Indonesia (tidak ada teks kosong).

Menambah bahasa (untuk kontributor): salin `ui/src/i18n/locales/id.ts` menjadi `locales/<kode>.ts`, terjemahkan nilainya (jangan ubah kunci atau `{placeholder}`), lalu daftarkan satu baris di `LOCALES` pada `ui/src/i18n/index.ts`. Kunci berakhiran `.one` dipakai untuk bentuk tunggal bila bahasa membedakannya. Tes `i18n.test.ts` memastikan setiap bahasa memiliki kunci dan placeholder yang sama dengan bahasa dasar.

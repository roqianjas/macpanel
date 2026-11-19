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

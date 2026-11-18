# Arsitektur kode

Dokumen ini adalah peta untuk siapa pun yang mau menambah atau mengubah fitur. Keputusan di balik struktur ini ada di [ADR 0001](../adr/0001-struktur-repo.md).

## Lapisan dan aturan ketergantungan

```
cmd/panel            titik masuk: serve, user, token, settings, init-key        (lapisan 6)
internal/testenv     utilitas tes (aplikasi uji + fake `lab`)                   (lapisan 6)
internal/server      akar komposisi: tabel rute, NewApp, Handler                (lapisan 5)
internal/overview    agregator: peringatan dari sites + services + system       (lapisan 4)
internal/<domain>    handler dan logika satu domain                              (lapisan 3)
internal/core        App, middleware, sesi, runner `lab`, validator bersama      (lapisan 2)
internal/store       SATU-SATUNYA tempat SQL (SQLite): skema, migrasi, query    (lapisan 1)
internal/crypto      hash kata sandi, TOTP, enkripsi, token acak                (lapisan 0)
```

Aturan, dijaga otomatis oleh `internal/archtest` (gagal di `make check` bila dilanggar):

1. Lapisan bawah tidak mengimpor lapisan atas.
2. **Paket domain tidak saling mengimpor.** Kode yang dipakai dua domain dipindah ke `core`.
3. SQL (`database/sql`, driver SQLite) hanya di `internal/store`.
4. Semua perintah ke mesin lewat `core.App.RunLab` / `RunLabTimeout` / `Mutate` (argumen berupa list, tanpa shell). Tidak ada `exec.Command` di domain.

## Paket

| Paket | Isi |
|---|---|
| `core` | `App` (kontainer ketergantungan), `Route` + `Level` (Public/Pwd/Full/Any), middleware `Wrap` (sesi, CSRF, token API, step-up, viewer), `RunLab`, `Record` (audit + job), `StartAsync` (aksi latar belakang), validator bersama (nama situs, port terlarang, versi PHP), `Local[T]` (state milik domain) |
| `store` | `Store` dan semua query: pengguna, sesi, percobaan login, audit berantai, job, token API, perangkat tepercaya, 2FA, pengaturan, webhook, versi konfigurasi nginx, metrik |
| `crypto` | Argon2id, TOTP, recovery code, `Seal`/`Open` (AES-GCM) |
| `auth` | login, TOTP, logout, step-up, `/me`, audit, perangkat tepercaya, 2FA, token API |
| `accounts` | manajemen akun panel (admin) |
| `settings` | nama panel dan tema |
| `sites` | situs nginx: daftar, tambah/hapus, konfigurasi dan templat, parser kebijakan nginx, basic-auth dan IP, domain tambahan, `.env`, log dan trafik, pasang cepat dan kloning |
| `deploy` | langkah deploy per situs dan penerima webhook GitHub (listener terpisah) |
| `databases` | MariaDB/MySQL/PostgreSQL: buat, hapus, dump, restore, unggah, konfigurasi, user dan hak, kata sandi root |
| `redis` | penelusur kunci Redis |
| `backup` | backup terjadwal, daftar, ekstrak, pembersihan |
| `phpruntime` | versi PHP, ekstensi, ini pool FPM, Composer, runtime mise |
| `updates` | pembaruan paket Homebrew |
| `tasks` | worker (launchd) dan cron job |
| `jobs` | status job latar belakang |
| `services` | layanan dan aplikasi (daftar, mulai/hentikan, pasang/hapus) |
| `docker` | container, image, proyek Compose |
| `system` | pengumpul metrik, proses, disk, keamanan, DNS, `healthz` |
| `overview` | peringatan |
| `docs` | dokumentasi di dalam panel |
| `web` | berkas statis UI |

## Pola yang dipakai

- **Handler**: `func Nama(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session)`. Didaftarkan satu kali di `internal/server/server.go` (tabel `Routes`, satu-satunya daftar rute). Tes `TestEveryRoute` memastikan semua rute selain `Public` menolak permintaan tanpa sesi.
- **Aksi mutasi**: `a.Mutate(args, stdin)` lalu `a.Record(...)` (audit). Aksi lama: `a.StartAsync(...)` (job, hasil dipantau UI).
- **State milik domain**: jangan menambah field ke `core.App`. Pakai `core.Local[T](a, "kunci")` dengan struct privat di paket domain.
- **Respons**: `core.WriteJSON`, `core.WriteError`. Input: `core.DecodeJSON` (batas ukuran, field asing ditolak).
- **Nama**: handler diekspor tanpa awalan `h` (`auth.Login`). Struct JSON memakai tag, sehingga bentuk API tidak berubah saat nama Go berubah.

## Menambah fitur baru

1. Tentukan domainnya. Kalau belum ada, buat `internal/<domain>/` (lapisan 3; jangan mengimpor domain lain).
2. Tulis handler di paket itu; validasi input di sana; kerjakan lewat `core`.
3. Daftarkan rute di `internal/server/server.go` dengan `Level` yang tepat (aksi destruktif: `core.Full` + `RequireStepUp` + ketik ulang nama).
4. Tabel atau kolom baru: tambahkan di `internal/store` (skema + migrasi, tanpa mengabaikan galat).
5. Tes HTTP di `internal/server/<domain>_test.go` memakai `testenv` (fake `lab`); tes fungsi murni di paket domain itu sendiri.
6. Perintah baru di CLI: modul di `cli/`, lalu sebut di `docs/guide`. UI: halaman di `ui/src/features/<domain>`.
7. `make check` harus lulus. Perbarui CHANGELOG.

## Peta riwayat (nama lama ke domain)
Sebelum refactor, berkas Go bernama menurut urutan pengerjaan. Untuk membaca riwayat git lama: `ops.go`, `ops7`, `ops11`, `siteconf`, `nginxconf`, `apps.go` (situs) menjadi `sites`; `ops3`, `ops5` (unggah DB), `ops6`, `ops8` (user DB), `ops20` (user DB), `ops10` (kata sandi saya diambil `auth`; Redis ke `redis`) menjadi `databases`; `ops12` dan bagian `ops4`/`ops8` menjadi `backup`; `ops2`/`ops4` menjadi `phpruntime`, `tasks`, `updates`; `ops13` menjadi `deploy`; `ops15`, `ops19` menjadi `auth`, `settings`, `store`; `ops20` (akun) menjadi `accounts`; `ops22` menjadi `docker`; `ops9`, `metrics` menjadi `system`; `ops16` menjadi `docs`. Tag git `pre-phase2` menandai keadaan sebelum refactor.

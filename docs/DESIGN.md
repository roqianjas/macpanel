# Panel Mac mini: desain (v0, belum ada kode)

Nama kerja: **panel** (nama final belum diputuskan). Dokumen ini harus disetujui sebelum kode ditulis.

## 1. Tujuan dan non-tujuan
**Tujuan:** antarmuka web di atas perintah `lab` (site, php, runtime, db, worker, job, update) supaya environment lokal bisa dikelola seperti aaPanel/cPanel.

**Bukan tujuan (fase 1):** Files, Terminal, FTP, mail, WAF, multi-user, akses dari internet publik, aksi yang butuh `sudo`.

## 2. Arsitektur
```
Browser -> nginx panel.lab (TLS mkcert, tailnet_only) -> panel (Go, 127.0.0.1:9100)
                                                           |- SQLite: user, sesi, audit, job
                                                           '- eksekutor -> `lab ...` (argv list, tanpa shell)
```
- Satu binary Go, LaunchAgent `com.macminiops.panel`, user `rokianjas` (bukan root).
- SQLite lewat `modernc.org/sqlite` (tanpa cgo). UI = React yang sudah ada (`portal-ui`) dibangun jadi file statis yang disajikan binary yang sama.
- `home.lab` (portal read-only) tetap hidup sampai panel setara (fase 4), baru digantikan.

## 3. Model ancaman
Aset: akun `rokianjas` di Mac mini (kode, SSH key, database, backup). Panel yang bisa menjalankan perintah = **bila dibobol, setara pengambilalihan akun itu.** Ini risiko inti yang tidak bisa dihilangkan, hanya dipersempit.

| Ancaman | Mitigasi |
|---|---|
| Orang di LAN/Wi-Fi menjangkau panel | Hanya `127.0.0.1`; nginx `tailnet_only`; LAN harus 403 (diuji) |
| Tebak password / credential stuffing | argon2id, rate-limit dan lockout **di server**, TOTP wajib |
| Bypass login (kelas CVE 1Panel/Webmin) | Tidak ada parameter klien yang memengaruhi keputusan auth; semua endpoint (kecuali login) lewat satu middleware; tes otomatis "tanpa sesi = 401" untuk **setiap** rute |
| Pencurian sesi / CSRF | Cookie `__Host-`, `HttpOnly`, `Secure`, `SameSite=Strict`; token CSRF per sesi di header; cek `Origin`/`Host` |
| Injeksi perintah/argumen | Aksi = allowlist tetap; parameter diketik dan divalidasi regex (sama dengan `lab`); `exec` dengan argv list; argumen berawalan `-` ditolak |
| Path traversal | Folder harus di bawah `~/Developer` setelah `realpath` (symlink diikuti lalu diperiksa) |
| Aksi destruktif tak sengaja | Konfirmasi ketik nama + **TOTP ulang** (step-up, berlaku 5 menit) untuk drop db, hapus situs, remove php, update |
| Kebocoran rahasia | Password db tidak pernah dikirim ke browser atau log; output perintah disaring (`scrub`); tidak ada tombol "tampilkan password" |
| Pelaku menghapus jejak | Audit append-only dengan hash berantai, tidak ada endpoint hapus/ubah |
| Panel sendiri diserang dari dalam (XSS) | CSP ketat tanpa inline script, React tanpa `dangerouslySetInnerHTML`, header `no-store` untuk API |
| Dependency berbahaya | Dependensi Go minimal (sqlite, argon2 dari `x/crypto`, TOTP ditulis sendiri ±30 baris), `govulncheck` di tes |

**Risiko sisa (jujur):** pelaku yang sudah menguasai akun `rokianjas` bisa mengubah database panel dan audit. Hash berantai hanya mendeteksi, tidak mencegah. Pelaku di tailnet yang memegang password **dan** TOTP bisa menjalankan semua aksi allowlist.

## 4. Autentikasi
- **Tidak ada halaman setup web.** Admin dibuat lewat CLI lokal: `panel user add <nama>` (password dimasukkan interaktif, tidak lewat argumen/chat). Nginx UI sempat punya endpoint install publik; kita tidak mengulanginya.
- Password: argon2id (m=64MB, t=3, p=2), minimal 12 karakter.
- TOTP: RFC 6238 (SHA-1, 6 digit, 30 dtk, toleransi ±1 langkah), **langkah yang sudah dipakai ditolak** (anti-replay). Secret dienkripsi dengan kunci dari Keychain macOS. Enrol: QR + 10 recovery code (disimpan sebagai hash, sekali pakai).
- Sesi: token acak 256-bit, disimpan sebagai hash; idle 30 menit, maksimum 12 jam; logout menghapus di server.
- Lockout: 5 gagal per (akun) dan 10 per IP dalam 15 menit -> jeda eksponensial; pesan galat sama untuk "akun tidak ada" dan "password salah"; waktu respons disamakan.
- IP klien hanya dipercaya dari `X-Forwarded-For` bila koneksi berasal dari 127.0.0.1 (nginx).

## 5. Eksekutor (satu-satunya pintu ke sistem)
Setiap aksi: nama tetap -> argv `lab ...` -> validator parameter. Tidak ada "jalankan perintah bebas".
- Timeout per aksi, batas ukuran output (256 KB), satu aksi mutasi per sumber daya pada satu waktu.
- Hasil berupa **job** (id, status, log tersaring) yang bisa dipantau dari UI.
- Env dibersihkan (PATH tetap); `sudo` tidak pernah dipanggil. Aksi yang butuh root ditampilkan sebagai perintah untuk dijalankan manual.

## 6. Endpoint fase 1
Semua `POST/DELETE` butuh CSRF; mutasi destruktif butuh step-up.

| Rute | Aksi | Lab |
|---|---|---|
| `POST /api/auth/login`, `/totp`, `/logout` | autentikasi | - |
| `GET /api/me`, `/api/audit` | profil, audit (baca saja) | - |
| `GET /api/overview` | data portal yang ada | (dari `build.py`) |
| `GET/POST/DELETE /api/sites` | daftar, tambah, hapus situs | `lab site` |
| `GET/POST/DELETE /api/db` | daftar, buat, drop (step-up) | `lab db` |
| `GET/POST /api/php`, `/api/runtime` | versi terpasang, pasang | `lab php`, `lab runtime` |
| `GET/POST/DELETE /api/workers`, `/api/jobs` | queue dan cron | `lab worker`, `lab job` |
| `GET /api/updates`, `POST /api/updates/apply` (step-up) | cek dan update | `lab update` |
| `GET /api/jobs/{id}` | status/log aksi | - |

## 7. Tes keamanan (syarat "selesai")
1. Tiap rute tanpa sesi -> 401; sesi tanpa TOTP lengkap -> 401 (tabel rute diuji otomatis, rute baru tanpa tes = gagal).
2. Kode TOTP lama dipakai ulang -> ditolak; lockout terpicu dan tidak bisa dilewati lewat header palsu.
3. CSRF: tanpa token / `Origin` asing -> 403.
4. Injeksi: `; touch x`, `$(...)`, `--flag`, nama berawalan `-`, null byte, unicode -> semua ditolak dan tidak ada file tercipta.
5. Traversal: `../`, symlink keluar `~/Developer` -> ditolak.
6. Dari IP LAN -> 403; port 9100 tidak terbuka selain loopback.
7. Pemindaian rahasia pada log/job/audit: tidak ada password.
8. `govulncheck` dan `go vet` bersih.

## 8. Fase
| Fase | Isi | Selesai bila |
|---|---|---|
| P0 | Desain ini disetujui | Anda setuju |
| P1 | Kerangka Go, SQLite, auth + TOTP + sesi + audit, CLI `panel user add`, tes 1-3 | tes keamanan auth lulus |
| P2 | Eksekutor + modul Website dan Database + tes 4-7 | uji nyata di mesin |
| P3 | PHP/Runtime, Worker/Cron, Update | semua `lab` terbungkus |
| P4 | UI React penuh, LaunchAgent, `panel.lab`, gantikan `home.lab` | dipakai harian |
| P5 | Dokumen, runbook, backup database panel | terdokumentasi |

## 9. Keputusan yang masih terbuka
1. Nama final (sementara `panel`; calon: Anjungan).
2. Port 9100 dan host `panel.lab` (disarankan).
3. Kunci enkripsi secret TOTP di Keychain macOS (disarankan) atau berkas 0600.

## 10. Keputusan (2026-11-16) dan modul Website
- Disetujui. Nama: **panel**. Kunci enkripsi secret TOTP di **Keychain macOS**.
- 2FA = TOTP standar RFC 6238, kompatibel dengan Google Authenticator, Authy, 1Password, Aegis.
- **Website (P2)**: daftar situs; tab Umum (domain, folder, PHP, mode), Rewrite (preset Laravel/WordPress/SPA/kustom), SSL (mkcert per situs, paksa HTTPS, masa berlaku), Config (editor mentah, mode lanjutan), Log (baca saja).
- Alur simpan: backup -> tulis -> `nginx -t` -> reload; gagal = rollback otomatis; riwayat versi dan pulihkan.
- Editor mentah: `root`/`alias` wajib di bawah `~/Developer`; tolak `include`, `load_module`, `perl_*`, `js_*`, `lua_*`; path log/sertifikat dibatasi; simpan butuh step-up TOTP; daftar tolak tidak dijamin lengkap, jadi pengaturan normal lewat form.
- Let's Encrypt tidak berlaku untuk `.lab`; hanya relevan bila kelak ada domain publik (Cloudflare Tunnel).

## 11. Status P1 (2026-11-16): auth, TOTP, sesi, audit, tes
Kode: `crypto.go`, `store.go`, `server.go`, `main.go`, `panel_test.go` (Go 1.27, SQLite `modernc.org/sqlite`, `x/crypto`, `x/term`).
CLI: `panel init-key | user add <nama> [--password-stdin] | user list | audit verify | serve`.
- Teruji: 22 tes unit (`go test -race` lulus), 5 uji mutasi (rute dijadikan publik, penjaga replay atomik, CSRF, lockout, XFF palsu: semuanya membuat tes GAGAL, lalu kode dipulihkan), dan 17 skenario ujung-ke-ujung pada binary + Keychain + server sungguhan (item uji dihapus). Vektor RFC 6238 lulus. Port hanya di loopback. Secret TOTP tidak ada polos di file database. Rantai audit utuh dan terdeteksi bila diubah.
- **Penyimpangan dari desain:** (1) lockout memakai jendela tetap 15 menit (5 gagal per akun, 10 per IP), **bukan** eksponensial; (2) enrol TOTP lewat setup key teks/URI, **belum QR**; (3) item Keychain dibuat dengan `security -w <nilai>`, sehingga kunci terlihat di argumen proses sesaat saat `init-key` (sekali, mesin satu pengguna); (4) kode TOTP yang dipakai login tidak bisa langsung dipakai step-up, tunggu langkah 30 detik berikutnya.
- **Belum teruji:** `govulncheck` (gagal dipasang: jaringan ke sum.golang.org), `serve` di bawah launchd membaca Keychain (sesi non-interaktif), akses lewat nginx `panel.lab`, dan semua rute selain auth (belum ada).

## 12. Status P2a (2026-11-16): eksekutor + Website + Database (API)
Kode: `executor.go` (runLab, validator, scrub), `ops.go` (handler), `ops_test.go`, `real_test.go`.
- **Rute baru** (semua Full + CSRF): `GET/POST /api/sites`, `POST /api/sites/{name}/remove` (step-up + ketik nama), `GET/POST /api/db`, `POST /api/db/{engine}/{name}/dump`, `.../drop` (step-up + ketik nama), `GET /api/jobs`.
- **Eksekutor:** biner `/opt/homebrew/bin/lab` (path tetap), argv list tanpa shell, env dibersihkan (hanya PATH/HOME/LANG), timeout 120 dtk dengan pembunuhan grup proses, output maks 256 KB dan disaring (`scrub`), satu aksi mutasi pada satu waktu, tiap aksi masuk tabel `jobs` dan audit.
- **Validasi:** nama situs/db ketat; folder harus di bawah `~/Developer` setelah `EvalSymlinks`; port 1024-65535 minus 22/3306/5432/6379/9100; PHP 8.2-8.5; file env harus `.env`/`.env.*`. Situs sistem (`home, nginx, db, kuma, stats, hello, panel`) tidak bisa dihapus dari panel.
- **Teruji:** 22 tes lama + 13 tes P2a (`-race`); 10 mutasi (step-up, situs sistem, konfirmasi, batas ~/Developer, symlink, env, scrub, regex nama, port) semuanya membuat tes GAGAL; pembacaan nyata (lab + config nginx asli); satu siklus mutasi nyata buat/hapus database `paneltest` lewat panel -> lab -> MySQL dengan step-up (hasil bersih, dump uji dihapus).
- **Belum:** `site remove` nyata lewat panel (hanya argv-nya yang diuji dengan lab palsu); tidak ada UI; PHP/Runtime/Worker/Cron/Update belum dibungkus (P3); editor nginx, rewrite, SSL (P2b); aksi panjang (update, install) masih sinkron dengan timeout 120 dtk, belum job asinkron.
- Catatan: password database tidak pernah ada di respons API; hanya jalur file kredensial.

## 13. Status P2b (2026-11-16): pengaturan situs, rewrite, SSL, editor config berpagar, log
Kode: `nginxconf.go` (parser + kebijakan), `siteconf.go` (model, template, pipeline, handler), `siteconf_test.go`, `real_test.go`. Perintah `lab` baru: `lab nginx test|reload`, `lab site cert <nama>`.
- **Rute baru** (Full + CSRF): `GET/POST /api/sites/{name}/settings`, `GET/POST /api/sites/{name}/config` (step-up), `POST .../restore` (step-up), `GET .../ssl`, `POST .../ssl/renew` (step-up), `GET .../logs?kind=access|error&n=`.
- **Tiga mode config:** `template` (dibuat panel dari model, ada header `# panel-model:`), `legacy` (buatan addsite.sh, disimpan pertama kali = versi lama dicadangkan), `custom` (diedit manual; menimpa dengan template butuh `replace=true`), `kustom` (tak dikenali; hanya editor).
- **Pengaturan terstruktur:** mode (php/proxy/statis), target, versi PHP, preset (laravel, wordpress, static, spa), aturan rewrite (from/to/flag dengan karakter dibatasi, maks 50), batas upload, timeout, paksa HTTPS, gzip. Config dihasilkan dari model, lalu **diperiksa kebijakan yang sama** dengan editor mentah (sabuk pengaman).
- **Kebijakan editor mentah (fail-closed):** parser ketat meniru token nginx (komentar hanya di awal token, backslash, kutip; `${` ditolak karena nginx menafsirkannya sebagai variabel). Hanya direktif dalam allowlist; setiap blok server WAJIB `include tailnet_only.conf`; `allow/deny/satisfy`, `alias`, `if`, `set`, `map`, `upstream`, `error_log`, `load_module`, modul perl/lua/js dilarang; `listen` hanya 80/443; `server_name` hanya host situs; sertifikat hanya milik situs; `root` di bawah ~/Developer (symlink diikuti); `proxy_pass` hanya `127.0.0.1:PORT` (bukan 22/3306/5432/6379/9100); `fastcgi_pass` hanya socket php-fpm-dev; `PHP_VALUE`/`PHP_ADMIN_VALUE` dan `SCRIPT_FILENAME` bebas dilarang; `include` hanya 3 snippet tetap; `access_log` hanya log situs.
- **Pipeline simpan:** cek kebijakan -> (step-up) -> backup versi lama (maks 20/situs, tanpa duplikat) -> tulis atomik (`.nama.conf.tmp`, tidak ikut include) -> `nginx -t` -> reload; gagal di tahap mana pun = config lama dipulihkan (dan reload diulang). Optimistic lock lewat `sha`.
- **Situs sistem** (`home, nginx, db, kuma, stats, hello, panel`): hanya bisa dibaca.
- **Teruji:** 55+ tes (`-race`); 30 uji mutasi pada kebijakan, pipeline, dan step-up (semuanya membuat tes GAGAL; dua celah tes ditemukan dan ditutup: penjaga `${` dan lolosnya mutasi yang gagal build); **uji nyata PANEL_REAL=3** pada nginx, lab, dan mkcert asli dengan situs sementara: buat, SPA fallback, rewrite 301, `.env` 404, LAN 403, edit yang ditolak `nginx -t` -> rollback identik byte-per-byte dan situs tetap melayani, 'allow all' ditolak, edit sah, pulihkan versi, info SSL, perbarui sertifikat, log akses, hapus. Tidak ada sisa.
- **Temuan:** `nginx -s reload` kembali sebelum worker baru aktif (config baru efektif setelah ~180 ms); UI nanti jangan langsung memeriksa hasilnya tanpa jeda singkat.
- **Risiko sisa (jujur):** (1) parser saya dan nginx bisa berbeda tafsir pada kasus langka; `nginx -t` hanya memeriksa sintaks, bukan kebijakan; (2) `root` di bawah ~/Developer bisa menyajikan folder proyek lain (dotfile dilindungi template, tidak dijamin di editor mentah); (3) pelaku dengan sesi penuh + TOTP bisa membuat situs yang menyajikan folder apa pun di ~/Developer; (4) log akses per situs baru aktif setelah pengaturan disimpan lewat panel; (5) `govulncheck` belum jalan.

## 14. Status P3 dan P4 (2026-11-16): PHP/runtime/worker/cron/update + UI + layanan
- P3: rute `/api/php`, `/api/runtime`, `/api/tasks/{worker|job}`, `/api/updates`, `/api/jobs/{id}`. Aksi lama (install, update) berjalan asinkron, satu mutasi per waktu, job yang masih "berjalan" saat panel start ditandai "terputus". Perintah worker/cron: hanya php/node/python/npm/pnpm/composer, tanpa kutip/pipa/`$`, flag kode-sebaris dan debug dilarang, path absolut harus di bawah `~/Developer`.
- Ditemukan dan diperbaiki saat uji: (1) batas body 8 KB membuat config nginx > 8 KB gagal 413; (2) database sistem hanya disembunyikan di UI padahal server menerima drop `dev` (lab sendiri menolak); kini ditolak di panel; (3) `WriteTimeout` 30 dtk lebih pendek dari aksi sinkron 120 dtk, jadi 150 dtk.
- P4: UI di `ui/` (React, build statis, disajikan Go). Header keamanan di semua respons; aset hash di-cache lama, HTML `no-store`. LaunchAgent + `panel.lab` aktif.
- Disengaja tidak dikerjakan: mengganti `home.lab` (panel belum punya grafik, status, log, dokumen), QR enrol TOTP, govulncheck (jaringan).
- Belum teruji: reboot penuh + Keychain di bawah launchd, upgrade paket nyata lewat panel, cron `--at`, browser selain Chromium, pembaca layar.

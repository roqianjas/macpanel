# ADR 0001: Struktur repo dan arsitektur target

Status: diterima. Tanggal: 2026-11-18.

## Konteks
Repo tumbuh cepat dengan satu paket Go datar (`package main`, 58 berkas), berkas bernama menurut urutan pengerjaan (`ops2.go` ... `ops22.go`), CLI Python tanpa ekstensi dan tanpa tes, serta UI dengan halaman datar dan tipe API yang ditulis tangan. Gerbang kualitas hanya ada di kepala pengelola. Untuk jangka panjang dan kerja tim, struktur harus mengikuti **domain**, bukan urutan sejarah.

## Keputusan
1. **Go**: paket per domain di `internal/`, titik masuk tipis di `cmd/panel`. Tidak ada kode bisnis di `main`. Satu-satunya tempat SQL adalah paket `store`. Satu-satunya pintu ke perintah `lab` adalah paket `runner`.
2. **CLI**: paket Python `lab/` berisi `core/` (fungsi bersama) dan satu modul per domain di `cmd/`. Berkas entri tipis. Tes `pytest` di `cli/tests`.
3. **UI**: `app/` (shell, routing), `shared/` (komponen, API client, util), `features/<domain>/` (halaman, komponen, tipe). Tipe API dipusatkan.
4. **Gerbang**: `make check` (format, vet, golangci-lint, tes Go dengan `-race`, tsc, vitest, ruff, pytest, pemeriksaan rahasia). CI GitHub menjalankan gerbang yang sama. Pre-commit menjalankan pemeriksaan cepat.
5. **Dokumen**: sumber panduan pengguna ada di repo (`docs/guide`), arsitektur di `docs/architecture`, keputusan di `docs/adr`.
6. **Migrasi bertahap**, satu fase satu commit, setiap fase lulus gerbang penuh sebelum lanjut. Perilaku tidak berubah selama refactor; daftar tes baseline dibandingkan sebelum dan sesudah.

## Konsekuensi
- Biaya awal tinggi (memindahkan dan mengganti nama seluruh kode), tetapi riwayat git tetap terlacak lewat `git mv`.
- Simbol yang tadinya privat satu paket harus diekspor di batas domain. Batas itu sengaja dibuat sempit.
- Aturan ketergantungan: `domain/*` boleh memakai `core`, `store`, `runner`, `httpx`, `auth`; domain tidak saling mengimpor. `server` (akar komposisi) satu-satunya yang mengimpor semua domain.

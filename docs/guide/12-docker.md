# Docker

Docker bersifat opsional dan tidak memakai sumber daya sampai dipasang dan dijalankan. Panel memakai **Colima**: mesin virtual ringan tanpa aplikasi GUI, berjalan di latar belakang lewat launchd.

## Memasang dan menjalankan
1. Menu **Aplikasi**, cari **Docker (Colima)**, lalu **Pasang**. Panel memasang Colima, Docker CLI, dan plugin Compose lewat Homebrew (butuh internet).
2. Klik **Mulai**. Start pertama mengunduh image VM dan bisa beberapa menit.
3. Menu **Docker** menampilkan container, proyek Compose, dan image.

Mesin virtual dibatasi 2 CPU, 4 GB RAM, dan 40 GB disk. Dari komputer Anda, **hanya folder wwwroot** yang dibagikan ke VM, bukan seluruh folder rumah. Mencopot Docker menghapus VM beserta semua container, image, dan volume di dalamnya.

## Container
Tab **Container** menampilkan status, image, port, serta pemakaian CPU dan RAM. Anda dapat menjalankan, menghentikan, memulai ulang, melihat log, dan menghapus container (verifikasi ulang dan ketik ulang nama). Data di volume tidak ikut terhapus saat container dihapus.

## Proyek Compose
Taruh `compose.yaml` (atau `docker-compose.yml`) di folder situs, misalnya `wwwroot/<nama>`. Tab **Proyek Compose** menampilkannya; **Jalankan** menjalankan `docker compose up -d`, **Hentikan** menjalankan `down`.

Agar tetap aman, berkas diperiksa sebelum dijalankan dan **ditolak** bila memuat:
- `privileged`, `network_mode: host`, `pid: host`, `ipc: host`, `cap_add`, atau `devices`;
- `security_opt` dengan `unconfined`;
- mount di luar wwwroot atau yang memuat `docker.sock`;
- port yang tidak diikat ke `127.0.0.1`. Tulis `"127.0.0.1:8080:80"`, bukan `"8080:80"`.

Untuk membuka aplikasi container lewat HTTPS, buat situs **Proxy** ke port tersebut di menu Website.

## Image
Tab **Image**: tarik image (nama huruf kecil, mis. `nginx:1.27`), hapus image, dan bersihkan image yang tidak terpakai.

## Yang sengaja tidak ada
- Menjalankan `docker run` dengan argumen bebas, dan shell di dalam container. Akses ke Docker setara akses administrator di VM, jadi panel hanya menyediakan aksi yang sudah dibatasi di atas.
- Memasang, membuat, atau menghapus volume dan network dari panel.

## Pemecahan masalah
- **Docker belum berjalan**: mulai dari menu Aplikasi; lihat log `~/Library/Logs/colima.log`.
- **Compose ditolak kebijakan**: pesan menyebut layanan dan baris yang melanggar; ubah berkas sesuai daftar di atas.
- **Disk VM penuh**: tab Image, **Bersihkan tak terpakai**.

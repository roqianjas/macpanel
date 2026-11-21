# PHP dan runtime

## PHP
Beberapa versi PHP (8.2 sampai 8.5) dapat dipasang berdampingan. Setiap situs memilih versinya sendiri, dan tiap versi memiliki pool PHP-FPM sendiri.

### Versi PHP untuk web dan terminal
- **Web**: ditentukan per situs (soket FPM di konfigurasinya). Ringkasannya tampil di kartu **Versi PHP default**.
- **Terminal (CLI)**: perintah `php` memilih versi lewat shim, dengan urutan: berkas `.php-version` di folder itu atau di atasnya, lalu situs MacPanel yang foldernya mencakup folder itu (jadi sama dengan web), lalu **default global** yang diatur di kartu tadi.
- Beberapa versi tetap terpasang berdampingan; hanya perintah `php` yang memilih satu. Perintah `php8.2`, `php8.3`, `php8.4`, `php8.5` selalu memakai versi itu.
- Aktifkan sekali di Terminal: `lab php shims` (menambah satu blok PATH ke `~/.zshrc` dengan cadangan; batalkan dengan `lab php shims --remove`). Lalu buka terminal baru. Cek dengan `lab php which`.
- Hanya terminal interaktif zsh yang memakai shim. Proses latar (cron, worker) memakai versi yang Anda pilih eksplisit, dan skrip non-interaktif memakai `php` bawaan Homebrew.

Menu **PHP dan runtime** menyediakan:
- Pasang atau hapus versi PHP.
- **Ekstensi**: saklar untuk ekstensi PECL (redis, apcu, xdebug, imagick, dan lain-lain). Ekstensi bawaan PHP Homebrew sudah terkompilasi dan tidak dapat dimatikan.
- **php.ini**: ubah nilai umum seperti `memory_limit`, `upload_max_filesize`, `max_execution_time`.
- **php.ini CLI**: `lab ini` hanya mengubah pool FPM. Untuk batas CLI (mis. `memory_limit` bagi test suite besar), buat file `.ini` di `/opt/homebrew/etc/php/<versi>/conf.d/` (mis. `zz-lab-memory.ini`). Nilai `php_admin_value` pada pool FPM tetap mengalahkannya.
- **Pool FPM**: mode proses dan `pm.max_children` dengan validasi konsistensi.
- **Composer**: lihat dan perbarui.

## Runtime
Node, pnpm, Go, dan Python dikelola oleh `mise`. Menu ini memperlihatkan versi yang terpasang dan memungkinkan memasang versi lain atau mengubah versi bawaan. Pemasangan berjalan di latar belakang; progresnya tampil di pojok kanan bawah.

## Worker dan cron
Menu **Worker dan cron** adalah pengganti Supervisor dan crontab, berjalan di atas launchd.
- **Worker**: proses yang selalu hidup dan dihidupkan ulang otomatis (queue Laravel, bot, dan sejenisnya). Tentukan folder, perintah, versi PHP, dan jumlah proses.
- **Cron**: tugas terjadwal, per interval (misalnya tiap 1 menit) atau pada jam tertentu.
- Log tiap worker dan cron dapat dilihat dari halaman yang sama.

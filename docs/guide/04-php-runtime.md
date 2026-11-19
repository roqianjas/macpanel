# PHP dan runtime

## PHP
Beberapa versi PHP (8.2 sampai 8.5) dapat dipasang berdampingan. Setiap situs memilih versinya sendiri, dan tiap versi memiliki pool PHP-FPM sendiri.

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

"""write_pool: `lab php install` tidak boleh menimpa penyetelan pengguna di pool dev yang sudah ada."""

from labcli import php


def test_new_pool_is_written_from_template(tmp_path):
    p = tmp_path / "dev.conf"
    assert php.write_pool(p, "8.4") is True
    txt = p.read_text()
    assert "[dev]" in txt and "listen = /opt/homebrew/var/run/php-fpm-dev-8.4.sock" in txt and "listen.mode = 0600" in txt


def test_existing_valid_pool_with_user_tuning_is_left_untouched(tmp_path):
    p = tmp_path / "dev.conf"
    tuned = php.POOL.format(v="8.4", user="u") + "php_admin_value[disable_functions] = \nphp_admin_value[memory_limit] = 2G\n"
    p.write_text(tuned)
    assert php.write_pool(p, "8.4") is False
    assert p.read_text() == tuned, "penyetelan pengguna harus bertahan"
    assert list(tmp_path.iterdir()) == [p], "tanpa berkas cadangan bila tidak ada yang diubah"


def test_pool_for_wrong_socket_is_backed_up_then_rewritten(tmp_path):
    p = tmp_path / "dev.conf"
    p.write_text("[dev]\nlisten = 127.0.0.1:9000\n")  # TCP: tidak aman, harus diganti
    assert php.write_pool(p, "8.5") is True
    assert "php-fpm-dev-8.5.sock" in p.read_text() and "9000" not in p.read_text()
    baks = [x for x in tmp_path.iterdir() if ".bak." in x.name]
    assert len(baks) == 1 and "9000" in baks[0].read_text(), "isi lama harus tersimpan di cadangan"


def test_pool_for_other_version_is_not_reused(tmp_path):
    p = tmp_path / "dev.conf"
    p.write_text(php.POOL.format(v="8.3", user="u"))
    assert php.write_pool(p, "8.5") is True, "pool milik versi lain bukan pool 8.5"
    assert "php-fpm-dev-8.5.sock" in p.read_text()


def test_garbage_file_is_replaced(tmp_path):
    p = tmp_path / "dev.conf"
    p.write_text("")
    assert php.write_pool(p, "8.4") is True and "[dev]" in p.read_text()

package main

import "testing"

func TestMariaDBEngine(t *testing.T) {
	got := parseDBList("MariaDB:\n  shop\nMySQL:\n  (kosong)\nPostgreSQL:\n  dev\n  blog\n")
	if len(got["mariadb"]) != 1 || got["mariadb"][0] != "shop" {
		t.Fatalf("mariadb salah: %v", got)
	}
	if len(got["mysql"]) != 0 || len(got["pgsql"]) != 2 {
		t.Fatalf("mysql/pgsql salah: %v", got)
	}
	for _, e := range []string{"mariadb", "mysql", "pgsql"} {
		if !validEngine(e) {
			t.Fatalf("%s harus sah", e)
		}
	}
	for _, e := range []string{"", "redis", "MariaDB", "mariadb;", "../mysql"} {
		if validEngine(e) {
			t.Fatalf("%q harus ditolak", e)
		}
	}
	if _, ok := dbcfgKeys["mariadb"]["max_connections"]; !ok {
		t.Fatal("dbcfg mariadb tidak punya kunci")
	}
	if !dbcfgKeys["mariadb"]["max_connections"]("150") || dbcfgKeys["mariadb"]["max_connections"]("5000") {
		t.Fatal("validasi max_connections salah")
	}
	if !reservedNames["mariadb"] {
		t.Fatal("nama situs 'mariadb' harus dicadangkan")
	}
}

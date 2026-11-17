package main

import (
	"database/sql"
	"path/filepath"
	"testing"
)

func TestOldDatabaseMigratesToAdminRole(t *testing.T) {
	p := filepath.Join(t.TempDir(), "old.db")
	db, err := sql.Open("sqlite", p)
	if err != nil {
		t.Fatal(err)
	}
	// skema lama (sebelum kolom role)
	_, err = db.Exec(`CREATE TABLE users(id INTEGER PRIMARY KEY, name TEXT UNIQUE NOT NULL, pass_hash TEXT NOT NULL,
  totp_enc BLOB NOT NULL, totp_last INTEGER NOT NULL DEFAULT 0, disabled INTEGER NOT NULL DEFAULT 0, created INTEGER NOT NULL);
  INSERT INTO users(name,pass_hash,totp_enc,created) VALUES('admin','h',x'00',1);`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	st, err := openStore(p)
	if err != nil {
		t.Fatal(err)
	}
	rows := st.UserRows()
	if len(rows) != 1 || rows[0].Name != "admin" || rows[0].Role != "admin" {
		t.Fatalf("user lama harus menjadi admin: %+v", rows)
	}
	st2, err := openStore(p) // buka dua kali aman (idempoten)
	if err != nil || len(st2.UserRows()) != 1 {
		t.Fatalf("buka ulang: %v", err)
	}
}

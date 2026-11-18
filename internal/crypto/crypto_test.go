package crypto

import (
	"testing"
	"time"
)

func TestTOTPRFC6238Vectors(t *testing.T) {
	secret := []byte("12345678901234567890")
	for ts, want := range map[int64]string{59: "287082", 1111111109: "081804", 1234567890: "005924", 2000000000: "279037"} {
		if got := TotpAt(secret, ts/30); got != want {
			t.Errorf("T=%d: got %s want %s", ts, got, want)
		}
	}
}

func TestTOTPReplayAndWindow(t *testing.T) {
	s := []byte("12345678901234567890")
	now := time.Unix(1111111109, 0)
	step, ok := VerifyTOTP(s, "081804", now, 0)
	if !ok || step != 1111111109/30 {
		t.Fatal("kode valid ditolak")
	}
	if _, ok := VerifyTOTP(s, "081804", now, step); ok {
		t.Fatal("replay diterima")
	}
	if _, ok := VerifyTOTP(s, "000000", now, 0); ok {
		t.Fatal("kode salah diterima")
	}
	if _, ok := VerifyTOTP(s, TotpAt(s, now.Unix()/30-3), now, 0); ok {
		t.Fatal("kode di luar jendela diterima")
	}
}

func TestPasswordHash(t *testing.T) {
	h := HashPassword("rahasia-panjang-1")
	if !VerifyPassword(h, "rahasia-panjang-1") || VerifyPassword(h, "rahasia-panjang-2") || VerifyPassword("rusak", "x") {
		t.Fatal("verifikasi password salah")
	}
	h1, h2 := HashPassword("a"), HashPassword("a")
	if h1 == h2 {
		t.Fatal("salt tidak acak")
	}
}

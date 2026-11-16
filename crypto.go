package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" //nolint:gosec // TOTP RFC 6238 memakai HMAC-SHA1 agar kompatibel dengan Google Authenticator
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
)

const (
	argonT = 3
	argonM = 64 * 1024
	argonP = 2
)

func randBytes(n int) []byte {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return b
}

func randToken() string { return base64.RawURLEncoding.EncodeToString(randBytes(32)) }

func sha256hex(s string) string { h := sha256.Sum256([]byte(s)); return hex.EncodeToString(h[:]) }

// ---------- password (argon2id) ----------

func hashPassword(pw string) string {
	salt := randBytes(16)
	h := argon2.IDKey([]byte(pw), salt, argonT, argonM, argonP, 32)
	return fmt.Sprintf("$argon2id$v=19$m=%d,t=%d,p=%d$%s$%s", argonM, argonT, argonP,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(h))
}

func verifyPassword(enc, pw string) bool {
	p := strings.Split(enc, "$")
	if len(p) != 6 || p[1] != "argon2id" {
		return false
	}
	var m, t uint32
	var par uint8
	if _, err := fmt.Sscanf(p[3], "m=%d,t=%d,p=%d", &m, &t, &par); err != nil {
		return false
	}
	salt, err1 := base64.RawStdEncoding.DecodeString(p[4])
	want, err2 := base64.RawStdEncoding.DecodeString(p[5])
	if err1 != nil || err2 != nil {
		return false
	}
	got := argon2.IDKey([]byte(pw), salt, t, m, par, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// ---------- TOTP (RFC 6238) ----------

func totpAt(secret []byte, step int64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(step))
	m := hmac.New(sha1.New, secret)
	m.Write(buf[:])
	s := m.Sum(nil)
	o := s[len(s)-1] & 0xf
	code := (uint32(s[o]&0x7f)<<24 | uint32(s[o+1])<<16 | uint32(s[o+2])<<8 | uint32(s[o+3])) % 1000000
	return fmt.Sprintf("%06d", code)
}

// verifyTOTP menerima langkah sekarang dan +-1, tetapi menolak langkah <= lastStep (anti-replay).
func verifyTOTP(secret []byte, code string, now time.Time, lastStep int64) (int64, bool) {
	if len(code) != 6 {
		return 0, false
	}
	cur := now.Unix() / 30
	var hit int64
	for _, d := range []int64{-1, 0, 1} {
		st := cur + d
		if st > lastStep && subtle.ConstantTimeCompare([]byte(totpAt(secret, st)), []byte(code)) == 1 {
			hit = st
		}
	}
	return hit, hit != 0
}

var b32 = base32.StdEncoding.WithPadding(base32.NoPadding)

func newTOTPSecret() []byte { return randBytes(20) }

func otpauthURI(user string, secret []byte) string {
	return fmt.Sprintf("otpauth://totp/panel:%s?secret=%s&issuer=panel&algorithm=SHA1&digits=6&period=30", user, b32.EncodeToString(secret))
}

// ---------- enkripsi secret TOTP (AES-256-GCM, kunci dari Keychain) ----------

func seal(key, plain []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, errors.New("kunci harus 32 byte")
	}
	blk, _ := aes.NewCipher(key)
	g, _ := cipher.NewGCM(blk)
	n := randBytes(g.NonceSize())
	return append(n, g.Seal(nil, n, plain, nil)...), nil
}

func open(key, blob []byte) ([]byte, error) {
	blk, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, _ := cipher.NewGCM(blk)
	if len(blob) < g.NonceSize() {
		return nil, errors.New("blob terlalu pendek")
	}
	return g.Open(nil, blob[:g.NonceSize()], blob[g.NonceSize():], nil)
}

// ---------- recovery code ----------

func newRecoveryCodes(n int) []string {
	out := make([]string, n)
	for i := range out {
		s := strings.ToLower(b32.EncodeToString(randBytes(7)))[:10]
		out[i] = s[:5] + "-" + s[5:]
	}
	return out
}

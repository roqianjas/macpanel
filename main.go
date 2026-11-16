package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/term"
)

var nameRE = regexp.MustCompile(`^[a-z][a-z0-9_-]{2,31}$`)

func defaultDB() string {
	h, _ := os.UserHomeDir()
	return filepath.Join(h, ".config/macminiops/panel/panel.db")
}

// ---------- Keychain macOS ----------
// Catatan: `security -w <nilai>` menaruh kunci di argumen proses sesaat (hanya saat `init-key`, sekali). Dibaca ulang dengan find -w.

func keychainGet(service string) ([]byte, error) {
	out, err := exec.Command("/usr/bin/security", "find-generic-password", "-a", "panel", "-s", service, "-w").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 36 {
			return nil, errors.New("Keychain terkunci untuk sesi ini (SSH). Jalankan: security unlock-keychain ~/Library/Keychains/login.keychain-db")
		}
		return nil, errors.New("kunci tidak ada di Keychain (jalankan: panel init-key)")
	}
	return decodeHexKey(strings.TrimSpace(string(out)))
}

func keychainInit(service string) error {
	if _, err := keychainGet(service); err == nil {
		return errors.New("kunci sudah ada, tidak ditimpa (menimpa membuat secret TOTP lama tak terbaca)")
	}
	k := fmt.Sprintf("%x", randBytes(32))
	err := exec.Command("/usr/bin/security", "add-generic-password", "-a", "panel", "-s", service, "-w", k).Run()
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 36 {
		return errors.New("Keychain terkunci untuk sesi ini (SSH). Jalankan: security unlock-keychain ~/Library/Keychains/login.keychain-db")
	}
	return err
}

func decodeHexKey(s string) ([]byte, error) {
	var b []byte
	if _, err := fmt.Sscanf(s, "%x", &b); err != nil || len(b) != 32 {
		return nil, errors.New("kunci Keychain tidak valid")
	}
	return b, nil
}

func fatal(m string) { fmt.Fprintln(os.Stderr, "panel:", m); os.Exit(1) }

func main() {
	if len(os.Args) < 2 {
		fmt.Println("pakai: panel serve | init-key | user add <nama> [--password-stdin] | user remove <nama> | user list | audit verify")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("panel", flag.ExitOnError)
	dbPath := fs.String("db", defaultDB(), "path database")
	svc := fs.String("key-service", "panel-totp-key", "nama item Keychain")
	addr := fs.String("addr", "127.0.0.1:9100", "alamat listen (harus loopback)")
	origin := fs.String("origin", "https://panel.lab", "origin yang diizinkan")
	uiDir := fs.String("ui", filepath.Join(userHome(), "Developer/_ops/web/panel/ui/dist"), "folder hasil build UI")
	stdin := fs.Bool("password-stdin", false, "baca password dari stdin (otomasi/tes)")
	cmd, rest := os.Args[1], os.Args[2:]
	sub := ""
	if (cmd == "user" || cmd == "audit") && len(rest) > 0 {
		sub, rest = rest[0], rest[1:]
	}
	// argumen posisi (nama) boleh sebelum flag
	var pos []string
	for len(rest) > 0 && !strings.HasPrefix(rest[0], "-") {
		pos, rest = append(pos, rest[0]), rest[1:]
	}
	fs.Parse(rest)

	switch {
	case cmd == "init-key":
		if err := keychainInit(*svc); err != nil {
			fatal(err.Error())
		}
		fmt.Println("OK: kunci enkripsi dibuat di Keychain (service", *svc+")")
	case cmd == "serve":
		if !strings.HasPrefix(*addr, "127.0.0.1:") && !strings.HasPrefix(*addr, "[::1]:") {
			fatal("addr harus loopback (127.0.0.1 atau [::1])")
		}
		key, err := keychainGet(*svc)
		if err != nil {
			fatal(err.Error())
		}
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		st.MarkStaleJobs()
		app := newApp(st, key, Config{Origins: []string{*origin}})
		app.uiDir = *uiDir
		srv := &http.Server{Addr: *addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
		fmt.Println("panel mendengarkan di", *addr)
		fatal(srv.ListenAndServe().Error())
	case cmd == "user" && sub == "list":
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		fmt.Println(strings.Join(st.ListUsers(), "\n"))
	case cmd == "user" && sub == "remove":
		if len(pos) != 1 || !nameRE.MatchString(pos[0]) {
			fatal("pakai: panel user remove <nama>")
		}
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		if !st.DeleteUser(pos[0]) {
			fatal("user tidak ada")
		}
		st.Audit("cli", "user_remove", pos[0], "local", time.Now())
		fmt.Println("User dihapus:", pos[0], "(sesi dan recovery code ikut terhapus)")
	case cmd == "user" && sub == "add":
		if len(pos) != 1 || !nameRE.MatchString(pos[0]) {
			fatal("nama: huruf kecil/angka/_-, 3-32 karakter. Pakai: panel user add <nama>")
		}
		name := pos[0]
		key, err := keychainGet(*svc)
		if err != nil {
			fatal(err.Error())
		}
		pw := readPassword(*stdin)
		if len(pw) < 12 || strings.EqualFold(pw, name) {
			fatal("password minimal 12 karakter dan tidak boleh sama dengan nama")
		}
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		secret := newTOTPSecret()
		enc, _ := seal(key, secret)
		codes := newRecoveryCodes(10)
		hs := make([]string, len(codes))
		for i, c := range codes {
			hs[i] = sha256hex(c)
		}
		if err := st.CreateUser(name, hashPassword(pw), enc, hs); err != nil {
			fatal("gagal membuat user: " + err.Error())
		}
		st.Audit("cli", "user_add", name, "local", time.Now())
		fmt.Println("User dibuat:", name)
		fmt.Println("Setup key TOTP (masukkan ke Google Authenticator/Authy > 'Enter a setup key', jenis time-based):")
		fmt.Println("  ", b32.EncodeToString(secret))
		fmt.Println("URI:", otpauthURI(name, secret))
		fmt.Println("Recovery code (SIMPAN, hanya tampil sekali, sekali pakai):")
		for _, c := range codes {
			fmt.Println("  ", c)
		}
	case cmd == "audit" && sub == "verify":
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		n, err := st.VerifyAudit()
		if err != nil {
			fatal(fmt.Sprintf("%v (baris valid sebelumnya: %d)", err, n))
		}
		fmt.Printf("OK: rantai audit utuh (%d baris)\n", n)
	default:
		fatal("perintah tidak dikenal")
	}
}

func readPassword(fromStdin bool) string {
	if fromStdin {
		l, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		return strings.TrimRight(l, "\r\n")
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fatal("butuh terminal interaktif (atau --password-stdin)")
	}
	fmt.Fprint(os.Stderr, "Password: ")
	a, _ := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprint(os.Stderr, "\nUlangi  : ")
	b, _ := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if string(a) != string(b) {
		fatal("password tidak sama")
	}
	return string(a)
}

func userHome() string {
	h, _ := os.UserHomeDir()
	return h
}

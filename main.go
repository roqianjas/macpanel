package main

import (
	"bufio"
	"context"
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
		fmt.Println("pakai: panel serve | init-key | user add|remove|list|role|rename|passwd|2fa|disable|enable|reset | token create|list|revoke | settings get|set | audit verify")
		os.Exit(2)
	}
	fs := flag.NewFlagSet("panel", flag.ExitOnError)
	dbPath := fs.String("db", defaultDB(), "path database")
	svc := fs.String("key-service", "panel-totp-key", "nama item Keychain")
	addr := fs.String("addr", "127.0.0.1:9100", "alamat listen (harus loopback)")
	origin := fs.String("origin", "https://panel.lab", "origin yang diizinkan")
	uiDir := fs.String("ui", filepath.Join(rootDir(), "server/panel/ui/dist"), "folder hasil build UI")
	stdin := fs.Bool("password-stdin", false, "baca password dari stdin (otomasi/tes)")
	role := fs.String("role", "admin", "peran user: admin atau viewer (hanya-baca)")
	tname := fs.String("name", "", "nama token (panel token create)")
	scope := fs.String("scope", "", "tingkat token: read|operate|full")
	days := fs.Int("days", 0, "masa berlaku token (hari, bawaan 90)")
	hookAddr := fs.String("hook-addr", "", "alamat listen penerima webhook (loopback), kosong = nonaktif")
	cmd, rest := os.Args[1], os.Args[2:]
	sub := ""
	if (cmd == "user" || cmd == "audit" || cmd == "token" || cmd == "settings") && len(rest) > 0 {
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
		app.col = newCollector(app)
		go app.col.Run(context.Background())
		srv := &http.Server{Addr: *addr, Handler: app.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 150 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10}
		if *hookAddr != "" {
			if !strings.HasPrefix(*hookAddr, "127.0.0.1:") && !strings.HasPrefix(*hookAddr, "[::1]:") {
				fatal("hook-addr harus loopback")
			}
			hs := &http.Server{Addr: *hookAddr, Handler: app.HookHandler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 16 << 10}
			go func() { fatal(hs.ListenAndServe().Error()) }()
			fmt.Println("penerima webhook di", *hookAddr)
		}
		fmt.Println("panel mendengarkan di", *addr)
		fatal(srv.ListenAndServe().Error())
	case cmd == "user" && sub == "list":
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		for _, u := range st.UserRows() {
			fmt.Printf("%-20s %s\n", u.Name, u.Role)
		}
	case cmd == "user" && sub == "role":
		if len(pos) != 2 || !nameRE.MatchString(pos[0]) {
			fatal("pakai: panel user role <nama> admin|viewer")
		}
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		if !st.SetRole(pos[0], pos[1]) {
			fatal("user tidak ada atau peran tidak valid (admin|viewer)")
		}
		st.Audit("cli", "user_role", pos[0]+" -> "+pos[1], "local", time.Now())
		fmt.Println("Peran", pos[0], "sekarang", pos[1], "(sesi aktifnya dicabut)")
	case cmd == "user" && sub == "rename":
		if len(pos) != 2 || !nameRE.MatchString(pos[0]) || !nameRE.MatchString(pos[1]) {
			fatal("pakai: panel user rename <nama-lama> <nama-baru> (huruf kecil/angka/_-, 3-32 karakter)")
		}
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		if !st.RenameUser(pos[0], pos[1]) {
			fatal("gagal: user lama tidak ada atau nama baru sudah dipakai")
		}
		st.Audit("cli", "user_rename", pos[0]+" -> "+pos[1], "local", time.Now())
		fmt.Println("User", pos[0], "sekarang", pos[1], "(sesi aktifnya dicabut; kata sandi, 2FA, dan perangkat tepercaya tetap)")
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
		if *role != "admin" && *role != "viewer" {
			fatal("peran harus admin atau viewer")
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
		if *role != "admin" && !st.SetRole(name, *role) {
			fatal("peran harus admin atau viewer")
		}
		st.Audit("cli", "user_add", name+" ("+*role+")", "local", time.Now())
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
	case cmd == "token" || cmd == "settings" || (cmd == "user" && (sub == "passwd" || sub == "2fa" || sub == "disable" || sub == "enable" || sub == "reset")):
		st, err := openStore(*dbPath)
		if err != nil {
			fatal(err.Error())
		}
		if !cliAdmin(st, cmd, sub, pos, cliOpts{pwStdin: *stdin, svc: *svc, tname: *tname, scope: *scope, days: *days}) {
			fatal("perintah tidak dikenal")
		}
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

func rootDir() string {
	if r := os.Getenv("MACPANEL_HOME"); r != "" {
		return r
	}
	return filepath.Join(userHome(), "MacPanel")
}

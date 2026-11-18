package main

import (
	"fmt"
	"strings"
	"time"
)

type cliOpts struct {
	pwStdin bool
	svc     string
	tname   string
	scope   string
	days    int
}

// cliAdmin: perintah lokal di Mac (tanpa jaringan). Mengembalikan false bila perintah bukan miliknya.
//
//	panel user passwd <nama> [--password-stdin]
//	panel user 2fa on|off <nama>
//	panel token create <user> --name N --scope read|operate|full [--days 90]
//	panel token list | revoke <id>
//	panel settings get | set name|theme <nilai>
func cliAdmin(st *Store, cmd, sub string, pos []string, o cliOpts) bool {
	switch {
	case cmd == "user" && sub == "passwd":
		if len(pos) != 1 || !nameRE.MatchString(pos[0]) {
			fatal("pakai: panel user passwd <nama> [--password-stdin]")
		}
		u := st.GetUser(pos[0])
		if u == nil {
			fatal("user tidak ada")
		}
		pw := readPassword(o.pwStdin)
		if len(pw) < 12 || strings.EqualFold(pw, pos[0]) {
			fatal("password minimal 12 karakter dan tidak boleh sama dengan nama")
		}
		if err := st.SetPassword(u.ID, hashPassword(pw), ""); err != nil {
			fatal(err.Error())
		}
		st.Audit("cli", "user_passwd", pos[0]+" (semua sesi dan perangkat tepercaya dicabut)", "local", time.Now())
		fmt.Println("Kata sandi", pos[0], "diganti; semua sesi dan perangkat tepercaya dicabut.")
	case cmd == "user" && (sub == "disable" || sub == "enable"):
		if len(pos) != 1 || !nameRE.MatchString(pos[0]) {
			fatal("pakai: panel user disable|enable <nama>")
		}
		if sub == "disable" && st.AdminsExcept(pos[0]) < 1 {
			fatal("tidak boleh menyisakan nol admin aktif")
		}
		if !st.SetDisabled(pos[0], sub == "disable") {
			fatal("user tidak ada")
		}
		st.Audit("cli", "account_"+sub, pos[0], "local", time.Now())
		fmt.Println("OK:", pos[0], map[string]string{"disable": "dinonaktifkan (sesi dicabut)", "enable": "diaktifkan"}[sub])
	case cmd == "user" && sub == "reset":
		if len(pos) != 1 || !nameRE.MatchString(pos[0]) {
			fatal("pakai: panel user reset <nama>   (kata sandi sementara; wajib diganti saat masuk)")
		}
		u := st.GetUser(pos[0])
		if u == nil {
			fatal("user tidak ada")
		}
		temp := randToken()[:20]
		if err := st.SetTempPassword(u.ID, hashPassword(temp)); err != nil {
			fatal(err.Error())
		}
		st.Audit("cli", "account_reset", pos[0], "local", time.Now())
		fmt.Println("Kata sandi sementara untuk", pos[0], "(tampil sekali; sesi, perangkat tepercaya, dan token dicabut):")
		fmt.Println("  ", temp)
	case cmd == "user" && sub == "2fa":
		if len(pos) != 2 || (pos[0] != "on" && pos[0] != "off") || !nameRE.MatchString(pos[1]) {
			fatal("pakai: panel user 2fa on|off <nama>")
		}
		u := st.GetUser(pos[1])
		if u == nil {
			fatal("user tidak ada")
		}
		if pos[0] == "off" {
			if err := st.DisableTOTP(u.ID, ""); err != nil {
				fatal(err.Error())
			}
			st.Audit("cli", "2fa_disable", pos[1], "local", time.Now())
			fmt.Println("2FA dimatikan untuk", pos[1], "(login hanya kata sandi; semua sesi dan perangkat tepercaya dicabut)")
			return true
		}
		key, err := keychainGet(o.svc)
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
		if err := st.EnableTOTP(u.ID, enc, hs, 0, ""); err != nil {
			fatal(err.Error())
		}
		st.Audit("cli", "2fa_enable", pos[1], "local", time.Now())
		fmt.Println("2FA dinyalakan untuk", pos[1], "(semua sesi dan perangkat tepercaya dicabut). Setup key TOTP:")
		fmt.Println("  ", b32.EncodeToString(secret))
		fmt.Println("URI:", otpauthURI(pos[1], secret))
		fmt.Println("Recovery code (SIMPAN, hanya tampil sekali, sekali pakai):")
		for _, c := range codes {
			fmt.Println("  ", c)
		}
	case cmd == "token" && sub == "create":
		if len(pos) != 1 || !nameRE.MatchString(pos[0]) {
			fatal("pakai: panel token create <user> --name <nama> --scope read|operate|full [--days 90]")
		}
		u := st.GetUser(pos[0])
		if u == nil {
			fatal("user tidak ada")
		}
		if !tokNameRE.MatchString(o.tname) {
			fatal("--name wajib: 1-40 karakter (huruf, angka, spasi, _ . -)")
		}
		if _, ok := tokenScopes[o.scope]; !ok {
			fatal("--scope wajib: read, operate, atau full")
		}
		if o.days == 0 {
			o.days = 90
		}
		if o.days < 1 || o.days > 365 {
			fatal("--days 1-365")
		}
		tok, id, ok := st.AddToken(u.ID, o.tname, o.scope, o.days, time.Now())
		if !ok {
			fatal(fmt.Sprintf("gagal (maksimum %d token aktif per akun)", maxTokens))
		}
		st.Audit("cli", "token_create", fmt.Sprintf("%s untuk %s (%s, %d hari)", o.tname, pos[0], o.scope, o.days), "local", time.Now())
		fmt.Printf("Token #%d (%s, %s, %d hari). HANYA tampil sekali:\n%s\n", id, o.tname, o.scope, o.days, tok)
	case cmd == "token" && sub == "list":
		for _, t := range st.AllTokens(time.Now()) {
			fmt.Printf("#%-3d %-8s %-8s %-20s sampai %s terakhir %s\n", t.ID, t.Prefix, t.Scope, t.Owner+"/"+t.Name,
				time.Unix(t.Expires, 0).Format("2006-01-02"), map[bool]string{true: time.Unix(t.LastUsed, 0).Format("2006-01-02 15:04"), false: "-"}[t.LastUsed > 0])
		}
	case cmd == "token" && sub == "revoke":
		var id int64
		if len(pos) != 1 {
			fatal("pakai: panel token revoke <id>")
		}
		if _, err := fmt.Sscanf(pos[0], "%d", &id); err != nil || !st.RevokeTokenAny(id) {
			fatal("token tidak ada")
		}
		st.Audit("cli", "token_revoke", pos[0], "local", time.Now())
		fmt.Println("Token dicabut:", id)
	case cmd == "settings" && sub == "get":
		fmt.Println("name =", st.Setting("panel_name", defaultName))
		fmt.Println("theme =", st.Setting("theme", "system"))
	case cmd == "settings" && sub == "set":
		if len(pos) < 2 {
			fatal("pakai: panel settings set name|theme <nilai>")
		}
		v := strings.TrimSpace(strings.Join(pos[1:], " "))
		switch pos[0] {
		case "name":
			if !nameRE19.MatchString(v) {
				fatal("nama panel: 1-40 karakter (huruf, angka, spasi, _ . & ' -)")
			}
			st.SetSetting("panel_name", v)
		case "theme":
			if !themes[v] {
				fatal("tema: system, light, atau dark")
			}
			st.SetSetting("theme", v)
		default:
			fatal("kunci: name atau theme")
		}
		st.Audit("cli", "settings_change", pos[0]+"="+v, "local", time.Now())
		fmt.Println("OK:", pos[0], "=", v)
	default:
		return false
	}
	return true
}

package databases

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

type DumpFile struct {
	File string `json:"file"`
	At   int64  `json:"at"`
	Size int64  `json:"size"`
}

func DumpDir(a *core.App) string { return filepath.Join(a.Root, "backup", "db") }

func dumpRE(engine, name string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `-` + engine + `-\d{8}-\d{6}\.sql\.gz$`)
}

func DBDumps(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	re := dumpRE(engine, name)
	list := []DumpFile{}
	ents, _ := os.ReadDir(DumpDir(a))
	for _, e := range ents {
		if !re.MatchString(e.Name()) {
			continue
		}
		fi, err := e.Info()
		if err != nil || !fi.Mode().IsRegular() { // symlink atau non-berkas diabaikan
			continue
		}
		list = append(list, DumpFile{File: e.Name(), At: fi.ModTime().Unix(), Size: fi.Size()})
	}
	sort.Slice(list, func(i, j int) bool { return list[i].File > list[j].File })
	if len(list) > 30 {
		list = list[:30]
	}
	core.WriteJSON(w, 200, map[string]any{"dumps": list})
}

func DBRestore(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{ File string }
	if !core.DecodeJSON(w, r, &in) {
		return
	}
	if !dumpRE(engine, name).MatchString(in.File) { // hanya nama tanpa path; cocok dengan database ini
		core.WriteError(w, 400, "berkas dump tidak valid untuk database ini")
		return
	}
	fi, err := os.Lstat(filepath.Join(DumpDir(a), in.File))
	if err != nil || !fi.Mode().IsRegular() {
		core.WriteError(w, 404, "berkas dump tidak ada")
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	core.RespondAsync(w, a.StartAsync(ss, a.ClientIP(r), "db_restore", engine+"/"+name+" <- "+strconv.Quote(in.File), []string{"db", "restore", name, "--engine", engine, "--file", in.File}, ""))
}

const MaxUpload = 256 << 20

// hDBUpload menyimpan dump unggahan ke ~/MacPanel/backup/db sebagai <db>-<engine>-<waktu>.sql.gz (gzip bila belum).
// Isi tidak dipercaya: `lab db restore` memindai dan menjalankannya sebagai user proyek, hanya ke database kosong.
func DBUpload(a *core.App, w http.ResponseWriter, r *http.Request, ss *store.Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	if !a.RequireStepUp(w, ss) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxUpload)
	br := bufio.NewReader(r.Body)
	head, _ := br.Peek(2)
	isGz := len(head) == 2 && head[0] == 0x1f && head[1] == 0x8b
	if len(head) == 0 {
		core.WriteError(w, 400, "berkas kosong")
		return
	}
	dir := DumpDir(a)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		core.WriteError(w, 500, "folder dump tidak bisa dibuat")
		return
	}
	tmp, err := os.CreateTemp(dir, ".upload-*")
	if err != nil {
		core.WriteError(w, 500, "tidak bisa menyimpan")
		return
	}
	defer os.Remove(tmp.Name())
	_ = tmp.Chmod(0o600)
	var cerr error
	if isGz {
		_, cerr = io.Copy(tmp, br)
	} else {
		gz := gzip.NewWriter(tmp)
		if _, cerr = io.Copy(gz, br); cerr == nil {
			cerr = gz.Close()
		}
	}
	if cerr == nil {
		cerr = tmp.Close()
	}
	if cerr != nil {
		code, msg := 400, "unggahan gagal"
		if _, big := cerr.(*http.MaxBytesError); big {
			code, msg = 413, fmt.Sprintf("berkas lebih dari %d MB", MaxUpload>>20)
		}
		core.WriteError(w, code, msg)
		return
	}
	var final string
	for i := 0; i < 20; i++ {
		final = fmt.Sprintf("%s-%s-%s.sql.gz", name, engine, a.Now().Add(time.Duration(i)*time.Second).Format("20060102-150405"))
		if _, err := os.Lstat(filepath.Join(dir, final)); os.IsNotExist(err) {
			break
		}
		final = ""
	}
	if final == "" {
		core.WriteError(w, 409, "terlalu banyak unggahan dalam waktu dekat")
		return
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, final)); err != nil {
		core.WriteError(w, 500, "tidak bisa menyimpan")
		return
	}
	fi, _ := os.Stat(filepath.Join(dir, final))
	a.Store.Audit(ss.User, "db_upload", engine+"/"+name+" "+final, a.ClientIP(r), a.Now())
	core.WriteJSON(w, 200, map[string]any{"file": final, "size": fi.Size()})
}

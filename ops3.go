package main

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
)

type DumpFile struct {
	File string `json:"file"`
	At   int64  `json:"at"`
	Size int64  `json:"size"`
}

func (a *App) dumpDir() string { return filepath.Join(a.root, "backup", "db") }

func dumpRE(engine, name string) *regexp.Regexp {
	return regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `-` + engine + `-\d{8}-\d{6}\.sql\.gz$`)
}

func hDBDumps(a *App, w http.ResponseWriter, r *http.Request, _ *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	re := dumpRE(engine, name)
	list := []DumpFile{}
	ents, _ := os.ReadDir(a.dumpDir())
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
	jsonOut(w, 200, map[string]any{"dumps": list})
}

func hDBRestore(a *App, w http.ResponseWriter, r *http.Request, ss *Session) {
	engine, name, ok := dbParams(w, r)
	if !ok {
		return
	}
	var in struct{ File string }
	if !decode(w, r, &in) {
		return
	}
	if !dumpRE(engine, name).MatchString(in.File) { // hanya nama tanpa path; cocok dengan database ini
		jerr(w, 400, "berkas dump tidak valid untuk database ini")
		return
	}
	fi, err := os.Lstat(filepath.Join(a.dumpDir(), in.File))
	if err != nil || !fi.Mode().IsRegular() {
		jerr(w, 404, "berkas dump tidak ada")
		return
	}
	if !a.requireStepUp(w, ss) {
		return
	}
	respondAsync(w, a.startAsync(ss, a.clientIP(r), "db_restore", engine+"/"+name+" <- "+strconv.Quote(in.File), []string{"db", "restore", name, "--engine", engine, "--file", in.File}, ""))
}

package system

import (
	"bytes"
	"context"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/roqianjas/macpanel/internal/core"
	"github.com/roqianjas/macpanel/internal/store"
)

const (
	sampleEvery   = 30 * time.Second
	retainSeconds = 7 * 24 * 3600
)

type Runner func(timeout time.Duration, name string, args ...string) (string, error)

func realRun(timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = []string{"PATH=/usr/bin:/bin:/usr/sbin:/sbin", "LANG=C", "LC_ALL=C"}
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	return out.String(), err
}

var topIdleRE = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)% idle`)

var vmPageRE = regexp.MustCompile(`page size of (\d+) bytes`)

var loadRE = regexp.MustCompile(`\{\s*([0-9.]+)\s+([0-9.]+)\s+([0-9.]+)\s*\}`)

var bootRE = regexp.MustCompile(`sec\s*=\s*(\d+)`)

var ipv4RE = regexp.MustCompile(`^\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}$`)

var ifaceNameR = regexp.MustCompile(`^[a-z]+[0-9]+\*?$`)

// parseTop mengambil sampel TERAKHIR (sampel pertama top mencerminkan waktu sejak boot, bukan saat ini).
func parseTop(out string) (float64, bool) {
	cpu, ok := 0.0, false
	for _, ln := range strings.Split(out, "\n") {
		if !strings.Contains(ln, "CPU usage:") {
			continue
		}
		if m := topIdleRE.FindStringSubmatch(ln); m != nil {
			idle, _ := strconv.ParseFloat(m[1], 64)
			if idle >= 0 && idle <= 100 {
				cpu, ok = 100-idle, true
			}
		}
	}
	return cpu, ok
}

// parseVMStat menghitung "Memory Used" ala Activity Monitor: App Memory (halaman anonim dikurangi purgeable)
// + Wired + Compressed, dibagi memori fisik. Cache file TIDAK dihitung terpakai, sama seperti Activity Monitor.
func parseVMStat(out string, total uint64) (float64, bool) {
	if total == 0 {
		return 0, false
	}
	ps := vmPageRE.FindStringSubmatch(out)
	if ps == nil {
		return 0, false
	}
	page, _ := strconv.ParseUint(ps[1], 10, 64)
	get := func(label string) (uint64, bool) {
		m := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(label) + `:\s*(\d+)\.`).FindStringSubmatch(out)
		if m == nil {
			return 0, false
		}
		v, _ := strconv.ParseUint(m[1], 10, 64)
		return v, true
	}
	anon, ok1 := get("Anonymous pages")
	purg, ok2 := get("Pages purgeable")
	wired, ok3 := get("Pages wired down")
	comp, ok4 := get("Pages occupied by compressor")
	if !ok1 || !ok2 || !ok3 || !ok4 || page == 0 {
		return 0, false
	}
	if purg > anon {
		purg = anon
	}
	pct := float64((anon-purg+wired+comp)*page) / float64(total) * 100
	if pct < 0 || pct > 100 {
		return 0, false
	}
	return pct, true
}

func parseLoad(out string) ([3]float64, bool) {
	m := loadRE.FindStringSubmatch(out)
	if m == nil {
		return [3]float64{}, false
	}
	var l [3]float64
	for i := 0; i < 3; i++ {
		l[i], _ = strconv.ParseFloat(m[i+1], 64)
	}
	return l, true
}

func parseBoot(out string) (int64, bool) {
	m := bootRE.FindStringSubmatch(out)
	if m == nil {
		return 0, false
	}
	v, _ := strconv.ParseInt(m[1], 10, 64)
	return v, v > 0
}

type netCounter struct {
	RX, TX uint64
	IP     string
}

// parseNetstat membaca `netstat -ibn`. Kolom dihitung dari KANAN karena kolom Address kosong pada utun*.
// Hanya interface ber-IPv4 non-loopback yang dilaporkan (LAN dan Tailscale), bukan puluhan interface tidur.
func ParseNetstat(out string) map[string]netCounter {
	cnt := map[string]netCounter{}
	ips := map[string]string{}
	for _, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		if len(f) < 10 || !ifaceNameR.MatchString(f[0]) {
			continue
		}
		name := strings.TrimSuffix(f[0], "*")
		if strings.HasPrefix(f[2], "<Link#") {
			n := len(f)
			rx, e1 := strconv.ParseUint(f[n-5], 10, 64)
			tx, e2 := strconv.ParseUint(f[n-2], 10, 64)
			if e1 == nil && e2 == nil {
				cnt[name] = netCounter{RX: rx, TX: tx}
			}
		} else if ipv4RE.MatchString(f[3]) {
			ips[name] = f[3]
		}
	}
	res := map[string]netCounter{}
	for name, ip := range ips {
		if c, ok := cnt[name]; ok && !strings.HasPrefix(ip, "127.") {
			c.IP = ip
			res[name] = c
		}
	}
	return res
}

func IfaceLabel(name, ip string) string {
	if strings.HasPrefix(ip, "100.") {
		return "Tailscale (" + name + ")"
	}
	return "LAN (" + name + ")"
}

type ProcInfo struct {
	Name  string  `json:"name"`
	CPU   float64 `json:"cpu"`
	RSSMB float64 `json:"rss_mb"`
}

// parsePS membaca `ps -Aco pid,pcpu,rss,comm -r` (comm saja: argumen proses bisa memuat rahasia dan tidak ditampilkan).
func parsePS(out string, n int) []ProcInfo {
	res := []ProcInfo{}
	for i, ln := range strings.Split(out, "\n") {
		f := strings.Fields(ln)
		if i == 0 || len(f) < 4 {
			continue
		}
		cpu, e1 := strconv.ParseFloat(f[1], 64)
		rss, e2 := strconv.ParseFloat(f[2], 64)
		if e1 != nil || e2 != nil {
			continue
		}
		res = append(res, ProcInfo{Name: strings.Join(f[3:], " "), CPU: cpu, RSSMB: rss / 1024})
		if len(res) >= n {
			break
		}
	}
	return res
}

type Collector struct {
	a        *core.App
	memTotal uint64
	Runner   Runner
	Disk     func() (total, free uint64)
	mu       sync.Mutex
	prev     map[string]netCounter
	prevT    time.Time
	lastAt   int64
	last     struct {
		CPU, Mem, Disk float64
		Load           [3]float64
		Nets           map[string]netCounter
	}
}

func NewCollector(a *core.App) *Collector {
	return &Collector{a: a, Runner: realRun, Disk: diskUsage}
}

func diskUsage() (uint64, uint64) {
	var st syscall.Statfs_t
	if err := syscall.Statfs("/System/Volumes/Data", &st); err != nil {
		if err := syscall.Statfs("/", &st); err != nil {
			return 0, 0
		}
	}
	return st.Blocks * uint64(st.Bsize), st.Bavail * uint64(st.Bsize)
}

// Sample mengambil satu sampel dan menyimpannya. Nilai yang gagal dibaca TIDAK dikarang: sampel dilewati bila CPU dan memori sama-sama gagal.
func (c *Collector) Sample(now time.Time) {
	t := now.Unix()
	cpu, cpuOK := 0.0, false
	if o, err := c.Runner(10*time.Second, "/usr/bin/top", "-l", "2", "-n", "0", "-s", "1"); err == nil {
		cpu, cpuOK = parseTop(o)
	}
	mem, memOK := 0.0, false
	if c.memTotal == 0 {
		if o, err := c.Runner(3*time.Second, "/usr/sbin/sysctl", "-n", "hw.memsize"); err == nil {
			c.memTotal, _ = strconv.ParseUint(strings.TrimSpace(o), 10, 64)
		}
	}
	if o, err := c.Runner(5*time.Second, "/usr/bin/vm_stat"); err == nil {
		mem, memOK = parseVMStat(o, c.memTotal)
	}
	var load [3]float64
	if o, err := c.Runner(3*time.Second, "/usr/sbin/sysctl", "-n", "vm.loadavg"); err == nil {
		load, _ = parseLoad(o)
	}
	total, free := c.Disk()
	diskPct := 0.0
	if total > 0 {
		diskPct = float64(total-free) / float64(total) * 100
	}
	if cpuOK || memOK {
		c.a.Store.AddSample(t, cpu, mem, diskPct, load[0], cpuOK, memOK)
	}
	nets := map[string]netCounter{}
	if o, err := c.Runner(5*time.Second, "/usr/sbin/netstat", "-ibn"); err == nil {
		nets = ParseNetstat(o)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.prev != nil && now.After(c.prevT) {
		dt := now.Sub(c.prevT).Seconds()
		for name, cur := range nets {
			p, ok := c.prev[name]
			if !ok || cur.RX < p.RX || cur.TX < p.TX { // interface baru atau penghitung diatur ulang: tidak ada laju
				continue
			}
			c.a.Store.AddNet(t, name, IfaceLabel(name, cur.IP), float64(cur.RX-p.RX)/dt, float64(cur.TX-p.TX)/dt, int64(cur.RX-p.RX), int64(cur.TX-p.TX))
		}
	}
	c.prev, c.prevT, c.lastAt = nets, now, t
	c.last.CPU, c.last.Mem, c.last.Disk, c.last.Load, c.last.Nets = cpu, mem, diskPct, load, nets
}

func (c *Collector) Run(ctx context.Context) {
	c.Sample(time.Now())
	tk := time.NewTicker(sampleEvery)
	defer tk.Stop()
	n := 0
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-tk.C:
			c.Sample(now)
			if n++; n%120 == 0 {
				c.a.Store.PruneMetrics(now.Unix() - retainSeconds)
				pruneHistory(c.a.Store, now)
			}
		}
	}
}

type rangeSpec struct {
	Sec, Bucket int64
}

var ranges = map[string]rangeSpec{"1h": {3600, 30}, "24h": {86400, 300}, "7d": {7 * 86400, 1800}}

func Metrics(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	rk := r.URL.Query().Get("range")
	if rk == "" {
		rk = "24h"
	}
	rs, ok := ranges[rk]
	if !ok {
		core.WriteError(w, 400, "range harus 1h, 24h, atau 7d")
		return
	}
	now := a.Now().Unix()
	cpu, mem, disk, load := a.Store.Series(now-rs.Sec, rs.Bucket)
	core.WriteJSON(w, 200, map[string]any{"now": now, "range": rk, "bucket": rs.Bucket, "interval": int(sampleEvery.Seconds()),
		"cpu": cpu, "mem": mem, "disk": disk, "load": load, "nets": a.Store.NetSeries(now-rs.Sec, rs.Bucket)})
}

type SystemInfo struct {
	Host      string         `json:"host"`
	OS        string         `json:"os"`
	Cores     int            `json:"cores"`
	MemTotal  uint64         `json:"mem_total"`
	DiskTotal uint64         `json:"disk_total"`
	DiskFree  uint64         `json:"disk_free"`
	Boot      int64          `json:"boot"`
	Load      [3]float64     `json:"load"`
	CPU       float64        `json:"cpu"`
	Mem       float64        `json:"mem"`
	Disk      float64        `json:"disk"`
	SampleAt  int64          `json:"sample_at"`
	Tailnet   string         `json:"tailnet_ip"`
	Nets      []NetNow       `json:"nets"`
	Top       []ProcInfo     `json:"top"`
	Backup    *BackupInfo    `json:"backup"`
	Extra     map[string]any `json:"-"`
}

type NetNow struct {
	Iface string  `json:"iface"`
	Label string  `json:"label"`
	IP    string  `json:"ip"`
	RXBps float64 `json:"rx_bps"`
	TXBps float64 `json:"tx_bps"`
}

type BackupInfo struct {
	At    int64  `json:"at"`
	Count int    `json:"count"`
	Local bool   `json:"local_only"`
	Name  string `json:"name"`
}

func backupInfo(a *core.App) *BackupInfo {
	dirs, _ := os.ReadDir(a.Root + "/backup/staging")
	names := []string{}
	for _, d := range dirs {
		if d.IsDir() {
			names = append(names, d.Name())
		}
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	last := names[len(names)-1]
	st, err := os.Stat(a.Root + "/backup/staging/" + last)
	if err != nil {
		return nil
	}
	return &BackupInfo{At: st.ModTime().Unix(), Count: len(names), Local: true, Name: last}
}

func systemInfo(a *core.App) SystemInfo {
	c := ActiveCollector(a)
	si := SystemInfo{Cores: 0, OS: "macOS"}
	si.Host, _ = os.Hostname()
	si.Host = strings.TrimSuffix(si.Host, ".local")
	run := realRun
	if c != nil {
		run = c.Runner
	}
	if o, err := run(3*time.Second, "/usr/sbin/sysctl", "-n", "hw.ncpu"); err == nil {
		si.Cores, _ = strconv.Atoi(strings.TrimSpace(o))
	}
	if o, err := run(3*time.Second, "/usr/sbin/sysctl", "-n", "hw.memsize"); err == nil {
		si.MemTotal, _ = strconv.ParseUint(strings.TrimSpace(o), 10, 64)
	}
	if o, err := run(3*time.Second, "/usr/sbin/sysctl", "-n", "kern.boottime"); err == nil {
		si.Boot, _ = parseBoot(o)
	}
	if o, err := run(3*time.Second, "/usr/bin/sw_vers", "-productVersion"); err == nil {
		si.OS = "macOS " + strings.TrimSpace(o)
	}
	if o, err := run(4*time.Second, "/bin/ps", "-Aco", "pid,pcpu,rss,comm", "-r"); err == nil {
		si.Top = parsePS(o, 8)
	}
	if c != nil {
		si.DiskTotal, si.DiskFree = c.Disk()
		c.mu.Lock()
		si.CPU, si.Mem, si.Disk, si.Load, si.SampleAt = c.last.CPU, c.last.Mem, c.last.Disk, c.last.Load, c.lastAt
		cur := c.last.Nets
		prevT := c.prevT
		_ = prevT
		c.mu.Unlock()
		names := make([]string, 0, len(cur))
		for n := range cur {
			names = append(names, n)
		}
		sort.Strings(names)
		for _, n := range names {
			rx, tx := a.Store.LastRate(n)
			si.Nets = append(si.Nets, NetNow{Iface: n, Label: IfaceLabel(n, cur[n].IP), IP: cur[n].IP, RXBps: rx, TXBps: tx})
			if strings.HasPrefix(cur[n].IP, "100.") {
				si.Tailnet = cur[n].IP
			}
		}
	}
	si.Backup = backupInfo(a)
	return si
}

// cachedSystemInfo: ringkasan sistem dengan cache 8 detik; dipakai hSystem dan peringatan.
func CachedSystemInfo(a *core.App) SystemInfo {
	st := core.Local[sysState](a, "system")
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.cache == nil || a.Now().Sub(st.at) > 8*time.Second {
		si := systemInfo(a)
		st.cache, st.at = &si, a.Now()
	}
	return *st.cache
}

func System(a *core.App, w http.ResponseWriter, r *http.Request, _ *store.Session) {
	core.WriteJSON(w, 200, CachedSystemInfo(a))
}

// sysState: cache ringkasan sistem (domain system).
type sysState struct {
	mu    sync.Mutex
	cache *SystemInfo
	at    time.Time
}

// colHolder: pengumpul metrik yang sedang berjalan (kosong di tes).
type colHolder struct{ c *Collector }

func SetCollector(a *core.App, c *Collector) { core.Local[colHolder](a, "collector").c = c }

func ActiveCollector(a *core.App) *Collector { return core.Local[colHolder](a, "collector").c }

"""lab firewall: tutup port dari jaringan rumah (LAN) lewat pf, tanpa panel memegang hak root.

Model keamanan: perintah ini dijalankan USER di terminal; sudo dipanggil per langkah dengan argumen tetap (tanpa shell dari string).
Yang berjalan sebagai root hanyalah loader milik root di /usr/local/libexec (disalin dengan `install`, bukan dari folder user)
dan berkas daftar port milik root. Panel hanya menampilkan perintahnya.

Aturan per port: loopback boleh; tailnet (100.64.0.0/10 lewat antarmuka Tailscale) boleh bila mode `tailnet`; sisanya diblokir
(TCP dan UDP). Saat boot, loader memblokir DULU baru menunggu Tailscale, jadi tidak ada celah terbuka.

Anchor `macminiops-block` berbagi /etc/pf.macminiops.conf dengan kunci RustDesk (anchor `macminiops-rustdesk`), jadi keduanya
tidak saling menimpa saat `pfctl -f`.
"""

import os
import re
import subprocess
import sys
import tempfile
import time

ANCHOR = "macminiops-block"
ANCHOR_FILE = f"/etc/pf.anchors/{ANCHOR}"
PORTS_FILE = "/etc/macminiops-pf.ports"
CONF = "/etc/pf.macminiops.conf"
BASE_CONF = "/etc/pf.conf"
LABEL = "com.macminiops.pf-block"
LOADER = "/usr/local/libexec/macminiops-pf-block"
PLIST = f"/Library/LaunchDaemons/{LABEL}.plist"
LOG = "/var/log/macminiops-pf-block.log"
RUSTDESK_ANCHOR = "macminiops-rustdesk"
MODES = ("tailnet", "none")
PROTECTED = {22: "SSH: memblokirnya dari LAN bisa mengunci Anda dari mesin ini; atur lewat konfigurasi SSH"}
MAX_PORTS = 50


class FirewallError(Exception):
    pass


def valid_port(p):
    return isinstance(p, int) and not isinstance(p, bool) and 1 <= p <= 65535


def parse_ports(text):
    """`port mode` per baris; baris kosong dan #komentar dilewati. Kesalahan = FirewallError (jangan menebak isi berkas keamanan)."""
    out = {}
    for n, raw in enumerate(text.splitlines(), 1):
        line = raw.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split()
        if len(parts) != 2 or not parts[0].isdigit() or parts[1] not in MODES:
            raise FirewallError(f"{PORTS_FILE} baris {n} tidak valid: {line[:40]!r}")
        p = int(parts[0])
        if not valid_port(p):
            raise FirewallError(f"{PORTS_FILE} baris {n}: port di luar 1-65535")
        out[p] = parts[1]
    return out


def render_ports(ports):
    head = "# Dikelola `lab firewall`. Format: port mode (tailnet = tailnet tetap boleh, none = hanya loopback).\n"
    return head + "".join(f"{p} {ports[p]}\n" for p in sorted(ports))


def render_anchor(ports, tsif):
    """Isi anchor. Harus identik dengan keluaran loader (dijaga tes)."""
    lines = ["pass in quick on lo0 all"]
    for p in sorted(ports):
        if ports[p] == "tailnet" and tsif:
            lines.append(f"pass in quick on {tsif} proto {{ tcp udp }} from 100.64.0.0/10 to any port {p}")
        lines.append(f"block drop in quick proto tcp from any to any port {p}")
        lines.append(f"block drop in quick proto udp from any to any port {p}")
    return "\n".join(lines) + "\n"


LOADER_TEMPLATE = """#!/bin/sh
# Dibuat oleh `lab firewall`. Milik root; jangan ubah dengan tangan, jalankan ulang `lab firewall block|unblock`.
PORTS=@PORTS@
ANCHOR=@ANCHOR@
CONF=@CONF@
PFCTL=@PFCTL@
IFCONFIG=@IFCONFIG@

render() {
  echo "pass in quick on lo0 all"
  [ -r "$PORTS" ] || return 0
  while read -r port mode; do
    case "$port" in ''|\\#*) continue;; esac
    case "$port" in *[!0-9]*) continue;; esac
    [ "$port" -ge 1 ] && [ "$port" -le 65535 ] || continue
    if [ "$mode" = tailnet ] && [ -n "$1" ]; then
      echo "pass in quick on $1 proto { tcp udp } from 100.64.0.0/10 to any port $port"
    fi
    echo "block drop in quick proto tcp from any to any port $port"
    echo "block drop in quick proto udp from any to any port $port"
  done < "$PORTS"
}

apply() {
  render "$1" > "$ANCHOR.new" && mv "$ANCHOR.new" "$ANCHOR" && $PFCTL -f "$CONF"
}

# 1) blokir dulu (tanpa izin tailnet), supaya tidak ada celah selama menunggu Tailscale
apply ""
# 2) tunggu antarmuka Tailscale, lalu tambahkan izin tailnet
TSIF=""
i=0
while [ "$i" -lt 30 ]; do
  for u in $($IFCONFIG -l); do
    case "$u" in utun*) $IFCONFIG "$u" | grep -q 'inet 100\\.' && TSIF="$u";; esac
  done
  [ -n "$TSIF" ] && break
  i=$((i + 1)); sleep @WAIT@
done
[ -n "$TSIF" ] && apply "$TSIF"
$PFCTL -e 2>/dev/null || true
echo "tailscale iface: ${TSIF:-TIDAK TERDETEKSI (port mode tailnet ikut tertutup)}"
"""


def render_loader(ports_file=PORTS_FILE, anchor=ANCHOR_FILE, conf=CONF, pfctl="/sbin/pfctl", ifconfig="/sbin/ifconfig", wait=3):
    for v in (ports_file, anchor, conf, pfctl, ifconfig):
        if not re.fullmatch(r"/[A-Za-z0-9._/-]+", v):
            raise FirewallError(f"path tidak aman untuk loader: {v!r}")
    s = LOADER_TEMPLATE
    for k, v in (("@PORTS@", ports_file), ("@ANCHOR@", anchor), ("@CONF@", conf), ("@PFCTL@", pfctl), ("@IFCONFIG@", ifconfig), ("@WAIT@", str(int(wait)))):
        s = s.replace(k, v)
    return s


def render_plist():
    return (
        '<?xml version="1.0" encoding="UTF-8"?>\n'
        '<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">\n'
        '<plist version="1.0"><dict>\n'
        f"  <key>Label</key><string>{LABEL}</string>\n"
        f"  <key>ProgramArguments</key><array><string>{LOADER}</string></array>\n"
        "  <key>RunAtLoad</key><true/>\n"
        f"  <key>StandardOutPath</key><string>{LOG}</string>\n"
        f"  <key>StandardErrorPath</key><string>{LOG}</string>\n"
        "</dict></plist>\n"
    )


def build_conf(existing, base):
    """Konfigurasi pf utama = (konfigurasi yang ada, atau bawaan sistem) + anchor kita. Idempoten."""
    text = existing if existing is not None else base
    marker = f'anchor "{ANCHOR}"'
    if marker in text:
        return text
    if text and not text.endswith("\n"):
        text += "\n"
    return text + f'{marker}\nload anchor "{ANCHOR}" from "{ANCHOR_FILE}"\n'


def check_port_args(port, mode):
    if not valid_port(port):
        raise FirewallError("port harus angka 1-65535")
    if mode not in MODES:
        raise FirewallError(f"mode harus salah satu dari {', '.join(MODES)}")
    if port in PROTECTED:
        raise FirewallError(f"port {port} dilindungi: {PROTECTED[port]}")


def pf_syntax_ok(anchor_text):
    """Validasi sintaks aturan dengan `pfctl -n` (tanpa root, tanpa memuat apa pun). Mengembalikan (ok, pesan)."""
    with tempfile.NamedTemporaryFile("w", suffix=".anchor", delete=False) as f:
        f.write(anchor_text)
        name = f.name
    try:
        r = subprocess.run(["/sbin/pfctl", "-n", "-a", ANCHOR, "-f", name], capture_output=True, text=True, timeout=15)
        msg = "\n".join(ln for ln in (r.stderr + r.stdout).splitlines() if ln.strip() and not re.search(r"flushing|present in|See /etc|ALTQ", ln))
        return r.returncode == 0, msg.replace(name, "aturan")
    finally:
        os.unlink(name)


def read_ports_file(path=PORTS_FILE):
    try:
        with open(path, encoding="utf-8") as f:
            return parse_ports(f.read(20000))
    except FileNotFoundError:
        return {}


def guarded_listing():
    """Port yang dicakup aturan block di anchor milik kita (bukti TERTULIS, bukan AKTIF)."""
    from labcli.exposure import pf_guarded_ports

    return sorted(pf_guarded_ports())


def status():
    ports = read_ports_file()
    return {
        "ports": [{"port": p, "mode": ports[p]} for p in sorted(ports)],
        "anchor_file": os.path.exists(ANCHOR_FILE),
        "loader": os.path.exists(LOADER),
        "daemon_plist": os.path.exists(PLIST),
        "port_tertulis_diblokir": guarded_listing(),
        "aktif": "tidak diketahui tanpa sudo; jalankan: lab firewall verify",
    }


class Runner:
    """Menjalankan langkah sudo. Dipisah supaya perencanaan bisa diuji dan --dry-run tidak menyentuh apa pun."""

    def __init__(self, dry=False, out=sys.stdout):
        self.dry, self.out = dry, out
        self.tmp = []

    def say(self, *a):
        print(*a, file=self.out)

    def tmpfile(self, text):
        f = tempfile.NamedTemporaryFile("w", delete=False, prefix="labfw-")
        f.write(text)
        f.close()
        self.tmp.append(f.name)
        return f.name

    def sudo(self, desc, argv, tolerate=False, capture=False):
        full = ["sudo", *argv]
        self.say(f"  - {desc}: {' '.join(full)}")
        if self.dry:
            return subprocess.CompletedProcess(full, 0, "", "")
        r = subprocess.run(full, text=True, capture_output=capture)
        if r.returncode and not tolerate:
            raise FirewallError(f"langkah gagal ({desc}), kode {r.returncode}")
        return r

    def cleanup(self):
        for n in self.tmp:
            try:
                os.unlink(n)
            except OSError:
                pass


def install_plan(ports, run, base_conf_text=None):
    """Urutan aman: pasang loader+daftar port, tulis anchor, validasi konfigurasi baru SEBELUM mengganti yang lama (dengan cadangan),
    muat, lalu daftarkan daemon boot. Gagal di tengah tidak meninggalkan konfigurasi pf rusak."""
    ts = time.strftime("%Y%m%d-%H%M%S")
    ok, msg = pf_syntax_ok(render_anchor(ports, "utun0"))
    if not ok:
        raise FirewallError(f"aturan tidak valid menurut pfctl: {msg}")
    run.say("Langkah (butuh sudo, password diketik di terminal ini):")
    run.sudo("folder loader", ["/bin/mkdir", "-p", os.path.dirname(LOADER)])
    run.sudo("pasang loader (milik root)", ["/usr/bin/install", "-o", "root", "-g", "wheel", "-m", "755", run.tmpfile(render_loader()), LOADER])
    run.sudo("simpan daftar port (milik root)", ["/usr/bin/install", "-o", "root", "-g", "wheel", "-m", "644", run.tmpfile(render_ports(ports)), PORTS_FILE])
    run.sudo("pasang daemon boot", ["/usr/bin/install", "-o", "root", "-g", "wheel", "-m", "644", run.tmpfile(render_plist()), PLIST])
    run.sudo("tulis berkas anchor (loader; pfctl boleh gagal di tahap ini)", [LOADER], tolerate=True)
    existing = None
    if os.path.exists(CONF) or run.dry:
        r = run.sudo("baca konfigurasi pf kita", ["/bin/cat", CONF], tolerate=True, capture=True)
        existing = r.stdout if (r.returncode == 0 and r.stdout) else None
    new_conf = build_conf(existing, base_conf_text if base_conf_text is not None else open(BASE_CONF).read())
    tmpconf = run.tmpfile(new_conf)
    if existing is not None:
        run.sudo("cadangkan konfigurasi lama", ["/bin/cp", "-p", CONF, f"{CONF}.bak.{ts}"])
    run.sudo("uji sintaks konfigurasi baru (tidak memuat)", ["/sbin/pfctl", "-n", "-f", tmpconf])
    run.sudo("pasang konfigurasi baru", ["/usr/bin/install", "-o", "root", "-g", "wheel", "-m", "600", tmpconf, CONF])
    run.sudo("muat aturan dan nyalakan pf", [LOADER])
    run.sudo("lepas daemon lama (boleh belum ada)", ["/bin/launchctl", "bootout", f"system/{LABEL}"], tolerate=True)
    run.sudo("daftarkan daemon boot", ["/bin/launchctl", "bootstrap", "system", PLIST])


def verify(ports, run, rustdesk=True):
    """Bukti AKTIF: pf menyala dan aturan benar-benar termuat di kernel. Butuh sudo."""
    problems = []
    r = run.sudo("status pf", ["/sbin/pfctl", "-s", "info"], tolerate=True, capture=True)
    enabled = "Status: Enabled" in (r.stdout or "")
    run.say("pf menyala" if enabled else "PERINGATAN: pf TIDAK menyala")
    if not enabled and not run.dry:
        problems.append("pf mati")
    r = run.sudo("aturan termuat", ["/sbin/pfctl", "-a", ANCHOR, "-s", "rules"], tolerate=True, capture=True)
    rules = r.stdout or ""
    for p in sorted(ports):
        hit = re.search(rf"block drop in quick proto tcp .* port (?:= )?{p}\b", rules) is not None
        run.say(f"port {p}: " + ("aturan blokir TERMUAT" if hit else "aturan blokir TIDAK ditemukan"))
        if not hit and not run.dry:
            problems.append(f"port {p} tidak terblokir")
    if rustdesk and os.path.exists(f"/etc/pf.anchors/{RUSTDESK_ANCHOR}"):
        r = run.sudo("aturan RustDesk termuat", ["/sbin/pfctl", "-a", RUSTDESK_ANCHOR, "-s", "rules"], tolerate=True, capture=True)
        ok = "21118" in (r.stdout or "")
        run.say("RustDesk 21118: " + ("aturan termuat" if ok else "aturan TIDAK ditemukan"))
        if not ok and not run.dry:
            problems.append("aturan RustDesk tidak termuat")
    return problems


def firewall_cmd(a):
    if a.mode == "status":
        import json

        print(json.dumps(status(), indent=2, ensure_ascii=False))
        return None
    run = Runner(dry=getattr(a, "dry_run", False))
    try:
        if a.mode == "verify":
            probs = verify(read_ports_file(), run)
            if probs:
                print("GAGAL:", "; ".join(probs), file=sys.stderr)
                return 1
            print("OK: aturan aktif di kernel. Uji dari perangkat LAN lain untuk bukti akhir (uji dari mesin ini lewat loopback tidak valid).")
            return None
        port = int(a.port)
        ports = read_ports_file()
        if a.mode == "block":
            check_port_args(port, a.allow)
            if ports.get(port) == a.allow:
                print(f"port {port} sudah diblokir (mode {a.allow}); memasang ulang agar pasti termuat")
            elif len(ports) >= MAX_PORTS:
                raise FirewallError(f"maksimal {MAX_PORTS} port")
            ports[port] = a.allow
        else:
            if port not in ports:
                raise FirewallError(f"port {port} tidak ada di daftar blokir (lab firewall status)")
            del ports[port]
        if not run.dry:
            r = subprocess.run(["sudo", "-v"])
            if r.returncode:
                raise FirewallError("sudo gagal; jalankan dari terminal Anda sendiri")
        install_plan(ports, run)
        if not run.dry:
            probs = verify(ports, run)
            if probs:
                print("PERINGATAN:", "; ".join(probs), file=sys.stderr)
                return 1
        if run.dry:
            print("DRY-RUN: tidak ada yang dijalankan atau diubah.")
            return None
        if a.mode == "block":
            print(f"OK: port {port} diblokir dari LAN" + (" (tailnet tetap boleh)" if a.allow == "tailnet" else "") + ".")
            print("Belum terbukti dari luar: uji dari perangkat lain di LAN yang sama (mis. pemindai port di HP pada Wi-Fi rumah).")
        else:
            print(f"OK: port {port} dibuka kembali.")
        return None
    except FirewallError as e:
        print("lab:", e, file=sys.stderr)
        return 1
    finally:
        run.cleanup()

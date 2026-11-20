import { useEffect, useState } from "react";
import { Feather, Activity, Archive, ArrowRightLeft, Boxes, Box, Calendar, Circle, CircleStop, Clock, Cog, Cpu, Database, DatabaseBackup, Eraser, FileCode, FileText, Globe, HardDrive, KeyRound, Layers, LayoutGrid, Lock, LogIn, Network, Package, Pause, Play, Power, RefreshCw, ScrollText, ShieldCheck, SlidersHorizontal, Trash2, TriangleAlert, Wrench, Zap, Terminal, Route, Settings, Gauge, Rocket, Waypoints, CalendarClock, Radio, type LucideIcon } from "lucide-react";

// Ikon dekoratif tab dan filter, dipetakan dari id supaya tab baru otomatis ikut tanpa mengubah tiap daftar.
// Setiap ikon dipilih karena relevan dengan isinya (aria-hidden: label teks tetap ada). Id tak dikenal: titik netral, bukan ikon asal.
const MAP: Record<string, LucideIcon> = {
  // filter umum
  all: LayoutGrid, php: FileCode, proxy: ArrowRightLeft, statis: FileText, apache: Feather, kustom: SlidersHorizontal, sistem: ShieldCheck, jeda: Pause,
  "Web server": Globe, PHP: FileCode, Database: Database, "Cache dan antrean": Layers, Monitoring: Activity, Jaringan: Network, "Alat pengembangan": Wrench, Sistem: ShieldCheck,
  _bermasalah: TriangleAlert, _opsional: Power,
  mariadb: Database, mysql: Database, pgsql: Database, redis: Zap,
  baru: Clock, bulan: Calendar, lama: Archive,
  running: Play, stopped: CircleStop,
  login: LogIn, fail: TriangleAlert, backup: DatabaseBackup, clean: Eraser, token: KeyRound, gagal: TriangleAlert, berjalan: RefreshCw, timeout: Clock,
  // tab halaman
  umum: Settings, domain: Globe, akses: Lock, env: ScrollText, trafik: Gauge, deploy: Rocket, rewrite: Route, ssl: ShieldCheck, publik: Radio,
  ext: Package, ini: SlidersHorizontal, fpm: Terminal,
  container: Box, compose: Layers, image: Package,
  worker: Cog, job: CalendarClock,
  keamanan: ShieldCheck, proses: Cpu, disk: HardDrive, dns: Network, tailscale: Waypoints, docker: Boxes, uninstall: Trash2,
};

export function TabIcon({ id }: { id: string }) {
  const I = MAP[id] ?? Circle;
  return <I size={id in MAP ? 16 : 8} className="tab-ic" aria-hidden="true" />;
}

/** Deretan yang digeser ke samping (bukan turun baris): tanda pudar di tepi yang masih berisi, roda mouse vertikal menggeser deretan bila bisa, tab aktif otomatis terlihat. */
export function useScrollRow(activeKey: string) {
  // Elemen disimpan di state: deret bisa baru dirender setelah data dimuat, dan efek harus berjalan ulang saat itu.
  const [el, ref] = useState<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!el) return;
    const mark = () => {
      el.dataset.left = el.scrollLeft > 2 ? "1" : "0";
      el.dataset.right = el.scrollLeft + el.clientWidth < el.scrollWidth - 2 ? "1" : "0";
    };
    const wheel = (e: WheelEvent) => {
      if (e.deltaX !== 0 || el.scrollWidth <= el.clientWidth) return;
      const max = el.scrollWidth - el.clientWidth;
      if ((e.deltaY < 0 && el.scrollLeft <= 0) || (e.deltaY > 0 && el.scrollLeft >= max)) return; // di ujung: biarkan halaman bergulir
      e.preventDefault();
      el.scrollLeft += e.deltaY;
    };
    mark();
    el.addEventListener("scroll", mark, { passive: true });
    el.addEventListener("wheel", wheel, { passive: false });
    const ro = typeof ResizeObserver !== "undefined" ? new ResizeObserver(mark) : null;
    ro?.observe(el);
    return () => {
      el.removeEventListener("scroll", mark);
      el.removeEventListener("wheel", wheel);
      ro?.disconnect();
    };
  }, [el]);
  useEffect(() => {
    const on = el?.querySelector<HTMLElement>(".on");
    if (!on || !el) return;
    const r = el;
    const a = on.getBoundingClientRect();
    const b = r.getBoundingClientRect();
    if (a.left < b.left + 24) r.scrollLeft -= b.left + 24 - a.left;
    else if (a.right > b.right - 24) r.scrollLeft += a.right - (b.right - 24);
  }, [el, activeKey]);
  return ref;
}

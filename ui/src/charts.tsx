import { useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent, type PointerEvent } from "react";
import { RANGES, fmtClock, fmtSpan, geometry, summarize, useCountUp, type Pt, type RangeKey } from "./lib";
import { decimalSep, useT } from "./i18n";

export function CountUp({ value, decimals = 0, suffix = "" }: { value: number | null; decimals?: number; suffix?: string }) {
  const t = useT();
  const v = useCountUp(value);
  if (v == null) return <>{t("fmt.na")}</>;
  return (
    <>
      {v.toFixed(decimals).replace(".", decimalSep())}
      {suffix}
    </>
  );
}

/** Cincin proporsi. Terisi dari kosong saat dipasang. */
export function Ring({ pct, label, tone = "primary" }: { pct: number | null; label: string; tone?: "primary" | "warn" | "bad" }) {
  const t = useT();
  const size = 96,
    sw = 10,
    r = (size - sw) / 2,
    c = 2 * Math.PI * r;
  const val = Math.min(100, Math.max(0, pct ?? 0));
  return (
    <svg className={`ring ring-${tone}`} width={size} height={size} viewBox={`0 0 ${size} ${size}`} role="img" aria-label={pct == null ? t("system.chart.unknown", { label }) : `${label}: ${Math.round(pct)}%`} style={{ ["--c" as string]: c }}>
      <circle cx={size / 2} cy={size / 2} r={r} className="ring-track" strokeWidth={sw} fill="none" />
      <circle cx={size / 2} cy={size / 2} r={r} className="ring-val" strokeWidth={sw} fill="none" strokeLinecap="round" strokeDasharray={c} strokeDashoffset={c * (1 - val / 100)} transform={`rotate(-90 ${size / 2} ${size / 2})`} />
    </svg>
  );
}

export const toneOf = (pct: number | null): "primary" | "warn" | "bad" => (pct != null && pct >= 90 ? "bad" : pct != null && pct >= 75 ? "warn" : "primary");

export function Sparkline({ pts, label, hi }: { pts: Pt[]; label: string; hi?: number }) {
  const t = useT();
  if (pts.length < 2) return <p className="small muted">{t("system.chart.sparkWait", { n: pts.length })}</p>;
  const top = hi ?? 100;
  const g = geometry(pts, 220, 44, 0, top, { l: 0, r: 0, t: 3, b: 3 });
  return (
    <svg className="spark" viewBox="0 0 220 44" role="img" aria-label={label}>
      <path d={g.area} className="spark-area" />
      <path d={g.line} className="spark-line" pathLength={1} />
    </svg>
  );
}

export function RangePicker({ value, onChange }: { value: RangeKey; onChange: (k: RangeKey) => void }) {
  const t = useT();
  return (
    <div className="seg" role="group" aria-label={t("system.chart.rangeAria")}>
      {RANGES.map((r) => (
        <button key={r.key} type="button" aria-pressed={value === r.key} className={value === r.key ? "on" : undefined} onClick={() => onChange(r.key)}>
          {r.label}
        </button>
      ))}
    </div>
  );
}

export interface ChartSeries {
  key: string;
  label: string;
  pts: Pt[];
  fmt: (v: number) => string;
}

const H = 250,
  PAD = { l: 56, r: 8, t: 8, b: 22 };

/** Grafik garis. Data nyata saja; kurang dari 2 titik tampil sebagai keterangan, bukan grafik palsu. */
export function LineChart({ series, rangeKey, title, domain, tickFmt }: { series: ChartSeries[]; rangeKey: RangeKey; title: string; domain: [number, number]; tickFmt: (v: number) => string }) {
  const t = useT();
  const range = RANGES.find((r) => r.key === rangeKey) ?? RANGES[1];
  const main = series[0]?.pts ?? [];
  const [cursor, setCursor] = useState<number | null>(null);
  const svg = useRef<SVGSVGElement>(null);
  const box = useRef<HTMLElement>(null);
  const [W, setW] = useState(640); // lebar viewBox = lebar kartu, supaya teks sumbu tetap 11px di ukuran berapa pun
  useEffect(() => {
    setCursor(null);
  }, [rangeKey]);
  useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    const set = () => setW(Math.max(280, Math.round(el.clientWidth)));
    set();
    const ro = new ResizeObserver(set);
    ro.observe(el);
    return () => ro.disconnect();
  }, [main.length >= 2]); // figure baru ada setelah data cukup

  if (main.length < 2) {
    return (
      <div className="state" role="status">
        <p className="state-title">{t("system.chart.notEnough")}</p>
        <div className="state-body">
          <p>{t("system.chart.notEnoughBody", { n: main.length })}</p>
        </div>
      </div>
    );
  }
  const span = main[main.length - 1][0] - main[0][0];
  const geos = series.map((s) => (s.pts.length >= 2 ? geometry(s.pts, W, H, domain[0], domain[1], PAD) : null));
  const idx = Math.min(cursor ?? main.length - 1, main.length - 1);
  const g0 = geos[0]!;
  const ticks = [0, 0.25, 0.5, 0.75, 1].map((k) => domain[0] + (domain[1] - domain[0]) * k);
  const sum = series.map((s) => ({ label: s.label, s: summarize(s.pts), fmt: s.fmt }));
  const aria = `${title}. ` + sum.map((x) => (x.s ? t("system.chart.item", { label: x.label, last: x.fmt(x.s.last), min: x.fmt(x.s.min), max: x.fmt(x.s.max) }) : "")).join(". ") + `. ${t("system.chart.arrows")}`;
  const withDate = span > 86400;

  const move = (e: PointerEvent<SVGSVGElement>) => {
    const r = svg.current?.getBoundingClientRect();
    if (!r) return;
    const x = ((e.clientX - r.left) / r.width) * W;
    let best = 0,
      d = Infinity;
    g0.xs.forEach((gx, i) => {
      const dd = Math.abs(gx - x);
      if (dd < d) {
        d = dd;
        best = i;
      }
    });
    setCursor(best);
  };
  const key = (e: KeyboardEvent<SVGSVGElement>) => {
    if (e.key === "ArrowLeft") {
      e.preventDefault();
      setCursor(Math.max(0, idx - 1));
    } else if (e.key === "ArrowRight") {
      e.preventDefault();
      setCursor(Math.min(main.length - 1, idx + 1));
    } else if (e.key === "Home") {
      e.preventDefault();
      setCursor(0);
    } else if (e.key === "End") {
      e.preventDefault();
      setCursor(main.length - 1);
    }
  };
  const at = (s: ChartSeries) => s.pts.find((p) => p[0] === main[idx][0]);

  return (
    <figure className="chart" ref={box}>
      <svg ref={svg} viewBox={`0 0 ${W} ${H}`} role="img" aria-label={aria} tabIndex={0} onPointerMove={move} onPointerLeave={() => setCursor(null)} onKeyDown={key}>
        {ticks.map((t) => {
          const y = PAD.t + (H - PAD.t - PAD.b) * (1 - (t - domain[0]) / (domain[1] - domain[0]));
          return (
            <g key={t}>
              <line x1={PAD.l} x2={W - PAD.r} y1={y} y2={y} className="grid" />
              <text x={PAD.l - 6} y={y + 4} textAnchor="end" className="axis">
                {tickFmt(t)}
              </text>
            </g>
          );
        })}
        <text x={PAD.l} y={H - 5} className="axis">
          {fmtClock(main[0][0], withDate)}
        </text>
        <text x={W - PAD.r} y={H - 5} textAnchor="end" className="axis">
          {fmtClock(main[main.length - 1][0], withDate)}
        </text>
        {geos.map(
          (g, i) =>
            g && (
              <g key={series[i].key} className={`s s${i}`}>
                {i === 0 && <path d={g.area} className="area" />}
                <path d={g.line} className="line" pathLength={1} fill="none" />
              </g>
            ),
        )}
        {cursor != null && (
          <g>
            <line x1={g0.xs[idx]} x2={g0.xs[idx]} y1={PAD.t} y2={H - PAD.b} className="cursor" />
            {geos.map((g, i) => {
              const j = series[i].pts.findIndex((p) => p[0] === main[idx][0]);
              return g && j >= 0 ? <circle key={series[i].key} cx={g.xs[j]} cy={g.ys[j]} r="4" className={`dot s${i}`} /> : null;
            })}
          </g>
        )}
      </svg>
      <figcaption className="chart-cap" aria-live="polite">
        <span className="legend">
          {series.map((s, i) => {
            const p = at(s);
            return (
              <span key={s.key} className={`lg s${i}`}>
                <i />
                {s.label}
                {p ? `: ${s.fmt(p[1])}` : ""}
              </span>
            );
          })}
        </span>
        <span className="small muted">
          {cursor != null ? fmtClock(main[idx][0], withDate) : t("chart.points", { n: main.length, span: fmtSpan(span) })}
          {span < range.sec * 0.9 ? t("chart.requested", { label: range.label }) : ""}
        </span>
      </figcaption>
    </figure>
  );
}

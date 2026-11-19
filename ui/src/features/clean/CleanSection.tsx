import { useState } from "react";
import { Eraser, Eye, HardDrive, ScanSearch, ShieldCheck, Trash2, TriangleAlert } from "lucide-react";
import { errText, get, post } from "../../api";
import { fmtBytes } from "../../lib";
import { trackJob } from "../../jobs";
import { Alert, Card, ConfirmDialog, State, useLoad } from "../../ui";
import { ageLabel, estimate, group, isStale, parseRun, runSummary, scanAge, sizeLabel, type RunResult, type Scan, type ScanRow } from "./model";

// Ikon: ScanSearch = analisis, ShieldCheck = aman, TriangleAlert = perlu persetujuan, Eye = hanya ditampilkan, Trash2 = hapus, Eraser = bersihkan pilihan.

const shortId = (id: string) => id.replace(/^(bk|cx):/, "");

interface Review {
  ids: string[];
  preview: RunResult;
}

export default function CleanSection() {
  const q = useLoad(() => get<{ scan: Scan | null }>("/api/clean"));
  const scan = q.data?.scan ?? null;
  const [picked, setPicked] = useState<Set<string>>(new Set());
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");
  const [note, setNote] = useState<{ tone: "ok" | "warn" | "bad"; text: string } | null>(null);
  const [review, setReview] = useState<Review | null>(null);
  const [careful, setCareful] = useState<ScanRow | null>(null);
  const g = group(scan);
  const now = Math.floor(Date.now() / 1000);

  const rescan = () => {
    setBusy(true);
    setErr("");
    post("/api/clean/scan", {})
      .then((r: { job: number }) =>
        trackJob(r.job, "Analisis disk", () => {
          setBusy(false);
          setPicked(new Set());
          q.reload();
        }),
      )
      .catch((e) => {
        setErr(errText(e));
        setBusy(false);
      });
  };

  const afterRun = (output: string) => {
    setNote(runSummary(parseRun(output)));
    rescan();
  };

  const toggle = (id: string) =>
    setPicked((p) => {
      const n = new Set(p);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });

  const openReview = async () => {
    setErr("");
    setBusy(true);
    try {
      const r = await post<{ preview: RunResult }>("/api/clean/preview", { ids: [...picked] });
      setReview({ ids: [...picked], preview: r.preview });
    } catch (e) {
      setErr(errText(e));
    } finally {
      setBusy(false);
    }
  };

  const est = estimate(scan, picked);

  return (
    <div className="stack">
      <Card
        title={
          <span className="with-icon">
            <ScanSearch aria-hidden="true" size={18} /> Analisis ruang disk
          </span>
        }
        action={
          <button className="btn btn-primary" onClick={rescan} disabled={busy}>
            {busy ? "Bekerja…" : scan ? "Analisis ulang" : "Analisis sekarang"}
          </button>
        }
      >
        <State loading={q.loading && !q.data} error={q.error} onRetry={q.reload}>
          {!scan ? (
            <p className="muted">Belum ada analisis. Tekan “Analisis sekarang”: panel mengukur lokasi yang dikenal (cache, folder backup lama) dan tidak mengubah apa pun. Butuh 1 sampai 3 menit.</p>
          ) : (
            <>
              <ul className="list">
                <li className="list-row"><span>Disk bebas saat analisis</span><span className="mono">{fmtBytes(scan.disk_free)}</span></li>
                <li className="list-row"><span>Aman dibersihkan (cache, dibuat ulang otomatis)</span><span className="mono">{fmtBytes(scan.safe_bytes)}</span></li>
                <li className="list-row"><span>Perlu persetujuan Anda (folder backup sekali pakai)</span><span className="mono">{fmtBytes(scan.careful_bytes)}</span></li>
              </ul>
              <p className="small muted">Dianalisis {scanAge(scan.at, now)}. Angka adalah hasil pengukuran, bukan janji: ruang yang kembali bergantung pada apa yang Anda pilih.</p>
              {isStale(scan.at, now) && <Alert tone="warn">Analisis ini lebih dari sehari. Ukuran bisa sudah berubah; analisis ulang sebelum membersihkan.</Alert>}
            </>
          )}
          {err && <Alert>{err}</Alert>}
          {note && <Alert tone={note.tone}>{note.text}</Alert>}
        </State>
      </Card>

      {scan && (
        <>
          <Card
            title={
              <span className="with-icon">
                <ShieldCheck aria-hidden="true" size={18} /> Aman dibersihkan
              </span>
            }
          >
            <p className="small muted">Semua ini dibuat ulang otomatis oleh alatnya. Harga yang dibayar hanya waktu: unduhan atau kompilasi pertama berikutnya lebih lambat.</p>
            {(() => {
              const filled = g.safe.filter((r) => r.bytes > 0).sort((x, y) => y.bytes - x.bytes);
              const empty = g.safe.filter((r) => r.bytes === 0);
              return (
                <>
                  <ul className="cgrid">
                    {filled.map((r) => (
                      <li key={r.id} className="crow">
                        <label className="clean-pick">
                          <input type="checkbox" checked={picked.has(r.id)} onChange={() => toggle(r.id)} aria-label={`Pilih ${r.label}`} />
                          <span>
                            <span className="clean-name">{r.label}</span>
                            <span className="small muted block">{r.why} {r.cost}</span>
                          </span>
                        </label>
                        <span className="mono csize">{sizeLabel(r)}</span>
                      </li>
                    ))}
                  </ul>
                  {filled.length === 0 && <p className="muted">Semua lokasi aman sudah kosong.</p>}
                  {empty.length > 0 && (
                    <details className="small muted">
                      <summary>{empty.length} lokasi sudah kosong</summary>
                      <p>{empty.map((r) => r.label).join(", ")}.</p>
                    </details>
                  )}
                </>
              );
            })()}
            <div className="sticky-bar">
              <span className="small" aria-live="polite">{picked.size === 0 ? "Belum ada yang dipilih" : `${picked.size} dipilih · ±${fmtBytes(est)}`}</span>
              <span className="row-end">
                <button className="btn" onClick={() => setPicked(new Set(g.safe.filter((r) => r.bytes > 0).map((r) => r.id)))} disabled={busy}>
                  Pilih semua
                </button>
                <button className="btn btn-primary" onClick={openReview} disabled={busy || picked.size === 0}>
                  <Eraser aria-hidden="true" size={16} /> Tinjau pembersihan
                </button>
              </span>
            </div>
          </Card>

          <Card
            title={
              <span className="with-icon">
                <TriangleAlert aria-hidden="true" size={18} /> Perlu persetujuan Anda
              </span>
            }
          >
            <p className="small muted">Folder ini dibuat saat migrasi atau unduhan, bukan backup rutin. Sebagian mungkin satu-satunya salinan; pastikan pemiliknya (agen terkait) sudah selesai. Dihapus satu per satu, dan tidak bisa dikembalikan.</p>
            {g.careful.length === 0 ? (
              <p className="muted">Tidak ada folder yang cocok.</p>
            ) : (
              <ul className="cgrid">
                {[...g.careful].sort((x, y) => y.bytes - x.bytes).map((r) => {
                  const it = r.items ?? [];
                  return (
                    <li key={r.id} className="crow crow-act">
                      <div>
                        <p className="mono clean-name">{shortId(r.id)}</p>
                        <p className="small muted">{ageLabel(r.age_days)} · {it.length ? `${it.slice(0, 3).join(", ")}${it.length > 3 ? ` +${it.length - 3} lainnya` : ""}` : "kosong"}</p>
                      </div>
                      <span className="mono csize">{sizeLabel(r)}</span>
                      <button className="btn btn-danger" onClick={() => setCareful(r)} disabled={busy}>
                        <Trash2 aria-hidden="true" size={16} /> Hapus…
                      </button>
                    </li>
                  );
                })}
              </ul>
            )}
          </Card>

          <Card
            title={
              <span className="with-icon">
                <Eye aria-hidden="true" size={18} /> Hanya ditampilkan, tidak pernah dihapus panel
              </span>
            }
          >
            <p className="small muted">Ini milik Anda, Hermes, atau layanan. Dilihat supaya Anda tahu ke mana ruang pergi; bersihkan lewat alatnya masing-masing.</p>
            <ul className="bars">
              {(() => {
                const max = Math.max(1, ...g.show.map((r) => r.bytes));
                return g.show.map((r) => (
                  <li key={r.id} className="bar-row" title={r.why}>
                    <span className="bar-name"><HardDrive aria-hidden="true" size={14} /> {r.label}</span>
                    <span className="bar-track" aria-hidden="true"><span className="bar-fill" style={{ width: `${Math.max(1, (r.bytes / max) * 100)}%` }} /></span>
                    <span className="mono bar-val">{r.complete ? fmtBytes(r.bytes) : `≥ ${fmtBytes(r.bytes)}`}</span>
                  </li>
                ));
              })()}
            </ul>
            <p className="small muted">Bagian sistem macOS (puluhan GB) tidak terbaca tanpa izin Full Disk Access, jadi tidak ikut dihitung di sini.</p>
          </Card>
        </>
      )}

      {review && (
        <ConfirmDialog
          title="Tinjau pembersihan"
          word="bersihkan"
          verb="Bersihkan"
          onClose={() => setReview(null)}
          body={
            <div className="stack">
              <p>Akan dibersihkan sekitar <strong>{fmtBytes(review.preview.bytes)}</strong>:</p>
              <ul className="bullets">
                {review.preview.items.map((i) => <li key={i.id}>{i.label}: {fmtBytes(i.bytes)}</li>)}
              </ul>
              {review.preview.skipped.length > 0 && (
                <Alert tone="warn">Dilewati: {review.preview.skipped.map((s) => `${s.id} (${s.reason})`).join("; ")}</Alert>
              )}
              <p className="small muted">Perlu verifikasi kata sandi ulang. Hanya isi cache yang dihapus, foldernya tetap ada.</p>
            </div>
          }
          onConfirm={async () => {
            const r = await post<{ job: number }>("/api/clean/run", { ids: review.ids });
            setBusy(true);
            trackJob(r.job, "Pembersihan disk", (j) => {
              setBusy(false);
              afterRun(j.output);
            });
          }}
        />
      )}

      {careful && (
        <ConfirmDialog
          title={`Hapus ${shortId(careful.id)}?`}
          word={shortId(careful.id)}
          verb="Hapus permanen"
          onClose={() => setCareful(null)}
          body={
            <div className="stack">
              <p>{sizeLabel(careful)} akan dihapus permanen. {ageLabel(careful.age_days)}.</p>
              <p className="small">{careful.cost}</p>
              <p className="small muted">Isi: {(careful.items ?? []).join(", ") || "kosong"}</p>
            </div>
          }
          onConfirm={async () => {
            const name = shortId(careful.id);
            const r = await post<{ job: number }>("/api/clean/run", { ids: [careful.id], typed: name });
            setBusy(true);
            trackJob(r.job, `Hapus ${name}`, (j) => {
              setBusy(false);
              afterRun(j.output);
            });
          }}
        />
      )}
    </div>
  );
}

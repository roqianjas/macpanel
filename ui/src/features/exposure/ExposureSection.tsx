import { useState } from "react";
import { ShieldAlert, ShieldCheck } from "lucide-react";
import { errText, fmtTime, get, post } from "../../api";
import { Alert, Card, Chip, ConfirmDialog, State, usePoll } from "../../ui";
import { displayName, isDefaultEntry, loopbackCount, statusInfo, summary, visibleItems, type ExItem, type ExView } from "./model";

// Ikon: ShieldAlert bila ada layanan yang perlu perhatian, ShieldCheck bila tidak. Keduanya menandai keadaan nyata (ada/tidak ada paparan),
// bukan hiasan; bentuknya berbeda sehingga status tidak hanya bergantung pada warna.

type Note = { tone: "ok" | "bad"; text: string } | null;

export default function ExposureSection() {
  // Data berubah tiap 5 menit di server; 60 detik cukup untuk melihat hasil setelah mengubah daftar izin.
  const q = usePoll(() => get<ExView>("/api/exposure"), 60000);
  const [ask, setAsk] = useState<ExItem | null>(null);
  const [note, setNote] = useState<Note>(null);
  const [busy, setBusy] = useState("");
  const v = q.data;
  const flagged = v ? v.items.filter((i) => i.status === "flagged").length : 0;

  const revoke = async (entry: string) => {
    setBusy(entry);
    setNote(null);
    try {
      await post("/api/exposure/revoke", { entry });
      await q.reload();
      setNote({ tone: "ok", text: `${entry} dicabut dari daftar izin.` });
    } catch (e) {
      setNote({ tone: "bad", text: errText(e) });
    } finally {
      setBusy("");
    }
  };

  return (
    <Card title={<>{flagged > 0 ? <ShieldAlert size={18} className="icon" aria-hidden="true" /> : <ShieldCheck size={18} className="icon" aria-hidden="true" />} Paparan layanan ke jaringan rumah</>}>
      <State loading={q.loading && !v} error={q.error} onRetry={q.reload}>
        {v && (
          <div className="stack">
            {note && <Alert tone={note.tone}>{note.text}</Alert>}
            <p role="status" aria-live="polite" className={flagged > 0 ? "" : "muted"}>
              {summary(v)}
            </p>
            {v.now && (
              <p className="small muted">
                Diperiksa {fmtTime(v.now.at)} dari IP LAN {v.now.lan_ip ?? "(tidak terbaca)"}, tiap {Math.round(v.interval / 60)} menit. Layanan dianggap terbuka hanya bila sambungan dari LAN benar-benar berhasil; peringatan muncul setelah terlihat {v.confirm_samples} kali berturut-turut. {loopbackCount(v.items)} layanan lain hanya di loopback.
              </p>
            )}
            {v.now && v.now.errors.length > 0 && <Alert tone="bad">{v.now.errors.join(" ")}</Alert>}
            {visibleItems(v.items).length > 0 && (
              <div className="tbl-wrap">
                <table className="tbl">
                  <caption className="sr">Layanan yang mendengar di luar loopback</caption>
                  <thead>
                    <tr>
                      <th scope="col">Layanan</th>
                      <th scope="col">Port</th>
                      <th scope="col">Alamat</th>
                      <th scope="col">Status</th>
                      <th scope="col">
                        <span className="sr">Aksi</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {visibleItems(v.items).map((i) => {
                      const s = statusInfo(i.status);
                      return (
                        <tr key={`${i.key}@${i.bind}`}>
                          <th scope="row" className="mono">
                            {displayName(i)}
                            {i.system && <span className="muted small"> (macOS)</span>}
                          </th>
                          <td className="mono">{i.port}</td>
                          <td className="mono">{i.bind === "?" ? "semua" : i.bind === "*" ? "semua antarmuka" : i.bind}</td>
                          <td>
                            <Chip tone={s.tone}>{s.label}</Chip>
                            <span className="sr"> {s.hint}</span>
                          </td>
                          <td className="cell-end">
                            {i.status === "flagged" && v.can_edit && (
                              <button className="btn btn-sm" onClick={() => setAsk(i)}>
                                Tandai disengaja<span className="sr"> {displayName(i)} port {i.port}</span>
                              </button>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
            <div>
              <h3 className="h3">Daftar izin</h3>
              <p className="small muted">Layanan yang Anda putuskan boleh terbuka. Mencabut entri membuat peringatan muncul lagi bila layanannya masih terbuka.</p>
              {v.allowed.length === 0 ? (
                <p className="small muted">Kosong.</p>
              ) : (
                <ul className="list">
                  {v.allowed.map((e) => (
                    <li key={e} className="list-row">
                      <span className="mono">
                        {e}
                        {isDefaultEntry(e, v.defaults) && <span className="muted small"> (bawaan)</span>}
                      </span>
                      {v.can_edit && (
                        <button className="btn btn-sm" disabled={busy === e} onClick={() => revoke(e)}>
                          {busy === e ? "Mencabut…" : "Cabut"}
                          <span className="sr"> {e}</span>
                        </button>
                      )}
                    </li>
                  ))}
                </ul>
              )}
              {!v.can_edit && <p className="small muted">Hanya admin yang dapat mengubah daftar izin.</p>}
            </div>
          </div>
        )}
      </State>
      {ask && (
        <ConfirmDialog
          title={`Tandai ${displayName(ask)} port ${ask.port} sebagai disengaja`}
          word="izinkan"
          verb="Tandai disengaja"
          body={
            <p>
              Peringatan untuk <span className="mono">{ask.key}</span> berhenti muncul. Layanan itu tetap terbuka ke semua perangkat di jaringan rumah; tindakan ini hanya mengubah peringatan, bukan menutup port. Untuk benar-benar menutupnya, ikat layanan ke 127.0.0.1.
            </p>
          }
          onConfirm={async () => {
            await post("/api/exposure/allow", { entry: ask.key });
            await q.reload();
            setNote({ tone: "ok", text: `${ask.key} ditambahkan ke daftar izin.` });
          }}
          onClose={() => setAsk(null)}
        />
      )}
    </Card>
  );
}

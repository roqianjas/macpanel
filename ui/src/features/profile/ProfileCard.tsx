import { useRef, useState } from "react";
import { Check, ImageUp, UserRound } from "lucide-react";
import { errText, post } from "../../api";
import { Alert, Card, Field } from "../../ui";
import { fileToLogo } from "../appearance/logo";
import LanguageSelect from "./LanguageSelect";
import { useT } from "../../i18n";
import { Avatar, AVATARS, setMe, useMe } from "./avatar";

// Ikon: UserRound = profil, ImageUp = unggah foto, Check = pilihan aktif.
export default function ProfileCard() {
  const me = useMe();
  const t = useT();
  const [name, setName] = useState(me.display_name);
  const [avatar, setAvatar] = useState(me.avatar);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ tone: "ok" | "bad"; text: string } | null>(null);
  const file = useRef<HTMLInputElement>(null);
  const dirty = name.trim() !== me.display_name || avatar !== me.avatar;
  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    setMsg(null);
    try {
      const r = await post<{ display_name: string; avatar: string }>("/api/me/profile", { display_name: name.trim(), avatar });
      setMe(r);
      setName(r.display_name);
      setMsg({ tone: "ok", text: "Profil disimpan." });
    } catch (x) {
      setMsg({ tone: "bad", text: errText(x) });
    } finally {
      setBusy(false);
    }
  };
  return (
    <Card title={<span className="with-icon"><UserRound size={18} aria-hidden="true" /> Profil</span>}>
      <form className="stack" onSubmit={save} autoComplete="off">
        <div className="profile-head">
          <Avatar name={me.user} display={name} avatar={avatar} size={72} />
          <div>
            <p className="profile-name">{name.trim() || me.user}</p>
            <p className="small muted mono">@{me.user}</p>
          </div>
        </div>
        <Field label="Nama tampilan" hint="Nama yang tampil di pojok kanan atas. Nama pengguna untuk masuk tidak berubah.">
          {(id, d) => <input id={id} aria-describedby={d} placeholder={me.user} maxLength={40} value={name} onChange={(e) => setName(e.target.value)} />}
        </Field>
        <div className="stack">
          <span className="field-label" id="av-label">Avatar</span>
          <div className="logo-row" role="group" aria-labelledby="av-label">
            <button type="button" className={`logo-opt ${avatar === "" ? "on" : ""}`} aria-pressed={avatar === ""} aria-label="Huruf awal nama" title="Huruf awal nama" onClick={() => setAvatar("")}>
              <Avatar name={me.user} display={name} size={28} />
            </button>
            {AVATARS.map(({ id, label, Icon }) => (
              <button key={id} type="button" className={`logo-opt ${avatar === `icon:${id}` ? "on" : ""}`} aria-pressed={avatar === `icon:${id}`} aria-label={label} title={label} onClick={() => setAvatar(`icon:${id}`)}>
                <Icon size={20} aria-hidden="true" />
              </button>
            ))}
            <button type="button" className={`logo-opt ${avatar.startsWith("data:") ? "on" : ""}`} aria-pressed={avatar.startsWith("data:")} aria-label="Unggah foto sendiri" title="Unggah foto sendiri" onClick={() => file.current?.click()}>
              {avatar.startsWith("data:") ? <Check size={20} aria-hidden="true" /> : <ImageUp size={20} aria-hidden="true" />}
            </button>
            <input ref={file} type="file" accept="image/png,image/jpeg,image/webp" className="sr" tabIndex={-1} aria-hidden="true" onChange={async (e) => {
              const f = e.target.files?.[0];
              e.target.value = "";
              if (!f) return;
              try {
                setMsg(null);
                const l = await fileToLogo(f, 96);
                if (l.kind === "img") setAvatar(l.src);
              } catch (x) {
                setMsg({ tone: "bad", text: x instanceof Error ? x.message : "Gagal memuat gambar." });
              }
            }} />
          </div>
          <p className="small muted">Foto (PNG, JPG, WebP, maks 2 MB) dipotong persegi dan dikecilkan ke 96 px. Disimpan di server untuk akun ini, jadi tampil di semua perangkat.</p>
        </div>
        <LanguageSelect />
        <p className="small muted">{t("lang.hint")}</p>
        {msg && <Alert tone={msg.tone}>{msg.text}</Alert>}
        <div className="row-end">
          <button className="btn btn-primary" disabled={busy || !dirty}>{busy ? "Menyimpan…" : "Simpan profil"}</button>
        </div>
      </form>
    </Card>
  );
}

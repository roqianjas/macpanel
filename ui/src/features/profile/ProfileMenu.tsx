import { useEffect, useRef, useState } from "react";
import { ChevronDown, LogOut, Settings, UserRound } from "lucide-react";
import { Avatar, useMe } from "./avatar";

/** Profil di pojok kanan atas: avatar + nama, menu berisi Profil saya, Pengaturan, dan Keluar. */
export default function ProfileMenu({ role, onLogout }: { role: string; onLogout: () => void }) {
  const me = useMe();
  const [open, setOpen] = useState(false);
  const box = useRef<HTMLDivElement>(null);
  const btn = useRef<HTMLButtonElement>(null);
  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => { if (!box.current?.contains(e.target as Node)) setOpen(false); };
    const esc = (e: KeyboardEvent) => { if (e.key === "Escape") { setOpen(false); btn.current?.focus(); } };
    document.addEventListener("mousedown", away);
    document.addEventListener("keydown", esc);
    box.current?.querySelector<HTMLElement>("[role=menuitem]")?.focus();
    return () => { document.removeEventListener("mousedown", away); document.removeEventListener("keydown", esc); };
  }, [open]);
  const shown = me.display_name || me.user;
  const arrows = (e: React.KeyboardEvent) => {
    if (e.key !== "ArrowDown" && e.key !== "ArrowUp") return;
    e.preventDefault();
    const items = [...(box.current?.querySelectorAll<HTMLElement>("[role=menuitem]") ?? [])];
    const i = items.indexOf(document.activeElement as HTMLElement);
    items[(i + (e.key === "ArrowDown" ? 1 : -1) + items.length) % items.length]?.focus();
  };
  return (
    <div className="pmenu" ref={box}>
      <button ref={btn} className="pmenu-btn" aria-haspopup="menu" aria-expanded={open} onClick={() => setOpen((o) => !o)} aria-label={`Menu profil ${shown}`}>
        <Avatar name={me.user} display={me.display_name} avatar={me.avatar} size={32} />
        <span className="pmenu-name">{shown}</span>
        <ChevronDown size={16} aria-hidden="true" className="pmenu-chev" />
      </button>
      {open && (
        <div className="pmenu-pop" role="menu" aria-label="Profil" onKeyDown={arrows}>
          <div className="pmenu-head">
            <Avatar name={me.user} display={me.display_name} avatar={me.avatar} size={40} />
            <div>
              <p className="pmenu-title">{shown}</p>
              <p className="small muted mono">@{me.user} · {role === "admin" ? "Admin" : "Hanya-baca"}</p>
            </div>
          </div>
          <a role="menuitem" className="pmenu-item" href="#/profil" onClick={() => setOpen(false)}><UserRound size={18} aria-hidden="true" /> Profil saya</a>
          <a role="menuitem" className="pmenu-item" href="#/pengaturan" onClick={() => setOpen(false)}><Settings size={18} aria-hidden="true" /> Pengaturan</a>
          <button role="menuitem" className="pmenu-item" onClick={() => { setOpen(false); onLogout(); }}><LogOut size={18} aria-hidden="true" /> Keluar</button>
        </div>
      )}
    </div>
  );
}

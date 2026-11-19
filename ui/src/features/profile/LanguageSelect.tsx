import { useId } from "react";
import { Languages } from "lucide-react";
import { errText, post } from "../../api";
import { LOCALES, setLang, useLang, useT } from "../../i18n";

/** Dropdown bahasa: <select> asli (aksesibel, keyboard, layar sentuh) dengan nama asli tiap bahasa. Bila sudah masuk, pilihan disimpan ke akun. */
export default function LanguageSelect({ signedIn = true, compact = false }: { signedIn?: boolean; compact?: boolean }) {
  const lang = useLang();
  const t = useT();
  const id = useId();
  const change = (l: string) => {
    setLang(l);
    if (signedIn) void post("/api/me/lang", { lang: l }).catch((e) => console.warn(errText(e)));
  };
  return (
    <div className={`langsel ${compact ? "langsel-compact" : ""}`}>
      <label htmlFor={id} className={compact ? "sr" : "field-label"}>{t("lang.label")}</label>
      <span className="langsel-box">
        <Languages size={16} aria-hidden="true" className="icon" />
        <select id={id} value={lang} onChange={(e) => change(e.target.value)} aria-label={t("lang.label")}>
          {Object.entries(LOCALES).map(([code, l]) => <option key={code} value={code} lang={code}>{l.native}</option>)}
        </select>
      </span>
    </div>
  );
}

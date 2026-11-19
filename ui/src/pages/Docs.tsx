import { Fragment, useMemo, useState, type ReactNode } from "react";
import { Search } from "lucide-react";
import { get } from "../api";
import { Card, PageHead, State, useLoad } from "../ui";
import { useT } from "../i18n";

interface Doc { name: string; title: string; content: string }

// ---------- Markdown aman: hanya membangun elemen React, tidak pernah menyisipkan HTML ----------

const DOC_LINK = /^(?:\.\/)?(\d\d-[a-z0-9-]+)\.md(?:#.*)?$/;

function inline(text: string, key: string): ReactNode[] {
  const out: ReactNode[] = [];
  const re = /(`[^`]+`)|(\*\*[^*]+\*\*)|(\[[^\]]+\]\([^)\s]+\))|(\*[^*\s][^*]*\*)/g;
  let last = 0;
  let m: RegExpExecArray | null;
  let i = 0;
  while ((m = re.exec(text))) {
    if (m.index > last) out.push(text.slice(last, m.index));
    const t = m[0];
    const k = `${key}-${i++}`;
    if (m[1]) out.push(<code key={k} className="mono">{t.slice(1, -1)}</code>);
    else if (m[2]) out.push(<strong key={k}>{t.slice(2, -2)}</strong>);
    else if (m[4]) out.push(<em key={k}>{t.slice(1, -1)}</em>);
    else {
      const mm = /^\[([^\]]+)\]\(([^)\s]+)\)$/.exec(t);
      const label = mm?.[1] ?? t;
      const href = mm?.[2] ?? "";
      const d = DOC_LINK.exec(href);
      if (d) out.push(<a key={k} href={`#/dokumen/${d[1]}`}>{label}</a>);
      else if (/^https?:\/\//i.test(href)) out.push(<a key={k} href={href} target="_blank" rel="noopener noreferrer">{label}</a>);
      else out.push(<Fragment key={k}>{label}</Fragment>);
    }
    last = m.index + t.length;
  }
  if (last < text.length) out.push(text.slice(last));
  return out;
}

const isSep = (s: string) => /^\s*\|?\s*:?-{2,}:?\s*(\|\s*:?-{2,}:?\s*)*\|?\s*$/.test(s);
const cells = (s: string) => s.trim().replace(/^\|/, "").replace(/\|$/, "").split("|").map((c) => c.trim());
const blockStart = (s: string) => /^(#{1,4}\s|```|\s*[-*]\s+|\s*\d+\.\s+|>\s?|---+\s*$|\|)/.test(s);

export function renderMarkdown(src: string): ReactNode[] {
  const lines = src.replace(/\r/g, "").split("\n");
  const out: ReactNode[] = [];
  let i = 0;
  let n = 0;
  while (i < lines.length) {
    const ln = lines[i];
    const key = `b${n++}`;
    if (!ln.trim()) { i++; continue; }
    if (ln.startsWith("```")) {
      const buf: string[] = [];
      i++;
      while (i < lines.length && !lines[i].startsWith("```")) buf.push(lines[i++]);
      i++;
      out.push(<pre key={key} className="mono docs-pre"><code>{buf.join("\n")}</code></pre>);
      continue;
    }
    const h = /^(#{1,4})\s+(.*)$/.exec(ln);
    if (h) {
      const L = h[1].length;
      const body = inline(h[2], key);
      out.push(L === 1 ? <h1 key={key} className="docs-h1">{body}</h1> : L === 2 ? <h2 key={key} className="docs-h2">{body}</h2> : <h3 key={key} className="docs-h3">{body}</h3>);
      i++;
      continue;
    }
    if (/^---+\s*$/.test(ln)) { out.push(<hr key={key} />); i++; continue; }
    if (ln.trim().startsWith("|") && i + 1 < lines.length && isSep(lines[i + 1])) {
      const head = cells(ln);
      i += 2;
      const rows: string[][] = [];
      while (i < lines.length && lines[i].trim().startsWith("|")) rows.push(cells(lines[i++]));
      out.push(
        <div key={key} className="tbl-wrap">
          <table className="tbl">
            <thead><tr>{head.map((c, j) => <th key={j} scope="col">{inline(c, `${key}h${j}`)}</th>)}</tr></thead>
            <tbody>{rows.map((r, a) => <tr key={a}>{r.map((c, j) => <td key={j}>{inline(c, `${key}r${a}c${j}`)}</td>)}</tr>)}</tbody>
          </table>
        </div>,
      );
      continue;
    }
    if (/^\s*[-*]\s+/.test(ln) || /^\s*\d+\.\s+/.test(ln)) {
      const ordered = /^\s*\d+\.\s+/.test(ln);
      const items: string[] = [];
      while (i < lines.length && (ordered ? /^\s*\d+\.\s+/.test(lines[i]) : /^\s*[-*]\s+/.test(lines[i]) || /^\s{2,}\S/.test(lines[i]))) {
        if (/^\s{2,}\S/.test(lines[i]) && !/^\s*[-*]\s+/.test(lines[i]) && !/^\s*\d+\.\s+/.test(lines[i])) items[items.length - 1] += " " + lines[i].trim();
        else items.push(lines[i].replace(/^\s*(?:[-*]|\d+\.)\s+/, ""));
        i++;
      }
      const li = items.map((t, j) => <li key={j}>{inline(t, `${key}l${j}`)}</li>);
      out.push(ordered ? <ol key={key} className="docs-list">{li}</ol> : <ul key={key} className="docs-list">{li}</ul>);
      continue;
    }
    if (/^>\s?/.test(ln)) {
      const buf: string[] = [];
      while (i < lines.length && /^>\s?/.test(lines[i])) buf.push(lines[i++].replace(/^>\s?/, ""));
      out.push(<blockquote key={key} className="docs-quote">{inline(buf.join(" "), key)}</blockquote>);
      continue;
    }
    const para: string[] = [ln.trim()];
    i++;
    while (i < lines.length && lines[i].trim() && !blockStart(lines[i])) para.push(lines[i++].trim());
    out.push(<p key={key}>{inline(para.join(" "), key)}</p>);
  }
  return out;
}

// ---------- halaman ----------

export default function Docs({ sub }: { sub: string }) {
  const t = useT();
  const q = useLoad(() => get<{ docs: Doc[] }>("/api/docs"));
  const [term, setTerm] = useState("");
  const docs = q.data?.docs ?? [];
  const needle = term.trim().toLowerCase();
  const shown = useMemo(() => (needle ? docs.filter((d) => d.title.toLowerCase().includes(needle) || d.content.toLowerCase().includes(needle)) : docs), [docs, needle]);
  const cur = docs.find((d) => d.name === sub) ?? (needle ? shown[0] : docs[0]);
  const body = useMemo(() => (cur ? renderMarkdown(cur.content.replace(/^\s*#\s+[^\n]*\n/, "")) : []), [cur]);
  return (
    <>
      <PageHead title={t("nav.docs")} desc={t("ops.dc.desc")} />
      <State loading={q.loading && !q.data} error={q.error} empty={!q.loading && docs.length === 0} emptyText={t("ops.dc.empty")} onRetry={q.reload}>
        <div className="docs-layout">
          <nav className="docs-nav" aria-label={t("ops.dc.list")}>
            <label className="docs-search">
              <Search size={16} aria-hidden="true" />
              <span className="sr">{t("ops.dc.search")}</span>
              <input type="search" placeholder={t("ops.search")} value={term} onChange={(e) => setTerm(e.target.value)} maxLength={80} />
            </label>
            <ul className="docs-index">
              {shown.map((d) => (
                <li key={d.name}>
                  <a href={`#/dokumen/${d.name}`} aria-current={cur?.name === d.name ? "page" : undefined}>{d.title}</a>
                </li>
              ))}
              {shown.length === 0 && <li className="small muted">{t("ops.noMatch")}</li>}
            </ul>
          </nav>
          <Card title={cur?.title ?? t("ops.dc.doc")}>
            <article className="docs-body">{body}</article>
          </Card>
        </div>
      </State>
    </>
  );
}

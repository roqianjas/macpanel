import { useCallback, useEffect, useRef, useState } from "react";
import { errText, get } from "../../api";
import type { GetPath } from "../../api.gen";
import { appendPage, emptyCursor, pagePath, type CursorPage, type CursorState } from "./paging";

/**
 * Daftar berhalaman dari server (cursor). Pencarian dikirim ke server (`q`) setelah jeda 300 ms.
 * Respons yang sudah basi (kueri berubah saat menunggu) diabaikan lewat penghitung generasi, jadi hasil lama tidak menimpa hasil baru.
 */
export function useCursorList<T extends { id: number }>(base: GetPath, limit: number) {
  const [q, setQ] = useState("");
  const [applied, setApplied] = useState("");
  const [st, setSt] = useState<CursorState<T>>(emptyCursor<T>());
  const [loading, setLoading] = useState(true);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const gen = useRef(0);

  useEffect(() => {
    const t = setTimeout(() => setApplied(q.trim()), 300);
    return () => clearTimeout(t);
  }, [q]);

  const first = useCallback(async () => {
    const my = ++gen.current;
    setLoading(true);
    try {
      const pg = await get<CursorPage<T>>(pagePath(base, { limit, q: applied }));
      if (my !== gen.current) return;
      setSt(appendPage(emptyCursor<T>(), pg));
      setError("");
    } catch (e) {
      if (my === gen.current) setError(errText(e));
    } finally {
      if (my === gen.current) setLoading(false);
    }
  }, [base, limit, applied]);

  useEffect(() => {
    void first();
  }, [first]);

  const more = useCallback(async () => {
    if (!st.next || loadingMore) return;
    const my = gen.current;
    setLoadingMore(true);
    try {
      const pg = await get<CursorPage<T>>(pagePath(base, { limit, cursor: st.next, q: applied }));
      if (my !== gen.current) return;
      setSt((cur) => appendPage(cur, pg));
      setError("");
    } catch (e) {
      if (my === gen.current) setError(errText(e));
    } finally {
      setLoadingMore(false);
    }
  }, [base, limit, applied, st.next, loadingMore]);

  return { q, setQ, searching: applied !== "", rows: st.rows, total: st.total, hasMore: st.next > 0, loading, loadingMore, error, reload: first, more };
}

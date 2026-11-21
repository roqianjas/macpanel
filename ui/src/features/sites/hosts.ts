/** Rapikan isian domain: URL lengkap yang ditempel menjadi nama host saja (tanpa skema, jalur, port, atau titik penutup), huruf kecil. */
export function normalizeHost(raw: string): string {
  let h = raw.trim().toLowerCase();
  h = h.replace(/^[a-z][a-z0-9+.-]*:\/\//, "");
  h = h.replace(/^[^@/]*@/, "");
  h = h.split(/[/?#]/)[0];
  h = h.replace(/:\d+$/, "");
  return h.replace(/\.+$/, "").trim();
}

export function normalizeHosts(text: string): string[] {
  return text.split("\n").map(normalizeHost).filter(Boolean);
}

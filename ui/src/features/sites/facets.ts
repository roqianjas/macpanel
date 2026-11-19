import type { Facet } from "../list/paging";
import type { Site } from "./types";

// Satu pilihan pada satu waktu. "Sistem" dan "Dijeda" memotong lintas jenis, jadi jumlahnya bisa tumpang tindih dengan tab jenis.
export const SITE_FACETS: Facet<Site>[] = [
  { id: "php", label: "PHP", test: (s) => s.mode === "php" },
  { id: "proxy", label: "Proxy", test: (s) => s.mode === "proxy" },
  { id: "statis", label: "Statis", test: (s) => s.mode === "statis" },
  { id: "kustom", label: "Kustom", test: (s) => s.mode === "kustom" },
  { id: "sistem", label: "Sistem", test: (s) => s.system },
  { id: "jeda", label: "Dijeda", test: (s) => !s.enabled },
];

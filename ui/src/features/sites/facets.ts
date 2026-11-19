import type { Facet } from "../list/paging";
import type { Site } from "./types";
import { t } from "../../i18n";

// Satu pilihan pada satu waktu. "Sistem" dan "Dijeda" memotong lintas jenis, jadi jumlahnya bisa tumpang tindih dengan tab jenis.
export const SITE_FACETS: Facet<Site>[] = [
  { id: "php", label: "PHP", test: (s) => s.mode === "php" },
  { id: "proxy", label: "Proxy", test: (s) => s.mode === "proxy" },
  { id: "statis", get label() { return t("dev.site.mode.statis"); }, test: (s) => s.mode === "statis" },
  { id: "apache", label: "Apache", test: (s) => s.mode === "apache" },
  { id: "kustom", get label() { return t("dev.site.mode.kustom"); }, test: (s) => s.mode === "kustom" },
  { id: "sistem", get label() { return t("nav.system"); }, test: (s) => s.system },
  { id: "jeda", get label() { return t("dev.site.paused"); }, test: (s) => !s.enabled },
];

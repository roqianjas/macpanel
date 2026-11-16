// Dimuat sebagai berkas terpisah (CSP melarang skrip inline). Mencegah kilatan tema saat halaman dibuka.
(function () {
  var pref = "system";
  try { pref = localStorage.getItem("panel-theme") || "system"; } catch (e) {}
  var dark = pref === "dark" || (pref === "system" && window.matchMedia("(prefers-color-scheme: dark)").matches);
  document.documentElement.setAttribute("data-theme", dark ? "dark" : "light");
  document.documentElement.setAttribute("data-pref", pref);
})();

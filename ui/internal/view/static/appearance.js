(() => {
  const data = document.getElementById("balemoh-font-watch");
  if (!data) return;
  const fonts = JSON.parse(data.textContent), root = document.documentElement;
  let failed = false;
  const media = matchMedia("(prefers-color-scheme: dark)");
  function applyFallback() {
    root.dataset.theme = "goshtoso";
    root.classList.toggle("dark", media.matches);
    const warning = document.getElementById("appearance-font-warning");
    if (warning) {
      warning.hidden = false;
      warning.textContent = "The saved font is unavailable. Using Goshtoso/System temporarily. Saved settings were preserved.";
    }
  }
  function fallback() {
    if (failed) return;
    failed = true;
    applyFallback();
    media.addEventListener("change", applyFallback);
  }
  document.addEventListener("alpine:initialized", () => { if (failed) applyFallback(); }, { once: true });
  for (const font of fonts) {
    const load = () => document.fonts.load("16px " + JSON.stringify(font.url ? "goshtoso-font-" + font.id.replace(/[:/.]/g, "-") : font.families[0])).then((faces) => {
      if (!faces.length) fallback();
    }, fallback);
    if (font.source === "google") {
      const link = document.createElement("link");
      link.rel = "stylesheet";
      link.href = "https://fonts.googleapis.com/css2?" + new URLSearchParams({ family: font.families[0], display: "swap" });
      link.addEventListener("load", load, { once: true });
      link.addEventListener("error", fallback, { once: true });
      document.head.append(link);
    } else load();
  }
})();

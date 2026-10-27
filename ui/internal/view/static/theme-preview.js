(() => {
  document.addEventListener("click", (event) => {
    if (event.target.closest("a")) {
      event.preventDefault();
      event.stopImmediatePropagation();
    }
  }, true);
  document.addEventListener("submit", (event) => {
    event.preventDefault();
    event.stopImmediatePropagation();
  }, true);
  let sequence = 0;
  window.addEventListener("message", (event) => {
    const message = event.data;
    if (event.origin !== location.origin || event.source !== parent || message?.type !== "balemoh:brand-preview" || message.version !== 1 || message.instanceId !== "presentation-theme-preview" || !Number.isSafeInteger(message.sequence) || message.sequence <= sequence || [message.logoID, message.iconID].some((id) => typeof id !== "string" || id !== "" && !/^upload:[A-Za-z0-9_-]{1,190}$/.test(id))) return;
    const root = document.querySelector("[data-site-logo]");
    if (!root) return;
    sequence = message.sequence;
    const custom = root.querySelector("[data-site-logo-custom]"), fallback = root.querySelector("[data-site-logo-default]");
    custom.hidden = !message.logoID;
    fallback.hidden = !!message.logoID;
    if (message.logoID) custom.src = "/icons/" + encodeURIComponent(message.logoID) + "/image";
    else custom.removeAttribute("src");
    const favicon = document.querySelector('link[rel="icon"]');
    if (favicon) favicon.href = message.iconID ? "/icons/" + encodeURIComponent(message.iconID) + "/image" : "/ui/brand/favicon.svg";
  });
})();

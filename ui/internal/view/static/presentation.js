(() => {
  function mount() {
    const root = document.querySelector("[data-presentation-themes]");
    if (!root || root.dataset.mounted) return;
    root.dataset.mounted = "true";
    const config = JSON.parse(root.dataset.presentationThemes), editor = document.getElementById("presentation-theme-editor"), form = document.getElementById("presentation-save"), selection = root.querySelector("[data-presentation-selection]"), name = root.querySelector("[data-presentation-name]"), mode = form.elements.namedItem("colorMode"), feedback = root.querySelector("[data-presentation-feedback]"), save = root.querySelector("[data-presentation-save]");
    let selected = config.selectedID, definition = JSON.parse(editor.querySelector("[data-theme-definition]").value || JSON.stringify(config.items.find((i) => i.id === selected).definition)), dirty = false, customized = false, valid = true, newID = config.creating ? "theme:" + crypto.randomUUID() : null, pending = null;
    const branding = {
      logo: { id: config.items.find((item2) => item2.id === selected)?.brandLogoId || "", prefix: "brand", label: "Logo", uploading: false, pending: false },
      icon: { id: config.items.find((item2) => item2.id === selected)?.brandIconId || "", prefix: "brand-icon", label: "Icon", uploading: false, pending: false }
    };
    let brandSequence = 0;
    const brandBlocked = () => Object.values(branding).some((image) => image.uploading || image.pending);
    function previewBrand() {
      const logo = root.querySelector("[data-brand-logo-preview]"), icon = root.querySelector("[data-brand-icon-preview]"), fallback = root.querySelector("[data-brand-default-preview]");
      const logoURL = branding.logo.id ? "/icons/" + encodeURIComponent(branding.logo.id) + "/image" : "", iconURL = branding.icon.id ? "/icons/" + encodeURIComponent(branding.icon.id) + "/image" : "/ui/brand/favicon.svg";
      if (logo) {
        logo.hidden = !branding.logo.id;
        if (branding.logo.id) logo.src = logoURL;
        else logo.removeAttribute("src");
      }
      if (icon) icon.src = iconURL;
      if (fallback) fallback.hidden = !!branding.logo.id;
      for (const image of Object.values(branding)) {
        const control = root.querySelector(`[data-${image.prefix}-reset-control]`);
        if (control) control.hidden = !image.id;
      }
      const frame = document.getElementById("presentation-theme-preview");
      frame?.contentWindow?.postMessage({ type: "balemoh:brand-preview", version: 1, instanceId: frame.id, sequence: ++brandSequence, logoID: branding.logo.id, iconID: branding.icon.id }, location.origin);
    }
    window.addEventListener("message", (event) => {
      const frame = document.getElementById("presentation-theme-preview"), message = event.data;
      if (event.origin === location.origin && frame && event.source === frame.contentWindow && message?.type === "goshtoso:theme-ready" && message.version === 1 && message.instanceId === frame.id) previewBrand();
    });
    previewBrand();
    const dialog = document.getElementById("presentation-confirm");
    let confirmation = null, accepted = false;
    let previewMode = "light", inspected = false, syncingPreview = false;
    const systemMode = matchMedia("(prefers-color-scheme: dark)");
    function syncPreview() {
      if (inspected) return;
      syncingPreview = true;
      editor.dispatchEvent(new CustomEvent("themeeditor:load", { detail: { id: "presentation-theme-editor", previewMode: mode.value === "system" ? systemMode.matches ? "dark" : "light" : mode.value } }));
      syncingPreview = false;
    }
    const closeConfirmation = () => window.dispatchEvent(new CustomEvent("modal:close", { detail: { id: dialog.id } }));
    dialog.querySelector("[data-presentation-confirm-accept]").addEventListener("click", () => {
      accepted = true;
      closeConfirmation();
    });
    dialog.querySelector("[data-presentation-confirm-cancel]").addEventListener("click", closeConfirmation);
    dialog.addEventListener("close", () => {
      const resolve = confirmation;
      confirmation = null;
      resolve?.(accepted);
    });
    const confirmChange = (message) => new Promise((resolve) => {
      if (confirmation) {
        resolve(false);
        return;
      }
      accepted = false;
      confirmation = resolve;
      dialog.querySelector("[data-presentation-confirm-message]").textContent = message;
      window.dispatchEvent(new CustomEvent("modal:open", { detail: { id: dialog.id } }));
    });
    const item = () => config.items.find((i) => i.id === selected);
    const title = () => item()?.name || "Custom";
    const setName = () => {
      name.value = config.creating ? title() + " custom" : title();
    };
    setName();
    const feedbackError = (target, e) => {
      target.textContent = (e.message || "The operation could not be confirmed.") + (e.usedBy?.length ? " Used by " + e.usedBy.join(", ") + "." : "");
    };
    function conflict(e) {
      const box = root.querySelector("[data-presentation-conflict]");
      box.hidden = false;
      const inspect = root.querySelector("[data-presentation-inspect]");
      inspect.href = "/presentation/conflict" + (item()?.revision ? "?themeID=" + encodeURIComponent(selected) : "");
    }
    function dirtyState() {
      dirty = true;
      save.disabled = !valid || brandBlocked();
    }
    async function request(url, options) {
      let response;
      try {
        response = await fetch(url, { ...options, headers: { Accept: "application/json", ...options.headers || {} } });
      } catch {
        throw { code: "unavailable", message: "Outcome unknown. Keep your draft and retry the same command or inspect the saved version." };
      }
      let body;
      try {
        body = await response.json();
      } catch {
        throw { code: "unavailable", message: "Response not confirmed. Keep your draft and inspect the saved version." };
      }
      if (!response.ok) throw body;
      return body;
    }
    editor.addEventListener("themeeditor:change", (e) => {
      if (e.detail.id !== "presentation-theme-editor") return;
      if (e.detail.previewMode && e.detail.previewMode !== previewMode) {
        if (!syncingPreview) inspected = true;
        previewMode = e.detail.previewMode;
      }
      definition = e.detail.definition;
      valid = e.detail.valid;
      if (e.detail.dirty) {
        customized = true;
        dirtyState();
      }
      save.disabled = !valid || brandBlocked();
    });
    selection.addEventListener("change", async () => {
      const previous = selected;
      if (dirty && !await confirmChange("Discard unsaved changes to this draft?")) {
        selection.value = previous;
        selection.dispatchEvent(new Event("input", { bubbles: true }));
        return;
      }
      const chosen = config.items.find((i) => i.id === selection.value);
      if (!chosen) return;
      selected = chosen.id;
      definition = JSON.parse(JSON.stringify(chosen.definition));
      branding.logo.id = chosen.brandLogoId || "";
      branding.icon.id = chosen.brandIconId || "";
      for (const image of Object.values(branding)) {
        image.pending = false;
        const file = root.querySelector(`[data-${image.prefix}-file]`);
        if (file) {
          file.value = "";
          file.dispatchEvent(new Event("change", { bubbles: true }));
        }
      }
      previewBrand();
      customized = false;
      newID = config.creating ? "theme:" + crypto.randomUUID() : null;
      pending = null;
      valid = true;
      setName();
      editor.dispatchEvent(new CustomEvent("themeeditor:load", { detail: { id: "presentation-theme-editor", definition, dirty: false } }));
      dirtyState();
    });
    root.querySelector("[data-presentation-mode]")?.addEventListener("change", () => {
      dirtyState();
      syncPreview();
    });
    syncPreview();
    name.addEventListener("input", () => {
      customized = true;
      dirtyState();
    });
    form.addEventListener("submit", async (e) => {
      e.preventDefault();
      if (!valid || brandBlocked()) return;
      let chosen;
      if (newID || !item()?.revision && customized) {
        newID ||= "theme:" + crypto.randomUUID();
        chosen = { kind: "newCustom", id: newID, name: name.value.trim(), definition, brandIconId: branding.icon.id, brandLogoId: branding.logo.id, expectedCatalogRevision: config.catalogRevision };
      } else if (item()?.revision) {
        chosen = { kind: "custom", id: selected, expectedThemeRevision: item().revision };
        if (customized) chosen.draft = { name: name.value.trim(), definition, brandIconId: branding.icon.id, brandLogoId: branding.logo.id };
      } else chosen = { kind: "builtin", key: selected.slice("builtin:".length), expectedCatalogRevision: config.catalogRevision };
      const payload = { expectedSettingsRevision: config.settingsRevision, colorMode: mode.value, selection: chosen }, serialized = JSON.stringify(payload);
      if (pending?.serialized !== serialized) pending = { serialized, command: { ...payload, commandId: crypto.randomUUID() } };
      save.disabled = true;
      feedback.textContent = "Saving\u2026";
      try {
        await request("/presentation/save", { method: "POST", headers: { "Content-Type": "application/json" }, body: JSON.stringify(pending.command) });
        dirty = false;
        editor.dispatchEvent(new CustomEvent("themeeditor:committed", { detail: { id: "presentation-theme-editor" } }));
        history.replaceState(null, "", "/presentation#themes");
        location.reload();
      } catch (error) {
        feedbackError(feedback, error);
        if (error.code === "conflict" || error.code === "unavailable") conflict(error);
        save.disabled = !valid || brandBlocked();
      }
    });
    root.querySelector("[data-presentation-reload]").addEventListener("click", async () => {
      if (dirty && !await confirmChange("Discard the draft and load the saved version?")) return;
      dirty = false;
      location.reload();
    });
    window.addEventListener("presentation-delete-theme", async () => {
      if (config.creating || !item()?.revision) return;
      if (!await confirmChange("Delete this saved theme? Unsaved changes to this draft will be discarded.")) return;
      const action = document.getElementById("theme-primary-delete"), message = document.querySelector("[data-theme-delete-feedback]");
      action.disabled = true;
      message.textContent = "";
      try {
        await request("/presentation/themes/" + encodeURIComponent(selected) + "/delete", { method: "POST", body: new URLSearchParams({ expectedRevision: item().revision }) });
        dirty = false;
        editor.dispatchEvent(new CustomEvent("themeeditor:committed", { detail: { id: "presentation-theme-editor" } }));
        location.assign(config.recovery ? "/presentation?appearance=recovery#themes" : "/presentation#themes");
      } catch (error) {
        feedbackError(message, error);
      } finally {
        action.disabled = false;
      }
    });
    function syncBrandControls() {
      const uploading = Object.values(branding).some((image) => image.uploading);
      for (const image of Object.values(branding)) {
        const file = root.querySelector(`[data-${image.prefix}-file]`), reset = root.querySelector(`[data-${image.prefix}-reset]`);
        if (file) file.disabled = uploading;
        if (reset) reset.disabled = uploading;
      }
      const base = document.getElementById("presentation-theme-selection-trigger");
      if (base) base.disabled = uploading;
      save.disabled = !valid || brandBlocked();
    }
    for (const image of Object.values(branding)) {
      let clearBrandError2 = function() {
        fileInput.removeAttribute("aria-invalid");
        fieldFeedback.classList.remove("text-danger");
        fieldFeedback.textContent = "";
        dismiss.hidden = true;
      }, showBrandError2 = function(message) {
        fileInput.value = "";
        fileInput.dispatchEvent(new Event("change", { bubbles: true }));
        fileInput.setAttribute("aria-invalid", "true");
        fieldFeedback.classList.add("text-danger");
        fieldFeedback.textContent = message;
        dismiss.hidden = false;
        window.dispatchEvent(new CustomEvent("notify", { detail: { kind: "toast", tone: "danger", title: image.label + " upload failed", message } }));
      };
      var clearBrandError = clearBrandError2, showBrandError = showBrandError2;
      const fileInput = root.querySelector(`[data-${image.prefix}-file]`), fieldFeedback = root.querySelector(`[data-${image.prefix}-feedback]`), dismiss = root.querySelector(`[data-${image.prefix}-dismiss]`);
      if (!fileInput) continue;
      dismiss.addEventListener("click", () => {
        image.pending = false;
        clearBrandError2();
        dirtyState();
        fileInput.focus();
      });
      root.querySelector(`[data-${image.prefix}-reset]`).addEventListener("click", () => {
        image.id = "";
        image.pending = false;
        fileInput.value = "";
        fileInput.dispatchEvent(new Event("change", { bubbles: true }));
        customized = true;
        fieldFeedback.textContent = "Default " + image.label.toLowerCase() + " restored in this draft. Save to apply.";
        previewBrand();
        dirtyState();
        fileInput.focus();
      });
      fileInput.addEventListener("change", async () => {
        clearBrandError2();
        const file = fileInput.files[0];
        if (!file) return;
        image.pending = true;
        dirtyState();
        if (file.size > 2097152) {
          showBrandError2("The image exceeds 2 MiB.");
          return;
        }
        image.uploading = true;
        syncBrandControls();
        fieldFeedback.textContent = "Uploading " + image.label.toLowerCase() + "\u2026";
        try {
          const body = new FormData();
          body.append("image", file);
          const result = await request("/presentation/brand/upload", { method: "POST", body });
          image.id = result.id;
          image.pending = false;
          fileInput.value = "";
          fileInput.dispatchEvent(new Event("change", { bubbles: true }));
          customized = true;
          dirtyState();
          previewBrand();
          fieldFeedback.textContent = image.label + " uploaded. Save to apply globally.";
        } catch (error) {
          showBrandError2(error.message || "The upload could not be confirmed. Keep your draft and retry.");
        } finally {
          image.uploading = false;
          syncBrandControls();
        }
      });
    }
    document.addEventListener("click", async (e) => {
      const link = e.target.closest("a");
      if (!dirty || !link || e.defaultPrevented || e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || link.hasAttribute("download") || link.target && link.target !== "_self") return;
      const destination = new URL(link.href, location.href);
      if (destination.origin === location.origin && destination.pathname === location.pathname && destination.search === location.search && destination.hash) return;
      e.preventDefault();
      if (!await confirmChange("Discard the draft and leave this page?")) return;
      dirty = false;
      location.assign(destination.href);
    });
    window.addEventListener("beforeunload", (e) => {
      if (dirty) {
        e.preventDefault();
        e.returnValue = "";
      }
    });
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", mount, { once: true });
  else mount();
})();

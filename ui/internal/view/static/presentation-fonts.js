(() => {
  function mount() {
    const root = document.querySelector("[data-presentation-fonts]");
    if (!root || root.dataset.mounted) return;
    root.dataset.mounted = "true";
    const upload = root.querySelector("[data-presentation-upload]"), uploadID = upload.elements.namedItem("id"), dialog = document.getElementById("presentation-font-confirm");
    let identity = "", confirmation = null, accepted = false;
    uploadID.value = "font:" + crypto.randomUUID();
    const close = () => window.dispatchEvent(new CustomEvent("modal:close", { detail: { id: dialog.id } }));
    dialog.querySelector("[data-font-confirm]").addEventListener("click", () => {
      accepted = true;
      close();
    });
    dialog.querySelector("[data-font-cancel]").addEventListener("click", close);
    dialog.addEventListener("close", () => {
      const resolve = confirmation;
      confirmation = null;
      resolve?.(accepted);
    });
    const confirmDelete = () => new Promise((resolve) => {
      if (confirmation) {
        resolve(false);
        return;
      }
      accepted = false;
      confirmation = resolve;
      window.dispatchEvent(new CustomEvent("modal:open", { detail: { id: dialog.id } }));
    });
    async function request(url, body) {
      let response, result;
      try {
        response = await fetch(url, { method: "POST", headers: { Accept: "application/json" }, body });
        result = await response.json();
      } catch {
        throw { message: "Outcome unknown. Keep this form and retry the same request." };
      }
      if (!response.ok) throw result;
      return result;
    }
    const showError = (message, error) => {
      message.textContent = (error.message || "The operation could not be confirmed.") + (error.usedBy?.length ? " Used by " + error.usedBy.join(", ") + "." : "");
    };
    upload.addEventListener("change", () => {
      const file = upload.elements.namedItem("file").files[0], next = upload.elements.namedItem("name").value + "@" + file?.name + "@" + file?.size + "@" + file?.lastModified;
      if (identity && identity !== next) uploadID.value = "font:" + crypto.randomUUID();
      identity = next;
    });
    upload.addEventListener("submit", async (event) => {
      event.preventDefault();
      const file = upload.elements.namedItem("file").files[0], message = upload.querySelector("[data-management-feedback]"), button = upload.querySelector('button[type="submit"]');
      if (!file) return;
      if (file.size > 10485760) {
        message.textContent = "The file exceeds 10 MiB.";
        return;
      }
      button.disabled = true;
      message.textContent = "Uploading\u2026";
      try {
        await request(upload.action, new FormData(upload));
        location.reload();
      } catch (error) {
        showError(message, error);
      } finally {
        button.disabled = false;
      }
    });
    root.querySelectorAll("[data-font-delete]").forEach((form) => form.addEventListener("submit", async (event) => {
      event.preventDefault();
      if (!await confirmDelete()) return;
      const message = form.querySelector("[data-management-feedback]"), button = form.querySelector('button[type="submit"]');
      button.disabled = true;
      try {
        await request(form.action, new URLSearchParams(new FormData(form)));
        location.reload();
      } catch (error) {
        showError(message, error);
      } finally {
        button.disabled = false;
      }
    }));
  }
  if (document.readyState === "loading") document.addEventListener("DOMContentLoaded", mount, { once: true });
  else mount();
})();

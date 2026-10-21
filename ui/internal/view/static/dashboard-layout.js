// Masonry belongs to the dashboard, independent of each widget's component.
(() => {
  if (!('ResizeObserver' in window)) return;
  let grid;
  let frame;
  const observed = new Set();
  const schedule = () => {
    if (!frame) frame = requestAnimationFrame(layout);
  };
  const sizes = new ResizeObserver(schedule);
  const contents = new MutationObserver(schedule);

  function layout() {
    frame = null;
    const next = document.getElementById('dashboard-grid');
    if (next !== grid) {
      sizes.disconnect();
      contents.disconnect();
      observed.clear();
      grid = next;
      if (grid) contents.observe(grid, { childList: true, subtree: true, characterData: true });
    }
    if (!grid) return;
    const layouts = [...grid.querySelectorAll('[data-dashboard-layout]')];
    const widgets = layouts.flatMap(layout => [...layout.children].filter(item => item.hasAttribute('data-dashboard-widget')));
    const current = new Set(widgets);
    for (const item of observed) {
      if (!current.has(item)) {
        sizes.unobserve(item);
        observed.delete(item);
      }
    }
    for (const layout of layouts) {
      const items = [...layout.children].filter(item => item.hasAttribute('data-dashboard-widget'));
      layout.classList.toggle('balemoh-masonry', items.length > 0);
      // Keep each section in source order, without moving nodes between columns.
      const gap = parseFloat(getComputedStyle(layout).columnGap) || 0;
      // offsetHeight is in layout pixels, independent of zoom and refresh transforms.
      const row = parseFloat(getComputedStyle(layout).gridAutoRows) || 1;
      const spans = items.map(item => Math.ceil((item.offsetHeight + gap) / row));
      items.forEach((item, index) => {
        const span = `span ${Math.max(1, spans[index])}`;
        if (item.style.gridRowEnd !== span) item.style.gridRowEnd = span;
        if (!observed.has(item)) {
          sizes.observe(item);
          observed.add(item);
        }
      });
    }
  }

  // Server snapshots do not own the client-computed masonry geometry. Removing
  // these attributes during a morph temporarily collapses the grid and clamps
  // the shell's scroll position before ResizeObserver can repair it.
  window.goshtosoDependencies?.ready.then(() => {
    window.htmx.registerExtension('balemoh-dashboard-layout', {
      htmx_before_morph_attr(element, detail) {
        if (element.matches('[data-dashboard-widget]') && detail.attrName === 'style') return false;
        if (element.matches('[data-dashboard-layout]') && detail.attrName === 'class') return false;
      },
    });
  });

  document.addEventListener('htmx:after:swap', schedule);
  // HTMX settlement can restore dimensions after the swap measurement.
  document.addEventListener('htmx:after:settle', schedule);
  window.addEventListener('resize', schedule);
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', schedule, { once: true });
  } else {
    schedule();
  }
})();

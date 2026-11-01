(() => {
  const initialize = () => {
    window.htmx.registerExtension('balemoh-dashboard-layout', {
      htmx_before_morph_attr(element, detail) {
        if (element.matches('details[data-dashboard-disclosure]') && detail.attrName === 'open') return false;
      },
    });
  };
  if (window.htmx) initialize(); else window.goshtosoDependencies?.ready.then(initialize);
})();

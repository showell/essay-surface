// Draws each diagram of a .note's page from its dot source, with viz.js,
// which the page loads first. A diagram is a figure.dot holding its source as
// code; the drawing takes the code's place in the figure, so the figure keeps
// its data-c mark, its reply button and its replies.
//
// DOT_DEFAULTS fill in only what a graph leaves unset, and an attribute in the
// dot source wins. They are essay-repl-server's (assets/dot.js), copied, so a
// diagram looks the same on both surfaces. Under node this file gives them to
// scripts/draw_diagrams.cjs, so the check draws what a reader sees.
(function () {
  "use strict";
  var DOT_DEFAULTS = {
    graphAttributes: { bgcolor: "transparent", fontname: "Helvetica" },
    nodeAttributes: { fontname: "Helvetica", fontsize: 11 },
    edgeAttributes: { fontname: "Helvetica", fontsize: 10 }
  };
  if (typeof module !== "undefined" && module.exports) {
    module.exports = DOT_DEFAULTS;
    return;
  }
  Viz.instance().then(function (viz) {
    document.querySelectorAll("figure.dot > pre > code.language-dot").forEach(function (code) {
      var pre = code.parentElement;
      try {
        pre.replaceWith(viz.renderSVGElement(code.textContent, DOT_DEFAULTS));
      } catch (e) {
        var err = document.createElement("div");
        err.className = "dot-error";
        err.textContent = "graphviz: " + e.message;
        pre.after(err);
      }
    });
  }).catch(function (e) {
    console.error("viz.js did not load", e);
  });
})();

// Copy buttons. Each code block on a note's page gets a "Copy" button in its
// top right corner, which puts the block's text on the clipboard exactly as
// the note has it: the code's text, which the browser gives unescaped, less
// the one newline the renderer ends every block with. A diagram's block is
// drawn over by diagrams.js and gets no button.
//
// The live surface is plain http on an IP address, which is not a secure
// context, so the browser gives it no navigator.clipboard. The copy tries
// the clipboard API, then a hidden textarea and document.execCommand("copy"),
// which still works in a click there; when both fail it selects the block's
// text, so a Ctrl+C copies it.
//
// The button sits in a pin of no height just before its pre, as a code
// block's reply button sits in one after it: inside the pre it would scroll
// sideways with a long line. Putting it before leaves the block's next
// sibling, which comments.js reads as its thread, as it was.
//
// The page runs this with a document; copy_test.cjs requires it under node
// for its pure half.
(function () {
  "use strict";

  // The text a code block copies, from its code element's text.
  function sourceOf(codeText) {
    return codeText.endsWith("\n") ? codeText.slice(0, -1) : codeText;
  }

  // Copies text by the first way that works, and resolves to which:
  // "clipboard", "textarea" or "none". ways.clipboard is navigator.clipboard
  // or undefined; ways.textarea(text) copies through a hidden textarea and
  // says whether the browser did.
  function copyText(text, ways) {
    function fallback() {
      try {
        return ways.textarea(text) ? "textarea" : "none";
      } catch (e) {
        return "none";
      }
    }
    if (ways.clipboard && typeof ways.clipboard.writeText === "function") {
      var written;
      try {
        written = ways.clipboard.writeText(text);
      } catch (e) {
        return Promise.resolve(fallback());
      }
      return Promise.resolve(written).then(function () { return "clipboard"; }, fallback);
    }
    return Promise.resolve(fallback());
  }

  // What the button says after each way, and for how long.
  var SAID = { clipboard: "Copied", textarea: "Copied", none: "Selected: Ctrl+C" };
  var SAID_MS = { clipboard: 1200, textarea: 1200, none: 3000 };

  function textareaCopy(text) {
    var area = document.createElement("textarea");
    area.value = text;
    area.setAttribute("readonly", "");
    area.style.position = "fixed";
    area.style.top = "0";
    area.style.left = "0";
    area.style.opacity = "0";
    document.body.append(area);
    area.select();
    try {
      return document.execCommand("copy");
    } finally {
      area.remove();
    }
  }

  function selectBlock(code) {
    var range = document.createRange();
    range.selectNodeContents(code);
    var sel = window.getSelection();
    sel.removeAllRanges();
    sel.addRange(range);
  }

  function wire(pre) {
    var code = pre.querySelector("code") || pre;
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "copybtn";
    btn.textContent = "Copy";
    btn.setAttribute("aria-label", "Copy this code");
    var timer = 0;
    btn.addEventListener("click", function () {
      copyText(sourceOf(code.textContent), { clipboard: navigator.clipboard, textarea: textareaCopy })
        .then(function (way) {
          if (way === "none") selectBlock(code);
          btn.focus();
          btn.textContent = SAID[way];
          clearTimeout(timer);
          timer = setTimeout(function () { btn.textContent = "Copy"; }, SAID_MS[way]);
        });
    });
    var pin = document.createElement("div");
    pin.className = "copypin";
    pin.append(btn);
    pre.before(pin);
  }

  if (typeof document !== "undefined") {
    document.querySelectorAll("article pre").forEach(function (pre) {
      if (!pre.closest("figure.dot")) wire(pre);
    });
  }
  if (typeof module !== "undefined") {
    module.exports = { sourceOf: sourceOf, copyText: copyText };
  }
})();

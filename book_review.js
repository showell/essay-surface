// The review widget on a staged book's page (books.go). A quiet Reply
// button in the corner opens a box for a reply to the whole chapter: a text
// and Send, so Tab then Enter sends it. The page's Next link asks for the
// chapter's stars before it leaves: a star (or a key 1 to 5) sends them and
// goes on, Skip (or Enter) goes on with no opinion given, and Escape stays.
// The server gives BOOK_REVIEW = {book, page}.
(function () {
  "use strict";
  var NAME_KEY = "essay-surface-name";
  var MARK = "data-essay-book-review";

  function storedName() {
    try { return localStorage.getItem(NAME_KEY) || ""; } catch (e) { return ""; }
  }
  function storeName(n) {
    try { localStorage.setItem(NAME_KEY, n); } catch (e) { /* a private window: not remembered */ }
  }

  // The account this browser is logged in as signs the reply, so the box
  // asks for no name; over loopback there is none, and it asks.
  var account = "";
  fetch("/whoami").then(function (r) { return r.ok ? r.json() : { name: "" }; })
    .then(function (who) { account = who.name || ""; })
    .catch(function () { account = ""; });

  function el(tag, cls, text) {
    var e = document.createElement(tag);
    if (cls) e.className = cls;
    if (text) e.textContent = text;
    return e;
  }

  var h1 = document.querySelector("h1");
  var title = h1 ? h1.textContent.trim() : document.title;

  var btn = el("button", "ebr-open", "Reply");
  btn.type = "button";
  btn.setAttribute(MARK, "");
  btn.setAttribute("aria-label", "Reply to this chapter");

  var dlg = el("dialog", "ebr-dialog");
  dlg.setAttribute(MARK, "");
  var form = el("form", "ebr-form");
  form.method = "dialog";
  var head = el("div", "ebr-head", "Reply to " + title);
  var name = el("input", "ebr-name");
  name.placeholder = "Your name"; name.maxLength = 80; name.autocomplete = "name";
  var text = el("textarea", "ebr-text");
  text.rows = 8; text.placeholder = "Your reply to the whole chapter";

  function post(body) {
    body.page = BOOK_REVIEW.page;
    return fetch("/books/" + encodeURIComponent(BOOK_REVIEW.book) + "/replies", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    }).then(function (r) {
      if (r.status === 201) return r.json();
      return r.text().then(function (t) { throw new Error(t.trim() || ("the server said " + r.status)); });
    });
  }

  var row = el("div", "ebr-row");
  var send = el("button", "ebr-send", "Send");
  send.type = "submit";
  var cancel = el("button", "ebr-cancel", "Cancel");
  cancel.type = "button";
  var err = el("span", "ebr-err");
  err.setAttribute("role", "status");
  row.append(send, cancel, err);
  form.append(head, name, text, row);
  dlg.append(form);

  function open() {
    err.textContent = "";
    send.disabled = false;
    name.value = account || storedName();
    name.hidden = !!account;
    dlg.showModal();
    (name.hidden || name.value ? text : name).focus();
  }
  btn.addEventListener("click", open);
  cancel.addEventListener("click", function () { dlg.close(); });

  form.addEventListener("submit", function (ev) {
    ev.preventDefault();
    send.disabled = true;
    err.textContent = "";
    post({ stars: null, text: text.value, name: name.value }).then(function () {
      if (!account) storeName(name.value.trim());
      text.value = "";
      dlg.close();
      btn.textContent = "Sent";
      setTimeout(function () { btn.textContent = "Reply"; }, 2500);
    }).catch(function (e) {
      err.textContent = e.message;
      send.disabled = false;
    });
  });

  // The rating, asked as the reader leaves by the Next link.
  var rate = el("dialog", "ebr-dialog ebr-rate");
  rate.setAttribute(MARK, "");
  var rateHead = el("div", "ebr-head", "You just read " + title + ". How many stars?");
  var rateRow = el("div", "ebr-stars");
  rateRow.setAttribute("role", "group");
  rateRow.setAttribute("aria-label", "Stars");
  var rateErr = el("span", "ebr-err");
  rateErr.setAttribute("role", "status");
  var skip = el("button", "ebr-cancel", "Skip");
  skip.type = "button";
  var nextHref = null;

  function goOn() { if (nextHref) window.location.href = nextHref; }
  function rateWith(n) {
    rateErr.textContent = "";
    post({ stars: n, text: "", name: account || storedName() }).then(goOn).catch(function (e) {
      rateErr.textContent = e.message;
    });
  }
  for (var i = 1; i <= 5; i++) {
    (function (n) {
      var b = el("button", "ebr-star", "☆");
      b.type = "button";
      b.title = n + (n === 1 ? " star" : " stars");
      b.setAttribute("aria-label", b.title);
      b.addEventListener("mouseenter", function () {
        rateRow.querySelectorAll(".ebr-star").forEach(function (s, j) { s.textContent = j < n ? "★" : "☆"; });
      });
      b.addEventListener("click", function () { rateWith(n); });
      rateRow.append(b);
    })(i);
  }
  rateRow.addEventListener("mouseleave", function () {
    rateRow.querySelectorAll(".ebr-star").forEach(function (s) { s.textContent = "☆"; });
  });
  skip.addEventListener("click", goOn);
  rate.addEventListener("keydown", function (ev) {
    if (ev.key >= "1" && ev.key <= "5") { ev.preventDefault(); rateWith(Number(ev.key)); }
  });
  var rateFoot = el("div", "ebr-row");
  rateFoot.append(skip, rateErr);
  rate.append(rateHead, rateRow, rateFoot);

  var next = document.querySelector('a[rel="next"]');
  if (next) {
    next.addEventListener("click", function (ev) {
      if (ev.ctrlKey || ev.metaKey || ev.shiftKey || ev.button !== 0) return;
      ev.preventDefault();
      nextHref = next.href;
      rateErr.textContent = "";
      rate.showModal();
      skip.focus();
    });
  }

  document.body.append(btn, dlg, rate);
})();

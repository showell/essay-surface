// The review widget on a staged book's page (books.go): a quiet Reply
// button in the corner opens a box for a reply to the whole chapter, with
// five stars that start unset. A click on a star sets it; a click on the
// star already set clears them. The server gives BOOK_REVIEW = {book, page}.
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

  var stars = null;
  var starRow = el("div", "ebr-stars");
  starRow.setAttribute("role", "group");
  starRow.setAttribute("aria-label", "Stars, optional");
  var starBtns = [];
  var starSaid = el("span", "ebr-said");
  function showStars() {
    starBtns.forEach(function (b, i) {
      var on = stars !== null && i < stars;
      b.textContent = on ? "★" : "☆";
      b.classList.toggle("on", on);
      b.setAttribute("aria-pressed", stars === i + 1 ? "true" : "false");
    });
    starSaid.textContent = stars === null ? "no stars" : stars + " of 5";
  }
  for (var i = 1; i <= 5; i++) {
    (function (n) {
      var b = el("button", "ebr-star");
      b.type = "button";
      b.setAttribute("aria-label", n + (n === 1 ? " star" : " stars"));
      b.addEventListener("click", function () {
        stars = stars === n ? null : n;
        showStars();
      });
      starBtns.push(b);
      starRow.append(b);
    })(i);
  }
  starRow.append(starSaid);

  var row = el("div", "ebr-row");
  var send = el("button", "ebr-send", "Send");
  send.type = "submit";
  var cancel = el("button", "ebr-cancel", "Cancel");
  cancel.type = "button";
  var err = el("span", "ebr-err");
  err.setAttribute("role", "status");
  row.append(send, cancel, err);
  form.append(head, name, text, starRow, row);
  dlg.append(form);

  function open() {
    err.textContent = "";
    send.disabled = false;
    name.value = account || storedName();
    name.hidden = !!account;
    showStars();
    dlg.showModal();
    (name.hidden || name.value ? text : name).focus();
  }
  btn.addEventListener("click", open);
  cancel.addEventListener("click", function () { dlg.close(); });

  form.addEventListener("submit", function (ev) {
    ev.preventDefault();
    send.disabled = true;
    err.textContent = "";
    fetch("/books/" + encodeURIComponent(BOOK_REVIEW.book) + "/replies", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ page: BOOK_REVIEW.page, stars: stars, text: text.value, name: name.value })
    }).then(function (r) {
      if (r.status === 201) return r.json();
      return r.text().then(function (t) { throw new Error(t.trim() || ("the server said " + r.status)); });
    }).then(function () {
      if (!account) storeName(name.value.trim());
      text.value = "";
      stars = null;
      dlg.close();
      btn.textContent = "Sent";
      setTimeout(function () { btn.textContent = "Reply"; }, 2500);
    }).catch(function (e) {
      err.textContent = e.message;
      send.disabled = false;
    });
  });

  document.body.append(btn, dlg);
})();

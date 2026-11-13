// Comments on a note's blocks. The server marks each commentable block
// data-c="K", puts each block's comments after it in a .cmts element, and
// gives the blocks' fingerprints in #blocks and the note's name in NOTE.
// This adds a quiet button to each block and a form to post a comment.
(function () {
  "use strict";
  var hashes = JSON.parse(document.getElementById("blocks").textContent);
  var NAME_KEY = "essay-surface-name";

  function storedName() {
    try { return localStorage.getItem(NAME_KEY) || ""; } catch (e) { return ""; }
  }
  function storeName(n) {
    try { localStorage.setItem(NAME_KEY, n); } catch (e) { /* a private window: not remembered */ }
  }

  // The thread after block el, or null.
  function threadOf(el) {
    var next = el.nextElementSibling;
    return next && next.classList.contains("cmts") ? next : null;
  }

  function label(btn, n) {
    btn.textContent = n ? String(n) : "note";
    btn.classList.toggle("has", n > 0);
    btn.setAttribute("aria-label", n ? n + (n === 1 ? " comment" : " comments") + "; add one" : "Comment on this");
  }

  function commentNode(c) {
    var d = document.createElement("div");
    d.className = "cmt";
    d.id = "c-" + c.id;
    var who = document.createElement("div");
    who.className = "who";
    var b = document.createElement("b");
    b.textContent = c.name;
    var date = document.createElement("span");
    date.className = "date";
    date.textContent = c.time.split("T")[0];
    who.append(b, " ", date);
    var said = document.createElement("div");
    said.className = "said";
    said.textContent = c.text;
    d.append(who, said);
    return d;
  }

  function openForm(el, k, btn) {
    var thread = threadOf(el);
    if (!thread) {
      thread = document.createElement("div");
      thread.className = "cmts";
      thread.dataset.for = k;
      el.after(thread);
    }
    var open = thread.querySelector(".cform");
    if (open) { open.querySelector("textarea").focus(); return; }
    var form = document.createElement("form");
    form.className = "cform";
    var name = document.createElement("input");
    name.name = "name"; name.placeholder = "Your name"; name.maxLength = 80; name.required = true;
    name.autocomplete = "name"; name.value = storedName();
    var text = document.createElement("textarea");
    text.name = "text"; text.rows = 3; text.placeholder = "Your comment"; text.required = true;
    var row = document.createElement("div");
    row.className = "row";
    var post = document.createElement("button");
    post.type = "submit"; post.textContent = "Post";
    var cancel = document.createElement("button");
    cancel.type = "button"; cancel.textContent = "Cancel";
    var err = document.createElement("span");
    err.className = "cerr"; err.setAttribute("role", "status");
    row.append(post, cancel, err);
    form.append(name, text, row);
    thread.append(form);
    (name.value ? text : name).focus();

    function close() {
      form.remove();
      if (!thread.querySelector(".cmt")) thread.remove();
    }
    cancel.addEventListener("click", close);
    form.addEventListener("submit", function (ev) {
      ev.preventDefault();
      post.disabled = true;
      err.textContent = "";
      fetch("/notes/" + encodeURIComponent(NOTE) + "/comments", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ block: k, hash: hashes[k], name: name.value, text: text.value })
      }).then(function (r) {
        if (r.status === 201) return r.json();
        return r.text().then(function (t) { throw new Error(t.trim() || ("the server said " + r.status)); });
      }).then(function (c) {
        storeName(name.value.trim());
        thread.insertBefore(commentNode(c), form);
        label(btn, thread.querySelectorAll(".cmt").length);
        close();
      }).catch(function (e) {
        err.textContent = e.message;
        post.disabled = false;
      });
    });
  }

  document.querySelectorAll("[data-c]").forEach(function (el) {
    var k = Number(el.dataset.c);
    var btn = document.createElement("button");
    btn.type = "button";
    btn.className = "cbtn";
    var thread = threadOf(el);
    label(btn, thread ? thread.querySelectorAll(".cmt").length : 0);
    btn.addEventListener("click", function () { openForm(el, k, btn); });
    // A paragraph, heading or list item holds its button. A code block or a
    // table scrolls sideways and would clip it, so its button sits in a pin
    // just after it (and after its thread).
    if (el.tagName === "PRE" || el.classList.contains("table-wrap")) {
      var pin = document.createElement("div");
      pin.className = "cpin";
      pin.append(btn);
      (thread || el).after(pin);
    } else {
      // A list item's text is a span: the li places the button in the margin.
      (el.tagName === "SPAN" ? el.parentElement : el).classList.add("chost");
      el.append(btn);
    }
  });
})();

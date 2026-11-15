// Sortable tables. Clicking a column's heading sorts the table's rows by that
// column, and clicking it again reverses the order. Each column's kind is
// read from its cells: numbers when every cell that isn't empty starts with
// one (thousands commas, a sign or a currency sign, a trailing %, and words
// after it, as in "2,857 days (7.8 years)"), dates when every one is a date
// like 2026-11-15, and text otherwise, compared without case and with the
// numbers in it in number order. Empty cells always go last, and rows that
// compare equal keep their order.
//
// A table is one commentable block, its data-c on the .table-wrap around it,
// and its thread and reply button come after that wrapper. Sorting moves the
// rows inside the tbody and nothing else, so a reply stays with its table;
// the block's fingerprint is the server's, made from the note's text, so the
// order on screen never reaches it.
//
// The page runs this with a document; tables_test.cjs requires it under node
// for its pure half.
(function () {
  "use strict";

  // A leading sign or currency sign (either first), the number, an optional
  // %, then nothing, or words after a space. "10x", "3rd" and "1.2.3" are
  // left as text, whose natural order suits them better.
  var NUMBER = /^([+\-−]?)[$€£¥]?([+\-−]?)((?:\d{1,3}(?:,\d{3})+|\d+)(?:\.\d+)?|\.\d+)(%?)(.*)$/;
  var DATE = /^(\d{4})-(\d{2})-(\d{2})(?:[T ](\d{2}):(\d{2})(?::(\d{2}))?Z?)?$/;

  // The number a cell starts with, or null.
  function parseNumber(cell) {
    var m = NUMBER.exec(cell.trim());
    if (!m || (m[1] && m[2])) return null;
    if (m[5] !== "" && !/^\s/.test(m[5])) return null;
    var v = Number(m[3].replace(/,/g, ""));
    var sign = m[1] || m[2];
    return sign && sign !== "+" ? -v : v;
  }

  // A date cell's time in milliseconds, or null.
  function parseDate(cell) {
    var m = DATE.exec(cell.trim());
    if (!m) return null;
    var month = Number(m[2]), day = Number(m[3]);
    if (month < 1 || month > 12 || day < 1 || day > 31) return null;
    return Date.UTC(Number(m[1]), month - 1, day, Number(m[4] || 0), Number(m[5] || 0), Number(m[6] || 0));
  }

  // "number", "date" or "text": what every non-empty cell of a column is.
  function inferType(cells) {
    var filled = cells.filter(function (c) { return c.trim() !== ""; });
    if (filled.length === 0) return "text";
    if (filled.every(function (c) { return parseDate(c) !== null; })) return "date";
    if (filled.every(function (c) { return parseNumber(c) !== null; })) return "number";
    return "text";
  }

  var collator = new Intl.Collator(undefined, { numeric: true, sensitivity: "base" });

  // The cells' indices in sorted order: by type, reversed when descending,
  // empty cells last either way, equal cells in their given order.
  function sortOrder(cells, type, descending) {
    var parse = type === "number" ? parseNumber : type === "date" ? parseDate : function (c) { return c.trim(); };
    var items = cells.map(function (c, i) {
      return { empty: c.trim() === "", key: c.trim() === "" ? null : parse(c), at: i };
    });
    var dir = descending ? -1 : 1;
    items.sort(function (a, b) {
      if (a.empty !== b.empty) return a.empty ? 1 : -1;
      if (!a.empty) {
        var c = type === "text" ? collator.compare(a.key, b.key) : a.key < b.key ? -1 : a.key > b.key ? 1 : 0;
        if (c !== 0) return c * dir;
      }
      return a.at - b.at;
    });
    return items.map(function (it) { return it.at; });
  }

  // A table's headings made into sort controls, when it has two rows or more.
  function wire(table) {
    var head = table.tHead && table.tHead.rows[0];
    var body = table.tBodies[0];
    if (!head || !body || body.rows.length < 2) return;
    var state = { col: -1, descending: false };
    var ths = Array.prototype.slice.call(head.cells);
    function sortBy(col) {
      var descending = state.col === col ? !state.descending : false;
      var rows = Array.prototype.slice.call(body.rows);
      var cells = rows.map(function (r) { return r.cells[col] ? r.cells[col].textContent : ""; });
      sortOrder(cells, inferType(cells), descending).forEach(function (i) { body.appendChild(rows[i]); });
      state.col = col;
      state.descending = descending;
      ths.forEach(function (th, j) {
        var on = j === col;
        th.setAttribute("aria-sort", on ? (descending ? "descending" : "ascending") : "none");
        th.querySelector(".sort-mark").textContent = on ? (descending ? "▼" : "▲") : "↕";
        th.classList.toggle("sorted", on);
      });
    }
    ths.forEach(function (th, col) {
      th.classList.add("sortable");
      th.tabIndex = 0;
      th.title = "Sort by this column";
      th.setAttribute("aria-sort", "none");
      var mark = document.createElement("span");
      mark.className = "sort-mark";
      mark.setAttribute("aria-hidden", "true");
      mark.textContent = "↕";
      th.append(mark);
      th.addEventListener("click", function () { sortBy(col); });
      th.addEventListener("keydown", function (ev) {
        if (ev.key === "Enter" || ev.key === " ") { ev.preventDefault(); sortBy(col); }
      });
    });
  }

  if (typeof document !== "undefined") {
    document.querySelectorAll("article table").forEach(wire);
  }
  if (typeof module !== "undefined") {
    module.exports = { parseNumber: parseNumber, parseDate: parseDate, inferType: inferType, sortOrder: sortOrder };
  }
})();

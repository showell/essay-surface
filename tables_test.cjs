// The sortable tables' pure half (tables.js): what kind a column is read as,
// and the order its rows sort in. Run by scripts/check_before_commit.shale,
// `node tables_test.cjs`; it prints one "ok:" line a check and exits 1 on
// the first that fails.
"use strict";
const t = require("./tables.js");

let passed = 0;
function check(what, got, want) {
  const g = JSON.stringify(got), w = JSON.stringify(want);
  if (g !== w) {
    console.log("FAIL: " + what + ": got " + g + ", want " + w);
    process.exit(1);
  }
  passed++;
  console.log("ok: " + what);
}

// The cells in the order sortOrder puts them.
function sorted(cells, descending) {
  return t.sortOrder(cells, t.inferType(cells), descending).map((i) => cells[i]);
}

// Numbers: each form that is one, and each near miss that isn't.
check("a plain integer", t.parseNumber("42"), 42);
check("thousands commas", t.parseNumber("2,857"), 2857);
check("days and years after the number", t.parseNumber("2,857 days (7.8 years)"), 2857);
check("a decimal", t.parseNumber("7.8"), 7.8);
check("a leading minus", t.parseNumber("-3"), -3);
check("a Unicode minus", t.parseNumber("−3"), -3);
check("a leading plus", t.parseNumber("+3"), 3);
check("a currency sign", t.parseNumber("$1,200.50"), 1200.5);
check("a minus before the currency sign", t.parseNumber("-$5"), -5);
check("a trailing percent", t.parseNumber("12.5%"), 12.5);
check("a word glued to the number is text", t.parseNumber("10x"), null);
check("an ordinal is text", t.parseNumber("3rd"), null);
check("a version is text", t.parseNumber("1.2.3"), null);
check("a misplaced comma is text", t.parseNumber("1,23"), null);
check("words first are text", t.parseNumber("about 5"), null);
check("a date is not a number", t.parseNumber("2026-11-15"), null);

// Dates: a day, a day and a time, and what isn't one.
check("a date", t.parseDate("2026-11-15"), Date.UTC(2026, 10, 15));
check("a date and a time", t.parseDate("2026-11-15T09:30"), Date.UTC(2026, 10, 15, 9, 30));
check("month 13 is no date", t.parseDate("2026-13-01"), null);
check("a date with words after is no date", t.parseDate("2026-11-15 or so"), null);
check("a slashed date is no date", t.parseDate("11/15/2026"), null);

// The kind of a column: each, and the fallbacks.
check("a column of numbers", t.inferType(["1", "2,000", "3%"]), "number");
check("a column of dates", t.inferType(["2026-11-15", "2025-01-02"]), "date");
check("a column of words", t.inferType(["b", "a"]), "text");
check("numbers and words fall back to text", t.inferType(["12", "many", "3"]), "text");
check("dates and numbers fall back to text", t.inferType(["2026-11-15", "4"]), "text");
check("empty cells don't change the kind", t.inferType(["5", "", " ", "10"]), "number");
check("a column of empty cells is text", t.inferType(["", ""]), "text");

// Sorting: numbers by value, not by their text.
check("numbers with commas, ascending", sorted(["10", "2,857", "9", "100"], false), ["9", "10", "100", "2,857"]);
check("numbers with commas, descending", sorted(["10", "2,857", "9", "100"], true), ["2,857", "100", "10", "9"]);
check("days and years, by the days",
  sorted(["2,857 days (7.8 years)", "365 days (1 year)", "1,000 days (2.7 years)"], false),
  ["365 days (1 year)", "1,000 days (2.7 years)", "2,857 days (7.8 years)"]);
check("negative numbers before positive", sorted(["5", "-10", "0"], false), ["-10", "0", "5"]);
check("dates, ascending", sorted(["2026-11-15", "2025-12-01", "2026-01-31"], false), ["2025-12-01", "2026-01-31", "2026-11-15"]);
check("dates, descending", sorted(["2026-11-15", "2025-12-01", "2026-01-31"], true), ["2026-11-15", "2026-01-31", "2025-12-01"]);
check("a mixed column sorts as text, numbers in number order", sorted(["item 10", "item 9", "Apple", "12"], false), ["12", "Apple", "item 9", "item 10"]);
check("text ignores case", sorted(["banana", "Cherry", "apple"], false), ["apple", "banana", "Cherry"]);
check("text, descending", sorted(["banana", "Cherry", "apple"], true), ["Cherry", "banana", "apple"]);

// Empty cells last in both directions, and equal cells in their order.
check("empty cells last, ascending", sorted(["3", "", "1", " ", "2"], false), ["1", "2", "3", "", " "]);
check("empty cells last, descending", sorted(["3", "", "1", " ", "2"], true), ["3", "2", "1", "", " "]);
check("empty text cells last", sorted(["", "b", "a"], false), ["a", "b", ""]);
const ties = ["2 a", "1 b", "2 c", "1 d"];
check("equal numbers keep their order, ascending", t.sortOrder(ties, "number", false), [1, 3, 0, 2]);
check("equal numbers keep their order, descending", t.sortOrder(ties, "number", true), [0, 2, 1, 3]);
check("text equal but for case keeps its order", t.sortOrder(["B", "a", "b"], "text", false), [1, 0, 2]);

console.log(passed + " checks passed");

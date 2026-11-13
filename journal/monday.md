# Monday: the essay surface, day one

The prompt is `journal/prompts/monday.md`: a Go server (standard library only) that serves `notes/*.md` as HTML at `/notes/<name>.md`, an index at `/` newest first, a markdown renderer in Go, a dozen real notes as sample data, renderer tests, and real numbers on speed with five hundred notes.

All shell work is Shale. Each script below is verbatim, with why I ran it and what it said.

## Reading the Shale docs, and the first stop

I wanted the "Before you reach for the shell" table out of `docs/SHALE-AGENTS.md`, so I wrote a loop that looked for its line number.

```shale
local ls = lines("docs/SHALE-AGENTS.md")
local start = 0
for i = 1, #ls do
  if find(ls[i], "Before you reach") then start = i; break end
end
for j = start, start + 120 do
  if j <= #ls then print(ls[j]) end
end
```

```text
── stopped at stdin:7:28: index: index 0 is outside the list (length 1) (54 steps)
```

The mistake: `lines(s)` splits a *string*, not a file, so `ls` was one line (the path itself) and nothing matched. A file's lines are `read_lines`, or `lines(read(p))`. I read the doc with my file reader instead, which was the right tool for reading prose anyway.

## Surveying the real notes

Why: to pick a dozen varied notes, I wanted each note's size and how many fences, tables and list items it has. The first try used `re_lines` per file:

```shale
-- survey the real notes: size, and how many tables, fences, lists each has
local rows = {}
local all = glob("essay-repl-server/notes/*.md")
for p in all do
  local s = read(p)
  local n = count_lines(p)
  local fences = #grep(p, "```")
  local tables = #re_lines(s, "^\\|.*\\|\\s*$")
  local lists = #re_lines(s, "^\\s*([-*]|\\d+\\.) ")
  push(rows, {path = basename(p), lines = n, bytes = #s, fences = fences // 2, trows = tables, items = lists})
  yield()
end
...
```

```text
── stopped at the Linux prelude:223:15 (in `the Linux prelude's re_lines`): budget steps: the step budget (5000000 steps) ran out (5000001 steps)
── used: 5000001 of 5000000 steps, 874 of 10000 reads, 2947203 of 67108864 read bytes
```

`re_lines` is written in Shale (the prelude), so every line costs steps. I switched to literal `grep`s (a builtin) and raised the step budget, and hit the next wall:

```text
── stopped at stdin:6:19: budget arena: the arena budget ran out (2059959 steps)
```

The docs say why: a whole-tree loop keeps each file's work in a function and calls `yield()` after each file, so the arena holds one file at a time. With that, it ran:

```shale
-- survey the real notes: size, and how many tables, fences, lists each has
function measure(p)
  return {path = basename(p), lines = count_lines(p), bytes = stat(p).size,
    fences = #grep(p, "```") // 2, tables = #grep(p, "|---"), dashes = #grep(p, "- ")}
end
local rows = {}
local all = glob("essay-repl-server/notes/*.md")
for p in all do
  push(rows, measure(p))
  yield()
end
print(#all .. " notes")
local total = 0
local wt = 0
local wf = 0
for r in rows do
  total = total + r.bytes
  if r.tables > 0 then wt = wt + 1 end
  if r.fences > 0 then wf = wf + 1 end
end
print("total bytes " .. total .. "; with tables " .. wt .. ", with fences " .. wf)
sort(rows, "bytes", "desc")
print(table(take(rows, 6), {"bytes", "lines", "fences", "tables", "dashes", "path"}))
sort(rows, "bytes")
print(table(take(rows, 5), {"bytes", "lines", "fences", "tables", "dashes", "path"}))
```

```text
502 notes
total bytes 4747688; with tables 217, with fences 196
 bytes  lines  fences  tables  dashes  path
167408   5154      49       0      19  gopher-roc-demo.md
135703   3197     160      10      59  safari-tutorial-draft.md
 64320    556       0       1     249  review-queue.md
 54827   1113      19       7      80  roc-machine-emulator.md
...
bytes  lines  fences  tables  dashes  path
 1115     21       0       0       0  sunset-eye-test.md
 1238      9       0       0       0  opening-game-tie-ins-2.md
 1446     18       0       0       4  roc-bird-stills.md
```

So there are 502 notes, 4.7 MB in all, averaging about 9.5 KB, with a long tail: the biggest is 167 KB. The prompt said "five hundred", and that is the real count.

Then I looked for mid-sized notes with tables, fences and links all together:

```shale
-- mid-sized notes that have both tables and code fences, plus ones with numbered lists
function measure(p)
  return {path = basename(p), bytes = stat(p).size,
    fences = #grep(p, "```") // 2, tables = #grep(p, "|---"), nums = #grep(p, "1. "), links = #grep(p, "](")}
end
local rows = {}
for p in glob("essay-repl-server/notes/*.md") do
  local m = measure(p)
  if m.fences > 0 and m.tables > 0 and m.bytes < 20000 and m.links > 0 then push(rows, m) end
  yield()
end
sort(rows, "bytes")
print(table(rows, {"bytes", "fences", "tables", "nums", "links", "path"}))
print(must(run({"head", "-12", "essay-repl-server/notes/sunset-eye-test.md"}, {timeout_ms = 5000, whole = 4000, want = "peek: a note's head"})).out)
```

```text
bytes  fences  tables  nums  links  path
 7146       4       2     3      2  basic-machine-design.md
 8937       7       3     1      3  arrays-that-are-values.md
10016       5       1     1      2  three-repos-one-library.md
14372       4       2     4      1  dog-physics-and-the-report.md
15213       5       2     0      1  the-on-ramp.md
...
# The sunset rule: eye test

*2026-11-07. Branch `sunset-rule`, not merged yet.*
```

A confession: I reached for `head` with a `want`, which `run` logs as an escape. Shale has `head_lines(p, 12)` for exactly that. It's a shell habit, and the table I'd just read names it.

## Picking and copying the dozen

The picks span the range: the four biggest (167 KB, 136 KB, 64 KB of nested lists, 55 KB), five mid-sized ones with tables, fences and links, and three short ones. I copied with `cp -p` so each keeps its real modified time; "newest first" needs a date, and the notes have no front matter.

```shale
-- copy a dozen real notes into essay-surface/notes, keeping each file's mtime (cp -p) so "newest first" means something
local picks = {"gopher-roc-demo.md", "safari-tutorial-draft.md", "review-queue.md", "roc-machine-emulator.md",
  "basic-machine-design.md", "arrays-that-are-values.md", "three-repos-one-library.md", "dog-physics-and-the-report.md",
  "the-on-ramp.md", "sunset-eye-test.md", "opening-game-tie-ins-2.md", "roc-bird-stills.md"}
mkdir("essay-surface/notes")
for name in picks do
  must(run({"cp", "-p", "essay-repl-server/notes/" .. name, "essay-surface/notes/" .. name}, {timeout_ms = 5000, tail = 5}))
end
local rows = {}
for p in glob("essay-surface/notes/*.md") do push(rows, {path = basename(p), bytes = stat(p).size, mtime = stat(p).mtime}) end
...
```

```text
── stopped at stdin:10:115: index: no key "mtime" in the map (`has_key` asks first) (6837 steps)
```

The copies had all been made by then; only the listing stopped. I guessed the field's name. `print(json_write(stat(p)))` showed it:

```text
{"size":1115,"kind":"file","modified":1794056568}
```

```shale
-- the copied notes, newest first by modified time
local rows = {}
for p in glob("essay-surface/notes/*.md") do
  local s = stat(p)
  push(rows, {path = basename(p), bytes = s.size, modified = s.modified})
end
sort(rows, "modified", "desc")
print(table(rows, {"modified", "bytes", "path"}))
```

```text
  modified   bytes  path
1794319103    1238  opening-game-tie-ins-2.md
1794158723   64320  review-queue.md
1794056568    1115  sunset-eye-test.md
1793974954  167408  gopher-roc-demo.md
...
1789254127    8937  arrays-that-are-values.md
```

## What the samples hold, beyond the brief's list

Why: before writing the renderer, I wanted to know which markdown the real notes use, so I'd know what it must handle and what it can skip.

```shale
-- which markdown constructs the samples use, beyond the brief's list
function count_in(p, lit)
  return #grep(p, lit)
end
local probes = {"> ", "<br", "<div", "![", "~~~", "    - ", "  - ", "***", "---", "__", "\\|", "<http", "**", "&"}
...
print(table(rows, {"probe", "lines", "files"}))
```

```text
probe   lines  files
>         782      9
<div        8      1
![         13      3
~~~         2      1
    -       6      1
  -        98      4
---        82     10
__         21      3
**        466     10
&          42      3
```

Then I printed sample lines for the ones I hadn't decided on:

```text
gopher-roc-demo.md:6: <div class=gopher-roc-demo>
gopher-roc-demo.md:17: <div class="body"><p>We&#39;re starting fresh. ...
gopher-roc-demo.md:5099: ~~~~
gopher-roc-demo.md:4278: 		"3__4",
roc-machine-emulator.md:65: 	(machine3, machine__4) = ({
review-queue.md:44:      - `ArgbColor.from_rgb(color, { alpha })`, used by ...
```

Decisions from that: block-level raw HTML passes through (one big note is a chat transcript exported as HTML); fences can be four tildes, so a fence closes only on its own character, at least as long; `_` inside a word (`machine__4`) is never emphasis; lists nest by indentation, as deep as `review-queue.md`'s six spaces.

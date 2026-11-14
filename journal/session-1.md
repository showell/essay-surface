# Session 1: the essay surface, from scratch

The prompt is `journal/prompts/session-1.md`: a Go server (standard library only) that serves `notes/*.md` as HTML at `/notes/<name>.md`, an index at `/` newest first, a markdown renderer in Go, a dozen real notes as sample data, renderer tests, and real numbers on speed with five hundred notes.

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

## The code

Four Go files, standard library only:

- `markdown.go`: the renderer. A block pass over lines (fences, ATX headings, rules, blockquotes, raw HTML blocks, GFM tables with alignment, nested lists tight and loose, paragraphs), then an inline pass per block (code spans, links, images, autolinks, hard breaks, entities, and emphasis matched with CommonMark's delimiter algorithm, rule of three included, so `**a *b* c**` nests and `snake_case_name` stays). Inline HTML is escaped; `javascript:` links become `#`. Every paragraph gets an id (`p1`, `p2`, ...) and every heading a slug, ready for paragraph comments.
- `server.go`: a `store` that renders each note once and keeps the whole page, keyed by modified time and size. A note request is one `stat` and one write; an edited note re-renders on its next request. The index lists the folder, stats each file, sorts newest first (then by name), and writes the page. Each response carries `Server-Timing: app;dur=...`. It listens on `127.0.0.1:9300` and takes `-notes DIR`.
- `markdown_test.go`, `server_test.go`: the tests and benchmarks.

## Building

```shale
-- first build: does it compile, and does vet like it?
local b = run({"go", "build", "-o", "essay-surface", "."}, {timeout_ms = 120000, tail = 30})
print("build exit " .. b.code .. " in " .. b.ms .. " ms")
print(b.out)
print(run({"go", "vet", "."}, {timeout_ms = 120000, tail = 30}).out)
```

```text
build exit 0 in 21591 ms
── time: 2 programs, 25.2 s in all; the slowest 21.5 s (85%): go build -o essay-surface .; 3.6 s (14%): go vet .
```

That 21.5 s was a cold build cache (the standard library compiled from nothing). Later builds took 0.1 to 0.9 s.

## The tests, and the first red

```shale
-- the tests: every failure line, then the verdict
local r = run({"go", "test", "-count=1", "."}, {timeout_ms = 180000, tail = 80})
print("exit " .. r.code .. " in " .. r.ms .. " ms")
print(r.out)
```

```text
exit 1 in 7841 ms
--- FAIL: TestRender/table_without_outer_pipes,_short_and_long_rows (0.00s)
     got: "...<tbody>\n</tbody></table></div>\n<p id=\"p1\">1\n1 | 2 | 3</p>\n"
```

The test had a pipe-less line (`1`) as a table row. GFM takes one as a one-cell row. My renderer ends the table at a line with no pipe. I kept the renderer's rule, since a pipe-less line after a table reads to me as prose, and wrote the rule down as its own test ("a line without a pipe ends a table") instead of hiding it.

I saved the gate as **`scripts/check.shale`**: gofmt must list nothing, vet must be quiet, every test must pass. Its first run stopped, which is what it's for:

```text
gofmt: these files need gofmt -w:
markdown.go
markdown_test.go
── stopped at scripts/check.shale:8:3: fail: gofmt (131 steps)
```

Then I tried to fix and re-check in one Bash call, two launches, and the gate refused it before anything ran:

```text
shale-gate: the heredoc has no last line `EOF`; only a launch, `shale [--verbose] ... <<'TAG'` ...
```

The gate takes one launch per call. Fixed in two calls:

```shale
-- what gofmt wants, then apply it
print(run({"gofmt", "-d", "markdown.go", "markdown_test.go"}, {timeout_ms = 30000, tail = 40}).out)
must(run({"gofmt", "-w", "markdown.go", "markdown_test.go"}, {timeout_ms = 30000, tail = 5}))
```

```text
-	indent  int  // spaces before the marker
+	indent  int // spaces before the marker
...
```

```text
gofmt: clean
vet: clean, 454 ms
test: ok  	essay-surface	0.038s
```

A green is a claim, so I counted what ran:

```shale
-- a green is a claim: how many tests and subtests actually ran?
local r = must(run({"go", "test", "-count=1", "-v", "."}, {timeout_ms = 180000, whole = 200000}))
print("PASS lines: " .. #find_all(r.out, "--- PASS"))
print("FAIL lines: " .. #find_all(r.out, "--- FAIL"))
print("top-level: " .. #re_lines(r.out, "^=== RUN   Test\\w+$"))
```

```text
PASS lines: 69
FAIL lines: 0
top-level: 7
```

Committed (`ff0ab50`) with `git_commit` and an explicit path list, plus a `.gitignore` for the built binary.

## The server, running

```shale
-- start the server on the sample notes, fetch the index and a note, then stop it
must(run({"go", "build", "-o", "essay-surface", "."}, {timeout_ms = 120000, tail = 20}))
mkdir(".zig-cache")
local job = job_start({"./essay-surface"}, {log = ".zig-cache/server.log", cwd = ".", want = "build: the server we just built"})
local w = job_wait(job, {timeout_ms = 10000, contains = "listening"})
print("wait: " .. w.ended)
print(read(".zig-cache/server.log"))
local idx = http_get("http://127.0.0.1:9300/")
print("index " .. idx.status .. ", " .. #idx.body .. " bytes, " .. idx.ms .. " ms")
local n = http_get("http://127.0.0.1:9300/notes/review-queue.md")
print("note " .. n.status .. ", " .. #n.body .. " bytes, " .. n.ms .. " ms")
local miss = http_get("http://127.0.0.1:9300/notes/nope.md")
print("missing " .. miss.status)
job_stop(job)
```

```text
wait: found
rendered 12 notes from notes in 27ms
listening on http://127.0.0.1:9300/
index 200, 3742 bytes, 9 ms
note 200, 82011 bytes, 10 ms
missing 404
```

Two things were wrong here. First, `http_get`'s 9 to 10 ms is mostly curl starting up: it's one process per call, so it can't measure a server that answers in a fraction of a millisecond. Second, putting the log in `.zig-cache/` (a habit from Zig repos) left a folder git would list as untracked. I removed it, and logs now go in an ignored `.run/`:

```shale
-- tidy: the first run's log went in .zig-cache, which git would list; logs go in an ignored .run/
remove_tree(".zig-cache")
replace(".gitignore", "/essay-surface\n/.run/\n")
print(json_write(git_status(".")))
```

## Measuring: scripts/bench.shale

To time the server and not curl, I saved **`scripts/bench.shale`**. It builds, starts the server on a folder (`dir=`, default `notes`), reads the note list from the index, and runs *one* curl over many URLs on a kept-alive connection, with curl's own `-w %{time_total}` per request. It prints the median, p90, p99 and max for the index, for every note once, and for the biggest note many times, then stops the server. Its first run stopped before doing anything:

```text
── stopped at scripts/bench.shale:71:7: name: no function `first` above this line ...
```

I'd guessed at a builtin. `lines(...)[1]` it is, and I swapped `tostring(n)` for `"" .. n` before it could stop too. Then, on the twelve samples:

```text
rendered 12 notes from notes in 17ms
12 notes in the index
index: 200 requests, 0 not 200, 3742 bytes average; median 0.158 ms, p90 0.190 ms, p99 0.504 ms, max 1.895 ms
notes: 12 requests, 0 not 200, 50021 bytes average; median 0.145 ms, p90 0.232 ms, p99 0.458 ms, max 0.458 ms
largest note: 200 requests, 0 not 200, 189765 bytes average; median 0.171 ms, p90 0.214 ms, p99 0.471 ms, max 1.000 ms
```

And on all the real notes. The server only reads the folder, and I copied nothing:

```
shale scripts/bench.shale dir=/home/steve/showell_repos/essay-repl-server/notes
```

```text
rendered 503 notes from /home/steve/showell_repos/essay-repl-server/notes in 178ms
503 notes in the index
index: 200 requests, 0 not 200, 67790 bytes average; median 2.089 ms, p90 2.823 ms, p99 4.106 ms, max 4.828 ms
notes: 503 requests, 0 not 200, 13733 bytes average; median 0.118 ms, p90 0.163 ms, p99 0.262 ms, max 1.393 ms
```

(503, not the 502 of this morning's survey: the live folder grew a note during the day.)

A note is about 0.13 ms. The index is about 2 ms, fifteen times slower, so I measured it by phase instead of guessing. I added two benchmarks that split it in two:

```shale
-- the index's time by phase: the folder scan versus writing the page
local r = must(run({"go", "test", "-run", "^$", "-bench", "Index", "-benchmem", "."}, {timeout_ms = 180000, tail = 20, re = "Benchmark"}))
print(r.out)
```

```text
BenchmarkIndex500-2       	     820	   1419190 ns/op	  632308 B/op	    3062 allocs/op
BenchmarkIndexScan500-2   	    1083	   1109817 ns/op	  258487 B/op	    2520 allocs/op
BenchmarkIndexPage500-2   	    6486	    173394 ns/op	  114496 B/op	     502 allocs/op
```

The folder scan (a `ReadDir` and a `stat` per note) is 1.1 ms of the 1.4 ms. Writing the 68 KB page is 0.17 ms. The scan is what keeps the index honest when a note is edited on disk. I left it in for today, and it's the first thing for tomorrow (below). The other benchmarks: a cached note is 22 µs in process, and rendering the 167 KB note cold is 1.24 ms (135 MB/s).

## Auditing the rendering of all 503 notes: scripts/audit.shale

The tests check the cases I thought of. To find the ones I didn't, I rendered every real note through the server and looked for markdown left over outside code: `**`, `](`, `|---`, or a backtick. The first try fetched each page with `http_get`:

```text
── stopped at the Linux prelude:11:10 (in `the Linux prelude's run`): budget runs: 64 programs a run, the most: run the rest in another (26125230 steps)
   called from the Linux prelude:134:13 (in `the Linux prelude's curl_http`)
```

Each `http_get` is a curl, and a run gets 64 programs. Worse, the stop came after `job_start`, so **the server was left running**. `job_stop` on the log path was refused (`.run/server.log is not a job's record (<log>.job)`). The record is the log's name plus `.job`:

```shale
-- the job's record is <log>.job
print(job_alive(".run/server.log.job"))
job_stop(".run/server.log.job")
print(job_alive(".run/server.log.job"))
```

```text
true
false
```

The fix in the saved script: fetch every page with one curl into `.run/pages/`, and stop the server *before* `must` checks the fetch, so a stop can't leave the server up. The second try ran out of 80 million steps in `re_replace` (it runs in the prelude, a step a character, over 7 MB of HTML). Cutting code out with `split` and `find`, which are builtins, ran in under a second:

```text
503 pages, 37 with leftovers
stars  links  tables  ticks  note
    0      0       0      3  essay-surface-quality.md.html
    0      0       0      2  review-gopher-roc-DateTime.md.html
...
```

No stray `**`, links or tables anywhere. The backticks were two real bugs:

```text
review-gopher-roc-DateTime.md: <title>Review: gopher-roc `app/DateTime.roc`</title>
essay-surface-quality.md: <tr><td><strong>pre-commit hook</strong> ( )</td><td>Every staged note's ```dot diagrams render ... | On every commit, once   sets
```

1. **Titles** kept their markdown: 35 notes showed backticks in `<title>` and the index. Now `TitleHTML` renders the title inline and drops the tags.
2. **A table cell with an unpaired ```** (the source: ``Every staged note's ```dot diagrams render``) started a code span that never closed, and it swallowed the rest of the row's pipes. Now a backtick run hides pipes only when its span really closes.

Each fix got a test (the table one copied from the note that found it). After them:

```text
503 pages, 2 with leftovers
    0      0       0      3  essay-surface-quality.md
    0      0       0      2  update-53-and-the-checklists-first-day.md
```

Both are faithful now. One is the literal ```` ```dot ````. The other's source is `` `SEED \`<hash>`` ``, where a backslash can't escape a backtick inside a code span, so CommonMark renders it the same way. Committed as `d793c5a`.

Final numbers, 500 rounds, on all 503 notes:

```text
rendered 503 notes from /home/steve/showell_repos/essay-repl-server/notes in 173ms
index: 500 requests, 0 not 200, 67478 bytes average; median 2.038 ms, p90 2.735 ms, p99 3.544 ms, max 4.628 ms
notes: 503 requests, 0 not 200, 13732 bytes average; median 0.133 ms, p90 0.172 ms, p99 0.220 ms, max 0.729 ms
largest note: 500 requests, 0 not 200, 189765 bytes average; median 0.149 ms, p90 0.212 ms, p99 0.296 ms, max 0.723 ms
```

Last, I checked nothing was left listening (after a stop on `"" .. true`: `..` won't take a boolean):

```shale
-- is anything still listening? the job record, and any essay-surface process on the box
print(job_alive(".run/server.log.job"))
local n = 0
for p in procs() do
  if p.name == "essay-surface" then n = n + 1 end
end
print("essay-surface processes: " .. n)
```

```text
false
essay-surface processes: 0
```

## The scripts saved

- `scripts/check.shale`: gofmt, vet, and every test; the gate before a commit.
- `scripts/bench.shale`: real HTTP timings, curl's clock, one kept-alive connection; `dir=` and `rounds=`.
- `scripts/audit.shale`: every note rendered through the server, with markup left outside code counted per page; `dir=`.

## Report

**How to run it**

```
cd ~/showell_repos/essay-surface
go build -o essay-surface . && ./essay-surface          # http://127.0.0.1:9300/
./essay-surface -notes ~/showell_repos/essay-repl-server/notes   # any folder
shale scripts/check.shale                                # gofmt, vet, tests
shale scripts/bench.shale dir=/abs/path/to/notes         # timings
shale scripts/audit.shale dir=/abs/path/to/notes         # rendering audit
```

It listens on localhost only. Edit a note and reload: the page re-renders on its next request.

**What I measured** (this droplet, two vCPUs, curl's clock over a kept-alive local connection, all 503 real notes, 4.7 MB):

| | median | p90 | p99 | max |
|---|---|---|---|---|
| a note (every one once, 13.7 KB average) | 0.133 ms | 0.172 ms | 0.220 ms | 0.729 ms |
| the largest note (190 KB page), 500 times | 0.149 ms | 0.212 ms | 0.296 ms | 0.723 ms |
| the index (503 entries, 67 KB), 500 times | 2.038 ms | 2.735 ms | 3.544 ms | 4.628 ms |

Start-up renders all 503 notes in 173 ms. Rendering the largest note cold, after an edit, takes 1.24 ms. In process, the index is 1.1 ms of folder scan plus 0.17 ms of page; a cached note is 22 µs.

**What I'd do differently tomorrow**

- **Make the index as fast as a note.** Its 2 ms is almost all `stat`-ing 503 files on every request. Linux inotify (the standard library's `syscall`) on the notes folder would mark the cache dirty only when a file changes, and the index page could be kept whole, which I expect to bring it down to about 0.15 ms. I'd keep the scan as the fallback, and a rescan on an inotify overflow.
- **Dates from the notes, not the file system.** "Newest first" is the file's modified time, which `cp -p` kept today but a `git clone` would reset. Most notes open with an italic date line (`*2026-11-07. ...*`); I'd read that first, then fall back to git's last commit date.
- **Audit earlier.** The audit found two bugs that 62 hand-written cases missed, in under a second. I'd write it before the renderer's tests, not after.
- **Shale habits:** a file's lines are `read_lines`, not `lines`; one launch per Bash call; `http_get` is a process, so batch fetches through one curl; stop a job before anything that can stop the run; reach for builtins (`split`, `find`, `grep`) over prelude functions (`re_replace`, `re_lines`) in a loop over megabytes.
- Not done, on purpose: comments, gzip, caching headers (ETag), and reference-style links (`[x][y]`), which the audit can't see (it looks for `](`). I haven't checked whether any note uses them.

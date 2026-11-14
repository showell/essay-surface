# Session 3: an index that listens, and two checks to keep

The prompt is `journal/prompts/session-3.md`. Two jobs. The index: make the server hear about changes instead of stat-ing every note on every request, take a note's date from the note or from git rather than the file's modified time, and measure before and after. The checks: turn Session 1's and Session 2's four scripts into a **fast** check for every edit, a **full** check before shipping that says plainly whether it is safe, and keep the timing out of the verdict on a busy box (or hold it to a budget a busy box can't break). The code should say what it does.

All shell work is Shale, one launch per Bash call. Each script is verbatim below, with why I ran it and what it said, the stops and retries included. When a script carried a long Go text into an `edit`, I've cut the Go and named the commit, as Session 2's journal did. The new Go files and scripts I wrote whole with my file writer, and I say so where they appear.

## Orienting

My first call was `cat` on the prompt, out of habit, and the gate refused it before it ran:

```text
PreToolUse:Bash hook error: shale-gate: `cat` is not `shale`; only a launch, ...
```

Day three, same first stop as day two. Prose goes to the file reader. I read the prompt, both journals, every Go file the store touches, the four scripts, `docs/SHALE-AGENTS.md` whole, and `cookbook/times.shale`.

```shale
-- orient: the repo's files, their sizes, and the history
for p in glob("*") do print(p .. " " .. stat(p).size) end
for p in glob("scripts/*") do print(p .. " " .. stat(p).size) end
print(must(run({"git", "log", "--oneline"}, {timeout_ms = 5000, tail = 30})).out)
print(json_write(git_status(".")))
```

```text
server.go 14884
scripts/audit.shale 3324
scripts/bench.shale 6401
scripts/check.shale 1003
scripts/comments_smoke.shale 3518
d0b6ab5 Journal: the Wednesday prompt
...
[]
```

The full check would run several scripts, so I needed to know whether one Shale script can run another. `cookbook/listen.shale` does it with `run({"shale", ...})`, each child getting its own 64 programs:

```shale
-- how a script runs another: what the docs say of child runs and of running shale itself
for p in {"README.md", "docs/SHALE.md", "docs/SHALE-AGENTS.md", "cookbook/listen.shale"} do
  for m in grep(p, "child run") do print(p .. ":" .. m.line .. ": " .. sub(m.text, 1, 200)) end
  for m in grep(p, "\"shale\"") do print(p .. ":" .. m.line .. ": " .. sub(m.text, 1, 200)) end
end
for m in grep("docs/SHALE.md", "prelude.shale") do print("SHALE.md:" .. m.line .. ": " .. sub(m.text, 1, 160)) end
```

```text
cookbook/listen.shale:46: -- child run of this script (`batch=1`, every value passed on) with its own
cookbook/listen.shale:630:     local argv = {"shale", "--root", ".", SELF_PATH, "batch=1", ...
SHALE.md:1026:   `prelude.shale` under `.shale`) is every script's that runs at that root,
```

That gave me the two tools for the checks: child runs for the stages, and a repo prelude, `.shale/prelude.shale`, for the helpers the scripts had been copying from each other.

## What a note's date is: the survey

Session 1's report guessed that most notes open with an italic date line. Before writing a rule, I counted. The first try used `head_lines`:

```shale
-- the real notes' dates: which notes carry a date near their top, and in what shape
function first_date(p)
  local hs = head_lines(p, 12)
  ...
end
...
```

```text
── stopped at the Linux prelude:238:15 (in `the Linux prelude's head_lines`): budget steps: the step budget (5000000 steps) ran out (5000001 steps)
```

`head_lines` is written in Shale and costs a step per byte. I tried the builtin `read(p, 0, 1500)` next and ran out again, this time in `re_find`, which also costs a step per character. The third try checks for "20" with `find` before calling `re_find`, and raises the budget:

```shale
-- the same survey, with room: re_find costs steps by the character
function first_date(p)
  local size = stat(p).size
  local hs = lines(read(p, 0, min(size, 1500)))
  for i = 1, min(#hs, 12) do
    if find(hs[i], "20") ~= nil then
      local m = re_find(hs[i], "(20[0-9][0-9]-[01][0-9]-[0-3][0-9])")
      if m ~= nil then return {line = i, text = sub(hs[i], 1, 70), date = m.group} end
    end
  end
  return nil
end
local shapes = {}
local none = {}
local atline = {}
local n = 0
for p in glob("essay-repl-server/notes/*.md") do
  n = n + 1
  local d = first_date(p)
  if d == nil then push(none, basename(p)) else
    incr(atline, "" .. d.line)
    local shape = "other"
    if starts_with(d.text, "*" .. d.date) then shape = "*DATE" elseif starts_with(d.text, "_" .. d.date) then shape = "_DATE" elseif starts_with(d.text, d.date) then shape = "DATE" elseif starts_with(d.text, "# ") then shape = "in title" end
    incr(shapes, shape)
    if shape == "other" or shape == "in title" then push(none, "  [" .. shape .. "] " .. basename(p) .. ":" .. d.line .. ": " .. d.text) end
  end
  yield()
end
print(n .. " notes")
print(json_write(shapes))
print("first date at line: " .. json_write(atline))
print(#none .. " without a leading-date line, or another shape:")
for x in take(none, 60) do print(x) end
```

(launched as `shale --budget steps=200000000 <<'EOF'`)

```text
505 notes
{"*DATE":197,"other":97,"in title":17,"DATE":5}
first date at line: {"3":285,"1":18,"4":6,"5":3,"12":1,"7":2,"9":1}
  [other] a-day-of-alignment-and-names.md:3: *Claude, 2026-10-27, looking back on one long session of orchestrating
  [other] a-rust-front-end-for-codex.md:3: Written 2026-09-01, before any code. The ask: a native Rust compiler,
  [in title] autonomous-run-2026-11-12.md:1: # Autonomous run, 2026-11-12: the log
  [other] braking-overshoot.md:3: **Answered (Steve, 2026-11-08):** "for braking I agree with your conse
  ...
```

The date is nearly always on line 3, the paragraph right after the title, and it doesn't always come first on that line ("*Claude, 2026-10-27, ..."). So the rule is: **the first YYYY-MM-DD in the title or in the paragraph after it.** That dates 309 of the 506 notes. The other ~190 need git.

## Git as the second source, measured first

```shale
-- git as the second source: is the notes folder in a repo, and what does one log of its files' last commits cost?
local r = must(run({"git", "log", "--format=%x00%ct", "--name-only", "--", "."}, {cwd = "essay-repl-server/notes", timeout_ms = 60000, whole = 20000000}))
print("git log: " .. r.ms .. " ms wall, " .. r.cpu_ms .. " ms cpu, " .. #r.out .. " bytes")
local commits = #find_all(r.out, "\0")
print(commits .. " commits touch notes/")
local s = must(run({"git", "status", "--porcelain", "--", "."}, {cwd = "essay-repl-server/notes", timeout_ms = 60000, tail = 30}))
print("status: " .. s.ms .. " ms; " .. #lines(s.out) .. " lines")
print(s.out)
local t = must(run({"git", "rev-parse", "--show-toplevel"}, {cwd = "essay-repl-server/notes", timeout_ms = 5000, tail = 3}))
print(t.out)
print(must(run({"uptime"}, {timeout_ms = 5000, tail = 3})).out)
print(must(run({"nproc"}, {timeout_ms = 5000, tail = 3})).out)
```

```text
git log: 843 ms wall, 822 ms cpu, 43318 bytes
932 commits touch notes/
status: 6 ms; 0 lines
/home/steve/showell_repos/essay-repl-server
 16:48:11 up 81 days, 20:16,  3 users,  load average: 0.06, 0.86, 1.03
2
```

0.8 s is a lot for one log of a 932-commit folder, so I took it apart:

```shale
-- where git log's 0.8 s goes: rename detection, and asking only about the notes that carry no date
const N = "essay-repl-server/notes"
local a = must(run({"git", "log", "--no-renames", "--format=%x00%ct", "--name-only", "--", "."}, {cwd = N, timeout_ms = 60000, whole = 20000000}))
print("no renames: " .. a.ms .. " ms wall, " .. a.cpu_ms .. " ms cpu")
local b = must(run({"git", "log", "--no-renames", "--format=%x00%ct", "--name-only", "--", "a-day-by-repository.md", "animation.md", "basic-status.md"}, {cwd = N, timeout_ms = 60000, whole = 20000000}))
print("three files: " .. b.ms .. " ms wall, " .. b.cpu_ms .. " ms cpu")
local c = must(run({"git", "log", "-1", "--format=%ct", "--", "animation.md"}, {cwd = N, timeout_ms = 60000, tail = 3}))
print("one file, -1: " .. c.ms .. " ms wall, " .. c.cpu_ms .. " ms cpu: " .. c.out)
local d = must(run({"git", "log", "--no-renames", "--format=%x00%ct", "--name-only", "--", "."}, {cwd = N, timeout_ms = 60000, whole = 20000000, env = {GIT_CONFIG_NOSYSTEM = "1"}}))
print("again: " .. d.ms .. " ms")
local e = must(run({"git", "count-objects", "-v"}, {cwd = N, timeout_ms = 60000, tail = 20}))
print(e.out)
```

```text
no renames: 690 ms wall, 686 ms cpu
three files: 335 ms wall, 333 ms cpu
one file, -1: 183 ms wall, 182 ms cpu: 1794067934
again: 702 ms
count: 3961
...
packs: 0
```

The notes repo has 3,961 loose objects and no pack, so git inflates each one from its own file. I proved that later on a packed clone (below). It isn't mine to `git gc`, so the server pays it once at start-up.

One more fact before designing. Every test makes a store, and every store will hold an inotify instance:

```shale
-- inotify's per-user limits on this box, before a server and every test store take an instance each
print(must(run({"sysctl", "fs.inotify"}, {timeout_ms = 5000, tail = 10})).out)
```

```text
fs.inotify.max_queued_events = 16384
fs.inotify.max_user_instances = 128
fs.inotify.max_user_watches = 61662
```

128 instances per user, shared with everything else on the box, so tests must close their stores.

## Before: the index, measured

A copy of the real notes first, since the bench posts comments. My first try passed `real("../essay-repl-server/notes")`:

```text
── stopped at the Linux prelude:39:10 (in `the Linux prelude's real`): argument: `host`: "../essay-repl-server/notes" has a part ("..") that is not a name
```

`real` takes the root's own paths only. An argv is just strings, so `cp` gets the absolute path:

```shale
-- before: a copy of the real notes (cp -rp keeps their times), and the in-process index benchmarks
const REAL = "/home/steve/showell_repos/essay-repl-server/notes"
if exists(".run/real") then remove_tree(".run/real") end
mkdir(".run")
local c = must(run({"cp", "-rp", REAL, ".run/real"}, {timeout_ms = 60000, tail = 5}))
print("copied " .. #glob(".run/real/*.md") .. " notes in " .. c.ms .. " ms")
local r = must(run({"go", "test", "-count=1", "-run", "^$", "-bench", "Index|NoteCached", "-benchmem", "."}, {timeout_ms = 180000, tail = 20, re = "Benchmark|ok"}))
print("go test -bench: " .. r.ms .. " ms wall, " .. r.cpu_ms .. " ms cpu")
print(r.out)
```

```text
copied 505 notes in 31 ms
BenchmarkIndex500-2             	     727	   2006181 ns/op	  632287 B/op	    3062 allocs/op
BenchmarkIndexScan500-2         	     903	   1392390 ns/op	  258459 B/op	    2520 allocs/op
BenchmarkIndexPage500-2         	    5894	    185678 ns/op	  114496 B/op	     502 allocs/op
BenchmarkNoteCached-2           	   23937	     49208 ns/op	  112353 B/op	      49 allocs/op
```

And over HTTP, with Session 2's bench as it stood:

```
shale scripts/bench.shale dir=.run/real rounds=500
```

```text
2026/11/13 16:50:27 rendered 505 notes from .run/real in 264ms; comments in .run/bench/comments
index: 500 requests, 0 not 200, 67712 bytes average; median 2.276 ms, p90 3.296 ms, p99 4.090 ms, max 7.934 ms
notes, no comments: 505 requests, 0 not 200, 22341 bytes average; median 0.147 ms, p90 0.208 ms, p99 0.423 ms, max 1.099 ms
largest note, no comments: 500 requests, 0 not 200, 199171 bytes average; median 0.217 ms, p90 0.282 ms, p99 0.809 ms, max 1.499 ms
```

**Before: the index takes 2.0 ms in process (1.4 ms of it the scan) and 2.28 ms median over HTTP, fifteen times a note.**

## The design

Three new files, written whole (commit `4a74731`):

- **`watch.go`**: inotify through the standard library's `syscall`, so no dependency was needed. One instance watches the notes folder, the comments folder, and the folder that holds the comments folder, so it hears that folder being made or removed. The descriptor is **non-blocking and drained at the start of each request**, with no goroutine. The kernel queues an event before the write that caused it returns, so a request made after a write has finished always hears of it. That keeps the tests deterministic, with no sleeps. When nothing changed, the drain is one `read` that returns `EAGAIN`.
- **`store.go`**: the store moved out of `server.go`. Changes drained wait in a pending set until they're applied, so a note that can't be read yet is tried again. A changed note is read and rendered again. A note gone is dropped. A comments file changed drops that note's page (the next request makes it). The index page is kept until a note changes. **A full rescan happens only at start, after `IN_Q_OVERFLOW` (the kernel dropped events), or when the notes folder itself moves.** A note in the map is never changed in place: a page dropped is a new note value, so a page being built from the old comments can see it was superseded and not keep its result.
- **`dates.go`**: `noteDate` (the rule above), `gitDates` (one `git log` for many names), and `dateFrom`, which says which of note, git or file a date came from. A note's own date counts as the start of that day.

One correctness gain came free. The old cache keyed a note on modified time and size, and the kernel's file clock moves in ticks of a few milliseconds. Two same-size saves inside one tick were invisible to it. An event is not, and `TestEditInOneClockTick` holds that.

The edit that cut the old store out of `server.go` cut too much. I used `edit_between` from the header comment to `serveIndex`, and the imports and `validName` lay in between:

```shale
-- server.go: the store moves to store.go; the handlers ask it instead of stat-ing
local src = read("server.go")
local a = find(src, "// A note is never changed once made")
local b = find(src, "func (s *store) serveIndex(")
print("cut " .. a .. " to " .. b)
edit_between("server.go", "// Editing a note or its comments on disk rebuilds the page on the next\n// request for it.\n", "func (s *store) serveIndex(", "")
print("ok")
```

```text
cut 474 to 4493
edit server.go: 1 hunk, -175 +3 lines
ok
```

-175 lines said so before I looked. I put the imports and `validName` back with my editor, along with the new handlers, which ask the store (`s.note`, `s.page`, `s.indexPage`) instead of stat-ing. Then:

```shale
-- the pages show a note's date, wherever it came from; main logs where the dates came from
edit("server.go", "	b.WriteString(n.modified.Format(\"2006-01-02\"))\n	b.WriteString(\"</span></nav>", "	b.WriteString(n.date.Format(\"2006-01-02\"))\n	b.WriteString(\"</span></nav>")
edit("server.go", "		b.WriteString(n.modified.Format(\"2006-01-02\"))\n		b.WriteString(\"</span></li>", "		b.WriteString(n.date.Format(\"2006-01-02\"))\n		b.WriteString(\"</span></li>")
edit("server.go", [=[	s := newStore(*dir, *comments)
...]=], [=[	start := time.Now()
	s, err := newStore(*dir, *comments)
...]=])
-- (main's start-up log; the Go is in 4a74731)
print(run({"go", "vet", "."}, {timeout_ms = 120000, tail = 30}).out)
```

```text
vet: ./comments_test.go:350:12: s.get undefined (type *store has no field or method get)
```

The tests came next. `testStore` now takes the testing value and closes the store's watch at cleanup. Session 2's `BenchmarkIndexScan500` went, since the scan it measured is gone. Two benchmarks replace it: `BenchmarkIndexAfterTouch500`, the first index after a note changes, and `BenchmarkRescan500`, the start-up and overflow cost.

```shale
-- the tests: a store is made with its testing value, closed after; the scan benchmark goes with the scan
edit("server_test.go", [=[// testStore serves dir, with its comments in a folder inside it.
func testStore(dir string) *store { return newStore(dir, filepath.Join(dir, "comments")) }]=], [=[...]=])
edit("server_test.go", "testStore(benchDir(b, 500))", "testStore(b, benchDir(b, 500))", {all = true})
edit("server_test.go", "testStore(dir)", "testStore(t, dir)", {all = true})
edit("comments_test.go", ...)   -- (the two benchmarks there; s.get becomes s.note)
edit("comments_test.go", "testStore(dir)", "testStore(t, dir)", {all = true})
edit("server_test.go", [=[// The index's two phases apart: ...]=], [=[...]=])
-- (the Go is in 4a74731)
local v = run({"go", "vet", "."}, {timeout_ms = 120000, tail = 30})
print("vet exit " .. v.code)
print(v.out)
```

```text
edit server_test.go: 1 hunk, -2 +10 lines
edit server_test.go: 4 hunks, -4 +4 lines
edit server_test.go: 4 hunks, -4 +4 lines
edit comments_test.go: 1 hunk, -3 +3 lines
edit comments_test.go: 1 hunk, -5 +4 lines
edit comments_test.go: 2 hunks, -2 +2 lines
edit server_test.go: 1 hunk, -19 +37 lines
vet exit 0
```

```shale
-- the tests on the new store, first run
local r = run({"go", "test", "-count=1", "."}, {timeout_ms = 180000, tail = 60})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, " .. r.cpu_ms .. " ms cpu")
print(r.out)
```

```text
exit 0, 2326 ms wall, 2984 ms cpu
ok  	essay-surface	1.077s
```

## The new tests, and proving they can fail

`store_test.go` (written whole) covers:

- **`TestIndexKeptUntilANoteChanges`**: three requests make the index once, a comment doesn't make it again, and an edit does. It counts builds with a counter the store keeps for tests.
- **`TestEditInOneClockTick`**: a same-size edit with the same modified time is heard.
- **`TestNotesComeAndGo`**: a note added, renamed away, renamed in, removed, and a folder named `e.md`.
- **`TestCommentsFolderRemovedAndMadeAgain`**: the comments folder deleted under the store, then made again. A post lands, and a hand edit in the new folder is heard, which proves its watch was made again.
- **`TestQueueOverflowRescans`**: more events than `max_queued_events` (read from `/proc`), alternating between two files because the kernel folds a repeat of the event before it. Then an edit made *after* the queue is full, which only a rescan can see.

My first version reached for a store through the handler (`h.(*storeHandler).s`), which doesn't exist, so the test makes its own store:

```shale
-- the index test makes its own store, to count its builds; the store counts its rescans
edit("store_test.go", [=[	dir, h, hs := setup(t)
	s := h.(*storeHandler).s
	for range 3 {]=], [=[	dir := t.TempDir()
	writeNote(t, dir, "a.md", commentNote, time.Now())
	s := testStore(t, dir)
	h := s.handler()
	_, page := get(t, h, "/notes/a.md")
	hs := hashesOf(t, page)
	for range 3 {]=])
edit("store.go", {
  {old = "	indexBuilds atomic.Int64 // how often the index page was made, for the tests\n", new = "	// How often the index page was made and the folder read whole, for the\n	// tests.\n	indexBuilds, rescans atomic.Int64\n"},
  {old = "func (s *store) rescan() error {\n	var err error\n", new = "func (s *store) rescan() error {\n	s.rescans.Add(1)\n	var err error\n"},
})
local r = run({"go", "test", "-count=1", "-run", "Index|Tick|ComeAndGo|Folder|Overflow|Comment|Edited|NotFound|NotePage", "-v", "."}, {timeout_ms = 180000, tail = 60, re = "^(--- |ok|FAIL|panic)|_test.go"})
print("exit " .. r.code .. ", " .. r.ms .. " ms")
print(r.out)
```

```text
--- PASS: TestIndexKeptUntilANoteChanges (0.01s)
--- PASS: TestEditInOneClockTick (0.00s)
--- PASS: TestNotesComeAndGo (0.01s)
--- PASS: TestCommentsFolderRemovedAndMadeAgain (0.01s)
--- PASS: TestQueueOverflowRescans (0.07s)
ok  	essay-surface	0.174s
```

All green on the first run. A green is a claim, so I planted two faults, once each: a deaf watch, and an overflow ignored.

```shale
-- plants, once each: a deaf watch (drain hears nothing), then an overflow ignored. Each must turn tests red; then the code goes back
const T = "Index|Tick|ComeAndGo|Folder|Overflow|Comment|Edited|NotFound|NotePage|Thread|Post"
edit("watch.go", "		n, err := syscall.Read(w.fd, w.buf[:])\n", "		n, err := syscall.Read(w.fd, w.buf[:])\n		n, err = 0, syscall.EAGAIN\n")
local r1 = run({"go", "test", "-count=1", "-run", T, "."}, {timeout_ms = 110000, tail = 40, re = "^--- FAIL"})
edit("watch.go", "		n, err := syscall.Read(w.fd, w.buf[:])\n		n, err = 0, syscall.EAGAIN\n", "		n, err := syscall.Read(w.fd, w.buf[:])\n")
print("a deaf watch:")
print(r1.out)
edit("store.go", "	if overflow {\n		return s.rescan()\n	}\n", "	if overflow && false {\n		return s.rescan()\n	}\n")
local r2 = run({"go", "test", "-count=1", "-run", T, "."}, {timeout_ms = 110000, tail = 40, re = "^--- FAIL"})
edit("store.go", "	if overflow && false {\n		return s.rescan()\n	}\n", "	if overflow {\n		return s.rescan()\n	}\n")
print("an overflow ignored:")
print(r2.out)
print(must(run({"go", "test", "-count=1", "."}, {timeout_ms = 110000, tail = 5})).out)
print(json_write(git_status(".")))
```

```text
a deaf watch:
--- FAIL: TestPostComment (0.01s)
--- FAIL: TestPostFromAStalePage (0.00s)
--- FAIL: TestCommentsFollowEdits (0.01s)

an overflow ignored:
--- FAIL: TestQueueOverflowRescans (0.06s)
ok  	essay-surface	1.172s
```

Only three reds for a deaf watch was too few: the edit tests should all have failed. My filter kept only `--- FAIL` lines, so I ran it again with the panic lines shown:

```shale
-- the deaf-watch plant again, with the panic lines shown: did three reds hide a crash that stopped the rest?
const T = "Index|Tick|ComeAndGo|Folder|Overflow|Comment|Edited|NotFound|NotePage|Thread|Post"
edit("watch.go", "		n, err := syscall.Read(w.fd, w.buf[:])\n", "		n, err := syscall.Read(w.fd, w.buf[:])\n		n, err = 0, syscall.EAGAIN\n")
local r1 = run({"go", "test", "-count=1", "-run", T, "."}, {timeout_ms = 110000, tail = 40, re = "^--- FAIL|panic|_test.go:|^FAIL|^ok"})
edit("watch.go", "		n, err := syscall.Read(w.fd, w.buf[:])\n		n, err = 0, syscall.EAGAIN\n", "		n, err := syscall.Read(w.fd, w.buf[:])\n")
print(r1.out)
```

```text
--- FAIL: TestCommentsFollowEdits (0.01s)
    comments_test.go:185: a deleted paragraph's comment is not with the orphans
panic: runtime error: slice bounds out of range [:-1] [recovered, repanicked]
	/home/steve/showell_repos/essay-surface/comments_test.go:187 +0x885
FAIL	essay-surface	0.042s
```

**A bug in Session 2's test.** When the orphans section was missing, `TestCommentsFollowEdits` sliced `page[:i]` with `i = -1`. The panic killed the test binary, and every test after it went unreported. It hid nothing yesterday only because yesterday nothing failed. Now it fails plainly:

```shale
-- Session 2's test sliced page[:i] with i = -1 when the orphans are missing: a panic that stopped every test after it. Fatal there instead; then the deaf watch again
edit("comments_test.go", [=[	if i < 0 || !strings.Contains(page[i:], "on the claim") || strings.Contains(page[:i], "on the claim") {
		t.Error("a deleted paragraph's comment is not with the orphans")
	}]=], [=[	if i < 0 {
		t.Fatal("a deleted paragraph's comment left no orphans section")
	}
	if !strings.Contains(page[i:], "on the claim") || strings.Contains(page[:i], "on the claim") {
		t.Error("a deleted paragraph's comment is not with the orphans")
	}]=])
const T = "Index|Tick|ComeAndGo|Folder|Overflow|Comment|Edited|NotFound|NotePage|Thread|Post"
edit("watch.go", "		n, err := syscall.Read(w.fd, w.buf[:])\n", "		n, err := syscall.Read(w.fd, w.buf[:])\n		n, err = 0, syscall.EAGAIN\n")
local r1 = run({"go", "test", "-count=1", "-run", T, "."}, {timeout_ms = 110000, tail = 40, re = "^--- FAIL|panic|^FAIL|^ok"})
edit("watch.go", "		n, err := syscall.Read(w.fd, w.buf[:])\n		n, err = 0, syscall.EAGAIN\n", "		n, err := syscall.Read(w.fd, w.buf[:])\n")
print(r1.out)
print(must(run({"go", "test", "-count=1", "."}, {timeout_ms = 110000, tail = 5})).out)
```

```text
--- FAIL: TestPostComment (0.01s)
--- FAIL: TestPostFromAStalePage (0.01s)
--- FAIL: TestCommentsFollowEdits (0.01s)
--- FAIL: TestCommentsFileChangedOnDisk (0.01s)
--- FAIL: TestConcurrentPosts (0.03s)
--- FAIL: TestThreadFollowsEveryKindOfBlock (0.01s)
--- FAIL: TestEditedNoteRerenders (0.01s)
--- FAIL: TestIndexKeptUntilANoteChanges (0.01s)
--- FAIL: TestEditInOneClockTick (0.00s)
--- FAIL: TestNotesComeAndGo (0.01s)
--- FAIL: TestCommentsFolderRemovedAndMadeAgain (0.01s)
--- FAIL: TestQueueOverflowRescans (0.05s)
FAIL
ok  	essay-surface	1.146s
```

Twelve reds for a deaf watch, and back to green once the code was restored.

### The dates' tests

`dates_test.go` (written whole) covers `TestNoteDate`: twelve shapes, among them a date in the second paragraph (ignored), `2026-13-01` (passed over for the real date after it), digits run together, and a date past the bytes looked at. It also has `TestDatesFromNoteGitAndFile`, which builds a real git repo in a temp folder with a fixed commit date, sets every file's time to 2030 so the times mislead, and checks each source and the order. One note there is committed but missing when the store starts, then restored by `git checkout`, so git must be asked about a note that arrives later. And `TestDatesWithoutGit` checks that git is asked once outside a repo, then not again.

```shale
-- the date tests, first run
local r = run({"go", "test", "-count=1", "-run", "Date", "-v", "."}, {timeout_ms = 180000, tail = 40, re = "^(--- |ok|FAIL|panic)|_test.go"})
print("exit " .. r.code .. ", " .. r.ms .. " ms")
print(r.out)
```

```text
--- PASS: TestNoteDate (0.00s)
--- PASS: TestDatesFromNoteGitAndFile (0.03s)
--- PASS: TestDatesWithoutGit (0.01s)
ok  	essay-surface	0.050s
```

```shale
-- plants, once each: git never asked; then git asked at the start only, not for a note that comes later
edit("store.go", "	if len(ask) > 0 && s.gitErr == nil {\n", "	if false && len(ask) > 0 && s.gitErr == nil {\n")
local r1 = run({"go", "test", "-count=1", "-run", "Date", "."}, {timeout_ms = 110000, tail = 40, re = "^--- FAIL|_test.go"})
edit("store.go", "	if false && len(ask) > 0 && s.gitErr == nil {\n", "	if len(ask) > 0 && s.gitErr == nil {\n")
print("git never asked:")
print(r1.out)
edit("store.go", "		}\n	}\n	s.datesFromGit(fresh)\n	s.mu.Lock()\n	for _, name := range gone {", "		}\n	}\n	s.mu.Lock()\n	for _, name := range gone {")
local r2 = run({"go", "test", "-count=1", "-run", "Date", "."}, {timeout_ms = 110000, tail = 40, re = "^--- FAIL|_test.go"})
edit("store.go", "		}\n	}\n	s.mu.Lock()\n	for _, name := range gone {", "		}\n	}\n	s.datesFromGit(fresh)\n	s.mu.Lock()\n	for _, name := range gone {")
print("git not asked about a note that comes later:")
print(r2.out)
print(must(run({"go", "test", "-count=1", "."}, {timeout_ms = 110000, tail = 5})).out)
```

```text
git never asked:
--- FAIL: TestDatesFromNoteGitAndFile (0.05s)
    dates_test.go:89: committed.md is out of order
    dates_test.go:93: committed.md: want 2026-01-02 in "href=\"/notes/committed.md\">Committed</a><span class=\"date\">2030-01-01</span>...
    dates_test.go:99: committed.md: dated from &{committed.md Committed {0 64029052800 0xa05dc0} file ... <h1 id="committed" data-c="0">Committed</h1>
...
--- FAIL: TestDatesWithoutGit (0.00s)
    dates_test.go:117: git's refusal was not kept
git not asked about a note that comes later:
--- FAIL: TestDatesFromNoteGitAndFile (0.04s)
    dates_test.go:89: later.md is out of order
...
ok  	essay-surface	1.457s
```

Both reds came out right. The 2030 that shows through the first plant is the misleading file time doing its job. But a failure printed a whole note struct with `%v`, which I wouldn't want to read twice. I fixed the message and ran the gate in the same Bash call, which put two launches in one call and broke Session 1's rule:

```text
PreToolUse:Bash hook error: shale-gate: the heredoc has no last line `EOF`; ...
```

One launch per call:

```shale
-- a failing date test names the source, not the whole note
edit("dates_test.go", [=[		if n := s.lookup(name); n == nil || n.from != f {
			t.Errorf("%s: dated from %v, want %s", name, n, f)
		}]=], [=[		if n := s.lookup(name); n == nil {
			t.Errorf("%s is not in the store", name)
		} else if n.from != f {
			t.Errorf("%s: dated from its %s, want its %s", name, n.from, f)
		}]=])
```

```
shale scripts/check.shale
```

```text
gofmt: clean
vet: clean, 454 ms
js: comments.js parses
test: ok  	essay-surface	1.333s
test: 3063 ms
```

Committed as `4a74731`.

## After: the index, measured

```shale
-- after: the in-process index benchmarks, same box, load first
print(must(run({"uptime"}, {timeout_ms = 5000, tail = 3})).out)
local r = must(run({"go", "test", "-count=1", "-run", "^$", "-bench", "Index|NoteCached|Rescan", "-benchmem", "."}, {timeout_ms = 180000, tail = 20, re = "Benchmark|ok"}))
print("go test -bench: " .. r.ms .. " ms wall, " .. r.cpu_ms .. " ms cpu")
print(r.out)
```

```text
 16:57:35 up 81 days, 20:25,  3 users,  load average: 0.34, 0.32, 0.64
BenchmarkIndex500-2             	   10000	    101014 ns/op	  275320 B/op	      39 allocs/op
BenchmarkIndexAfterTouch500-2   	    1258	    880238 ns/op	  667500 B/op	    1666 allocs/op
BenchmarkIndexPage500-2         	    7336	    166851 ns/op	  114496 B/op	     502 allocs/op
BenchmarkRescan500-2            	       1	1010204056 ns/op	1284276776 B/op	 1406748 allocs/op
BenchmarkNoteCached-2           	   28194	     40924 ns/op	  111575 B/op	      40 allocs/op
```

The index went from 2,006 µs to 101 µs in process, and from 3,062 allocations to 39. Most of the 101 µs is the test recorder copying a 68 KB page. The first index after an edit is 0.88 ms, the note rendered again included. (`Rescan500` is 1 s because the bench folder is 500 copies of the samples, which average 50 KB. The real notes start in 0.3 s.)

```
shale scripts/bench.shale dir=.run/real rounds=500
```

```text
2026/11/13 16:57:56 rendered 505 notes from .run/real in 319ms; comments in .run/bench/comments
index: 500 requests, 0 not 200, 67712 bytes average; median 0.105 ms, p90 0.143 ms, p99 0.184 ms, max 0.664 ms
notes, no comments: 505 requests, 0 not 200, 22341 bytes average; median 0.115 ms, p90 0.164 ms, p99 0.273 ms, max 0.527 ms
largest note, no comments: 500 requests, 0 not 200, 199171 bytes average; median 0.157 ms, p90 0.219 ms, p99 0.322 ms, max 0.616 ms
notes, with comments: 505 requests, 0 not 200, 23137 bytes average; median 0.106 ms, p90 0.149 ms, p99 0.246 ms, max 0.688 ms
largest note, 100 comments: 500 requests, 0 not 200, 224175 bytes average; median 0.143 ms, p90 0.186 ms, p99 0.296 ms, max 0.638 ms
```

**After: the index's median is 0.105 ms over HTTP, against 2.276 ms before, now as fast as a note.** Its p99 went from 4.09 ms to 0.18 ms.

## The dates on the real notes, and a slow git

I started the new server on the real notes, read only, with its comments in scratch:

```shale
-- the new server on the real notes (read only; comments to a scratch folder): its start-up log, and the index's top against the old order by file time
const REAL = "/home/steve/showell_repos/essay-repl-server/notes"
must(run({"go", "build", "-o", "essay-surface", "."}, {timeout_ms = 120000, tail = 20}))
if exists(".run/look") then remove_tree(".run/look") end
mkdir(".run/look")
local job = job_start({"./essay-surface", "-notes", REAL, "-comments", ".run/look/comments"}, {log = ".run/look.log", cwd = ".", want = "build: the server we just built"})
local w = job_wait(job, {timeout_ms = 30000, contains = "listening"})
local idx = attempt(http_get, "http://127.0.0.1:9300/")
job_stop(job)
print("server stopped (" .. w.ended .. ")")
print(read(".run/look.log"))
if not idx.ok then fail(idx.error) end
local rows = {}
for m in take(re_find_all(idx.value.body, "href=\"/notes/([^\"]+)\">[^<]*</a><span class=\"date\">([0-9-]+)<"), 12) do
  push(rows, {note = m.groups[1], date = m.groups[2]})
end
print("new order:")
print(table(rows, {"date", "note"}))
local old = {}
for p in glob("../essay-repl-server/notes/*.md") do push(old, {note = basename(p), modified = stat(p).modified}) end
```

```text
server stopped (found)
2026/11/13 16:58:12 rendered 506 notes from /home/steve/showell_repos/essay-repl-server/notes in 3.603s; comments in .run/look/comments
2026/11/13 16:58:12 dates: 309 from the notes, 197 from git, 0 from their files (git in 3.194s)
new order:
date        note
2026-11-13  shale-book-draft-2-plan.md
2026-11-13  essay-surface-tuesday.md
...
── stopped at the prelude:86:35 (in `the prelude's glob`): fail: `glob`: "../essay-repl-server/notes/*.md" has a part ("..") that is not a name
```

Two findings. The stop at the end was the same `..` mistake as before, so I dropped the comparison by file time; the clone check below does it better. The real finding: **git took 3.2 s**, against 0.7 s for the whole folder this morning. Passing 197 names as pathspecs makes git match each against every commit's changes. Then, before trusting the top of the index (twelve notes all dated today), I checked that today's commits weren't one bulk commit stamping everything:

```shale
-- why so many undated notes are "today": the recent commits that touch notes/, and how many notes each touches
local r = must(run({"git", "log", "-12", "--no-renames", "--format=%x00%h %ci %s", "--name-only", "--", "notes"}, {timeout_ms = 60000, whole = 2000000}))
for c in split(r.out, "\0") do
  local ls = lines(c)
  if #ls > 0 then
    local n = 0
    for l in ls do if ends_with(l, ".md") then n = n + 1 end end
    print(n .. " notes: " .. sub(ls[1], 1, 110))
  end
end
```

```text
1 notes: a3cd28d 2026-11-13 16:57:18 +0000 notes: draft 2 of the Shale book, the week as it's happening
1 notes: 2c371c8 2026-11-13 16:45:38 +0000 notes: the essay surface, Tuesday
...
1 notes: fd4c635 2026-11-13 12:59:07 +0000 notes: index term count
```

One note a commit: they really were written today. So the last commit is an honest date. The pathspec fix: past sixteen names, log the folder once and keep the names asked for.

```shale
-- 197 pathspecs made git log 3.2 s against 0.7 s for the whole folder: past a few names, ask about the folder and keep the names asked for
edit("dates.go", {
  {old = [=[// gitDates gives the time of the last commit ...]=], new = [=[// Git matches every pathspec against every commit's changes, so past a
// few names one log of the whole folder is faster: on the real notes, 197
// names took 3.2 s and the folder 0.7 s.
const gitPathspecsMost = 16
...]=]},
  {old = [=[		if _, seen := out[line]; line != "" && !seen {]=], new = [=[		if _, seen := out[line]; wanted[line] && !seen {]=]},
})
-- (the Go is in 3f507cb)
print(must(run({"go", "test", "-count=1", "-run", "Date", "."}, {timeout_ms = 110000, tail = 5})).out)
must(run({"go", "build", "-o", "essay-surface", "."}, {timeout_ms = 120000, tail = 20}))
const REAL = "/home/steve/showell_repos/essay-repl-server/notes"
if exists(".run/look") then remove_tree(".run/look") end
mkdir(".run/look")
local job = job_start({"./essay-surface", "-notes", REAL, "-comments", ".run/look/comments"}, {log = ".run/look.log", cwd = ".", want = "build: the server we just built"})
local w = job_wait(job, {timeout_ms = 30000, contains = "listening"})
job_stop(job)
print("server stopped (" .. w.ended .. ")")
print(read(".run/look.log"))
```

```text
edit dates.go: 2 hunks, -9 +23 lines
ok  	essay-surface	0.045s
2026/11/13 16:58:40 rendered 506 notes from /home/steve/showell_repos/essay-repl-server/notes in 1.032s; comments in .run/look/comments
2026/11/13 16:58:40 dates: 309 from the notes, 197 from git, 0 from their files (git in 693ms)
```

Was the remaining 0.7 s git's, or the repo's loose objects?

```shale
-- is the 0.7 s git's, or the notes repo's 3,961 loose objects? a packed clone of it (in .run, read only from the original) answers
const SRC = "/home/steve/showell_repos/essay-repl-server"
if exists(".run/packed") then remove_tree(".run/packed") end
local c = must(run({"git", "clone", "-q", "--no-local", SRC, ".run/packed"}, {timeout_ms = 120000, tail = 10}))
print("clone: " .. c.ms .. " ms")
local r = must(run({"git", "log", "--no-renames", "--relative", "--format=%x00%ct", "--name-only", "--", "."}, {cwd = ".run/packed/notes", timeout_ms = 60000, whole = 20000000}))
print("git log in the packed clone: " .. r.ms .. " ms wall, " .. r.cpu_ms .. " ms cpu")
local o = must(run({"git", "count-objects", "-v"}, {cwd = ".run/packed", timeout_ms = 60000, tail = 20, re = "^(count|in-pack)"}))
print(o.out)
```

```text
clone: 2147 ms
git log in the packed clone: 79 ms wall, 78 ms cpu
count: 0
in-pack: 3964
```

79 ms packed, against 690 ms loose. A `git gc` in essay-repl-server would make the server start in about 0.4 s instead of 1.0 s. That repo is yours, so I left it alone.

And the point of the whole change: a fresh clone resets every file's time. Does it give the same index?

```shale
-- the point of the dates: a fresh clone (every file's time reset) must give the same index as the original folder
const REAL = "/home/steve/showell_repos/essay-repl-server/notes"
function index_of(dir, tag)
  local log = ".run/" .. tag .. ".log"
  local job = job_start({"./essay-surface", "-notes", dir, "-comments", ".run/" .. tag .. "-comments"}, {log = log, cwd = ".", want = "build: the server we just built"})
  local w = job_wait(job, {timeout_ms = 30000, contains = "listening"})
  local r = attempt(http_get, "http://127.0.0.1:9300/")
  job_stop(job)
  if w.ended ~= "found" then fail(tag .. ": the server did not start: " .. w.ended) end
  if not r.ok then fail(tag .. ": " .. r.error) end
  print(tag .. ": " .. lines(read(log))[2])
  return re_find_all(r.value.body, "<li><a href=\"/notes/[^\"]+\">")
end
local a = index_of(REAL, "original")
local b = index_of(".run/packed/notes", "clone")
local same = 0
local first_diff = 0
for i = 1, min(#a, #b) do
  if a[i].text == b[i].text then same = same + 1 elseif first_diff == 0 then first_diff = i end
end
print(#a .. " and " .. #b .. " entries; " .. same .. " in the same place; first difference at " .. first_diff)
local mt = {}
for p in take(glob(".run/packed/notes/*.md"), 400) do incr(mt, "" .. stat(p).modified) end
print(#keys(mt) .. " distinct file times among the clone's first 400 notes")
```

```text
original: 2026/11/13 16:59:03 dates: 309 from the notes, 197 from git, 0 from their files (git in 786ms)
clone: 2026/11/13 16:59:04 dates: 309 from the notes, 197 from git, 0 from their files (git in 117ms)
506 and 506 entries; 506 in the same place; first difference at 0
1 distinct file times among the clone's first 400 notes
```

All 506 notes sit in the same place, though every file in the clone has one and the same time. That comparison became a stage of the full check (`scripts/dates.shale`). Committed as `3f507cb`.

## The checks

### What I changed in yesterday's scripts, and why

- **`check.shale` became `fast.shale`.** It now runs the smoke test as a child, holds every stage to a CPU budget, and says how many tests ran ("44 tests passed, 63 subtests"), since a green is a claim. `check.shale` is deleted; git has it.
- **What the scripts shared by copy moved into `.shale/prelude.shale`**: build, start (with a wait for "listening", and a stop that prints the log), stop (which reads the server's CPU line), a fresh scratch folder, a page's fingerprints, an index's names, and the box's load. Session 2's three scripts each had their own start block.
- **Each script owns `.run/<script>/`.** Session 2's audit ran `remove_tree(".run")`, which deleted the bench's record along with its own (Session 2's journal says so in passing).
- **`audit.shale` has a verdict.** It fails on any leftover not known to be faithful. The three examined pages are named with their counts (a lone ```` ``` ```` quoted in prose, and `` `SEED \`<hash>`` ``), so a page whose count changes is looked at again. That is a short named list, not a blanket.
- **`bench.shale` has a verdict, and copies the notes first** (it posts comments). Each phase runs on its own server, and the server reports its CPU time when stopped. The medians are judged only on a quiet box. More on this below.
- **`comments_smoke.shale`** uses the prelude and clears its whole scratch folder. My first edit cleared only its `notes/` and would have kept the last run's comments.
- **The anchoring simulation (`calibrate_test.go`) is simpler, and gives a verdict on the real notes.** It used to try four thresholds, Session 2's experiment, which is kept in `8c05754`. It now calls `resolve` itself at the one threshold in use. With `ESSAY_NOTES` set it used to log and return. Now it fails if an edit in place loses a comment, or if more than a set rate lands on the wrong block (split 1%, merged 2%, moved 1%, deleted 2.5%). The deleted-block examples print one to a line, so the full check can show them.

### The server reports its CPU

For a timing that a busy box can't break, I wanted CPU time, and only the server knows its own. It now logs its CPU at start, and on SIGTERM (which `job_stop` sends) it logs the requests it served and its CPU since the start:

```shale
-- the server says its CPU time: at the start, and on SIGTERM with the requests it served, so a bench can hold CPU a request to a budget
edit("server.go", {
  {old = [=[	log.Printf("listening on http://%s/", *addr)
	log.Fatal(http.ListenAndServe(*addr, s.handler()))
}]=], new = [=[...]=]},
  {old = "	\"net/http\"\n	\"path/filepath\"\n	\"strconv\"\n	\"strings\"\n	\"time\"\n)", new = "..."},
})
-- (the counting handler, the SIGTERM line and cpuTime; the Go is in a4a2029)
local v = run({"go", "vet", "."}, {timeout_ms = 120000, tail = 30})
print("vet exit " .. v.code)
print(v.out)
```

```text
edit server.go: 2 hunks, -9 +39 lines
vet exit 0
```

### The simulation, simplified

```shale
-- the simulation: one threshold, the one in use (Session 2's four were the experiment, kept in 8c05754); the real notes get a verdict, not just a log
edit("calibrate_test.go", {
  -- (five pairs: tally, simulate, TestSimulateAnchoring; the Go is in a4a2029)
})
local r = run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-v", "."}, {timeout_ms = 110000, tail = 30, re = "cases|ok|FAIL|_test.go"})
print("samples: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
local real = run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-v", "."}, {timeout_ms = 110000, tail = 30, re = "cases|ok|FAIL|_test.go", env = {ESSAY_NOTES = "/home/steve/showell_repos/essay-repl-server/notes"}})
print("real: exit " .. real.code .. ", " .. real.ms .. " ms, cpu " .. real.cpu_ms)
print(real.out)
```

```text
./calibrate_test.go:193:5: inPlace redeclared in this block
./calibrate_test.go:214:16: cannot index inPlace (constant 2 of int type how)
```

`inPlace` is already `anchor.go`'s name for rule 2. The edits got their own name:

```shale
-- inPlace is already anchor.go's name for rule 2: the edits get their own
edit("calibrate_test.go", "inPlace", "editsInPlace", {all = true, word = true})
local r = run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-v", "."}, {timeout_ms = 110000, tail = 30, re = "cases|ok|FAIL|_test.go"})
print("samples: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
local real = run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-v", "."}, {timeout_ms = 110000, tail = 30, re = "cases|ok|FAIL|_test.go", env = {ESSAY_NOTES = "/home/steve/showell_repos/essay-repl-server/notes"}})
print("real: exit " .. real.code .. ", " .. real.ms .. " ms, cpu " .. real.cpu_ms)
print(real.out)
```

```text
samples: exit 0, 2680 ms, cpu 3561
    calibrate_test.go:204: split        cases   385: right   384, on another block    1, orphaned    0
    calibrate_test.go:204: merged       cases   519: right   279, on another block    6, orphaned  234
    calibrate_test.go:204: moved, typo  cases   526: right   497, on another block    6, orphaned   23
    calibrate_test.go:204: deleted      cases   528: right   520, on another block    8, orphaned    0
real: exit 0, 13357 ms, cpu 14778
    calibrate_test.go:204: typo         cases 20830: right 20830, on another block    0, orphaned    0
    ...
    calibrate_test.go:204: split        cases 16050: right 15970, on another block   80, orphaned    0
    calibrate_test.go:204: merged       cases 20374: right 11640, on another block  164, orphaned 8570
    calibrate_test.go:204: moved, typo  cases 20830: right 19566, on another block   96, orphaned 1168
    calibrate_test.go:204: deleted      cases 20866: right 20600, on another block  266, orphaned    0
```

The samples' pinned counts are exactly Session 2's, so calling `resolve` directly changed no decision. On the real notes: split 0.50% wrong, merged 0.80%, moved 0.46%, deleted 1.27%, all under their limits.

13 s is over the ten that makes slowness a finding, so I profiled it:

```shale
-- 13 s on the real notes is a finding: where it goes, by CPU profile
mkdir(".run/prof")
must(run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-cpuprofile", ".run/prof/sim.prof", "-o", ".run/prof/sim.test", "."}, {timeout_ms = 110000, tail = 5, env = {ESSAY_NOTES = "/home/steve/showell_repos/essay-repl-server/notes"}}))
print(must(run({"go", "tool", "pprof", "-top", "-cum", "-nodecount=22", ".run/prof/sim.test", ".run/prof/sim.prof"}, {timeout_ms = 60000, tail = 30})).out)
```

```text
         0     0%  0.07%     10.47s 73.11%  essay-surface.resolve
     1.02s  7.12%  7.19%      8.66s 60.47%  essay-surface.(*blockIndex).place
     3.58s 25.00% 32.19%      3.60s 25.14%  essay-surface.dice (inline)
     0.08s  0.56% 32.75%      3.46s 24.16%  essay-surface.wordFeatures
     0.10s   0.7% 33.45%      1.89s 13.20%  essay-surface.indexBlocks
```

It's the work being measured: 187,000 simulated edits, and 60% of the time is the similarity search for blocks that were moved, merged or deleted. Sampling fewer blocks barely helps, because most notes have fewer than 50 blocks and all of them are taken anyway. It stays at about 12 s of the full check, and I say so in the report.

### The prelude, the fast check, the smoke

`.shale/prelude.shale` and `scripts/fast.shale` were written whole. The smoke moved onto the prelude:

```shale
-- comments_smoke.shale onto the repo prelude: the start, the stop and the fingerprints are shared now
edit("scripts/comments_smoke.shale", {
  {old = [=[  local p = page()
  local hs = json_read(between(p, "<script id=\"blocks\" type=\"application/json\">", "</script>"))]=], new = [=[  local p = page()
  local hs = essay_hashes(p)]=]},
  {old = [=[must(run({"go", "build", "-o", "essay-surface", "."}, {timeout_ms = 120000, tail = 20}))
...
print("server stopped")]=], new = [=[essay_build()
essay_fresh(DIR .. "/notes")
create(NOTE, ...)
local job = essay_start(DIR .. "/notes", DIR .. "/comments", DIR .. "/server.log")
local result = attempt(flow)
essay_stop(job, DIR .. "/server.log")
print("server stopped")]=]},
})
```

```text
edit scripts/comments_smoke.shale: 2 hunks, -16 +9 lines
```

I caught on reading it back that `essay_fresh(DIR .. "/notes")` keeps the last run's comments:

```shale
-- the smoke's scratch must be cleared whole, comments and all, not just its notes; then run it and fast
edit("scripts/comments_smoke.shale", "essay_fresh(DIR .. \"/notes\")\n", "essay_fresh(DIR)\nmkdir(DIR .. \"/notes\")\n")
```

```
shale scripts/fast.shale
```

```text
ok  gofmt, 28 ms, cpu 38 ms
ok  vet, 355 ms, cpu 537 ms
ok  js, 36 ms, cpu 35 ms
ok  test, 1768 ms, cpu 2029 ms
ok  smoke, 1293 ms, cpu 1683 ms
fast: PASS
```

It passed, but a green should say how many ran. I made `test` run with `-v` and count, and `smoke` count its `ok:` lines (the edit is in `a4a2029`). The first run of that stopped at once:

```text
── stopped at scripts/fast.shale:17:16: name: no function `take_last` above this line: a function is called only after it is declared (0 steps)
```

That was a guessed builtin. The end of a list is `last`:

```shale
-- a guessed builtin: the list's end is `last`
edit("scripts/fast.shale", "take_last(lines(r.out), 60)", "last(lines(r.out), 60)")
```

```text
ok  gofmt, 27 ms, cpu 41 ms
ok  vet, 198 ms, cpu 335 ms
ok  js, 45 ms, cpu 44 ms
ok  test, 1870 ms, cpu 2148 ms: 44 tests passed, 63 subtests
ok  smoke, 309 ms, cpu 326 ms: 10 checks through the real server
fast: PASS
── time: 5 programs, 2.4 s in all; the slowest 1.8 s (76%): go test -count=1 -v .; 0.3 s (12%): shale scripts/comments_smoke.shale; 0.1 s (8%): go vet .
```

### The audit

`scripts/audit.shale` was rewritten whole. My first draft called `map_field`, another guessed builtin, which I replaced with a loop before running it:

```shale
-- no guessed builtin this time: the names of the unexplained pages by a loop
edit("scripts/audit.shale", [=[if #unexplained > 0 then
  fail(#unexplained .. " pages with leftovers not known to be faithful: " .. join(take(map_field(unexplained, "note"), 10), ", "))
end]=], [=[if #unexplained > 0 then
  local which = {}
  for a in take(unexplained, 10) do push(which, a.note .. " (" .. a.ticks .. " ticks)") end
  fail(#unexplained .. " pages with leftovers not known to be faithful: " .. join(which, ", "))
end]=])
```

```
shale scripts/audit.shale dir=/home/steve/showell_repos/essay-repl-server/notes
```

```text
506 pages, 24013 commentable blocks, 3 pages with leftovers
stars  links  tables  ticks  note
    0      0       0      3  essay-surface-monday.md
    0      0       0      3  essay-surface-quality.md
    0      0       0      2  update-53-and-the-checklists-first-day.md
audit: PASS
```

### The bench: CPU a request, by phase

`scripts/bench.shale` was rewritten whole, and its scratch folder fixed to be cleared whole, as the smoke's was:

```shale
-- the bench's scratch is cleared whole, so last run's notes and comments go too
edit("scripts/bench.shale", "essay_fresh(RUN .. \"/pages\")\n", "essay_fresh(RUN)\nmkdir(RUN .. \"/pages\")\n")
```

I gave it a placeholder CPU budget of 200 µs a request, to be set from what it measured:

```
shale scripts/bench.shale dir=/home/steve/showell_repos/essay-repl-server/notes rounds=500
```

```text
2026/11/13 17:04:35 cpu: 327 ms to start
index: 500 requests, 0 not 200, 67842 bytes average; median 0.104 ms, ...
── stopped at scripts/bench.shale:182:28: fail: cpu: 246 us a request over 5646 requests (1394 ms), budget 200
```

The budget stopped it, as it should have, and the servers were already stopped. But one figure for the whole mix is the wrong measure. The 1,615 posts dominate it, and going back to scanning would add about 1.4 ms to each of only 500 index requests, about 125 µs on average, which hides under a budget sized for posts. So **each phase gets its own server: index, notes, comments**, and its CPU is its own:

```shale
-- one CPU figure for the whole mix let the posts hide the index (a return to the scan would add about 1.4 ms a request to 500 of 5,646): each phase gets its own server, so its CPU is its own
edit("scripts/bench.shale", {
  -- (measure split into phase_index, phase_notes, phase_comments; phase_end; three starts and stops: in a4a2029)
})
```

```text
edit scripts/bench.shale: 3 hunks, -41 +79 lines
```

Two runs, to see the spread:

```text
cpu, index: 160 us a request over 500 requests (80 ms), budget 1000: PASS
cpu, notes: 179 us a request over 1513 requests (272 ms), budget 1000: PASS
cpu, comments: 365 us a request over 3634 requests (1328 ms), budget 1000: PASS
latency: every median within budget (load 0.20 on 2 CPUs before, 0.27 on 2 CPUs after): PASS
```

```text
cpu, index: 140 us a request over 500 requests (70 ms), budget 1000: PASS
cpu, notes: 173 us a request over 1513 requests (262 ms), budget 1000: PASS
cpu, comments: 344 us a request over 3634 requests (1251 ms), budget 1000: PASS
```

The budgets went to a little over twice those numbers: index 400, notes 400, comments 800. To prove the index budget would catch a step back, I planted the old scan (a stat of every note on every index request) and ran it once:

```shale
-- budgets from two runs (index 140 and 160 us, notes 173 and 179, comments 344 and 365): a little over twice; then a plant, the old per-request stat of every note, which the index budget must catch
edit("scripts/bench.shale", [=[-- The most CPU a request may take in each phase, in microseconds.
const CPU_BUDGET_US = {index = 1000, notes = 1000, comments = 1000}]=], [=[-- The most CPU a request may take in each phase, in microseconds: a
-- little over twice what the real notes took on 2026-11-13 (index 140 and
-- 160, notes 173 and 179, comments 344 and 365). Reading every note's file
-- for each index request, as the server did before, costs about 1,400.
const CPU_BUDGET_US = {index = 400, notes = 400, comments = 800}]=])
edit("store.go", "	if s.index == nil {\n		s.index = indexPage(s.list())\n", "	for _, n := range s.list() {\n		os.Stat(filepath.Join(s.dir, n.name))\n	}\n	if s.index == nil {\n		s.index = indexPage(s.list())\n")
local r = run({"shale", "scripts/bench.shale", "dir=/home/steve/showell_repos/essay-repl-server/notes", "rounds=500"}, {timeout_ms = 110000, tail = 4})
edit("store.go", "	for _, n := range s.list() {\n		os.Stat(filepath.Join(s.dir, n.name))\n	}\n	if s.index == nil {\n		s.index = indexPage(s.list())\n", "	if s.index == nil {\n		s.index = indexPage(s.list())\n")
print("planted: exit " .. r.code)
print(r.out)
print(json_write(git_status(".")))
```

```text
planted: exit 1
── stopped at scripts/bench.shale:193:26 (in `phase_end`): fail: cpu, index: 2170 us a request over 500 requests (1085 ms), budget 400 (113025 steps)
```

The plant cost 2,170 µs of CPU a request against a budget of 400, so the budget catches it on any box.

### The dates check

`scripts/dates.shale` was written whole: it clones the folder's repo into `.run/dates` and compares the two indexes, note by note. A note that isn't committed yet is counted, not compared. Its first run used `real(dir)` on an absolute path, so I fixed that first:

```shale
-- real() takes the root's paths only: an absolute dir goes to git as it is
edit("scripts/dates.shale", {
  {old = [=[...local top = trim(must(run({"git", "-C", real(dir), ...]=], new = [=[...local os_dir = dir
if not starts_with(dir, "/") then os_dir = real(dir) end
...]=]},
})
```

```
shale scripts/dates.shale dir=/home/steve/showell_repos/essay-repl-server/notes
```

```text
cloned /home/steve/showell_repos/essay-repl-server in 316 ms
── stopped at the prelude:244:15 (in `the prelude's re_find_all`): budget steps: the step budget (5000000 steps) ran out (5000001 steps)
```

`re_find_all` over two 68 KB pages costs a step a character. My fix ran into a new trap. The Shale script that would add the `-- budget:` header carried that line inside a long string, and `shale` reads budget lines from its own script before running it:

```text
shale: a `-- budget:` line after the script's first line of code is never read: put it among the comments opening the script
```

So nothing ran. I made the edit with my editor instead (`-- budget: steps=30000000`, with the reason above it). Then:

```text
── budgets from the script's header: steps=30000000
cloned /home/steve/showell_repos/essay-repl-server in 329 ms
folder: dates: 309 from the notes, 197 from git, 0 from their files (git in 914ms)
clone:  dates: 309 from the notes, 197 from git, 0 from their files (git in 788ms)
506 notes with the same date in the same place; 0 in the folder alone, not committed yet
dates: PASS
```

A plant, once, to prove it can fail: git is never asked, so the undated notes fall back to their files' times.

```shale
-- plant once: git never asked, so undated notes fall to their files' times; the dates check must say so
edit("store.go", "	if len(ask) > 0 && s.gitErr == nil {\n", "	if false && len(ask) > 0 && s.gitErr == nil {\n")
local r = run({"shale", "scripts/dates.shale", "dir=/home/steve/showell_repos/essay-repl-server/notes"}, {timeout_ms = 110000, tail = 6})
edit("store.go", "	if false && len(ask) > 0 && s.gitErr == nil {\n", "	if len(ask) > 0 && s.gitErr == nil {\n")
print("planted: exit " .. r.code)
print(r.out)
```

```text
planted: exit 1
folder: dates: 309 from the notes, 0 from git, 197 from their files (git in 0s)
clone:  dates: 309 from the notes, 0 from git, 197 from their files (git in 0s)
── stopped at scripts/dates.shale:48:57: fail: in the clone, some notes are dated by their files: dates: 309 from the notes, 0 from git, 197 from their files (git in 0s)
```

### The full check

The race detector's cost, warm, before giving it a stage:

```shale
-- the race detector over everything but the simulation (pure, single-goroutine, and ten times slower under -race): its cost, warm
local r = run({"go", "test", "-race", "-count=1", "-skip", "TestSimulateAnchoring", "."}, {timeout_ms = 110000, tail = 10})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
```

```text
exit 0, 3652 ms wall, cpu 3318 ms
ok  	essay-surface	1.598s
```

`scripts/full.shale` was written whole, plus one small tidy (an unused local, and the header now says that a CPU budget's stop ends the check at once). Its first run:

```text
== fast: PASS, 3946 ms, cpu 4753 ms
...
== race: PASS, 2821 ms, cpu 2131 ms
── stopped at the Linux prelude:11:10 (in `the Linux prelude's run`): argument: `run`'s `whole` keeps everything, and takes no filter or keep (`re`, ...)
```

The anchoring stage filters its lines with `re`, and `whole` takes no filter. So the stage now keeps a tail, and it shows a stage's own lines rather than a child's time summary:

```shale
-- run's whole takes no filter: keep the last 400 lines instead, and show a stage's own lines, not a child's time summary
edit("scripts/full.shale", {
  {old = "  opts.whole = 400000\n", new = "  opts.tail = 400\n"},
  {old = [=[  for l in last(lines(r.out), shown) do print("   " .. l) end]=], new = [=[  local own = {}
  for l in lines(r.out) do
    if not starts_with(l, "── ") then push(own, l) end
  end
  for l in last(own, shown) do print("   " .. l) end]=]},
})
```

The second run passed, `SAFE TO SHIP` in 27.6 s. Two lines in it were noise: a stray `%{http_code} http://1; 1 job wait` (the second line of a child's time summary, since curl's `-w` argument holds a newline), and the deleted-block examples, which the filter dropped because they spanned two lines:

```shale
-- a stage's shown lines end where the child's time summary starts (its curl argv holds a newline); the deleted-block examples go on one line each, so the anchoring stage can show them
edit("scripts/full.shale", [=[...]=], [=[...    if starts_with(l, "── time:") then break end...]=])
edit("scripts/full.shale", [=[re = "cases|--- |^ok|^FAIL|_test.go"}, 12, failed)]=], [=[re = "cases|->|--- |^ok|^FAIL|_test.go"}, 17, failed)]=])
edit("calibrate_test.go", [=[shown = append(shown, "  "+cut(blocks[k].norm)+"\n    -> "+cut(after[p.block].norm))]=], [=[shown = append(shown, "  "+cut(blocks[k].norm)+" -> "+cut(after[p.block].norm))]=])
local r = must(run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-v", "."}, {timeout_ms = 60000, tail = 10, re = "->"}))
print(r.out)
```

```text
          `app/main.roc` -> `platform/main.roc`
          🍃 from Cat.roc -> 🌳 from Cat.roc
          *[The beacon widget goes here: drag the clock and watch the masts blink out of step.]* -> *[The sky widget goes here: drag the clock and watch the sun set.]*
```

Those are the 1.3% of deleted blocks whose comments go to a look-alike. They are the honest cost of rule 3, and Session 2's report named it.

The README names the two checks, and `check.shale` went:

```shale
-- check.shale is fast.shale now; the README names the two checks, the only ones anyone needs to remember
remove("scripts/check.shale")
append("README.md", [=[
Two checks, run from the repo root:
...]=])
print(read("README.md"))
```

(with a follow-up edit for a missing blank line and one wrong claim, that every stage takes `dir=`; the smoke makes its own notes). Fast passed, and I committed `a4a2029`.

### A red full check, on purpose

A check that has only ever said SAFE hasn't been tested. I planted a renderer bug that a browser hides: `**x**` rendered as `<b>` instead of `<strong>`.

```shale
-- plant once: strong emphasis written as <b>, which looks right in a browser and is wrong. Full must say NOT SAFE, naming the stages that saw it; then the code goes back
edit("markdown.go", "				tag = \"strong\"\n", "				tag = \"b\"\n")
local r = run({"shale", "scripts/full.shale"}, {timeout_ms = 115000, tail = 80, not_re = "^       calibrate|^   (index|notes|largest|posted|506)"})
edit("markdown.go", "				tag = \"b\"\n", "				tag = \"strong\"\n")
print("planted: exit " .. r.code .. ", " .. r.ms .. " ms")
print(r.out)
print(json_write(git_status(".")))
```

```text
== fast: FAIL, 3206 ms, cpu 4221 ms
   === RUN   TestQueueOverflowRescans
   --- PASS: TestQueueOverflowRescans (0.09s)
   FAIL
   FAIL	essay-surface	1.264s
   FAIL
      called from scripts/fast.shale:38:14
== race: FAIL, 2982 ms, cpu 3375 ms
== anchoring: PASS ...
== audit: PASS ...
== dates: PASS ...
== bench: PASS, 7441 ms, cpu 2464 ms
   cpu, comments: 419 us a request over 3634 requests (1523 ms), budget 800: PASS
   latency: not judged, the box was busy (load 1.45 on 2 CPUs before, 1.65 on 2 CPUs after)
NOT SAFE: fast, race failed
```

The verdict was right, and three things came out of it:

1. **The red fast stage showed nothing useful**: the last lines of `go test -v` are passes. That's fixed below.
2. **The audit passed the `<b>` plant.** It looks for markdown left over, not for which tags are used. The renderer's tests caught it. That's the division of labor, and worth knowing.
3. **The box went busy mid-run** (load 1.45 to 1.65), and the bench said "not judged" for latency while the CPU budgets still held. The comments phase's CPU rose from about 350 to 419 µs under load, still well inside 800. That's the behavior the prompt asked for, seen live.

The fix: from `go test -v`'s output, fast keeps the failed top-level tests and only the lines that came under them, and its stop names them. A passing test's logs, such as the simulation's tallies, are left out. My first try matched `_test.go:N:` lines, which caught the simulation's passing logs too. The second try declared `failed_tests` below its caller, which Shale refuses, and used `said`, a function's name, as a local. So:

```shale
-- a function is called only after it is declared: failed_tests goes above stage; its map is logs, not said, which is a function's name
local src = read("scripts/fast.shale")
local a = find(src, "-- In go test -v's output")
local b = find(src, "function said(name, r, what)")
local block = sub(src, a, b - 1)
edit("scripts/fast.shale", {
  {old = block, new = ""},
  {old = "-- One stage: its program run, its verdict, its times on one line.\n", new = block .. "-- One stage: its program run, its verdict, its times on one line.\n"},
})
edit("scripts/fast.shale", {
  {old = "  local said = {}\n  local current", new = "  local logs = {}\n  local current"},
  {old = "for s in get(said, fail_m.group, {})", new = "for s in get(logs, fail_m.group, {})"},
  {old = "push_at(said, current, l)", new = "push_at(logs, current, l)"},
})
edit("markdown.go", "				tag = \"strong\"\n", "				tag = \"b\"\n")
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 30})
edit("markdown.go", "				tag = \"b\"\n", "				tag = \"strong\"\n")
print("planted: exit " .. r.code)
print(r.out)
```

```text
ok  gofmt, 59 ms, cpu 59 ms
ok  vet, 479 ms, cpu 487 ms
ok  js, 87 ms, cpu 52 ms
    markdown_test.go:100: Render("*a* _b_ **c** __d__")
         got: "<p id=\"p1\"><em>a</em> <em>b</em> <b>c</b> <b>d</b></p>\n"
        want: "<p id=\"p1\"><em>a</em> <em>b</em> <strong>c</strong> <strong>d</strong></p>\n"
    ...
--- FAIL: TestRender (0.00s)
── stopped at scripts/fast.shale:53:3 (in `stage`): fail: test: 1 failed: TestRender (86718 steps)
```

That red now says what failed and where. Committed as `ec31e6a`. Then I tidied the old layout's scratch files. My `glob(".run/*")` ran out of steps walking every file below, so I used `ls(".run")`:

```shale
-- the old layout's logs and pages go; each script now keeps its scratch in its own folder
for n in ls(".run") do
  if not has_item({"audit", "bench", "dates", "smoke"}, n) then
    if is_dir(".run/" .. n) then remove_tree(".run/" .. n) else remove(".run/" .. n) end
  end
end
print(json_write(ls(".run")))
```

```text
["audit","bench","dates","smoke"]
```

## Last: nothing of mine left up

```shale
-- is anything of mine still up? every essay-surface process and where it runs from; the scratch folders' jobs
for p in procs() do
  if p.name == "essay-surface" then print("essay-surface pid " .. p.pid .. ": " .. sub(p.args, 1, 140)) end
end
for d in ls(".run") do
  for f in ls(".run/" .. d) do
    if ends_with(f, ".job") then print(".run/" .. d .. "/" .. f .. " alive: " .. (job_alive(".run/" .. d .. "/" .. f) and "yes" or "no")) end
  end
end
print(json_write(git_status(".")))
```

```text
essay-surface pid 2477358: /home/steve/essay-dogfood/bin/essay-surface -notes /home/steve/essay-dogfood/notes -addr 127.0.0.1:9301
.run/audit/server.log.job alive: no
.run/bench/comments.log.job alive: no
.run/bench/index.log.job alive: no
.run/bench/notes.log.job alive: no
.run/dates/clone.log.job alive: no
.run/dates/folder.log.job alive: no
.run/smoke/server.log.job alive: no
[]
```

The one process is your preview on 9301. I didn't touch it.

## Report

**The two checks.** Both are named in the README, so nobody has to remember a list:

- `shale scripts/fast.shale`, for every edit: about **3 seconds** (2.4 to 3.9 s across today's runs; CPU about 3 to 4.4 s).
- `shale scripts/full.shale`, before shipping, on all the real notes: about **28 seconds** (27.6 and 29.1 s on clean runs; 36.7 s on a busy box). The anchoring simulation is 12 of those seconds, explained above.

The last lines of a real run of each, from the end of the day:

```text
ok  gofmt, 41 ms, cpu 58 ms
ok  vet, 380 ms, cpu 576 ms
ok  js, 53 ms, cpu 53 ms
ok  test, 2159 ms, cpu 2340 ms: 44 tests passed, 63 subtests
ok  smoke, 482 ms, cpu 445 ms: 10 checks through the real server
fast: PASS
```

```text
box: load 1.43 on 2 CPUs; notes: /home/steve/showell_repos/essay-repl-server/notes
== fast: PASS, 3599 ms, cpu 4376 ms
== race: PASS, 4174 ms, cpu 3785 ms
   ok  	essay-surface	1.709s
== anchoring: PASS, 12077 ms, cpu 12916 ms
       calibrate_test.go:204: split        cases 16050: right 15970, on another block   80, orphaned    0
       calibrate_test.go:204: merged       cases 20374: right 11640, on another block  164, orphaned 8570
       calibrate_test.go:204: moved, typo  cases 20830: right 19566, on another block   96, orphaned 1168
       calibrate_test.go:204: deleted      cases 20866: right 20600, on another block  266, orphaned    0
== audit: PASS, 1984 ms, cpu 716 ms
   506 pages, 24013 commentable blocks, 3 pages with leftovers
   audit: PASS
== dates: PASS, 3109 ms, cpu 573 ms
   506 notes with the same date in the same place; 0 in the folder alone, not committed yet
   dates: PASS
== bench: PASS, 4194 ms, cpu 1960 ms
   cpu, index: 104 us a request over 500 requests (52 ms), budget 400: PASS
   cpu, notes: 136 us a request over 1513 requests (207 ms), budget 400: PASS
   cpu, comments: 351 us a request over 3634 requests (1276 ms), budget 800: PASS
   latency: not judged, the box was busy (load 1.29 on 2 CPUs before, 1.27 on 2 CPUs after)
   bench: PASS
by hand still: an eye test of a note and its comments on a phone and a desktop browser
SAFE TO SHIP
```

**What each stage's time is, from that run:** fast 3.6 s, race 4.2 s, anchoring 12.1 s, audit 2.0 s, dates 3.1 s, bench 4.2 s. The audit, dates and bench stages each start the server on the real notes, which takes about 1 s, mostly git.

**Timing and a busy box.** The verdict uses CPU time a request, which the server reports itself when stopped, per phase. A busy box doesn't break that: under load 1.45 the comments phase rose from about 350 to 419 µs against a budget of 800. The wall-clock medians are judged only when the load is under half the CPUs before and after the bench. Otherwise the check says "not judged" and why, as it did above. A planted return to the old scan cost 2,170 µs of CPU per index request, against a budget of 400, and failed.

**The index, before and after** (all real notes, 500 rounds, curl's clock):

| | before | after |
|---|---|---|
| index, median over HTTP | 2.276 ms | 0.105 ms |
| index, p99 over HTTP | 4.090 ms | 0.184 ms |
| index in process | 2,006 µs, 3,062 allocs | 101 µs, 39 allocs |
| index, CPU a request | about 1,400 µs of scan alone | 104 to 160 µs |
| first index after a note changes | (every request) | 0.88 ms in process |

The server now hears changes through inotify and drains the queue at the start of each request. That is one `read` when nothing changed, and an event is always heard by the next request after the write. A side gain: two same-size saves inside one tick of the kernel's file clock were invisible to the old mtime-and-size cache, and they aren't now.

**Dates.** A note's date is the first date in its title or in the paragraph after it (309 of 506 notes), else its last commit's time (197), else its file's time (0 today). A fresh clone, every file with the same time, gives the same index in all 506 places, and the full check proves that each run.

**Still checked by hand:**

1. An eye test of a note and its comments on a phone and a desktop browser. There's still no browser on this box. From your laptop: `ssh -L 9300:127.0.0.1:9300`.
2. Latency on a busy box. When the bench says "not judged", run `shale scripts/bench.shale dir=<notes>` again when `uptime` is quiet.
3. A new faithful leftover in the audit. When a note quotes a lone backtick fence in prose, the audit fails and names it. Someone looks, and if it's faithful, adds it to `FAITHFUL_TICKS` with its count.
4. The anchoring limits are rates I set today from today's notes (split 1%, merged 2%, moved 1%, deleted 2.5%). If the notes drift close to a limit, the limit is a judgment call, not a test bug.

**Calls you may want to unmake:**

- An undated note keeps its last commit's date through uncommitted edits, and a note first seen after start that git doesn't know is dated by its file until the server restarts.
- The server is Linux only now (inotify through `syscall`). No dependency was needed.
- The comments folder may be missing at start, but its parent folder must exist (it's watched for the folder's arrival).
- `git gc` in essay-repl-server would cut the server's start from about 1.0 s to 0.4 s: git log takes 690 ms over 3,961 loose objects there, and 79 ms in a packed clone. That repo is yours, so I left it alone.

**Commits today:** `4a74731` (the store hears changes; dates), `3f507cb` (one git log past sixteen names), `a4a2029` (the two checks), `ec31e6a` (a red check names its failures), and this journal.

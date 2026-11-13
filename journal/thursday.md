# Thursday: every stage seen red, and the reply buttons checked

The prompt is `journal/prompts/thursday.md`. Yesterday's full check says SAFE TO SHIP; today is about believing it. For each stage of `full.shale`, put in the one fault it exists to catch, run it, watch it go red, and take the fault out. That technique is old and has names: **fault seeding**, or **mutation testing** when it's done by machine. We also call a seeded fault a **plant**. Then keep the cheap ones as permanent **known-bad and known-good fixtures** (we call those plants too): small inputs a check must reject or accept, run every time, as data the check reads, never a build of their own. `full.shale` stays under thirty seconds. And Tuesday's failure gets a check: a page that hides its reply buttons must fail, without a browser if that can be done honestly.

The rules I held to for every plant: one fault at a time, put in by an `edit` inside the script that runs the stage, taken out by the reverse `edit` in the same script, then `git diff` empty and `fast.shale` PASS before the next one. No planted fault is ever committed.

All shell work is Shale, one launch per Bash call. Each script is below verbatim, with why I ran it and what it said.

## Orienting

My first call was `cat` on the prompt, and the gate refused it, the same first stop as on days two and three:

```text
PreToolUse:Bash hook error: [...shale-gate.sh]: shale-gate: `cat` is not `shale`; only a launch, ...
```

I read the prompt, the three journals' reports, `scripts/fast.shale`, `scripts/full.shale`, `.shale/prelude.shale`, the other four scripts, `comments.css`, `comments.js`, `server.go`, `store.go` and `docs/SHALE-AGENTS.md` with the file reader. My second launch walked too much:

```shale
for p in glob("**/*", {hidden = true}) do
  if not starts_with(p, ".git/") then print(p, count_lines(p)) end
end
```

```text
── stopped at the prelude:102:19 (in `the prelude's glob`): budget steps: the step budget (5000000 steps) ran out (5000001 steps)
```

`{hidden = true}` walks `.git` and `.run` before my filter sees them. The tracked files are what I wanted:

```shale
-- orient: the tracked files and their sizes, the history, the tree's state
for p in git_ls_files("") do print(p, count_lines(p)) end
for c in git_log("", {n = 25}) do print(c.hash, c.subject) end
print(json_write(git_status("")))
```

```text
.shale/prelude.shale	62
...
comments.css	30
comments.js	128
...
scripts/full.shale	62
server.go	343
store.go	417
watch.go	90
0c2e82d	Journal: the Thursday prompt
9d70b61	Merge branch 'visible-note-buttons'
...
[]
```

The one change since Wednesday, the merge:

```shale
-- what the reply-button merge changed
print(git_diff("", {range = "0b39136..9d70b61"}))
```

```text
-.cbtn{position:absolute;top:.1rem;right:-2.6rem;...;opacity:0;transition:opacity .15s}
+.cbtn{position:absolute;top:.1rem;right:-3.6rem;...;opacity:.6;transition:opacity .15s}
...
-    btn.textContent = n ? String(n) : "+";
+    btn.textContent = n ? String(n) : "reply";
```

So Tuesday's page had `opacity:0` on the resting button, and only `:hover`, `:focus-visible` or `.has` (a block with comments) raised it. That CSS, from `0b39136`, is the known-bad input the new check must reject.

## Before: full.shale as it stands

```shale
-- before: full.shale as it stands, its time and verdict
local r = run({"shale", "scripts/full.shale"}, {timeout_ms = 115000, tail = 60})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
```

```text
exit 0, 30870 ms wall, cpu 24142 ms
box: load 2.38 on 2 CPUs; notes: /home/steve/showell_repos/essay-repl-server/notes
== fast: PASS, 3704 ms, cpu 4220 ms
   ok  test, 1953 ms, cpu 2064 ms: 44 tests passed, 63 subtests
   ok  smoke, 1396 ms, cpu 1645 ms: 10 checks through the real server
== race: PASS, 3737 ms, cpu 2995 ms
== anchoring: PASS, 12952 ms, cpu 13407 ms
== audit: PASS, 2079 ms, cpu 761 ms
== dates: PASS, 3222 ms, cpu 681 ms
== bench: PASS, 5163 ms, cpu 2061 ms
   latency: not judged, the box was busy (load 3.11 on 2 CPUs before, 3.10 on 2 CPUs after)
SAFE TO SHIP
```

**Before: 30.9 s wall and 24.1 s of CPU, on a box at load 2.4 to 3.1 on two CPUs.** Wednesday's quiet runs were 27.6 and 29.1 s. CPU is the steady measure here, since wall time on this box depends on what else is running.

## The stages, each seen red

`full.shale` has six stages, and its first, `fast`, has five of its own. Each of the eleven gets a plant here. For fast's five I run `shale scripts/fast.shale`, exactly as `full.shale` runs it; for the other five I run the stage's own argv from `full.shale`. Every launch has the same shape: the plant, the run, the reverse edit, then `git diff` checked and `fast.shale` run.

### fast: gofmt

```shale
-- plant 1, fast's gofmt stage: one line of server.go indented with spaces. Run fast as full runs it, take the fault out, prove the tree is back
const OLD = "	start := time.Now()\n	page, err := s.indexPage()\n"
const NEW = "	start := time.Now()\n        page, err := s.indexPage()\n"
edit("server.go", OLD, NEW)
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 12})
edit("server.go", NEW, OLD)
print("planted: exit " .. r.code)
print(r.out)
local d = git_diff("")
if d ~= "" then fail("the tree is not back: " .. d) end
print("git diff: empty")
local f = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 3})
print("after: exit " .. f.code .. "; " .. f.out)
```

The plant's diff:

```diff
 	start := time.Now()
-	page, err := s.indexPage()
+        page, err := s.indexPage()
```

```text
planted: exit 1
server.go
── stopped at scripts/fast.shale:63:3: fail: gofmt: these files need gofmt -w (151 steps)
git diff: empty
after: exit 0; ... fast: PASS
```

### fast: vet

The same launch with these two lines changed (I give only the changed lines from here on; the rest of each script is the shape above, verbatim):

```shale
-- plant 2, fast's vet stage: a Printf verb that does not fit its argument, which compiles and passes every test
const OLD = "log.Printf(\"cpu: %d ms to start\", started.Milliseconds())"
const NEW = "log.Printf(\"cpu: %s ms to start\", started.Milliseconds())"
```

```diff
-	log.Printf("cpu: %d ms to start", started.Milliseconds())
+	log.Printf("cpu: %s ms to start", started.Milliseconds())
```

```text
planted: exit 1
ok  gofmt, 56 ms, cpu 57 ms
server.go:316:19: log.Printf format %s has arg started.Milliseconds() of wrong type int64
── stopped at scripts/fast.shale:50:5 (in `stage`): fail: vet: go exited 1 (836 steps)
git diff: empty
after: exit 0; ... fast: PASS
```

### fast: js

```shale
-- plant 3, fast's js stage: a lost parenthesis in comments.js, which go test embeds but never parses
const OLD = "    btn.addEventListener(\"click\", function () { openForm(el, k, btn); });\n"
const NEW = "    btn.addEventListener(\"click\", function () { openForm(el, k, btn); };\n"
edit("comments.js", OLD, NEW)
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 14})
edit("comments.js", NEW, OLD)
```

```diff
-    btn.addEventListener("click", function () { openForm(el, k, btn); });
+    btn.addEventListener("click", function () { openForm(el, k, btn); };
```

```text
planted: exit 1
ok  gofmt, 35 ms, cpu 43 ms
ok  vet, 359 ms, cpu 494 ms
/home/steve/showell_repos/essay-surface/comments.js:113
    btn.addEventListener("click", function () { openForm(el, k, btn); };
SyntaxError: missing ) after argument list
── stopped at scripts/fast.shale:50:5 (in `stage`): fail: js: node exited 1 (2694 steps)
git diff: empty
after: exit 0; ... fast: PASS
```

### fast: test, and the first hole

For the tests I wanted a fault they had not been written around. Wednesday's plant was the renderer; today's is the comment's name, written to the page without escaping. Tuesday's tests check the text's escaping, and the smoke checks it again, but nothing I could find checked the name.

```shale
-- plant 4, fast's test stage: a comment's name written to the page unescaped (the smoke checks only the text's escaping). Which test sees it?
const OLD = "	b.WriteString(`<div class=\"who\"><b>`)\n	b.WriteString(html.EscapeString(c.Name))\n"
const NEW = "	b.WriteString(`<div class=\"who\"><b>`)\n	b.WriteString(c.Name)\n"
edit("server.go", OLD, NEW)
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 20})
edit("server.go", NEW, OLD)
```

```diff
 	b.WriteString(`<div class="who"><b>`)
-	b.WriteString(html.EscapeString(c.Name))
+	b.WriteString(c.Name)
```

```text
planted: exit 0
ok  gofmt, 33 ms, cpu 45 ms
ok  vet, 309 ms, cpu 487 ms
ok  js, 44 ms, cpu 43 ms
ok  test, 2474 ms, cpu 3151 ms: 44 tests passed, 63 subtests
ok  smoke, 1277 ms, cpu 1603 ms: 10 checks through the real server
fast: PASS
git diff: empty
```

**Green with the fault in.** Anyone who posts a comment named `<script>...` would run it on every reader's page, and every stage of the full check would pass (none of the later stages posts a name with markup either). `cleanComment` refuses an empty name, a long one and one with a newline, nothing else, which is right: the page escapes, the store keeps what was written. So the missing piece was a test. I wrote one that writes a comment with markup in every field the page shows (id, time, name, text, the earlier version's quote) and requires all five escaped:

```go
func TestEveryCommentFieldEscaped(t *testing.T) {
	const m = `<i x="1">&`
	var b bytes.Buffer
	writeComment(&b, &Comment{ID: m, Time: m, Name: m, Text: m, Quote: m}, true)
	if got := b.String(); strings.Contains(got, "<i") || strings.Count(got, "&lt;i x=&#34;1&#34;&gt;&amp;") != 5 {
		t.Errorf("a field is not escaped: %s", got)
	}
}
```

Then the same plant again. This time the tree has to come back to the diff from before the plant (the new test), not to an empty one:

```shale
-- plant 4 again, with TestEveryCommentFieldEscaped written: the name unescaped must now go red. The tree must come back to the diff before the plant (the new test)
const OLD = "	b.WriteString(`<div class=\"who\"><b>`)\n	b.WriteString(html.EscapeString(c.Name))\n"
const NEW = "	b.WriteString(`<div class=\"who\"><b>`)\n	b.WriteString(c.Name)\n"
local before = git_diff("")
local g = run({"go", "test", "-count=1", "-run", "TestEveryCommentFieldEscaped", "."}, {timeout_ms = 60000, tail = 5})
print("the new test, unplanted: exit " .. g.code .. "; " .. g.out)
edit("server.go", OLD, NEW)
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 20})
edit("server.go", NEW, OLD)
print("planted: exit " .. r.code)
print(r.out)
if git_diff("") ~= before then fail("the tree is not back") end
print("git diff: as before the plant (the new test only)")
local f = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 3})
print("after: exit " .. f.code .. "; " .. f.out)
```

```text
the new test, unplanted: exit 0; ok  	essay-surface	0.004s
planted: exit 1
    comments_test.go:107: a field is not escaped: <div class="cmt" id="c-&lt;i x=&#34;1&#34;&gt;&amp;"><details class="was">...
--- FAIL: TestEveryCommentFieldEscaped (0.00s)
── stopped at scripts/fast.shale:53:3 (in `stage`): fail: test: 1 failed: TestEveryCommentFieldEscaped (81888 steps)
git diff: as before the plant (the new test only)
after: exit 0; ... fast: PASS
```

Green before the plant, red with it, green after: the test can fail, and fails for this. Committed as `45a2c32`:

```shale
-- commit the escaping test the plant found missing
print(commit("", "A test that every comment field reaching the page is escaped\n\n...\n\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>\n", {add = {"comments_test.go"}}))
```

### fast: smoke

What only the smoke sees is the real binary: `main`, its flags, the server as it runs. The Go tests make their stores directly. So the plant is in `main`: `-comments` ignored, the comments kept inside the notes folder.

```shale
-- plant 5, fast's smoke stage: main ignores -comments and keeps comments inside the notes folder. The Go tests make their stores directly and never run main; only the smoke runs the real binary
const OLD = "	s, err := newStore(*dir, *comments)\n"
const NEW = "	s, err := newStore(*dir, filepath.Join(*dir, \"comments\"))\n"
edit("server.go", OLD, NEW)
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 12})
edit("server.go", NEW, OLD)
```

```diff
-	s, err := newStore(*dir, *comments)
+	s, err := newStore(*dir, filepath.Join(*dir, "comments"))
```

```text
planted: exit 1
ok  test, 2341 ms, cpu 2979 ms: 45 tests passed, 63 subtests
server stopped
── stopped at scripts/comments_smoke.shale:60:23: fail: the smoke test stopped: `read(".run/smoke/comments/smoke.md.jsonl")`: not found (FileNotFound): `exists(p)` asks first (line 36)
── stopped at scripts/fast.shale:50:5 (in `stage`): fail: smoke: shale exited 1 (57460 steps)
git diff: empty
after: exit 0; ... fast: PASS
```

Red, and the tests passed with the fault in, as they should: this was the smoke's fault to catch. But the red said a file was missing, not which check failed. The smoke now reads a missing comments file as empty, so the check itself fails by name:

```shale
-- the smoke's red named a missing file, not the check that failed: a comments file not there reads as empty, so the check itself says so
edit("scripts/comments_smoke.shale", {
  {old = "function want(cond, what, out)", new = "-- The comments file's text, or \"\" before there is one.\nfunction comments_file()\n  if not exists(FILE) then return \"\" end\n  return read(FILE)\nend\n\nfunction want(cond, what, out)"},
  {old = "want(find(read(FILE), \"\\\"text\\\"", new = "want(find(comments_file(), \"\\\"text\\\""},
  {old = "want(find(read(FILE), \"\\\"anchor\\\"", new = "want(find(comments_file(), \"\\\"anchor\\\""},
  {old = "want(#lines(read(FILE)) == 1", new = "want(#lines(comments_file()) == 1"},
})
const OLD = "	s, err := newStore(*dir, *comments)\n"
const NEW = "	s, err := newStore(*dir, filepath.Join(*dir, \"comments\"))\n"
local before = git_diff("")
edit("server.go", OLD, NEW)
local r = run({"shale", "scripts/comments_smoke.shale"}, {timeout_ms = 60000, tail = 4})
edit("server.go", NEW, OLD)
print("planted: exit " .. r.code)
print(r.out)
if git_diff("") ~= before then fail("the tree is not back") end
print("git diff: as before the plant (the smoke's edit only)")
local f = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 3})
print("after: exit " .. f.code .. "; " .. f.out)
```

```text
edit scripts/comments_smoke.shale: 4 hunks, -4 +10 lines
planted: exit 1
── stopped at scripts/comments_smoke.shale:66:23: fail: the smoke test stopped: not so: the file keeps the text as written (line 27) (47627 steps)
git diff: as before the plant (the smoke's edit only)
after: exit 0; ... fast: PASS
```

Committed as `8dd7d03`.

### race

The fault the race detector exists for: a read of the notes map without its lock, while `makePage` and `apply` write it. My first launch had a bad option, and **it stopped after the plant went in and before the reverse edit**:

```shale
-- plant 6, the race stage: lookup reads the notes map without its read lock, while makePage and apply write it. Run full's race argv, and fast too, to see whether the plain tests notice
const OLD = "func (s *store) lookup(name string) *note {\n	s.mu.RLock()\n	defer s.mu.RUnlock()\n"
const NEW = "func (s *store) lookup(name string) *note {\n"
edit("store.go", OLD, NEW)
local r = run({"go", "test", "-race", "-count=1", "-skip", "TestSimulateAnchoring", "."}, {timeout_ms = 110000, tail = 400, stage = "race"})
...
```

```text
edit store.go: 1 hunk, -3 +1 lines
── stopped at the Linux prelude:11:10 (in `the Linux prelude's run`): argument: `run`'s `stage` names the program in a `cpu_budget_ms` stop, and needs one (13317 steps)
```

So the planted fault was left in the tree. I took it out before anything else:

```shale
-- the last launch stopped after planting: take the plant out before anything else
git_restore("", {"store.go"})
print("git diff: [" .. git_diff("") .. "]")
```

```text
git diff: []
```

**From here on every plant runs under `shale --undo-on-stop`**, so a stop anywhere puts the planted file back. Plants 1 to 5 were lucky. The second try then stopped on my own print (`re_lines` gives `{line, text}`, not strings) and the undo did its job:

```text
planted, race: exit 1, 2774 ms, cpu 3371
── stopped at stdin:9:7: type: `join` takes strings and integers, and item 1 is a map (1261121 steps)
── undone: 1 path(s) the run changed put back
```

The third, verbatim:

```shale
-- plant 6, a third launch: re_lines gives {line, text}, so the lines are printed one by one
const OLD = "func (s *store) lookup(name string) *note {\n	s.mu.RLock()\n	defer s.mu.RUnlock()\n"
const NEW = "func (s *store) lookup(name string) *note {\n"
edit("store.go", OLD, NEW)
local r = run({"go", "test", "-race", "-count=1", "-skip", "TestSimulateAnchoring", "."}, {timeout_ms = 110000, tail = 400, stage = "race", cpu_budget_ms = 40000})
local plain = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 3})
edit("store.go", NEW, OLD)
print("planted, race: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
for l in take(re_lines(r.out, "WARNING: DATA RACE|^--- FAIL|race detected|^FAIL|^ok|store.go:"), 30) do print(l.text) end
print("planted, fast: exit " .. plain.code .. "; " .. plain.out)
local d = git_diff("")
if d ~= "" then fail("the tree is not back: " .. d) end
print("git diff: empty")
local f = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 3})
print("after: exit " .. f.code .. "; " .. f.out)
```

```diff
 func (s *store) lookup(name string) *note {
-	s.mu.RLock()
-	defer s.mu.RUnlock()
 	return s.notes[name]
```

```text
planted, race: exit 1, 1903 ms, cpu 1840
WARNING: DATA RACE
      /home/steve/showell_repos/essay-surface/store.go:314 +0x12a
      /home/steve/showell_repos/essay-surface/store.go:345 +0x143
      ...
planted, fast: exit 0; ... fast: PASS
git diff: empty
after: exit 0; ... fast: PASS
```

The race stage is the only one that sees this. `fast` passed with the fault in.

### anchoring

The anchoring's one tunable: the similarity an edited, moved block needs to keep its comments. Halved, comments on deleted blocks go to look-alikes.

```shale
-- plant 7, the anchoring stage: the similarity a moved, edited block needs halved (600 to 300 in a thousand), so a deleted block's comments go to look-alikes. full's anchoring argv on the real notes; then fast, whose samples are pinned
const OLD = "const similarEnough = 600\n"
const NEW = "const similarEnough = 300\n"
edit("anchor.go", OLD, NEW)
local r = run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-v", "."}, {timeout_ms = 110000, tail = 400, stage = "anchoring", cpu_budget_ms = 45000, env = {ESSAY_NOTES = "/home/steve/showell_repos/essay-repl-server/notes"}, re = "cases|--- |^ok|^FAIL|_test.go:2[01]"})
local plain = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 8})
edit("anchor.go", NEW, OLD)
print("planted, anchoring: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
print("planted, fast: exit " .. plain.code .. "; " .. plain.out)
local d = git_diff("")
if d ~= "" then fail("the tree is not back: " .. d) end
print("git diff: empty")
local f = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 3})
print("after: exit " .. f.code .. "; " .. f.out)
```

```diff
-const similarEnough = 600
+const similarEnough = 300
```

```text
planted, anchoring: exit 1, 18385 ms, cpu 14752
    calibrate_test.go:204: deleted      cases 20895: right 18580, on another block 2315, orphaned    0
    calibrate_test.go:217: merged: 806 of 20402 comments on another block, past 20 in a thousand
    calibrate_test.go:217: moved, typo: 238 of 20859 comments on another block, past 10 in a thousand
    calibrate_test.go:217: deleted: 2315 of 20895 comments on another block, past 25 in a thousand
--- FAIL: TestSimulateAnchoring (16.56s)
planted, fast: exit 1
    calibrate_test.go:235: deleted: 528 cases, 165 on another block, 0 orphaned; want 8 and 0
── stopped at scripts/fast.shale:53:3 (in `stage`): fail: test: 2 failed: TestAnchorDeletedNextToLookalike, TestSimulateAnchoring
git diff: empty
after: exit 0; ... fast: PASS
```

Red, with the three rates named. `fast` caught this one too, since the samples' counts are pinned. That's as intended: the pins catch any change to the anchoring, and the real-notes stage says whether a change is acceptable on today's notes.

(While this ran, in the background, I made one useless launch, a `read` of an absolute path to check on it, which Shale refused at once: `"/tmp/claude-1000/x" is absolute`. Waiting is the notification's job.)

### audit

A renderer fault that shows only on notes that use it: a delimiter row with alignment colons (`|:---|`) no longer starts a table.

```shale
-- plant 8, the audit stage: a table whose delimiter row aligns a column (:---, ---:) is no longer a table, and renders as a paragraph of pipes. full's audit argv on the real notes; then fast
const OLD = "		core := strings.Trim(c, \":\")\n"
const NEW = "		core := c\n"
edit("markdown.go", OLD, NEW)
local r = run({"shale", "scripts/audit.shale", "dir=/home/steve/showell_repos/essay-repl-server/notes"}, {timeout_ms = 110000, tail = 14, stage = "audit", cpu_budget_ms = 20000})
local plain = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 8})
edit("markdown.go", NEW, OLD)
```

```diff
-		core := strings.Trim(c, ":")
+		core := c
```

```text
planted, audit: exit 1, 3141 ms, cpu 2306
507 pages, 24042 commentable blocks, 7 pages with leftovers
    0      0       4      0  roc-chat-host-design.md
    0      0       3      0  pure-roc-chat-plan.md
    0      0      10      0  roc-chat-status.md
    0      0       4      0  what-we-own-outright.md
── stopped at scripts/audit.shale:89:3: fail: 4 pages with leftovers not known to be faithful: roc-chat-host-design.md (0 ticks), ...
planted, fast: exit 1
    markdown_test.go:100: Render("| a | b | c |\n|:--|:-:|--:|\n| 1 | `x|y` | **z** |")
── stopped at scripts/fast.shale:53:3 (in `stage`): fail: test: 1 failed: TestRender
git diff: empty
after: exit 0; ... fast: PASS
```

The audit names the four real notes that broke. `TestRender` has an aligned table too.

### dates

Wednesday's dates plant was git never asked. Today's is subtler, and something someone might do on purpose: an undated note takes the later of its last commit and its file's time, "so it shows its latest edit".

```shale
-- plant 9, the dates stage: an undated note takes the later of its last commit and its file's time (a plausible "show the latest edit"), so a fresh clone, every file new, re-dates it. full's dates argv; then fast
const OLD = "			n.date, n.from = t, dateFromGit\n"
const NEW = "			if n.modified.After(t) {\n				t = n.modified\n			}\n			n.date, n.from = t, dateFromGit\n"
edit("store.go", OLD, NEW)
local r = run({"shale", "scripts/dates.shale", "dir=/home/steve/showell_repos/essay-repl-server/notes"}, {timeout_ms = 110000, tail = 8, stage = "dates", cpu_budget_ms = 20000})
local plain = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 8})
edit("store.go", NEW, OLD)
```

```diff
 		if t := s.gitWhen[n.name]; n.from != dateFromNote && !t.IsZero() {
+			if n.modified.After(t) {
+				t = n.modified
+			}
 			n.date, n.from = t, dateFromGit
```

```text
planted, dates: exit 1, 4652 ms, cpu 2023
folder: dates: 309 from the notes, 198 from git, 0 from their files (git in 82ms)
clone:  dates: 309 from the notes, 198 from git, 0 from their files (git in 91ms)
── stopped at scripts/dates.shale:58:42: fail: roc-interludes.md: 2026-11-12 in the folder, 2026-11-13 in the clone (7291931 steps)
planted, fast: exit 1
    dates_test.go:93: committed.md: want 2026-01-02 in "...<span class=\"date\">2030-01-01</span>...
--- FAIL: TestDatesFromNoteGitAndFile (0.05s)
git diff: empty
after: exit 0; ... fast: PASS
```

The count line still says "0 from their files", since the source is still git; only the comparison of the two indexes caught it, naming a real note. (Git took 82 ms today against 821 ms this morning: the notes repo looks packed now, which Wednesday's report suggested.)

### bench

The fault only the bench can see: every page still right, but each note request reads and renders its note again.

```shale
-- plant 10, the bench stage: every note request reads and renders its note again, the cache kept but unused. Every page is still right, so only CPU can see it. full's bench argv; then fast
const OLD = "	n, err := s.page(name)\n	if err != nil {\n		// A comments file"
const NEW = "	s.load(name)\n	n, err := s.page(name)\n	if err != nil {\n		// A comments file"
edit("server.go", OLD, NEW)
local r = run({"shale", "scripts/bench.shale", "dir=/home/steve/showell_repos/essay-repl-server/notes", "rounds=500"}, {timeout_ms = 110000, tail = 10, stage = "bench", cpu_budget_ms = 40000})
local plain = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 2})
edit("server.go", NEW, OLD)
```

```diff
+	s.load(name)
 	n, err := s.page(name)
```

```text
planted, bench: exit 1, 6177 ms, cpu 1904
── stopped at scripts/bench.shale:193:26 (in `phase_end`): fail: cpu, notes: 1776 us a request over 1515 requests (2692 ms), budget 400 (1848012 steps)
planted, fast: exit 0; ... fast: PASS
git diff: empty
after: exit 0; ... fast: PASS
```

1,776 µs of CPU a request against a budget of 400, so any box catches it; `fast` passed with it in.

## Two additions from the human, mid-run

Halfway through, two notes came in, replying to Wednesday's report:

1. The comments folder's parent must exist at start, and should be a defensive check: if it's missing, the server stops with one clear message naming the path, and never makes the folder.
2. Phones are out of scope. Say so in the README in one plain sentence, and take the phone eye test off the by-hand list. The desktop browser eye test stays.

### The parent, checked

`newStore` already failed on a missing parent, but by accident: inotify's `add` refused it, and the message said `watching ...: no such file or directory`. Now it checks first, with my editor (commit `bb9f694`):

```go
	// The parent is checked, never made: a mistyped -comments would
	// otherwise grow a folder of comments somewhere nobody looks.
	parent := filepath.Dir(comments)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("the comments folder's parent %s is not a folder; make it, or pass another -comments", parent)
	}
```

`TestCommentsParentMustExist` covers every side: a parent that is missing, a parent that is a file, both refused with the message and nothing made, and a parent that is there with its comments folder not yet made, which starts. Then the test, unplanted, and its plant once:

```shale
-- the human's first addition: the store refuses a comments folder whose parent is missing, naming it. The new test unplanted, then its plant (the check taken out), then the tree back to the diff before the plant
const OLD = "	parent := filepath.Dir(comments)\n	if info, err := os.Stat(parent); err != nil || !info.IsDir() {\n		return nil, fmt.Errorf(\"the comments folder's parent %s is not a folder; make it, or pass another -comments\", parent)\n	}\n"
const NEW = ""
local g = run({"go", "test", "-count=1", "-run", "TestCommentsParentMustExist", "-v", "."}, {timeout_ms = 60000, tail = 8})
print("unplanted: exit " .. g.code .. "\n" .. g.out)
local before = git_diff("")
edit("store.go", OLD, NEW)
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 12})
edit("store.go", "	// otherwise grow a folder of comments somewhere nobody looks.\n", "	// otherwise grow a folder of comments somewhere nobody looks.\n" .. OLD)
print("planted: exit " .. r.code)
print(r.out)
if git_diff("") ~= before then fail("the tree is not back") end
print("git diff: as before the plant")
local f = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 3})
print("after: exit " .. f.code .. "; " .. f.out)
```

```text
unplanted: exit 0
--- PASS: TestCommentsParentMustExist (0.01s)
planted: exit 1
    store_test.go:25: /tmp/TestCommentsParentMustExist2865992362/001/missing: said "the comments folder's parent: watching /tmp/.../missing: no such file or direc…
    store_test.go:25: /tmp/TestCommentsParentMustExist2865992362/001/a.md: said "the comments folder's parent: watching /tmp/.../a.md: not a directory", want it t…
--- FAIL: TestCommentsParentMustExist (0.01s)
── stopped at scripts/fast.shale:53:3 (in `stage`): fail: test: 1 failed: TestCommentsParentMustExist
git diff: as before the plant
after: exit 0; ... fast: PASS
```

And the real binary, once, so the message is seen where it will be read:

```shale
-- what the real server says when the comments folder's parent is missing (the binary fast just built; it must stop, not listen)
local r = run({"./essay-surface", "-notes", "notes", "-comments", ".run/no-such-folder/comments"}, {timeout_ms = 10000, tail = 5, want = "build: the server we just built"})
print("exit " .. r.code .. ": " .. r.out)
print("made: " .. json_write(exists(".run/no-such-folder")))
```

```text
exit 1: 2026/11/13 17:28:20 the comments folder's parent .run/no-such-folder is not a folder; make it, or pass another -comments
made: false
```

### Phones

```shale
-- the human's second addition: phones are out of scope, said in the README; the by-hand list keeps the desktop browser alone
edit("README.md", "...a JavaScript front end.\n", "...a JavaScript front end.\n\nPhones are out of scope: the pages are made for, and checked in, a desktop browser.\n")
edit("scripts/full.shale", "print(\"by hand still: an eye test of a note and its comments on a phone and a desktop browser\")", "print(\"by hand still: an eye test of a note and its comments in a desktop browser\")")
for p in {"README.md", "scripts/full.shale", "scripts/fast.shale", "comments.css", "comments.js"} do
  for m in grep(p, "phone") do print(p .. ":" .. m.line .. ": " .. m.text) end
end
```

```text
edit README.md: 1 hunk, -1 +3 lines
edit scripts/full.shale: 1 hunk, -1 +1 lines
```

(The `edit` calls held the README's whole first paragraph; I've shortened them above.) No other file says "phone". `comments.css` keeps its narrow-width and `hover:none` rules, which a narrow desktop window also uses; out of scope means unchecked, not removed. The button check below judges desktop states only for the same reason. Both committed with the test as `bb9f694`, after fast passed.

## The reply buttons, checked without a browser

Tuesday's failure was CSS: `.cbtn{...opacity:0...}` on the resting button, raised only by `:hover`, `:focus-visible` or `.has`. Every check passed because none of them looks at what the CSS does. Without a browser, the honest version is to work the cascade out by hand for the one element that matters, on the CSS the page is actually served with. That's `buttons_test.go` (written whole with my file writer; commit `bb14884`):

- **The input** is a real note page from the store's handler, with a paragraph, a heading, a list item and a code block, and every `<style>` in it, in order: the page's own style and `comments.css` as embedded.
- **The places** are where `comments.js` puts a button: inside a `p.chost` or `h2.chost`, inside the `span.ct` of an `li.chost`, and in a `div.cpin` after a code block. Each is the chain of elements from `html` down to `button.cbtn[type=button]`, with and without `.has` (a block with comments). The test also requires those class names to still be in `comments.js` and the renderer. Otherwise a renamed class would make it pass by matching nothing.
- **The states** are every desktop state the CSS tells apart: one width on each side of every width a media query names (30, 31, 50, 51 and 80rem), light and dark. A desktop has hover, so `@media (hover:none)` never holds.
- **The cascade** is selectors (tags, classes, attributes, `:root`, the child and descendant combinators), specificity, then source order. A resting button is not hovered, focused or active, so a rule with any of those never applies.
- **Hidden** means `display:none` on the button or any box around it, `visibility:hidden` inherited or its own, or the opacities from the button out to `html` multiplying to less than **0.5**. Today's CSS has .6 on a wide screen and .5 on a narrow one, and Tuesday's had 0.
- **What it refuses rather than guesses at:** any at-rule other than `@media`, a nested `@media`, `!important`, a pseudo-class beyond those four, an unknown media feature. A stop, not a skip.
- **What it does not see**, said in its header: a script that stops making the buttons (fast parses the script, but nothing runs it), a button clipped or pushed off the page, and colours too faint to read.

### The fixtures: known-bad and known-good sheets

`testdata/buttons/` holds seven small sheets. Each is read after the page's own style, and the test names every file there, so none can be skipped:

| fixture | kind | what it must say |
|---|---|---|
| `tuesday.css` | known-bad, the real one from `0b39136` | opacity 0 on a wide screen |
| `narrow-none.css` | known-bad | `display:none` on the pinned button at 50rem and under |
| `host-hidden.css` | known-bad | `visibility:hidden` inherited from the `li` |
| `faint-article.css` | known-bad | .8 on `article` times .5 on the button is 0.4, under the floor though each passes alone |
| `important.css` | known-bad | refused: `!important` can't be judged |
| `specific-shows.css` | known-good | `.cbtn{opacity:0}` overridden by a more specific rule |
| `hover-only-hides.css` | known-good | only a hovered host's button is hidden |

```shale
-- the known-bad and known-good sheets for the reply-button check; Tuesday's is the real one, from git
mkdir("testdata/buttons")
create("testdata/buttons/tuesday.css", git_show("", "0b39136:comments.css"))
create("testdata/buttons/narrow-none.css", "/* A narrow window drops the pinned buttons. */\n@media (max-width:50rem){.cpin>.cbtn{display:none}}\n")
create("testdata/buttons/host-hidden.css", "/* A list item hides what it holds; the button inherits it. */\nli.chost{visibility:hidden}\n")
create("testdata/buttons/faint-article.css", "/* Each opacity passes alone; together they are 0.4. */\narticle{opacity:.8}\n.cbtn{opacity:.5}\n")
create("testdata/buttons/important.css", "/* The check cannot weigh !important, so it refuses the sheet. */\n.cbtn{opacity:1 !important}\n")
create("testdata/buttons/specific-shows.css", "/* Known good: a more specific rule shows what .cbtn hid. */\n.cbtn{opacity:0}\n.chost>.cbtn,.ct>.cbtn,.cpin>.cbtn{opacity:.8}\n")
create("testdata/buttons/hover-only-hides.css", "/* Known good: only a hovered host's button is hidden, never a resting one. */\n.chost:hover>.cbtn{opacity:0}\n")
must(run({"gofmt", "-w", "buttons_test.go"}, {timeout_ms = 10000, tail = 5}))
local r = run({"go", "test", "-count=1", "-run", "TestReplyButton", "-v", "."}, {timeout_ms = 60000, tail = 40})
print("exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
```

```text
--- PASS: TestReplyButtonsShow (0.01s)
--- PASS: TestReplyButtonFixtures (0.02s)
ok  	essay-surface	0.038s
```

Green on its first run, which proves nothing yet. So I made the test log what each fixture said, then planted the fault the check exists for:

```shale
-- what each fixture said; then the plant the check exists for: Tuesday's opacity:0 back on the resting button in comments.css, through fast
local v = run({"go", "test", "-count=1", "-run", "TestReplyButtonFixtures", "-v", "."}, {timeout_ms = 60000, tail = 20, re = "buttons_test.go"})
print(v.out)
const OLD = "cursor:pointer;opacity:.6;transition:opacity .15s}"
const NEW = "cursor:pointer;opacity:0;transition:opacity .15s}"
local before = git_diff("")
edit("comments.css", OLD, NEW)
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 16})
edit("comments.css", NEW, OLD)
print("planted: exit " .. r.code)
print(r.out)
if git_diff("") ~= before then fail("the tree is not back") end
print("git diff: as before the plant")
local f = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 4})
print("after: exit " .. f.code .. "; " .. f.out)
```

```diff
-.cbtn{...;cursor:pointer;opacity:.6;transition:opacity .15s}
+.cbtn{...;cursor:pointer;opacity:0;transition:opacity .15s}
```

```text
    buttons_test.go:523: faint-article.css: 48 hidden; a code block's reply button with no comments, 30rem wide, light: opacity 0.4, under 0.5
    buttons_test.go:523: host-hidden.css: 12 hidden; a list item's reply button with no comments, 30rem wide, light: visibility:hidden
    buttons_test.go:523: hover-only-hides.css: 0 hidden;
    buttons_test.go:523: important.css: 0 hidden; cannot judge !important in ".cbtn"
    buttons_test.go:523: narrow-none.css: 12 hidden; a code block's reply button with no comments, 30rem wide, light: display:none on its button
    buttons_test.go:523: specific-shows.css: 0 hidden;
    buttons_test.go:523: tuesday.css: 16 hidden; a code block's reply button with no comments, 51rem wide, light: opacity 0, under 0.5
planted: exit 1
    buttons_test.go:482: a paragraph's reply button with no comments, 51rem wide, dark: opacity 0, under 0.5
    ...
    buttons_test.go:482: a paragraph's reply button with no comments, 80rem wide, light: opacity 0, under 0.5
    ...
--- FAIL: TestReplyButtonsShow (0.01s)
── stopped at scripts/fast.shale:53:3 (in `stage`): fail: test: 1 failed: TestReplyButtonsShow
git diff: as before the plant
after: exit 0; ... ok  test, 1837 ms, cpu 1853 ms: 48 tests passed, 63 subtests ... fast: PASS
```

The detail I like: Tuesday's sheet is hidden at 51rem and wider, and not at 50rem and under, because its narrow rule set the button to .5. So it would have shown in a narrow window and not in a wide one.

A checker needs a plant of its own, and the fixtures are what should catch it. One fault in the cascade, with opacities no longer multiplying out to the page:

```shale
-- a plant in the checker itself: opacities no longer multiply out to the page (the button's own wins). The fixtures must catch it
const OLD = "						opacity *= f\n"
const NEW = "						opacity = f\n"
local before = git_diff("")
edit("buttons_test.go", OLD, NEW)
local r = run({"go", "test", "-count=1", "-run", "TestReplyButton", "."}, {timeout_ms = 60000, tail = 10, re = "FAIL|want|ok"})
edit("buttons_test.go", NEW, OLD)
print("planted: exit " .. r.code)
print(r.out)
if git_diff("") ~= before then fail("the tree is not back") end
print("git diff: as before the plant")
```

```text
planted: exit 1
--- FAIL: TestReplyButtonFixtures (0.02s)
    buttons_test.go:528: faint-article.css: want "a heading's reply button with no comments, 80rem wide, light: opacity 0.4" among:
git diff: as before the plant
```

Committed as `bb14884`, after fast passed.

## Fixtures for the checks themselves

Two of the checks are Shale scripts that judge output, and both could go blind with nothing turning red: an audit whose counts always come out zero would pass every real note. Each now reads a small known-bad input (and the audit a known-good one too) before its real work, and stops if it misjudges it.

### The audit

```shale
-- the audit's known-bad and known-good pages, and the audit reading them first, before any real page
mkdir("testdata/audit")
create("testdata/audit/bad.html", [==[...]==])   -- **strong**, a [link](...), a lone `, a |---| row; one data-c for two fingerprints
create("testdata/audit/good.html", [==[...]==])  -- the same markup only in <code>, <pre>, <style> and <script>; counts that agree
edit("scripts/audit.shale", [==[essay_build()
essay_fresh(RUN .. "/pages")]==], [==[-- A known-bad page and a known-good one, read as every real page is: an
-- audit that passes the bad one is blind, whatever the real pages say.
local bad = audit("testdata/audit/bad.html")
...]==])
local r = run({"shale", "scripts/audit.shale"}, {timeout_ms = 60000, tail = 6})
print("audit on the samples: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
```

(The pages and the edit are in `e2ab440`; I've cut them here.)

```text
── stopped at scripts/audit.shale:79:1: name: `bad` is already a local in this block (0 steps)
```

The script already has a `bad`, its list of mismatched pages. The fixtures' locals became `known_bad` and `known_good`, by an `edit` of the two blocks, and then:

```text
audit on the samples: exit 0, 1091 ms, cpu 1326
fixtures: bad.html caught, good.html passed
12 pages, 1785 commentable blocks, 0 pages with leftovers
audit: PASS
```

The plant, once, makes the audit blind to the `data-c` count:

```shale
-- plant once in the audit itself: the data-c count is never compared (mismatch always 0). The known-bad page must stop it, before any server starts; its stop's step count is the fixtures' cost
const OLD = "blocks = #blocks, mismatch = marked - #blocks}"
const NEW = "blocks = #blocks, mismatch = 0}"
local before = git_diff("")
edit("scripts/audit.shale", OLD, NEW)
local r = run({"shale", "scripts/audit.shale"}, {timeout_ms = 60000, tail = 4})
edit("scripts/audit.shale", NEW, OLD)
print("planted: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
if git_diff("") ~= before then fail("the tree is not back") end
print("git diff: as before the plant")
```

```text
planted: exit 1, 5 ms, cpu 4
── stopped at scripts/audit.shale:49:3: fail: the audit passes testdata/audit/bad.html: {"stars":2,"links":1,"tables":2,"ticks":1,"blocks":2,"mismatch":0} (2006 steps)
git diff: as before the plant
```

It stopped in 5 ms, before any build or server, at 2,006 steps for the bad page. Committed as `e2ab440`.

### fast's reader of go test

Wednesday found a panic that hid twelve tests. Fast's `failed_tests` is what turns `go test -v` into a red that names the failures, so it gets a known-bad output: a pass with logs, a failure with logs, and a panic.

```shale
-- fast's own reader of go test -v gets a known-bad output: a pass with logs, a failure with logs, a panic. It must name the failure and the panic, and show only their lines
mkdir("testdata/fast")
create("testdata/fast/go-test-v.txt", [==[=== RUN   TestPasses
    a_test.go:10: a passing test's log, never shown
--- PASS: TestPasses (0.00s)
=== RUN   TestFails
    b_test.go:20: what the failing test said
--- FAIL: TestFails (0.00s)
=== RUN   TestPanics
panic: runtime error: slice bounds out of range [:-1] [recovered]
	/home/x/c_test.go:30 +0x885
FAIL	essay-surface	0.042s
]==])
edit("scripts/fast.shale", [==[function said(name, r, what)
  ...
end
]==], [==[function said(name, r, what)
  ...
end

-- A known-bad go test -v output, read first: a reader that loses a failure
-- or a panic would let a red test stage say too little.
local known = failed_tests(read("testdata/fast/go-test-v.txt"))
if join(known.names, ",") ~= "TestFails,TestPanics (panic)" or find(join(known.lines, "\n"), "what the failing test said") == nil
    or find(join(known.lines, "\n"), "never shown") ~= nil or find(join(known.lines, "\n"), "c_test.go:30") == nil then
  fail("fast's reader of go test misreads testdata/fast/go-test-v.txt: " .. json_write(known))
end
]==])
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 8})
print("fast: exit " .. r.code)
print(r.out)
```

```text
fast: exit 0
ok  test, 1635 ms, cpu 1820 ms: 48 tests passed, 63 subtests
fast: PASS
```

Its plant: a panic line no longer recognized.

```shale
-- plant once in fast's reader: a panic line no longer recognized, Wednesday's hidden-tests trap. The known-bad output must stop fast before any program runs; the stop's steps are the fixture's cost
const OLD = "    elseif starts_with(l, \"panic:\") then\n"
const NEW = "    elseif starts_with(l, \"panic!\") then\n"
local before = git_diff("")
edit("scripts/fast.shale", OLD, NEW)
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 60000, tail = 4})
edit("scripts/fast.shale", NEW, OLD)
print("planted: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
if git_diff("") ~= before then fail("the tree is not back") end
print("git diff: as before the plant")
```

```text
planted: exit 1, 3 ms, cpu 3
── stopped at scripts/fast.shale:65:3: fail: fast's reader of go test misreads testdata/fast/go-test-v.txt: {"names":["TestFails"],...
── used: 3184 of 5000000 steps, 1 of 10000 reads, 337 of 67108864 read bytes, 0 of 65536 output bytes
git diff: as before the plant
```

3 ms and 3,184 steps. Committed as `8021e4e`.

### The fixtures I didn't keep, and why

- **gofmt, vet, js:** a known-bad Go file in the tree would turn `gofmt -l .` and `go vet .` red forever unless excluded, and an exclusion is a blanket. A known-bad `.js` costs a node launch (about 40 ms) and proves only that node parses. Today's plants proved those three stages once, and that's enough.
- **race:** a known racy program needs a `-race` build of its own, the cost the human named.
- **anchoring:** the samples' pinned counts already are its known-good fixture, and they caught plant 7 too.
- **dates:** a fixture needs a git repository; `TestDatesFromNoteGitAndFile` already builds one, in 0.05 s.
- **bench:** a slow fixture means a server of its own. Plant 10 proved the budget once.
- **smoke:** it is already a fixture, a known-good note of its own through the real server.

## After: full.shale

```shale
-- after: full.shale with today's tests and fixtures, its time and verdict
local r = run({"shale", "scripts/full.shale"}, {timeout_ms = 115000, tail = 60, not_re = "^       calibrate|^             "})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
```

```text
exit 0, 23317 ms wall, cpu 20664 ms
box: load 0.76 on 2 CPUs; notes: /home/steve/showell_repos/essay-repl-server/notes
== fast: PASS, 2964 ms, cpu 3581 ms
   ok  test, 1602 ms, cpu 1809 ms: 48 tests passed, 63 subtests
   ok  smoke, 1109 ms, cpu 1375 ms: 10 checks through the real server
== race: PASS, 3543 ms, cpu 3016 ms
== anchoring: PASS, 10377 ms, cpu 11474 ms
== audit: PASS, 1217 ms, cpu 583 ms
== dates: PASS, 1651 ms, cpu 385 ms
   folder: dates: 309 from the notes, 198 from git, 0 from their files (git in 65ms)
== bench: PASS, 3554 ms, cpu 1611 ms
   latency: every median within budget (load 0.97 on 2 CPUs before, 0.97 on 2 CPUs after): PASS
by hand still: an eye test of a note and its comments in a desktop browser
SAFE TO SHIP
```

```shale
-- is anything of mine still up? every essay-surface process; the human's on 9301 is left alone
for p in procs() do
  if p.name == "essay-surface" then print("pid " .. p.pid .. ": " .. sub(p.args, 1, 120)) end
end
print(json_write(git_status("")))
```

```text
pid 2484343: /home/steve/essay-dogfood/bin/essay-surface -notes /home/steve/essay-dogfood/notes -addr 127.0.0.1:9301
[{"code":"??","path":"journal/prompts/friday.md"},{"code":"??","path":"journal/thursday.md"}]
```

Only your dogfood server is running, and I didn't touch it. `journal/prompts/friday.md` appeared during the day; it's yours, so I left it uncommitted.

## Report

**Every stage was seen red.** One fault at a time, put in and taken out in the same launch, with `git diff` back as it was and `fast` PASS before the next. None was committed.

| stage | the fault planted | what the check said | caught elsewhere too? | fixture kept, and its cost |
|---|---|---|---|---|
| fast: gofmt | a line indented with spaces | `server.go` / `gofmt: these files need gofmt -w` | no | none |
| fast: vet | `%s` for an int64 in `log.Printf` | `format %s has arg started.Milliseconds() of wrong type int64` | no | none |
| fast: js | a lost `)` in comments.js | `SyntaxError: missing ) after argument list` at comments.js:113 | no | none |
| fast: test | a commenter's **name** written unescaped | **first green: nothing caught it.** After the new `TestEveryCommentFieldEscaped`: `1 failed: TestEveryCommentFieldEscaped` | no | the test itself, a known-bad comment (`<i x="1">&` in all five fields): under 1 ms |
| fast: smoke | `main` ignores `-comments` | `not so: the file keeps the text as written` (before a fix today, a bare file-not-found) | no, the tests never run `main` | none (the smoke is a known-good note already) |
| race | `lookup` reads the notes map without its lock | `WARNING: DATA RACE` at store.go:314 | no: fast passed | none |
| anchoring | `similarEnough` 600 down to 300 | merged 806, moved 238, deleted 2,315 on another block, each past its limit | yes, the pinned samples in fast | none new |
| audit | aligned tables (`:---`) no longer tables | 4 real notes named: roc-chat-host-design.md and 3 more | yes, TestRender | `testdata/audit/bad.html` and `good.html`, read first: about 2,000 steps and under 5 ms each, no build |
| dates | the later of commit and file time | `roc-interludes.md: 2026-11-12 in the folder, 2026-11-13 in the clone` | yes, TestDatesFromNoteGitAndFile | none |
| bench | every note request renders its note again | `cpu, notes: 1776 us a request ..., budget 400` | no: fast passed | none |
| new: comments' parent | the check taken out of `newStore` | `TestCommentsParentMustExist` failed for the missing parent and for the file | no | none |
| new: reply buttons | Tuesday's `opacity:0` back in comments.css | `TestReplyButtonsShow`: every place, 51 and 80rem, light and dark: `opacity 0, under 0.5` | no | `testdata/buttons/`, 5 known-bad sheets (Tuesday's real one among them) and 2 known-good: 0.02 s inside `go test`, no build |
| fast's reader | a `panic:` line not recognized | `fast's reader of go test misreads testdata/fast/go-test-v.txt` | no | `testdata/fast/go-test-v.txt`, read first: 3,184 steps, 3 ms |

The button checker had a plant of its own (opacities no longer multiplied), and `faint-article.css` caught it.

**What the plants found:**

1. **A real hole: a commenter's name could carry markup.** It passed all six stages. Now every field the page shows is tested for escaping (`45a2c32`).
2. **The smoke's red named a missing file, not the check that failed.** It names the check now (`8dd7d03`).
3. **My own trap:** a plant whose launch stops before the reverse edit leaves the fault in the tree. It happened once, on plant 6, and I restored it at once. Every plant after that ran under `shale --undo-on-stop`.
4. **Three stages are the only ones that see their faults:** race, bench and the smoke. Anchoring, audit and dates overlap with fast's unit tests on these plants. Their own job is the real notes, where the audit named four real pages and the dates check named a real note.

**The reply buttons, without a browser.** `buttons_test.go` works the cascade out by hand on the CSS the page is served with, for each place `comments.js` puts a button, at every desktop width the media queries name, light and dark. A hidden button (`display:none`, `visibility:hidden`, or opacity under 0.5 from the button out to the page) fails the test, and CSS it can't judge stops it. It is honest within those limits, which its header names: it doesn't run the script, doesn't see clipping, and doesn't judge contrast. On that last one: at .6 opacity, the muted grey on the light background is faint. I didn't put a contrast floor in, because the number is your call.

**Your two additions:** the server now stops at start with `the comments folder's parent <path> is not a folder; make it, or pass another -comments`, and makes nothing. It's tested on three sides and planted once (`bb9f694`). The README says phones are out of scope, and the by-hand line is the desktop browser alone.

**full.shale's time:** before, **30.9 s wall, 24.1 s CPU** (load 2.4 to 3.1). After, **23.3 s wall, 20.7 s CPU** (load 0.8 to 1.0). Today's additions cost tens of milliseconds. The difference is the quieter box, and git, which took 65 to 117 ms today against 750 to 820 ms this morning: the notes repo looks packed now. Under thirty seconds either way on a quiet box. On a busy one, wall time can pass thirty (this morning's 30.9 s), with the anchoring simulation at 10 to 13 s of it.

**Still by hand:** an eye test of a note and its comments in a desktop browser.

**Commits today:** `45a2c32` (every comment field escaped), `8dd7d03` (the smoke names its check), `bb9f694` (the comments' parent checked; phones out of scope), `bb14884` (the reply-button check and its sheets), `e2ab440` (the audit's fixtures), `8021e4e` (fast's reader fixture), and this journal.

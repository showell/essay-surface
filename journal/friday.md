# Friday: a release script

Today's prompt (`journal/prompts/friday.md`): write `scripts/release.shale`, which runs the full check, builds beside the old binary, copies the comments somewhere safe, swaps the servers, checks the index, a note, a note with comments and every comment from before, and puts the old server back if any check fails. Prove it on 9300 with a copy of the human's data, including one release that fails on purpose. Don't touch the live server on 9301.

The report is at the end.

## Orienting

I read the prompt with the Read tool. My first try was `cat`, and the gate refused it, as it should. Then the Shale docs, `shale --help` first, which also lists the mounts.

```sh
shale --help
```

The mounts that matter today: `repos:` is `~/showell_repos` and `scratch:` is `/tmp/claude-1000`. No mount reaches `~/essay-dogfood`. That shapes the real run, below.

```shale
for p in glob("**/*") do print(p, count_lines(p)) end
for p in glob(".shale/*") do print(p) end
for c in git_log("", {n = 25}) do print(c.hash, c.subject) end
print(json_write(git_status("")))
```

Why: the repo's files and where the history stands. Clean tree, HEAD `7f3fb5f` (the Friday prompt).

```shale
for p in {"journal/monday.md","journal/tuesday.md","journal/wednesday.md","journal/thursday.md"} do
  for m in grep(p, "## ") do print(p .. ":" .. m.line, m.text) end
end
print(read(".gitignore"))
```

Why: to find each day's report. I read all four with the Read tool, then the prelude, `full.shale`, `fast.shale`, `comments_smoke.shale` and `server.go`.

What I took from them for today:

- The live server is `/home/steve/essay-dogfood/bin/essay-surface -notes /home/steve/essay-dogfood/notes -addr 127.0.0.1:9301` (Wednesday's and Thursday's last checks print it). It passes no `-comments`, so its comments are `~/essay-dogfood/comments`, beside the notes.
- Monday's trap: a run that stops after `job_start` leaves the server running.
- `full.shale` ends `SAFE TO SHIP` or `NOT SAFE`, and exits 1 on the second.

```shale
for m in grep("docs/SHALE.md", "job") do print(m.line, sub(m.text, 1, 160)) end
print("---")
for m in grep("docs/SHALE.md", "undo-on-stop") do print(m.line, sub(m.text, 1, 160)) end
```

```shale
for w in {"copy", "rename", "move(", "chmod", "stat(", "ls(", "remove("} do
  for m in grep("docs/SHALE.md", w) do print(w, m.line, sub(m.text, 1, 150)) end
end
```

Why: what jobs do and what file moves Shale has (run from the Shale checkout). The second answer: Shale has no copy or rename. `replace` would write a binary without its executable bit, so binaries move by `mv` and `cp` under `run`. And `--undo-on-stop` never puts back what a program or a job did. That matters for the release: a stop after the old server is down can't be undone by Shale.

```shale
for p in glob("scripts/*.shale") do for m in grep(p, "930") do print(p .. ":" .. m.line, m.text) end end
for p in glob("*.go") do for m in grep(p, "930") do print(p .. ":" .. m.line, m.text) end end
for p in glob("journal/*.md") do for m in grep(p, "behind") do print(p .. ":" .. m.line, sub(m.text, 1, 200)) end end
print(date(now()))
print(json_write(ls("scratch:essay-friday-data")))
print(#ls("scratch:essay-friday-data/notes"))
for f in ls("scratch:essay-friday-data/comments") do print(f, stat("scratch:essay-friday-data/comments/" .. f).size, count_lines("scratch:essay-friday-data/comments/" .. f)) end
print(exists("scratch:essay-friday-data/notes/.git"))
print(exists("scratch:.shale"))
```

Why: which scripts use port 9300, and what the test data holds.

```text
scripts/audit.shale:60	local index = attempt(http_get, "http://127.0.0.1:9300/")
scripts/bench.shale:27	const BASE = "http://127.0.0.1:9300"
scripts/comments_smoke.shale:9	const BASE = "http://127.0.0.1:9300"
scripts/dates.shale:15	const BASE = "http://127.0.0.1:9300"
server.go:298		addr := flag.String("addr", "127.0.0.1:9300", "the address to listen on")
2026-11-13T17:37:32Z
["comments","notes"]
536
day-2026-11-02.md.jsonl	248	1
essay-surface-wednesday.md.jsonl	1337	5
shale-book-draft-2-plan.md.jsonl	650	1
false
false
```

**A conflict, found before any code:** every check's server listens on 9300, and today's trials put the "old" server on 9300 too. The release runs the full check while that server is up. So in a trial, the smoke, the audit, the dates and the bench would all collide with the server being released. The real release on 9301 wouldn't collide, but a trial that can't run the same script isn't a proof.

```shale
for p in glob("journal/*.md") do for w in {"left up", "still up", "still runs", "job_start", "trap"} do for m in grep(p, w) do print(p .. ":" .. m.line, sub(m.text, 1, 220)) end end end
for p in glob("scratch:essay-friday-data/comments/*") do print(p) print(read(p)) end
```

Why: the job traps the earlier days hit, and the human's seven real replies (all made today, in three files).

```shale
local d = fresh_dir("", "friday-probe")
print(d)
replace(d .. "/a", "hello\n")
local r = run({"cp", "-p", real(d .. "/a"), real(d) .. "/b"}, {timeout_ms = 5000, tail = 5})
print("cp", r.code, r.out)
r = run({"mv", real(d .. "/b"), real(d) .. "/c"}, {timeout_ms = 5000, tail = 5})
print("mv", r.code, r.out)
print(json_write(ls(d)))
print(attempt(real, d .. "/missing").ok)
local j = job_start({"sleep", "1"}, {log = d .. "/j.log"})
print(json_write(job_wait(j, {timeout_ms = 5000})))
append(d .. "/j.log", "old line\n")
j = job_start({"sleep", "0"}, {log = d .. "/j.log"})
print(json_write(job_wait(j, {timeout_ms = 5000})))
print("log after second job: [" .. read(d .. "/j.log") .. "]")
print(json_write(ls(d)))
print(read(d .. "/j.log.job"))
remove_tree(d)
```

Why: small experiments before the long ones (run from `/tmp/claude-1000`). `cp` and `mv` run under `run`, and `real` gives a path that doesn't exist yet. **A new job on a log starts the log empty** (`log after second job: []`), so the release must copy the old server's log before it starts the new one.

## The trial's "old" server

```shale
-- trial setup: a fresh copy of the human's data at scratch:essay-trial, today's HEAD built as the "old" server, started the way the live one is
const T = "scratch:essay-trial"
for p in procs() do
  if p.name == "essay-surface" then print("running before: pid " .. p.pid .. ": " .. p.args) end
end
if exists(T) then remove_tree(T) end
must(run({"cp", "-a", real("scratch:essay-friday-data"), real("scratch:") .. "/essay-trial"}, {timeout_ms = 30000, tail = 5}))
mkdir(T .. "/bin")
local b = must(run({"go", "build", "-o", real(T .. "/bin") .. "/essay-surface", "."}, {timeout_ms = 120000, tail = 20}))
print("built HEAD " .. git_head("") .. " in " .. b.ms .. " ms")
local argv = {real(T .. "/bin/essay-surface"), "-notes", real(T .. "/notes"), "-addr", "127.0.0.1:9300"}
local job = job_start(argv, {log = T .. "/server.log", want = "trial: the old server, as the live one is started"})
print(json_write(job_wait(job, {timeout_ms = 30000, contains = "listening"})))
local r = http_get("http://127.0.0.1:9300/")
print("index " .. r.status .. ", " .. #essay_names(r.body) .. " notes")
print(read(T .. "/server.log"))
```

```text
running before: pid 2484343: /home/steve/essay-dogfood/bin/essay-surface -notes /home/steve/essay-dogfood/notes -addr 127.0.0.1:9301
built HEAD 7f3fb5f in 939 ms
{"ended":"found"}
index 200, 507 notes
```

The trial folder is shaped like the live one: `bin/essay-surface`, `notes/`, `comments/`, `server.log`, with the server started by the same argv, ending at `-addr 127.0.0.1:9300`.

## A check that talked to the wrong server

With the trial server holding 9300, I ran the smoke check, as a trial release would:

```shale
-- before: the smoke check while the trial server holds 9300, as it would during a trial release
local r = run({"shale", "scripts/comments_smoke.shale"}, {timeout_ms = 60000, tail = 12})
print("exit " .. r.code .. ", " .. r.ms .. " ms")
print(r.out)
print(read(".run/smoke/server.log"))
```

```text
exit 1, 692 ms
server stopped
── stopped at scripts/comments_smoke.shale:66:23: fail: the smoke test stopped: the page: 404 (line 16) (20559 steps)
...
2026/11/13 17:38:54 listening on http://127.0.0.1:9300/
2026/11/13 17:38:54 listen tcp 127.0.0.1:9300: bind: address already in use
```

**This is a real bug, not just a collision.** The server logged "listening" *before* it bound the port, and every script waits for "listening". The smoke's own server died, and the smoke went on to ask the trial server for `smoke.md` and got a 404. Here it failed. With a different server holding the port, a check could pass against the wrong program, or a release could check a server that never started.

Two fixes:

1. `server.go` now calls `net.Listen` first and logs "listening" only once the port is its own.
2. The checks' servers listen on 9310, named once in the prelude (`essay_base()`, and `-addr` in `essay_start`), so a check runs beside a dev server on 9300. Port 9300 stays the server's default and the dev port.

```shale
-- the checks' scripts ask the prelude for their server's address
for p in {"scripts/bench.shale","scripts/dates.shale","scripts/comments_smoke.shale"} do
  edit(p, {{old = "const BASE = \"http://127.0.0.1:9300\"\n", new = ""}, {old = "BASE .. ", new = "essay_base() .. ", all = true}})
end
edit("scripts/audit.shale", "\"http://127.0.0.1:9300/", "essay_base() .. \"/", {all = true})
for p in glob("scripts/*.shale") do for m in grep(p, "930") do print("left: " .. p .. ":" .. m.line, m.text) end end
for p in glob("scripts/*.shale") do for m in grep(p, "BASE") do print("left: " .. p .. ":" .. m.line, m.text) end end
```

(run with `--undo-on-stop`; nothing was left)

```shale
-- after: fast (and its smoke) with the trial server still holding 9300
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 110000, tail = 10})
print("exit " .. r.code .. ", " .. r.ms .. " ms")
print(r.out)
print(json_write(git_diff("", {stat = true})))
```

```text
exit 0, 3708 ms
ok  smoke, 1119 ms, cpu 1395 ms: 10 checks through the real server
fast: PASS
```

```shale
-- the fix seen: a server whose port is taken never says "listening"
must(run({"go", "build", "-o", "essay-surface", "."}, {timeout_ms = 120000, tail = 20}))
mkdir(".run/taken")
local job = job_start({"./essay-surface", "-notes", "testdata/buttons", "-comments", ".run/taken/comments", "-addr", "127.0.0.1:9300"}, {log = ".run/taken/server.log", cwd = ".", want = "build: the server we just built"})
print(json_write(job_wait(job, {timeout_ms = 10000, contains = "listening"})))
print(read(".run/taken/server.log"))
```

```text
{"ended":"exited","code":1}
...
2026/11/13 17:39:25 listen tcp 127.0.0.1:9300: bind: address already in use
```

Committed as `d48216f`.

## How the real run names `~/essay-dogfood`

Shale refuses absolute paths, and no mount reaches the dogfood folder. Adding a mount is the human's call. So the script takes `data=` as a Shale path, and the real run launches from the home folder, where `essay-dogfood` is a plain relative folder. The script itself is named through `repos:`, so the trial and the real run are the same command, with only the launch folder, `data=` and `port=` changed:

```text
cd /tmp/claude-1000 && shale repos:essay-surface/scripts/release.shale data=essay-trial port=9300
cd /home/steve      && shale repos:essay-surface/scripts/release.shale data=essay-dogfood port=9301
```

A launch from the home folder makes it the root, and a root's `.shale/prelude.shale` would load. I checked that there isn't one:

```shale
-- what a run rooted at the home folder would load as the root's prelude (nothing else is read)
print(exists(".shale"))
```

```text
false
```

That also means the repo's prelude doesn't load for the release, so `release.shale` is self-contained. Its functions are `rel_*`, so none can collide with the user prelude.

`data=` and `port=` have no defaults: a release with neither stops before it reads anything. The server is found by `procs()` with `p.args` equal to exactly the argv the script starts, so the server it stops is the one it would start, and nothing else matches. A count other than one stops the release with nothing touched.

## The design

All of it is in the script's header, `scripts/release.shale`:

1. The checkout is clean (untracked files aside), and exactly one server runs on the data folder's argv.
2. `shale scripts/full.shale` must exit 0 *and* print `SAFE TO SHIP`.
3. `go build -o bin/essay-surface.new`, with a `.version` beside it: the commit, when it was built, and the commit's subject.
4. `releases/<when>/`: the comments copied with `cp -a` and compared file by file in Shale, and the old server's index and its commented pages, fetched by one curl, as the record of where each comment was.
5. The old server stopped (`proc_stop`), then `job_wait` on `server.log.job` until the record says it ended, because a log whose job hasn't ended takes no new job. Its log is copied. Then `bin/essay-surface` goes to `bin/essay-surface.prev`, `.new` goes to `bin/essay-surface`, and the new one is started as a job, which outlives the run.
6. The checks: the index lists the same notes as before; the first note renders with an `<article>`; on every note with comments, each comment is in the same place as on the old server (the block it follows, or the orphans); and every comment id is still in its file.
7. Steps 5 and 6 run under `attempt`. On a failure: stop the new server, keep its log, move it to `bin/essay-surface.failed` and `.prev` back, put back any comment missing from its file from the copy (keeping any made since), start the old server and check it the same way.

`rollback=1` swaps `bin/essay-surface` and `.prev` through steps 1 and 4 to 7, without the full check or a build. Run twice, it rolls forward again.

The comment check is stricter than "on the page": a comment must be on *the same block* as before. A release that changes the anchoring would fail it, and should then be looked at by hand.

## The trials

### Trial 1: two stops of my own, one of them the trap

First run: `run`'s `stage` option needs a `cpu_budget_ms`. It stopped before anything was touched. Fixed.

```shale
edit("scripts/release.shale", ", tail = 40, stage = \"full\"})", ", tail = 40})")
```

(My first try at this put the edit and the trial in one Bash call with two heredocs, and the gate refused it: one launch per call.)

```shale
-- trial 1, again: a good release, d48216f over the trial's 7f3fb5f, on 9300; timed from outside
local r = run({"shale", "repos:essay-surface/scripts/release.shale", "data=essay-trial", "port=9300"}, {timeout_ms = 280000, tail = 80})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
```

```text
exit 1, 24506 ms wall, cpu 21454 ms
...
full check: SAFE TO SHIP, 23454 ms
...
stopped the old server
started commit d48216f (...)
── stopped at the prelude:244:15 (in `the prelude's re_find_all`): budget steps: the step budget (5000000 steps) ran out (5000001 steps)
   called from repos:essay-surface/scripts/release.shale:107:12 (in `rel_names`)
   called from repos:essay-surface/scripts/release.shale:151:49 (in `rel_look`)
   called from repos:essay-surface/scripts/release.shale:165:15 (in `rel_check`)
```

**This is the trap the brief warned of, and the most important thing today taught.** `attempt` doesn't catch a budget stop. The run stopped after the old server was down and the new one was up, so the rollback never ran. The trial was left on the new, unchecked server, with the old binary at `.prev`: not where it started. On the live server, that would have been a release nobody checked and nobody rolled back.

Two fixes, both in the script:

- The cause: `re_find_all` over a 67 KB index costs a step a character. The page readings now use `split` and `find`, which are builtins.
- The defense: a `-- budget: steps=50000000` header, with the reason written above it. Trial 2 measured the real need: the whole release fits under 2.5M steps, so this is twenty times that. A run killed from outside (a Bash timeout) has the same hole, so the header says to give the call 300 s.

```shale
-- the page readings by find and split, not re_find_all, which costs a step a character; and full's whole tail
edit("scripts/release.shale", {
  {old = [=[  for m in re_find_all(index, "href=\"/notes/([^\"]+)\"") do push(out, m.group) end
  return out]=], new = [=[  for p in drop(split(index, "href=\"/notes/"), 1) do push(out, sub(p, 1, find(p, "\"") - 1)) end
  return out]=]},
  {old = [=[    local fors = re_find_all(page, "<div class=\"cmts\" data-for=\"([0-9]+)\">")
]=], new = [=[    local fors = {}
    local mark = "<div class=\"cmts\" data-for=\""
    local f = find(page, mark)
    while f ~= nil do
      local from_at = f + #mark
      push(fors, {at = f, group = sub(page, from_at, find(page, "\"", from_at) - 1)})
      f = find(page, mark, from_at)
    end
]=]},
  {old = ", tail = 40})", new = ", tail = 200})"},
})
local r = run({"shale", "--check", "scripts/release.shale"}, {timeout_ms = 10000, tail = 10})
print(r.code, r.out)
```

Putting the trial back by hand:

```shale
-- after the budget stop: what runs on the trial, stopped; the 7f3fb5f build kept for later trials; a fresh copy; the old server back
const T = "essay-trial"
const ARGS = "/tmp/claude-1000/essay-trial/bin/essay-surface -notes /tmp/claude-1000/essay-trial/notes -addr 127.0.0.1:9300"
for p in procs() do
  if p.name == "essay-surface" then print("pid " .. p.pid .. ": " .. p.args) end
  if p.args == ARGS then print("stopping it: " .. tostring(proc_stop(p))) end
end
print(json_write(job_wait(T .. "/server.log.job", {timeout_ms = 10000})))
print(json_write(ls(T .. "/bin")))
mkdir("essay-old")
must(run({"cp", "-p", real(T .. "/bin/essay-surface.prev"), real("essay-old") .. "/essay-surface-7f3fb5f"}, {timeout_ms = 10000, tail = 5}))
remove_tree(T)
must(run({"cp", "-a", real("essay-friday-data"), real(".") .. "/essay-trial"}, {timeout_ms = 30000, tail = 5}))
mkdir(T .. "/bin")
must(run({"cp", "-p", real("essay-old/essay-surface-7f3fb5f"), real(T .. "/bin") .. "/essay-surface"}, {timeout_ms = 10000, tail = 5}))
local job = job_start({real(T .. "/bin/essay-surface"), "-notes", real(T .. "/notes"), "-addr", "127.0.0.1:9300"}, {log = T .. "/server.log", want = "trial: the old server, as the live one is started"})
print(json_write(job_wait(job, {timeout_ms = 30000, contains = "listening"})))
```

```text
pid 2498862: /tmp/claude-1000/essay-trial/bin/essay-surface -notes /tmp/claude-1000/essay-trial/notes -addr 127.0.0.1:9300
stopping it: true
{"ended":"exited","code":0}
["essay-surface","essay-surface.prev","essay-surface.version"]
{"ended":"found"}
```

The `7f3fb5f` build is kept at `scratch:essay-old/essay-surface-7f3fb5f`, and every later trial starts from it. The third run of trial 1 released cleanly (22.8 s). Its last line was too long to read whole, so the last line now names versions by commit:

```shale
-- the last line short enough to read whole: versions by their commit
edit("scripts/release.shale", {
  {old = [=["commit " .. head.hash .. " (" .. head.subject .. "), built " .. date(now()) .. "\n"]=], new = [=[head.hash .. " built " .. date(now()) .. ": " .. head.subject .. "\n"]=]},
  {old = [=[function rel_version(t, suffix)
  return trim(read_or(t.data .. "/bin/essay-surface" .. suffix .. ".version", "unknown (built before release.shale)"))
end]=], new = [=[-- A binary's version: its commit, when it was built, and the commit's
-- subject; "unknown" for one built before this script.
function rel_version(t, suffix)
  return trim(read_or(t.data .. "/bin/essay-surface" .. suffix .. ".version", "unknown (built before release.shale)"))
end

function rel_commit(t, suffix)
  return words(rel_version(t, suffix))[1]
end]=]},
  {old = [=[  print("RELEASED: running " .. rel_version(t, "") .. " on " .. t.addr .. "; roll back to " .. rel_version(t, ".prev") .. " with: " .. back)]=], new = [=[  print("RELEASED: running " .. rel_commit(t, "") .. " on " .. t.addr .. "; to roll back to " .. rel_commit(t, ".prev") .. ": " .. back)]=]},
  {old = [=[  fail("ROLLED BACK: running " .. rel_version(t, "") .. " on " .. t.addr .. ", as before; the release failed: " .. done.error)]=], new = [=[  fail("ROLLED BACK: running " .. rel_commit(t, "") .. " on " .. t.addr .. ", as before; the release failed: " .. done.error)]=]},
})
local r = run({"shale", "--check", "scripts/release.shale"}, {timeout_ms = 10000, tail = 10})
print(r.code, r.out)
```

Committed as `fc558f2`. The trial reset, used before trials 2 and 3:

```shale
-- trial reset: what runs on the trial stopped; a fresh copy; the 7f3fb5f build started as the live one is
const T = "essay-trial"
const ARGS = "/tmp/claude-1000/essay-trial/bin/essay-surface -notes /tmp/claude-1000/essay-trial/notes -addr 127.0.0.1:9300"
for p in procs() do
  if p.args == ARGS then print("stopping pid " .. p.pid .. ": " .. tostring(proc_stop(p))) end
end
if exists(T .. "/server.log.job") then print(json_write(job_wait(T .. "/server.log.job", {timeout_ms = 10000}))) end
remove_tree(T)
must(run({"cp", "-a", real("essay-friday-data"), real(".") .. "/essay-trial"}, {timeout_ms = 30000, tail = 5}))
mkdir(T .. "/bin")
must(run({"cp", "-p", real("essay-old/essay-surface-7f3fb5f"), real(T .. "/bin") .. "/essay-surface"}, {timeout_ms = 10000, tail = 5}))
local job = job_start({real(T .. "/bin/essay-surface"), "-notes", real(T .. "/notes"), "-addr", "127.0.0.1:9300"}, {log = T .. "/server.log", want = "trial: the old server, as the live one is started"})
print(json_write(job_wait(job, {timeout_ms = 30000, contains = "listening"})))
```

### Trial 2: the good release, and a rollback by request

```shale
-- trial 2: the good release for the report, fc558f2 over 7f3fb5f; the step budget held to the default 5M to measure the headroom
local r = run({"shale", "--budget", "steps=5000000", "repos:essay-surface/scripts/release.shale", "data=essay-trial", "port=9300"}, {timeout_ms = 280000, tail = 80})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
```

The output is in the report. It passed under the default 5M steps without crossing half of them, so it used under 2.5M.

```shale
-- trial 2, then: rollback=1, back to the 7f3fb5f build; and the downtime, from the two servers' logs
local r = run({"shale", "repos:essay-surface/scripts/release.shale", "data=essay-trial", "port=9300", "rollback=1"}, {timeout_ms = 120000, tail = 40})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
print(json_write(ls("essay-trial/bin")))
for d in ls("essay-trial/releases") do
  print(d, json_write(ls("essay-trial/releases/" .. d)))
  for m in grep("essay-trial/releases/" .. d .. "/server.log", "stopping") do print("  old log: " .. m.text) end
end
for m in grep("essay-trial/server.log", "listening") do print("  new log: " .. m.text) end
```

```text
exit 0, 706 ms wall, cpu 73 ms
rollback: /tmp/claude-1000/essay-trial on 127.0.0.1:9300
running: pid 2500626, fc558f2 built 2026-11-13T17:44:30Z: release.shale: ...
...
ok: all 7 comments from before on their pages and in their files
RELEASED: running unknown on 127.0.0.1:9300; to roll back to fc558f2: cd /tmp/claude-1000 && shale repos:essay-surface/scripts/release.shale data=essay-trial port=9300 rollback=1
["essay-surface","essay-surface.prev","essay-surface.prev.version"]
```

A rollback works in 0.7 s. Two defects:

1. **The kept old log had no `stopping` line.** I copied it while the old server still ran, and the new job then started `server.log` empty, so the old server's last words were lost. The log is now copied after the stop. A failed server's log is kept too, as `server.failed.log`.
2. A rollback said `RELEASED`. It now says `ROLLED BACK, as asked`.

```shale
-- the old server's log kept after it stops, so its last line is in the copy; a rollback says so
edit("scripts/release.shale", {
  {old = [=[if exists(t.log) then must(run({"cp", "-p", real(t.log), real(t.dir) .. "/server.log"}, {timeout_ms = 10000, tail = 5})) end
]=], new = ""},
  {old = [=[function rel_install_and_check(t, before)
  rel_stop(t)
  print("stopped the old server")]=], new = [=[-- The log of the server just stopped, kept under name in the release's
-- folder: the next job on the log starts it empty.
function rel_keep_log(t, name)
  if exists(t.log) then must(run({"cp", "-p", real(t.log), real(t.dir) .. "/" .. name}, {timeout_ms = 10000, tail = 5})) end
end

function rel_install_and_check(t, before)
  rel_stop(t)
  rel_keep_log(t, "server.log")
  print("stopped the old server; its log kept")]=]},
  {old = [=[function rel_put_back(t, before)
  rel_stop(t)]=], new = [=[function rel_put_back(t, before)
  rel_stop(t)
  rel_keep_log(t, "server.failed.log")]=]},
  {old = [=[  print("RELEASED: running "]=], new = [=[  local said = "RELEASED"
  if t.rollback then said = "ROLLED BACK, as asked" end
  print(said .. ": running "]=]},
})
local r = run({"shale", "--check", "scripts/release.shale"}, {timeout_ms = 10000, tail = 10})
print(r.code, r.out)
```

Committed as `749242e`.

### Trial 3: a release that fails on purpose

The plant had to be a fault the full check passes and a release catches, because a fault the check catches never reaches the swap. I found one in a class nothing covers: `main`'s defaults. Every check passes `-comments` explicitly, and the live server passes none. The plant moves the default comments folder inside the notes folder. It is committed on a throwaway branch in its own worktree, so `main` stays clean and the release ships a commit, as a real bad release would.

```shale
print(git_commit("", "release.shale keeps the old server's log after it stops; a rollback says so\n\n...", {add = {"scripts/release.shale"}}))
git_worktree_add("", "scratch:essay-plant", {branch = "plant-comments-default", new = true})
print(json_write(ls("scratch:essay-plant")))
```

```shale
-- the plant, on a throwaway branch in its own worktree: a server with no -comments looks for them inside the notes folder. Every check passes -comments, so none should see it
edit("essay-plant/server.go", "*comments = filepath.Join(filepath.Dir(filepath.Clean(*dir)), \"comments\")", "*comments = filepath.Join(filepath.Clean(*dir), \"comments\")")
print(git_commit("essay-plant", "PLANT: the comments' default folder inside the notes folder\n\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>", {add = "tracked"}))
```

Then the trial reset above, and:

```shale
-- trial 3: the release that fails on purpose, the plant abacae9 from its worktree over 7f3fb5f; timed from outside; then what is left
local r = run({"shale", "repos:essay-surface/scripts/release.shale", "data=essay-trial", "port=9300", "repo=scratch:essay-plant"}, {timeout_ms = 280000, tail = 80})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
print(json_write(ls("essay-trial/bin")))
print(json_write(ls("essay-trial/notes/comments")))
for d in ls("essay-trial/releases") do print(d, json_write(ls("essay-trial/releases/" .. d))) end
for p in procs() do if p.name == "essay-surface" then print("pid " .. p.pid .. ": " .. p.args) end end
```

**The full check said SAFE TO SHIP on the plant, and the release caught it**, then rolled back. The output is in the report. The `ls` of `notes/comments` stopped my own script: the folder was never made, because the bad server took no posts. So I looked again:

```shale
-- after trial 3: what is left, and the comments against the data the trial started from
for d in ls("essay-trial/releases") do print(d, json_write(ls("essay-trial/releases/" .. d))) end
for f in ls("essay-friday-data/comments") do
  print(f, read("essay-friday-data/comments/" .. f) == read("essay-trial/comments/" .. f) and "same as the data's" or "DIFFERS")
end
for p in procs() do if p.name == "essay-surface" then print("pid " .. p.pid .. ": " .. p.args) end end
local d = "essay-trial/releases/" .. ls("essay-trial/releases")[1]
for m in grep(d .. "/server.log", "stopping") do print("old: " .. m.text) end
for m in grep(d .. "/server.failed.log", "stopping") do print("failed: " .. m.text) end
for m in grep(d .. "/server.failed.log", "listening") do print("failed: " .. m.text) end
for m in grep(d .. "/server.failed.log", "rendered") do print("failed: " .. m.text) end
for m in grep("essay-trial/server.log", "listening") do print("back: " .. m.text) end
```

```text
2026-11-13T174542Z	["after","before","comments","rolled-back","server.failed.log","server.log"]
day-2026-11-02.md.jsonl	same as the data's
essay-surface-wednesday.md.jsonl	same as the data's
shale-book-draft-2-plan.md.jsonl	same as the data's
pid 2484343: /home/steve/essay-dogfood/bin/essay-surface -notes /home/steve/essay-dogfood/notes -addr 127.0.0.1:9301
pid 2502492: /tmp/claude-1000/essay-trial/bin/essay-surface -notes /tmp/claude-1000/essay-trial/notes -addr 127.0.0.1:9300
old: 2026/11/13 17:46:12 stopping: 4 requests served in 2 ms of cpu after the start
failed: 2026/11/13 17:46:12 stopping: 5 requests served in 2 ms of cpu after the start
failed: 2026/11/13 17:46:12 listening on http://127.0.0.1:9300/
failed: 2026/11/13 17:46:12 rendered 507 notes from /tmp/claude-1000/essay-trial/notes in 316ms; comments in /tmp/claude-1000/essay-trial/notes/comments
back: 2026/11/13 17:46:12 listening on http://127.0.0.1:9300/
```

Every comments file is byte for byte the data's. The bad server's log names the cause on its first line (`comments in .../notes/comments`). The old server stopped, the bad one started and was checked and stopped, and the old one was back, all inside one second of the logs' clock.

### Trial 4: a release the full check refuses

```shale
-- trial 4: a release the full check refuses: a gofmt fault committed on the plant branch; nothing may be touched
edit("essay-plant/server.go", "\tdir := flag.String(", "        dir := flag.String(")
print(git_commit("essay-plant", "PLANT: a line gofmt refuses\n\nCo-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>", {add = "tracked"}))
local before = ""
for p in procs() do if p.name == "essay-surface" then before = before .. p.pid .. " " end end
local r = run({"shale", "repos:essay-surface/scripts/release.shale", "data=essay-trial", "port=9300", "repo=scratch:essay-plant"}, {timeout_ms = 280000, tail = 30})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
local after = ""
for p in procs() do if p.name == "essay-surface" then after = after .. p.pid .. " " end end
print("servers before: " .. before .. "; after: " .. after)
print(json_write(ls("essay-trial/bin")), json_write(ls("essay-trial/releases")))
```

```text
exit 1, 23279 ms wall, cpu 18423 ms
   == fast: FAIL, 28 ms, cpu 30 ms
   ...
   NOT SAFE: fast failed
── stopped at repos:essay-surface/scripts/release.shale:320:5: fail: full.shale did not say SAFE TO SHIP (exit 1): nothing touched (17787 steps)
servers before: 2484343 2502492 ; after: 2484343 2502492
["essay-surface","essay-surface.failed","essay-surface.failed.version"]	["2026-11-13T174542Z"]
```

The same server pids before and after, no build, no new release folder. (The `.failed` binary and the one release folder are trial 3's.) This refusal takes 23 s, because `full.shale` runs every stage even after `fast` fails. That's its design, so it reports every stage.

## Cleaning up

```shale
-- clean up: the plant's worktree and branch (plants are one-time), the trial server stopped
git_worktree_remove("", "scratch:essay-plant", {force = true})
git_branch_delete("", "plant-comments-default", {force = true})
for p in procs() do
  if p.args == "/tmp/claude-1000/essay-trial/bin/essay-surface -notes /tmp/claude-1000/essay-trial/notes -addr 127.0.0.1:9300" then print("stopped trial pid " .. p.pid .. ": " .. tostring(proc_stop(p))) end
end
print(json_write(job_wait("scratch:essay-trial/server.log.job", {timeout_ms = 10000})))
for p in procs() do if p.name == "essay-surface" then print("left: pid " .. p.pid .. ": " .. p.args) end end
print(read("README.md"))
```

```text
stopped trial pid 2502492: true
{"ended":"exited","code":0}
left: pid 2484343: /home/steve/essay-dogfood/bin/essay-surface -notes /home/steve/essay-dogfood/notes -addr 127.0.0.1:9301
```

Only the human's server is left, the same pid as this morning, never touched. I didn't read or write `~/essay-dogfood`. The README now names the release and the checks' port.

## Report

**`scripts/release.shale`** runs the full check (it must say `SAFE TO SHIP`), then builds `bin/essay-surface.new`. It copies the comments to `releases/<when>/comments` and compares the copy, and keeps the old server's pages there, as the record of where every comment was. Then it stops the old server, keeps its binary as `bin/essay-surface.prev`, and starts the new one. It checks the index, a note, and that every comment is on the same block as before and still in its file. If any check fails, it puts the old binary back, starts it, and checks it the same way. Its last line says what's running and how to roll back. `rollback=1` swaps back to the last binary.

**A good release** (trial 2, `fc558f2` over `7f3fb5f`): **25.2 s wall, 19.5 s CPU.** The full check is 24.0 s of that, the build 0.8 s, and the swap and checks under half a second.

```text
release: /tmp/claude-1000/essay-trial on 127.0.0.1:9300
running: pid 2499579, unknown (built before release.shale)
   == fast: PASS, 3326 ms, cpu 3437 ms
   == race: PASS, 2438 ms, cpu 1485 ms
   == anchoring: PASS, 11114 ms, cpu 11015 ms
   == audit: PASS, 1323 ms, cpu 579 ms
   == dates: PASS, 1707 ms, cpu 358 ms
   == bench: PASS, 4126 ms, cpu 1619 ms
full check: SAFE TO SHIP, 24045 ms
built bin/essay-surface.new: fc558f2 built 2026-11-13T17:44:30Z: release.shale: a release the same way every time, rolled back when a check fails, 767 ms
comments kept: 3 files, 7 comments, in essay-trial/releases/2026-11-13T174430Z/comments, each compared
the old server's index and 3 commented notes kept in essay-trial/releases/2026-11-13T174430Z/before
stopped the old server
started fc558f2 built 2026-11-13T17:44:30Z: release.shale: a release the same way every time, rolled back when a check fails
ok: the index, 507 notes, the same as before
ok: a note, 2026-11-01-a-day-of-judges.md, 18882 bytes
ok: day-2026-11-02.md, 200, its 1 comments where they were
ok: essay-surface-wednesday.md, 200, its 5 comments where they were
ok: shale-book-draft-2-plan.md, 200, its 1 comments where they were
ok: all 7 comments from before on their pages and in their files
RELEASED: running fc558f2 on 127.0.0.1:9300; to roll back to unknown: cd /tmp/claude-1000 && shale repos:essay-surface/scripts/release.shale data=essay-trial port=9300 rollback=1
```

**A failed release and its rollback** (trial 3, a planted bad default for the comments folder, over `7f3fb5f`): **30.3 s wall, 22.1 s CPU, on a busier box.** The full check is 28.2 s of that (it passed the plant), and the build 1.0 s. Stopping the old server, starting the bad one, checking it, rolling back and checking again all fit inside one second of the logs' clock. A rollback by request (`rollback=1`) takes 0.7 s.

```text
release: /tmp/claude-1000/essay-trial on 127.0.0.1:9300
running: pid 2501804, unknown (built before release.shale)
   == fast: PASS, 4665 ms, cpu 4617 ms
   == race: PASS, 4016 ms, cpu 2708 ms
   == anchoring: PASS, 11624 ms, cpu 11235 ms
   == audit: PASS, 1336 ms, cpu 641 ms
   == dates: PASS, 1678 ms, cpu 332 ms
   == bench: PASS, 4889 ms, cpu 1606 ms
full check: SAFE TO SHIP, 28218 ms
built bin/essay-surface.new: abacae9 built 2026-11-13T17:45:42Z: PLANT: the comments' default folder inside the notes folder, 1001 ms
comments kept: 3 files, 7 comments, in essay-trial/releases/2026-11-13T174542Z/comments, each compared
the old server's index and 3 commented notes kept in essay-trial/releases/2026-11-13T174542Z/before
stopped the old server; its log kept
started abacae9 built 2026-11-13T17:45:42Z: PLANT: the comments' default folder inside the notes folder
FAILED: comment f31eaaa22556 on day-2026-11-02.md: missing now, block 5 before (line 202)
started unknown (built before release.shale)
ok: the index, 507 notes, the same as before
ok: a note, 2026-11-01-a-day-of-judges.md, 18882 bytes
ok: day-2026-11-02.md, 200, its 1 comments where they were
ok: essay-surface-wednesday.md, 200, its 5 comments where they were
ok: shale-book-draft-2-plan.md, 200, its 1 comments where they were
ok: all 7 comments from before on their pages and in their files
── stopped at repos:essay-surface/scripts/release.shale:366:3: fail: ROLLED BACK: running unknown on 127.0.0.1:9300, as before; the release failed: comment f31eaaa22556 on day-2026-11-02.md: missing now, block 5 before
```

Afterwards, every comments file was byte for byte the data's, the old binary was back at `bin/essay-surface`, and the bad one was kept as `bin/essay-surface.failed`, with its log. A release the full check refuses (trial 4) stops with `nothing touched` after 23 s.

**What today found:**

1. **The full check passes a release that would hide every comment.** `main`'s defaults are run by no check, because every check passes `-comments`. Only the release's own check, run against the server as it is really started, sees it. That's the case for checking after the swap.
2. **A server said "listening" before it held its port**, so a check could talk to whatever held it. Fixed in `server.go`, and the checks moved to port 9310 (`d48216f`).
3. **The trap, hit for real in trial 1:** a budget stop isn't caught by `attempt`. It left the trial on the new, unchecked server. The page readings are now builtins, and the script's step budget is twenty times what it uses. A Bash timeout would leave the same hole, so give the call 300 s.

**Calls you may want to unmake:**

- A comment must be on *the same block* as before, not just on the page. A release that changes the anchoring will fail and roll back, and then needs a look by hand.
- The release refuses a checkout with uncommitted changes to tracked files, so what runs is always a commit. Untracked files are allowed.
- Each release adds a `releases/<when>/` folder to `~/essay-dogfood`: the comments, the old log and a few pages, tens of KB. Nothing prunes them.
- The first real release will show the running binary as `unknown`, because it was built before versions were written. `rollback=1` goes back to it all the same.

**The real release, for the orchestrator to run on 9301,** with the Bash call's timeout at 300000 ms, after this journal is committed:

```sh
cd /home/steve && shale repos:essay-surface/scripts/release.shale data=essay-dogfood port=9301
```

Its last line is `RELEASED: running <commit> on 127.0.0.1:9301; to roll back ...`, or a stop that starts `ROLLED BACK:`. To go back afterwards: `cd /home/steve && shale repos:essay-surface/scripts/release.shale data=essay-dogfood port=9301 rollback=1`.

**Commits today:** `d48216f` (listening once the port is its own; the checks on 9310), `fc558f2` (release.shale), `749242e` (the old log kept after the stop), and this journal with the README.

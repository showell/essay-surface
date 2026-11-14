# Session 2: comments that stay attached

The prompt is `journal/prompts/session-2.md`: comments on any paragraph, which stay with their paragraph through edits and moves and are shown as orphans (never lost) when it is deleted; a plain-JavaScript front end that works on a phone; comments stored as files I choose and defend; a page with comments as fast as one without, measured; Go tests for the anchoring; a report.

All shell work is Shale. Each script is verbatim, with why I ran it and what it said, stops and retries included. Where a script carried a long Go text into an `edit`, I've cut the Go and said where it is (the commit), since the Go is in the repo; the Shale around it is as it ran.

## Orienting

I read yesterday's journal, then the code. The first thing I did was reach for `cat`, and the gate refused it before it ran:

```text
PreToolUse:Bash hook error: shale-gate: `cat` is not `shale`; only a launch, ...
```

Fair. Files I read with my file reader; the repo's shape with Shale:

```shale
-- orient: the repo's files and history
for p in glob("*") do print(p .. " " .. stat(p).size) end
for p in glob("scripts/*") do print(p .. " " .. stat(p).size) end
print(must(run({"git", "log", "--oneline"}, {timeout_ms = 5000, tail = 30})).out)
print(json_write(git_status(".")))
```

```text
README.md 163
...
markdown.go 24473
server.go 7728
scripts/audit.shale 2321
scripts/bench.shale 3117
scripts/check.shale 748
9819161 Journal: the prompts for Monday and Tuesday
...
[]
```

Then a look at the old surface's comments, for reference only:

```shale
-- the old surface's comments, for reference: its size, its storage, one sample file
print(count_lines("comments.py") .. " lines in comments.py")
local fs = glob("data/comments/**/*")
print(#fs .. " files under data/comments")
for p in take(fs, 5) do print(p .. " " .. stat(p).size) end
```

```text
204 lines in comments.py
47 files under data/comments
data/comments/notes/animation-gaps.md.comments.json 932
...
```

A sample file stores `"para_index": 4`, a paragraph's position, which is exactly the fragility the prompt names: insert a paragraph above, and every comment below points one paragraph off.

And a stop on my way into the Shale docs: `grep` returns `{line, text}` maps, and `print` takes scalars.

```shale
-- the Shale reference: sizes, then the headings of the agents' doc
print(count_lines("README.md") .. " lines README, " .. count_lines("docs/SHALE-AGENTS.md") .. " lines SHALE-AGENTS")
for l in grep("docs/SHALE-AGENTS.md", "## ") do print(l) end
```

```text
363 lines README, 910 lines SHALE-AGENTS
── stopped at stdin:3:55: type: `print` takes scalars, not a map
```

Yesterday's journal says it plainly: prose is for the file reader. I read the doc whole.

## What a "paragraph" is

The renderer already numbered paragraphs (`<p id="p1">`), but only real `<p>`s: a tight list item has none, and `review-queue.md` is nearly all list items. So a commentable **block** is a paragraph, a heading, a tight list item's text, a code fence or a table. Raw HTML blocks aren't, since they're the author's own markup.

The renderer got a marked mode, `RenderBlocks`, that puts `data-c="K"` on each block (a tight item's text gets a `<span class="ct">`), and returns each block's markdown source and the offset just past its element, where its comments go. `Render` is unchanged, byte for byte: all 69 of yesterday's tests passed on the first run after the edit.

```shale
-- the renderer's marked mode: each commentable block gets data-c and its source text is kept
edit("markdown.go", {
  {old = [=[...Render...]=], new = [=[...Render, Block, RenderBlocks, open, close...]=]},
  -- (six old/new pairs: Render, the renderer's fields, fence, heading, table, paragraph; the Go is in 8c05754)
})
print(run({"go", "test", "-count=1", "."}, {timeout_ms = 180000, tail = 30}).out)
```

```text
edit markdown.go: 6 hunks, -52 +121 lines
ok  	essay-surface	0.030s
```

## The first idea, measured, and why it isn't enough

The obvious design: a comment remembers its block's text; on each render it goes to the block with the same text, else the most similar block above some threshold, else it's an orphan. I wrote two measures of similarity, Dice over words-and-word-pairs and Dice over character trigrams, and measured both on the real notes before choosing anything. The test `TestCalibrateFeatures` (since removed; see below) took every block of every note and computed how similar it is to its nearest *other* block, and to itself after a typo, one word changed, a split, and a sentence added. Scores are in thousandths.

```shale
-- tidy the experiment's number formatting (strconv does it), then run it on the samples and on all real notes
edit("calibrate_test.go", {
  {old = [=[func itoa(n int) string { ... }
...]=], new = ""},
  {old = [=["path/filepath"
	"sort"]=], new = [=["path/filepath"
	"sort"
	"strconv"]=]},
  {old = "func pcts(xs []int) string {", new = "var itoa = strconv.Itoa\n\nfunc pcts(xs []int) string {"},
})
local r = run({"go", "test", "-count=1", "-run", "TestCalibrateFeatures", "-v", "."}, {timeout_ms = 180000, tail = 30, re = "words|trigrams|ok|FAIL"})
print("samples, " .. r.ms .. " ms")
print(r.out)
local real = run({"go", "test", "-count=1", "-run", "TestCalibrateFeatures", "-v", "."},
  {timeout_ms = 110000, tail = 30, re = "words|trigrams|ok|FAIL", env = {ESSAY_NOTES = "/home/steve/showell_repos/essay-repl-server/notes"}})
print("real notes, " .. real.ms .. " ms")
print(real.out)
```

```text
real notes, 23567 ms
words: nearest other block: n=23959 p1=19 p10=111 p50=186 p90=333 p99=714 p99.9=1000 max=1000
words: one typo:           n=23921 p1=250 p10=666 p50=938 p90=983 p99=1000
words: one word changed:   n=23959 p1=0 p10=571 p50=930 p90=978
words: split, better half: n=18325 p1=636 p10=653 p50=676 p90=710
trigrams: nearest other block: n=23959 p1=92 p10=202 p50=327 p90=459 p99=778
trigrams: one typo:           n=23921 p1=555 p10=823 p50=969 p90=992
...
```

The distributions overlap. One typo in a short block drops it to 250, while 1% of blocks have another block in the same note at 714 or more. No threshold separates "the same paragraph, edited" from "a different paragraph". So I simulated the decision itself: for each block, apply an edit, then ask whether the best match is the edited block, another block, or nothing.

```text
words    typo     T=600: on its block 21543, on another   77, orphaned  1272
words    deleted  T=600: on its block     0, on another  344, orphaned 22584
trigrams typo     T=600: on its block 22530, on another   68, orphaned   294
trigrams deleted  T=300: on its block     0, on another 13996, orphaned  8932
trigrams deleted  T=600: on its block     0, on another  635, orphaned 22293
```

(Abridged: four thresholds, four edits, two measures.) At any threshold low enough to keep short edited blocks, deleting a block drags its comments onto a look-alike, 1.5% to 61% of the time. Content alone can't do it.

## The design: text, then neighbors, then similarity

What content can't tell, structure can. A comment also remembers the fingerprints of the blocks **before and after** it. Then, in order:

1. **The same text** (whitespace-normalized, so rewrapping is no edit): the block with that text, the nearest one to where it was if there are twins. This covers moves and paragraphs added or removed elsewhere.
2. **Between the same two neighbors:** the block edited in place, however much. A deleted block leaves its two neighbors side by side, with nothing between them, so this never mistakes a deletion for an edit.
3. **The most similar block**, at a Dice of 600 or more: a block edited *and* moved, or a split.
4. **Else an orphan**, shown at the end of the page under "Comments on text that is gone", with the text it was made on.

And **healing**: when a comment is found by rule 2 or 3, or its neighbors have changed, it remembers the new text and neighbors, so a chain of small edits (the block, then a neighbor, then the block again) never drifts it away. `quote` (the text when the comment was made) never changes; the page shows it under "on an earlier version" whenever the block's text differs, so an imperfect placement is always visible as such.

```shale
-- the resolver: exact text, then the block between the same neighbors, then the most similar block, else orphaned
append("anchor.go", [=[
// Comment is one reader's comment, one line of a note's comments file.
...
]=])
-- (Comment, resolve, nearest, heal, newComment; the Go is in 8c05754)
print("ok")
```

I fixed a first-block bug on sight (`len(blocks) > 1` should have been `> 0`, for a one-block note) and replaced the experiment with a simulation that calls the real `resolve`, with more edits: rewrapped, insert above, split, merged, moved-and-typo'd. First run:

```text
real notes, exit 1, 1046 ms
panic: runtime error: slice bounds out of range [21:20]
	/home/steve/showell_repos/essay-surface/calibrate_test.go:114 +0x41e
```

My helper `with(bs, k)` already drops block k when given nothing to put there, and I dropped it a second time:

```shale
-- with(bs, k) with no replacement already removes block k: don't remove it twice
edit("calibrate_test.go", {
  {old = [=[		out := with(bs, k)
		out = append(out[:k], out[k+1:]...)
		return append(out, Block{Text: t}), []int{len(out)}]=], new = [=[		out := with(bs, k)
		return append(out, Block{Text: t}), []int{len(out)}]=]},
  {old = [=[		out := with(bs, k)
		return append(out[:k], out[k+1:]...), nil]=], new = [=[		return with(bs, k), nil]=]},
})
local real = run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-v", "."},
  {timeout_ms = 110000, tail = 90, re = "T=|ok|FAIL|panic|\\.go:", env = {ESSAY_NOTES = "/home/steve/showell_repos/essay-repl-server/notes"}})
print("real notes, exit " .. real.code .. ", " .. real.ms .. " ms")
print(real.out)
```

```text
edit calibrate_test.go: 2 hunks, -5 +3 lines
── stopped at stdin:12:34: index: no key "code" in the map (`has_key` asks first) (8135 steps)
── time: 1 program, 110.0 s in all; the slowest 110.0 s (100%): go test ...
```

It ran into my 110-second timeout (a killed program has no `code`; `ended` says why). Slowness is a finding, not a wait: each simulated case re-normalized, re-hashed and re-featurized the whole note, and a note of 3,000 blocks swamps the rest. The fix helps the server too. A `Block` keeps its normal form, fingerprint and features once computed, so a copy with one block edited recomputes only that block. And the threshold moved out of the search: `place` gives the best block and its score, and `resolve` decides. One search then serves every threshold.

```shale
-- a block keeps its normal form, fingerprint and features; the threshold is applied after the search
edit("markdown.go", [=[type Block struct {
	Text string
	End  int
}]=], [=[type Block struct {
	Text string
	End  int
	norm string   // normalize(Text), once prepared
	hash string   // blockHash(Text)
	feat []uint64 // features(Text), made only when a comment needs it
}]=])
edit("anchor.go", {
  -- (five pairs: resolve over indexBlocks and place; the Go is in 8c05754)
})
print(run({"go", "vet", "."}, {timeout_ms = 60000, tail = 30}).out)
```

Still a timeout on the second measure, and the reason was mine again: the cached features were written onto each case's *copy* of the block list, and thrown away with it. Precomputing them once per note:

```shale
-- precompute features once per note (copies carry them); show some deletions that land on another block
edit("calibrate_test.go", {
  {old = [=[		indexBlocks(blocks)
		step := max(1, len(blocks)/150)]=], new = [=[		indexBlocks(blocks)
		for k := range blocks {
			blocks[k].feat = features(blocks[k].Text)
		}
		step := max(1, len(blocks)/150)]=]},
  -- (and simShown, cut, and the log of examples)
})
local real = run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-v", "."},
  {timeout_ms = 110000, whole = 30000, env = {ESSAY_NOTES = "/home/steve/showell_repos/essay-repl-server/notes"}})
print("real notes, ended " .. real.ended .. ", " .. real.ms .. " ms")
for l in lines(real.out) do
  if find(l, "T=") == nil or find(l, "T=600") ~= nil then print(l) end
end
```

```text
real notes, ended exit, 49158 ms
words    T=600 typo         cases 22892: right 22892, on another block    0, orphaned    0
words    T=600 one word     cases 22928: right 22928, on another block    0, orphaned    0
words    T=600 a sentence   cases 22928: right 22928, on another block    0, orphaned    0
words    T=600 rewrapped    cases 22928: right 22928, on another block    0, orphaned    0
words    T=600 insert above cases 22928: right 22928, on another block    0, orphaned    0
words    T=600 split        cases 17667: right 17563, on another block  104, orphaned    0
words    T=600 merged       cases 22426: right 12899, on another block  218, orphaned 9309
words    T=600 moved, typo  cases 22892: right 21498, on another block  128, orphaned 1266
words    T=600 deleted      cases 22928: right 22583, on another block  345, orphaned    0
words: deleted, then on another block at T=600:
      Proofread: The beacons
        -> The beacons
      without the 6;
        -> without the 7;
trigrams T=600 split        cases 17667: right 17521, on another block  146, orphaned    0
trigrams T=600 merged       cases 22426: right 12948, on another block  419, orphaned 9059
trigrams T=600 moved, typo  cases 22892: right 22529, on another block   74, orphaned  289
trigrams T=600 deleted      cases 22928: right 22293, on another block  635, orphaned    0
trigrams: deleted, then on another block at T=600:
      The truck
        -> The truck's motion
```

Every edit in place now keeps its comments, 100%, with either measure: the neighbors rule does it. The measure matters only for compound edits. Words misplace 1.5% of deleted blocks' comments (345) against 2.8% for trigrams (635), and orphan 5.5% of moved-and-edited ones against 1.3%. The misplacements are look-alikes: "without the 6;" goes to "without the 7;". **I chose words, at 600:** a wrong placement misleads, while an orphan is honest. The comparison is committed as it ran (`8c05754`), so anyone can rerun it; after that commit the trigram code lives only in history.

The simulation was too slow for the gate: 7.6 s on twelve samples. I measured before guessing:

```shale
-- where the simulation's 7.6 s goes: a CPU profile, the top entries
mkdir(".run")
must(run({"go", "test", "-count=1", "-run", "TestSimulateAnchoring", "-cpuprofile", ".run/sim.prof", "-o", ".run/sim.test", "."}, {timeout_ms = 110000, tail = 5}))
print(must(run({"go", "tool", "pprof", "-top", "-nodecount=18", ".run/sim.test", ".run/sim.prof"}, {timeout_ms = 60000, tail = 30})).out)
```

```text
      flat  flat%   sum%        cum   cum%
     3.42s 43.51% 43.51%      3.42s 43.51%  essay-surface.dice (inline)
     0.66s  8.40% 51.91%      4.84s 61.58%  essay-surface.(*blockIndex).place
     0.09s  1.15% 69.72%      1.80s 22.90%  essay-surface.indexBlocks
```

Three changes. The gate simulates only the production measure, at 50 blocks a note (150 with `ESSAY_NOTES`). And `place` skips any block whose size alone means its Dice can't beat the best so far (`diceBound`), which helps the server too. It went from 7.6 s to 1.0 s. On the samples, the test pins its tallies exactly: an edit in place must be 100%, and the rest are pinned at what they measured, so any change to the anchoring that moves them is seen. My first pins were guesses scaled from the 150-a-note run, and they failed, as guesses should:

```text
merged: 519 cases, 6 on another block, 234 orphaned; want 4 and 163
moved, typo: 526 cases, 6 on another block, 23 orphaned; want 3 and 21
deleted: 528 cases, 8 on another block, 0 orphaned; want 6 and 0
```

They're pinned at what was measured.

## The anchoring tests, and proving they can fail

`anchor_test.go` holds the hand-written cases: unchanged, a paragraph added above, a typo fixed, rewrapped, rewritten in place, moved, moved and edited, split (each way: the comment goes with the half holding more of the old text), deleted (then restored, and it finds its paragraph again), deleted next to a look-alike, twins, first and last blocks, a one-block note, an emptied note, every kind of block, `heal`, `dice`, `normalize` against `strings.Fields`, and `hashOf` against the library's FNV-1a. And `TestAnchorChainOfEditsNeedsHealing`, which edits the block, then its neighbor, then the block again. It asserts the healed comment follows and the unhealed one is orphaned: both sides, so healing is shown to be load-bearing.

All sixteen passed on their first run, and a green is a claim, so I planted two faults, once each:

```shale
-- plant 1: no in-place rule; plant 2: heal changes nothing. Each should turn tests red; then put the code back
edit("anchor.go", "	if len(between) > 0 {\n", "	if false && len(between) > 0 {\n")
local r1 = run({"go", "test", "-count=1", "-run", "Anchor|Heal", "."}, {timeout_ms = 110000, tail = 40, re = "^--- FAIL"})
edit("anchor.go", "	if false && len(between) > 0 {\n", "	if len(between) > 0 {\n")
print("no in-place rule:")
print(r1.out)
edit("anchor.go", "	c.Anchor, c.Prev, c.Next, c.Ord = anchor, prev, next, p.block\n	return true", "	_ = prev\n	_ = next\n	_ = anchor\n	return true")
local r2 = run({"go", "test", "-count=1", "-run", "Anchor|Heal", "."}, {timeout_ms = 110000, tail = 40, re = "^--- FAIL"})
edit("anchor.go", "	_ = prev\n	_ = next\n	_ = anchor\n	return true", "	c.Anchor, c.Prev, c.Next, c.Ord = anchor, prev, next, p.block\n	return true")
print("no healing:")
print(r2.out)
print(must(run({"go", "test", "-count=1", "-run", "Anchor|Heal", "."}, {timeout_ms = 110000, tail = 5})).out)
```

```text
no in-place rule:
--- FAIL: TestAnchorTypoFixed (0.00s)
--- FAIL: TestAnchorRewrittenInPlace (0.00s)
--- FAIL: TestAnchorFirstAndLastBlocks (0.00s)
--- FAIL: TestAnchorChainOfEditsNeedsHealing (0.00s)
--- FAIL: TestSimulateAnchoring (1.41s)
no healing:
--- FAIL: TestAnchorTypoFixed (0.00s)
...
--- FAIL: TestHeal (0.00s)
--- FAIL: TestSimulateAnchoring (1.34s)
ok  	essay-surface	1.033s
```

To be honest about the second plant: it reddened more than the chain test, because `newComment` sets a new comment's neighbors through `heal`, so a dead `heal` leaves every comment with no neighbors. The chain test's unhealed side is what isolates healing.

## Storage: one JSON line a comment

A note's comments are `comments/<note>.md.jsonl`, in a `comments/` folder beside `notes/` (or `-comments DIR`). One line is one comment, in the order made:

```json
{"id":"3f9c1a0b2d4e","time":"2026-11-13T16:35:12Z","name":"Ann","text":"I doubt this & that.","ord":2,"prev":"1x2k9q","next":"8hd03k","quote":"Second paragraph, the one with a claim.","anchor":"Second paragraph, the one with a clam."}
```

Why this format:

- **A line is a comment**, so `git diff` of the file reads as a list of comments added, or moved (an `anchor` line changed). Nothing else is in the file: no header, no index, no derived state.
- **The reader-facing text comes first** (`name`, `text`), and the long `quote` last. `anchor` is written only when the text has moved on from `quote`.
- **The file is the standard library's JSON**, with `<` and `&` kept as written (no `<`). It's read strictly: an unknown field, a broken line, or a comment without an id or quote is a 500 that names `file:line`, never a skipped line. A hand edit's mistake is seen.
- **The file is written whole, then renamed into place**, so a reader never sees half a file. One write at a time, under a lock.

I considered one JSON document a note, like the old surface. It diffs as well but makes every comment a multi-line object, and the old surface's `para_index` is what I set out to get rid of. A markdown-like format would read nicer, but it needs escaping rules of my own for a comment that contains the format's markers.

## The server and the front end

- A note's page is made with each block's comments spliced in right after its element, and the orphans at the end. It's kept until the note **or its comments file** changes (two stats a request), so a git pull of comments shows up on the next request.
- `POST /notes/{name}/comments` takes JSON: `{block, hash, name, text}`. The page carries every block's fingerprint, so a page loaded before an edit can still comment on a paragraph that moved (found by fingerprint), but gets a 409 ("reload") for one that changed. It takes JSON only, which a form on another site can't send without asking first. A name is at most 80 bytes and one line, a text at most 10,000 bytes, and both are required.
- A GET can write the comments file: when a note's text has changed, healing rewrites the comments that moved. It happens once per note edit, never on an unchanged note (a test checks the file's modified time).
- `comments.js` and `comments.css` are plain files, embedded with `embed` and inlined into the page. On a wide screen, each block gets a quiet round button in the right margin that shows on hover, or always (with its count) once the block has comments. On a narrow screen or a touch screen, it's a faint inline `+` after the block's text. It opens a name-and-comment form under the block; the name is remembered in `localStorage`, and a failure is wrapped in try/catch. Comments render as text (`textContent`) on the client and HTML-escaped on the server.

Tests (`comments_test.go`): post and read back, escaping, two comments forming one thread in order, every refusal (415, 400 times seven, 409, 404 twice) and exactly-at-the-limits, a stale page (moved: 201, changed: 409), edits that heal and a delete that orphans with the file keeping it, a comments file changed on disk, a broken one (500 with `a.md.jsonl:2:`, and a post doesn't overwrite it), the file round trip, 40 concurrent posts with 40 concurrent reads (all 40 kept, all on the page), and `TestThreadFollowsEveryKindOfBlock`, which pins the contract `comments.js` relies on: a block's thread is the element right after it.

The first full run turned two tests red, and both were right to. One was mine: the page's heading now carries `data-c="0"`, so the old test's expected `<h1>` changed with the page. The other was a test bug: I checked that a page had no orphans by looking for the word `orphans`, and the inlined CSS has `.orphans`. Each now looks for the element.

```shale
-- the anchoring tests, first run
local r = run({"go", "test", "-count=1", "-run", "Anchor|Heal|Dice", "-v", "."}, {timeout_ms = 110000, tail = 60, not_re = "^=== RUN|T=[0-9]|^ +(`|->|After|🍃|\\*\\[|##)"})
print(r.out)
```

```text
./anchor_test.go:189:42: undefined: itoa
```

(`itoa` went with the experiment.) Then yesterday's gate, `scripts/check.shale`, reused as is, stopped on gofmt (an aligned comment), and I fixed that in the next call. Then:

```shale
-- a green is a claim: how many ran; and the race detector over the server and comment tests
local r = must(run({"go", "test", "-count=1", "-v", "."}, {timeout_ms = 180000, whole = 400000}))
print("PASS lines: " .. #find_all(r.out, "--- PASS") .. ", FAIL lines: " .. #find_all(r.out, "--- FAIL") .. ", top-level: " .. #re_lines(r.out, "^=== RUN   Test\\w+$"))
local race = run({"go", "test", "-count=1", "-race", "-run", "Comment|Post|Note|Index|Edited", "."}, {timeout_ms = 110000, tail = 30})
print("race: exit " .. race.code .. ", " .. race.ms .. " ms")
print(race.out)
```

```text
PASS lines: 95, FAIL lines: 0, top-level: 32
race: exit 0, 59250 ms
ok  	essay-surface	1.080s
```

The 59 s was building the race runtime from nothing; the tests took 1.1 s. A clean race run over tests that send one request at a time proves little, so I added the concurrent test and ran it under `-race`: clean, 3.4 s.

## Measuring: bench.shale, improved

Yesterday's `bench.shale` had the flaw yesterday's audit had fixed: its `must`s ran while the server was up, so any failure would leave it running. Shale's `attempt` catches a stop. So now everything the server is up for runs in one function under `attempt`, the server is stopped, and only then does the script report or fail. It also posts comments now (`per=` on every note, 3 by default, and `big=` on the largest, 100) into `.run/bench/comments`, which is made fresh each run, and times the same pages again. Its first run:

```text
── stopped at scripts/bench.shale:152:1: store: `remove_tree(".run/bench")`: not found (FileNotFound): `exists(p)` asks first (133 steps)
```

Fixed with `if exists(RUN) then remove_tree(RUN) end`. On the samples it ran, but the comparison wasn't fair. A note's page is now made on its first request, and the "no comments" pass included those first builds (p90 2.0 ms). Both sides get a warm-up pass now. On all real notes:

```text
── budgets from the script's header: steps=60000000, reads=20000
2026/11/13 16:35:24 rendered 504 notes from /home/steve/showell_repos/essay-repl-server/notes in 638ms; comments in .run/bench/comments
── stopped at scripts/bench.shale:140:5 (in `measure`): budget kept: what the run keeps at a `yield` is past the kept budget (1048576 bytes)
```

**This one left the server running.** A budget stop is the one thing `attempt` can't catch. I stopped it by its record:

```shale
-- a budget stop is past attempt: is the server still up? stop it
print(job_alive(".run/bench.log.job"))
job_stop(".run/bench.log.job")
print(job_alive(".run/bench.log.job"))
```

```text
true
false
```

The lesson for scripts that hold a server: their budget header must leave room. The posts list, about 450 KB, is kept across each `yield`. The header now says `kept_bytes=16000000`.

### A slowdown found on the way

Start-up rendered 504 notes in 638 ms, against 173 ms yesterday. I checked the box before believing anything:

```shale
-- is the box busy? load and the busiest processes
print(must(run({"uptime"}, {timeout_ms = 5000, tail = 3})).out)
local rows = {}
for p in procs() do
  if p.name == "go" or p.name == "essay-surface" or p.name == "zig" or p.name == "roc" or p.name == "cc1" then push(rows, {pid = p.pid, name = p.name, args = sub(p.args, 1, 80)}) end
end
print(#rows .. " build or server processes")
if #rows > 0 then print(table(rows, {"pid", "name", "args"})) end
```

```text
 16:36:05 up 81 days, 20:04,  3 users,  load average: 1.98, 1.54, 0.88
2 build or server processes
2462578  essay-surface  /tmp/claude-1000/essay-preview/essay-surface -notes /home/steve/showell_repos/es
2470029  roc            /home/steve/tools/roc-nightly-2026-11-03-34bd006/roc test --verbose gates/Widget
```

Load 2 on two vCPUs, from other work: a `roc test`, and an `essay-surface` from another session's preview folder. That one isn't mine, so I left it alone; it's on its own port. So today I compare within a run, never with yesterday. Within one run, on the largest note, `Render` took 3.39 ms, `RenderBlocks` 3.71 ms (+9%) and `RenderBlocks` plus indexing 10.33 ms. The profile:

```text
         0     0%     0%      700ms 63.06%  essay-surface.indexBlocks
         0     0%     0%      640ms 57.66%  essay-surface.normalize
     110ms  9.91%  9.91%      490ms 44.14%  strings.Fields
         0     0%  9.91%      310ms 27.93%  essay-surface.blockHash
```

`strings.Fields` built a list of every word only for `Join` to glue them back, and `blockHash` normalized the same text a second time. The fixes:

- `normalize` copies word spans in one pass (a test holds it equal to `Fields` and `Join`, NBSP and U+2028 included).
- The fingerprint hashes the normal form it's given.
- FNV-1a runs inline instead of allocating a byte copy (a test holds it equal to `hash/fnv`).
- A text match is a fingerprint match confirmed by comparing the text, so there's one map, not two.
- Fences and tables build their source text only when marked, so plain `Render` pays nothing for comments.

Five runs each, same box:

```text
BenchmarkRenderBlocksLargest-2: 2699 2785 2802 2864 3019 us
BenchmarkRenderIndexLargest-2: 4804 5205 5361 5739 5830 us
BenchmarkRenderLargest-2: 2111 2114 2133 2417 2712 us
 16:37:53 up 81 days, 20:06,  3 users,  load average: 2.08, 1.67, 1.00
```

Indexing went from 6.6 ms over the marked render to about 2.4 ms. It's paid once per note change, never per request, so I stopped there.

### The numbers

All 504 real notes, 1,609 comments posted (3 on each note, 100 on the largest), curl's clock over one kept-alive connection, two runs, on a box at load 2:

```text
rounds=200
notes, no comments: 504 requests, 0 not 200, 22353 bytes average; median 0.158 ms, p90 0.334 ms, p99 2.080 ms, max 2.623 ms
notes, with comments: 504 requests, 0 not 200, 23150 bytes average; median 0.169 ms, p90 0.284 ms, p99 0.773 ms, max 1.323 ms
largest note, no comments: 200 requests, 0 not 200, 199171 bytes average; median 0.233 ms, p90 0.725 ms, p99 2.297 ms, max 2.732 ms
largest note, 100 comments: 200 requests, 0 not 200, 224175 bytes average; median 0.316 ms, p90 0.495 ms, p99 0.742 ms, max 1.327 ms
rounds=500
notes, no comments: 504 requests, 0 not 200, 22353 bytes average; median 0.202 ms, p90 0.266 ms, p99 0.370 ms, max 0.976 ms
notes, with comments: 504 requests, 0 not 200, 23150 bytes average; median 0.173 ms, p90 0.282 ms, p99 1.751 ms, max 2.580 ms
largest note, no comments: 500 requests, 0 not 200, 199171 bytes average; median 0.307 ms, p90 0.390 ms, p99 0.450 ms, max 1.130 ms
largest note, 100 comments: 500 requests, 0 not 200, 224175 bytes average; median 0.293 ms, p90 0.726 ms, p99 2.669 ms, max 4.971 ms
```

The first run's 0.233 against 0.316 for the largest note reversed in the second (0.307 against 0.293). The difference is inside this box's noise, and the tails swing both ways between runs. That's what the design predicts: a page with comments is a kept page, like one without.

## Auditing: audit.shale, improved

Yesterday's audit got three changes. It points the server at `.run/comments`, because otherwise the server looks for a comments folder beside the real notes. It cuts the page's `<script>` and `<style>` before counting stray markdown. And it checks the new structure: as many `data-c` blocks as fingerprints in `#blocks`. First run:

```text
── stopped at scripts/audit.shale:65:3: budget kept: what the run keeps at a `yield` is past the kept budget (1048576 bytes) (72090794 steps)
```

The same trap twice in one day. Its steps were nearly out too (72M of 80M at page 420). I raised both, with the reason written in the header. The server was already stopped here, by yesterday's design. Second run:

```text
504 pages, 23959 commentable blocks, 3 pages with leftovers
blocks  mismatch  note
    32         1  essay-surface-monday.md
    16         1  essay-surface-quality.md
...
── stopped at scripts/audit.shale:75:3: fail: 504 pages whose data-c count is not their fingerprint count
```

Every page off by exactly one, the same way: that's my counting, not the renderer. `comments.js` mentions `data-c="K"` in its header comment, so I count blocks with the scripts cut. Third run:

```text
504 pages, 23959 commentable blocks, 3 pages with leftovers
stars  links  tables  ticks  note
    0      0       0      3  essay-surface-monday.md
    0      0       0      3  essay-surface-quality.md
    0      0       0      2  update-53-and-the-checklists-first-day.md
```

Two of those are yesterday's faithful pair. `essay-surface-monday.md` is new in the real folder today (yesterday's journal, published), so I looked before calling it faithful. My first try used `type()`, which isn't in the language:

```text
── stopped at stdin:6:6: refused: `type` is not in this language (0 steps)
```

```shale
-- the new note's stray backticks: the rendered lines that hold one outside <code>
for m in grep(".run/pages/essay-surface-monday.md.html", "`") do
  if find(m.text, "<pre") == nil and find(m.text, "<script") == nil then print(m.line .. ": " .. sub(m.text, 1, 240)) end
end
```

```text
98: <p id="p4" data-c="17"><strong>The audit found two bugs that 62 renderer tests missed.</strong> Titles kept their backticks in 35 notes, and a lone ``` in a table cell swallowed the rest of the row.
```

Faithful: the prose quotes a lone ```` ``` ````, which CommonMark renders literally.

## The whole flow, live: comments_smoke.shale

No browser is on the box (`which` finds only `node`), so I can't eye-test the page or a phone. What I can do is run the flow through the real binary over HTTP. The new script `scripts/comments_smoke.shale` writes a note of its own, posts a comment, then fixes a typo and adds a paragraph above it **on disk while the server runs**, then deletes the paragraph. At each step it checks the page and the file. It stops the server under the same `attempt` pattern.

```text
server stopped
ok: 4 blocks: the title and three paragraphs
ok: a comment is posted (201)
ok: the comment follows its paragraph
ok: its text is escaped on the page
ok: the file keeps the text as written
ok: after a typo and a paragraph above, it follows
ok: and says it was made on an earlier version
ok: the file is healed to the new text
ok: a deleted paragraph's comment is with the orphans
ok: and the file still has it
```

## check.shale, improved

`go test` embeds `comments.js` but never parses it, so yesterday's gate couldn't see a broken front end. It now runs `node --check comments.js`, with a `want` saying why, since `run` refuses an interpreter without one. I planted a syntax error once:

```shale
-- plant once: a syntax error in comments.js must turn the js stage red; then put it back
edit("comments.js", "  \"use strict\";\n", "  \"use strict\";\n  var broken = ;\n")
local r = run({"node", "--check", "comments.js"}, {timeout_ms = 30000, tail = 6, want = "check: planting a syntax error once"})
edit("comments.js", "  \"use strict\";\n  var broken = ;\n", "  \"use strict\";\n")
print("planted: exit " .. r.code)
print(r.out)
print("restored: exit " .. run({"node", "--check", "comments.js"}, {timeout_ms = 30000, tail = 6, want = "check: the restored file"}).code)
```

```text
planted: exit 1
SyntaxError: Unexpected token ';'
restored: exit 0
```

```text
gofmt: clean
vet: clean, 338 ms
js: comments.js parses
test: ok  	essay-surface	1.049s
```

## The final anchoring numbers

From the final code, on every real note, 150 blocks a note (`ESSAY_NOTES=... go test -run TestSimulateAnchoring -v`, 15.5 s). The counts are the same as before the speedups: the anchoring is deterministic, and the optimizations changed no decision.

Last, nothing of mine left up:

```shale
-- is anything of mine still up? every job record, and every essay-surface process with where it runs from
for j in {".run/bench.log.job", ".run/server.log.job", ".run/smoke.log.job"} do
  if exists(j) then print(j .. " alive: " .. (job_alive(j) and "yes" or "no")) end
end
for p in procs() do
  if p.name == "essay-surface" then print("essay-surface pid " .. p.pid .. ": " .. sub(p.args, 1, 120)) end
end
print(json_write(git_status(".")))
```

```text
.run/server.log.job alive: no
.run/smoke.log.job alive: no
essay-surface pid 2462578: /tmp/claude-1000/essay-preview/essay-surface -notes /home/steve/showell_repos/essay-repl-server/notes -addr 127.0.0.1:93
[]
```

(The bench's record went with `.run/` when the audit cleared that folder; the bench had said "server stopped". The one process is the other session's preview.)

## The scripts

- `scripts/check.shale` (improved): gofmt, vet, **`node --check comments.js`**, every test, including the anchoring simulation pinned on the samples.
- `scripts/bench.shale` (improved): **with and without comments**, the same pages; everything the server is up for runs under `attempt`; comments go to a fresh `.run/bench/comments`; `per=`, `big=`, `rounds=`, `dir=`.
- `scripts/audit.shale` (improved): **`data-c` against the fingerprints** on every page; scripts and styles cut; a scratch comments folder.
- `scripts/comments_smoke.shale` (new): the comment flow through the real server, editing the note on disk under it.

## Report

**How comments stay attached.** A comment remembers three things: the text of its block, normalized so rewrapping is no edit; the fingerprints of the blocks just before and after it; and the text it was first made on, which never changes. On each render, a comment goes to:

1. the block with the same text (nearest to where it was, if there are twins), which covers moves and edits elsewhere;
2. else the block that still sits between the same two neighbors, which is the paragraph edited in place, however much (a deleted block leaves its neighbors touching, so this never mistakes a deletion for an edit);
3. else the most similar block, with word-and-word-pair Dice at 0.6 or more, which covers a paragraph edited and moved, or split;
4. else it's an orphan, shown at the foot of the page under "Comments on text that is gone", with the text it was on. A comment is never dropped from its file.

When a comment is found by rule 2 or 3, it remembers the new text and neighbors ("heals"), so a run of small edits never drifts it away. Whenever its block's text differs from what it was made on, the page offers "on an earlier version" with the original, so even an imperfect placement is visible.

**Storage:** `comments/<note>.md.jsonl` beside `notes/`, one JSON object a comment, a line each, in the order made. A line per comment makes git's diff a list of comments, and nothing derived is stored. It's read strictly (a bad line is an error naming `file:line`) and written whole with a rename.

**What I measured** (all 504 real notes, this droplet, which was at load 2 all afternoon from other work, so absolute times run about 1.7 times yesterday's):

| page | median | p90 | p99 |
|---|---|---|---|
| a note, no comments (504, 500 rounds run) | 0.202 ms | 0.266 ms | 0.370 ms |
| a note, 3 comments each | 0.173 ms | 0.282 ms | 1.751 ms |
| largest note (199 KB), no comments | 0.307 ms | 0.390 ms | 0.450 ms |
| largest note, 100 comments (224 KB) | 0.293 ms | 0.726 ms | 2.669 ms |

A page with comments is as fast as one without, within this box's noise (the 200-round run had the medians the other way round). The cost of comments is paid on the first request after the note or its comments change. In process, placing 100 comments and making the page took 0.48 ms, and indexing the largest note's blocks about 2.4 ms. A post through the real server, the file rewrite included, took about 0.7 ms (1,609 in 1.0 to 1.2 s).

Anchoring, simulated on 22,900 real blocks (150 a note): every edit in place, whether a typo, a changed word, an added sentence, a rewrap or a paragraph added above, kept its comments 100% of the time. Split: 99.4% right, 0.6% on another block. Moved and edited: 93.9% right, 0.6% wrong, 5.5% orphaned. Deleted: 98.5% orphaned as they should be, and 1.5% (345) went to a look-alike ("without the 6;" goes to "without the 7;"). Merged into the next block (not in your list): 57.5% right, 41.5% orphaned, 1.0% wrong. The audit found 23,959 commentable blocks on 504 pages, every one consistent.

**What I'd want checked every time from now on:**

1. `shale scripts/check.shale` before each commit. It's the gate, and it now parses the JS, and it pins the anchoring simulation on the samples, so any change to the anchoring shows its new numbers.
2. `shale scripts/comments_smoke.shale`: the real server, the real file, an edit on disk underneath it. One second.
3. `shale scripts/audit.shale dir=<real notes>`: rendering leftovers, and every block marked once.
4. When the anchoring changes, `ESSAY_NOTES=<real notes> go test -run TestSimulateAnchoring -v`, checked against the table above. Especially the "deleted, then on another block" count, which is the one failure that misleads.
5. `shale scripts/bench.shale dir=<real notes> rounds=500`, both sides, on a quiet box (`uptime` first; today's wasn't).
6. An eye test on a phone and a desktop browser. I couldn't: there's no browser on this box, and the server is localhost only. From your laptop, `ssh -L 9300:127.0.0.1:9300` and open a note.

**Calls I made that you may want to unmake:**

- A paragraph rewritten wholesale in place keeps its comments, marked "on an earlier version". Deleting it and writing a new one in its place looks the same to the server.
- A GET writes the comments file when a note's text has changed and a comment healed. That's once per edit, never on an unchanged note.
- The comment CSS and JS are inline in every note page: about 8 KB, so pages average 22.4 KB against 13.7 KB yesterday. Serving them as one cached file is the fix if pages leave localhost.
- No login, as asked. POSTs must be JSON, which a form on another site can't send without asking first.

# Demo: one note in the new format, shown and replied to

The prompt is `journal/prompts/demo.md`: write one note in the new format, with a few paragraphs, a list, a code block and a real dot diagram, and make the surface show it as a page that takes replies, like any other note, while the old notes render exactly as they do now. Find out where Roc runs and say why, and what that costs later. Change the types only where the demo pushes back, and name each change. Keep the fast check green and add a check for the new path. Don't release.

## Orienting

I read the prompt, the last journal (`journal/format-types.md`) with Steve's reply, the proposal in essay-repl-server, the README, `server.go`, `store.go`, `frontend.go`, the first part of `markdown.go`, `comments.js`, the two checks and the repo prelude, and roc-apps' `CLAUDE.md` and `docs/roc-style.md`.

My first Shale stumbled three times on `run`'s required options: a run with neither `timeout_ms` nor a keep stops before it starts, and each stop names the one that's missing, one per launch. I read `SHALE-AGENTS.md` then, and set both on every `run` after.

Two things I looked for before deciding anything:

```shale
for p in git_ls_files("") do
  for m in grep(p, "render_with_roc") do print(p .. ":" .. m.line, sub(m.text, 1, 200)) end
end
local r = run({"sh", "-c", "command -v dot; command -v node; ls -la ~/showell_repos/essay-repl-server/assets/viz-standalone.js; date -u"}, {want = "probe: graphviz, node, the vendored viz.js and the time", timeout_ms = 5000, tail = 20})
```

```text
essays.py:45	def render_with_roc(source):
ops/build_renderer.sh:2	# Build bin/render_note, the program behind essays.render_with_roc, with the
/usr/bin/dot
/usr/bin/node
-rw-rw-r-- 1 steve steve 1390377 Sep 10 12:20 .../essay-repl-server/assets/viz-standalone.js
Sat Nov 14 14:03:09 UTC 2026
```

The first matters most. essay-repl-server already runs Roc in production, in one way: `renderer/render_note.roc` is a headerless app on Roc's built-in echo platform, built with `--opt=dev` into `bin/render_note`, and the Python server runs it once per note with the note's text as its arguments, cut into 32,000-character parts because Linux stops one argument at 128 KiB.

## The first toolchain probe

Before writing any of the format, a headerless app at the pin that joins its arguments and prints a record as JSON:

```roc
main! = |args| {
	joined = Str.join_with(args, "|")
	echo!(Str.inspect(joined))
	echo!(Json.to_str({ b: "x\"y\n", a: [1, 2] }))
	Ok({})
}
```

```shale
local roc = real("tools:roc-nightly-2026-11-03-34bd006/roc")
local r = run({roc, "build", ".run/probe/probe.roc", "--opt=dev", "--output=.run/probe/probe"}, {timeout_ms = 90000, tail = 40})
local p = run({real(".run/probe/probe"), "a b", "c\"d"}, {timeout_ms = 5000, tail = 20, want = "probe: the scratch build"})
```

```text
build	0	131	ms
0 errors and 0 warnings found in 116ms while successfully building:
run	0	0
"a b|c\"d"{"a":[1.0,2.0],"b":"x\"y\n"}
```

It built in 131 ms and ran. `Json.to_str` is in the pin's builtins and escapes a string as JSON does, so the program can answer the server in JSON without an escaper of its own. An untyped number prints as `1.0`, so every number in the answer needs its type said.

## Where Roc runs: on the server, as a program of its own

I chose the arrangement essay-repl-server already runs, and nothing new: `format/render_note.roc` is a headerless app on the built-in platform, built with `--opt=dev` into `render-note` beside the server's binary. The store runs it once each time a `.note` changes, with the note's text as its arguments in parts of at most 32,000 bytes cut at a character's start, and reads the page it prints as JSON (`note.go`). The page is kept, as a markdown note's is, so a request never runs Roc.

Why this one:

- **There is no host to build.** The built-in platform comes with the compiler. A platform of our own means a Zig host built against roc's `builtins` from a roc checkout, and the checkout on this box is from 09-16 while the pin is 11-03, so the host would be built against the wrong builtins or need a second checkout.
- **It's proven here.** essay-repl-server serves its notes this way today, and its build script, argument parts and failure handling were a pattern to follow, not a design to make.
- **The seam is one JSON value.** `Page.to_json` uses the builtin `Json.to_str`, so neither side writes an escaper, and Go decodes it with unknown fields refused.
- **The store's rule holds.** A note is rendered when it changes, never on a request, so a process per render costs nothing a reader waits on. The demo note renders in about 2 ms.

What it costs later:

- **Arguments are the only way in.** The built-in platform gives no stdin and no file reads, so a note is bounded by Linux's whole argument space (`getconf ARG_MAX` says 4,194,304 bytes here, shared with the environment). The largest real note, `gopher-roc-demo.md`, is 167 KB, so that's twenty times the room, but it's a ceiling, and a note past it fails with `E2BIG`, shown on its page.
- **Two things to deploy.** The server needs `render-note` beside it. `release.shale` builds only the Go binary, so it needs a step that builds `render-note` into `bin/` before a released server can show a `.note`. I didn't add it, because I can't run a release to prove it.
- **A process per note at start.** With hundreds of `.note` files the start would spend a second or two in renders. That's fine, but it's the first thing to measure if most notes move.
- **The browser can't run it.** Widgets, or a live preview as you write, want Roc in the page, which is wasm and a JS host: a different arrangement. The types and `Parse`/`Page` would move unchanged, since they're pure. Only `render_note.roc`, 18 lines, is this platform's.

## The Roc

The format's code is two modules beside the types, and the app:

- **`Parse.roc`** reads a `.note` into a `Note`, or gives the first line it can't read and why (a `Problem` of a line and a `Fault` tag). Its header is the syntax. It is the markdown the notes are written in now, cut down to what the types hold: no escapes, a blank line before every block, styles holding only words and code.
- **`Page.roc`** turns a `Note` into the page: escaped HTML marked `data-c="K"` as `markdown.go` marks its blocks, and for each block the text a reply anchors to (the block as a reader reads it, with no markup) and the byte just past its element. A refused note becomes a page that says the problem and has no blocks.
- **`render_note.roc`** joins its arguments, parses, renders and prints.

The opaque types got their makers and readers: `Phrase.from_pieces`, `Text.from_spans`, `NoteName.from_str`, `WebAddress.from_str` and `Diagram.from_source`, each with the checks its doc promised, and `is_eq : _`. `Table`, `ImageFile` and `Widget` still have none, because the demo's syntax doesn't write them, so no note can hold one, and `Page` crashes on them with that reason.

The compiler stopped me four times, each time on my own Roc. `Parse.roc`'s first check had three errors: `List.take_while` doesn't exist at the pin, and twice a record pattern in a `match` on a `Try` lacked its `..` (`Ok({ rest, .. })`). `Text.roc`'s first test run had the fourth. An assignment can't be a match arm's whole body: `Strong(_) => $after_unstyled = False` is a `Bool`-typed branch beside a `{}` one, so the loop in `Text.from_spans` asks each span whether it is unstyled instead. Once they compiled, the demo note rendered right on the first run.

### Changes to the types

None of the ten types changed shape. The demo pushed back once, on what a `NoteName` link means. The renderer can't know whether `arrays-that-are-values` is a `.md` or a `.note`, which is what the proposal said ("whether such a note exists is the server's to know"). So a link is written as `/notes/<name>`, and the server answers that with a redirect to the `.note` if there is one, else the `.md` (`redirectToNote`). That's a server route, not a type change.

## The Go

- **`note.go`** runs the renderer and reads its answer. A note that can't be rendered, whether the renderer is missing, crashes or answers something that isn't a page, gets a page that says why and has no blocks. The server logs it and serves everything else.
- **`store.go`** picks the renderer by the extension. A markdown note goes through exactly the code it did before.
- **`server.go`** takes `.note` names, serves the redirect and two scripts, and adds a `.note`'s style and scripts to its page only. A markdown note's page is byte for byte what it was.
- **`assets/`** holds viz.js, copied from essay-repl-server, and `diagrams.js`, which draws each `figure.dot` with essay-repl-server's defaults. The figure keeps its `data-c` mark, so a diagram takes replies as a code block does.

## The new check, `scripts/note_check.shale`

It's a stage of `fast.shale` now. In order:

- **The Roc:** `roc fmt --check format`; `roc check format/render_note.roc` with no ✗ and no ●; `roc test` with every expect in `format/` passing, the count held to the `expect` lines in the files.
- **The renderer:** `render-note` built, every `.note` in `notes/` rendered, and each diagram drawn by the page's own viz.js and defaults under node (`scripts/draw_diagrams.cjs`).
- **The server:** it runs on 9310 with the demo note, a markdown note and a note that doesn't parse. Replies are posted to the diagram, a subpoint and a quote's paragraph, and the note is edited on disk under them. Then the link by name is checked, and viz.js served whole. The broken note's page names the line and the fault, and has no blocks, and the markdown note is as it was.

The Go tests (`note_test.go`) use the test binary as a fake renderer: it echoes its arguments as a page, fails, or answers junk. That covers a note too big for one argument arriving whole, a reply on a `.note`, the failures staying on their own page, the redirect, and the scripts' ETags.

My first run of the check stopped at its first line: `format/Page.roc failed roc fmt --check`. I'd appended Page's tests after the last `roc fmt`. So the check caught my own slip before it caught anything else.

The plants, each put back before the next:

```text
plant 1, each end one byte short: exit 1
   ── ✗ fail ──────────────────── format/Page.roc:371:1
       3 failed
plant 2, no scripts for one diagram: exit 1
   fail: the flow stopped: not so: it loads viz.js for its diagram (line 16)
plant 3, an expect nothing runs: exit 1
   fail: roc test: want all 61 expects in format/ to pass
plant 4, a diagram viz.js can't draw: exit 1
   1 of 1 diagrams not drawn
after: exit 0
```

Plant 3 was a `format/Stray.roc` that nothing imports, with one `expect`. `roc test` from the app said all 60 passed, which is true and is exactly the blind spot the count closes.

Shale's Roc hunts over `format/` found five names (`escaped`, three list parameters named as one thing, and a `label` field holding pieces) and 13 groups to stack. I renamed them, ran `stack.shale write=1`, and broke up four lines by hand that had nothing to stack. The hunts are clean now, apart from 18 `wide string` lines, which no gate holds. They have to run from `format/`, since the recipe looks for `roc-version` where it starts. They're not in the check.

```text
ok  gofmt, 24 ms, cpu 37 ms
ok  vet, 181 ms, cpu 309 ms
ok  js, 37 ms, cpu 37 ms
ok  test, 1858 ms, cpu 2063 ms: 58 tests passed, 63 subtests
ok  smoke, 301 ms, cpu 322 ms: 10 checks through the real server
ok  note, 1099 ms, cpu 1277 ms: 25 checks, from the Roc to the page
fast: PASS
```

## The full check, and a red that isn't this change's

`full.shale` passed every stage but the audit, which stopped on `shale-book-part-two.md (3 ticks)`, a leftover not known to be faithful. That note came into essay-repl-server at 13:25 today. To be sure it wasn't mine, I ran the audit from a worktree of HEAD, before any of this change, and it stopped the same way on the same note. Whether its three ticks are faithful is a call for whoever reads that page. I left `FAITHFUL_TICKS` alone. The bench, the dates, the race run and the anchoring simulation all passed.

## Time

The prompt came in at about 14:01. The first probe of the toolchain ran at 14:03, the Roc was written and rendering the demo by 14:13, and the server showed it at 14:14. The check and its plants were done by 14:19. The toolchain took about three minutes in all: two probes and three compile errors. It never fought.

## The release

The orchestrator, not the agent, shipped it. Two things blocked it.

- **The release script built only the Go binary,** so a released server would have shown every `.note` as a page that can't be shown. `release.shale` now builds `render-note` with the pinned Roc, renders the demo note with it before touching anything, and moves it with its server through every swap (8deea74). It was proved on dev first: a release, a rollback and a roll-forward, with the renderer following its server each way.
- **The full check's audit was red** on an essay note with a bare code fence in a sentence. The fix was the sentence, not an entry in the audit's list of faithful pages.

Released to the dogfood surface as 8deea74, with 17 comments where they were. The demo note was then copied in for the first eye test.

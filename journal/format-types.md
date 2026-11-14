# Format types: the Roc types for a note format of our own

The prompt is `journal/prompts/format-types.md`: read the real notes, then propose the Roc types for a note in a new format (its blocks, the text inside them, a diagram block drawn from a dot description, room for widgets later), say for each type what it's for and what it leaves out, count what the real notes use that the format would drop, and name the files. No Roc functions and no Go. The report is a note Steve replies to.

## Orienting

My first Shale launch was a shell habit: `run(["ls", ...])`. Shale lists are braces, not brackets, and it stopped before a step:

```text
── stopped at stdin:1:5: syntax: expected a value, found `[` (0 steps)
```

My second was `help`, as a bare word, which stopped on the name. `shale --help` named the docs, and I read `SHALE-AGENTS.md` before going on. Then the repo:

```shale
for p in glob("*") do print(p) end
for p in glob("journal/**/*") do print(p) end
local ns = glob("notes/**/*")
print(#ns)
for p in ns do print(count_lines(p), p) end
```

```text
12
222	notes/arrays-that-are-values.md
...
5154	notes/gopher-roc-demo.md
...
3197	notes/safari-tutorial-draft.md
```

The repo's `notes/` holds 12 notes, about 11,000 lines. Monday's journal says they're a sample copied from `essay-repl-server/notes/`, and the links journal counted the real notes there (514 then). So "my real notes in `notes/`" can mean either. I read the 12 here as the sample the prompt points at, and counted every feature over the whole folder in essay-repl-server, which is what Steve actually writes:

```shale
for l in git_log("", {n = 12}) do print(l.hash, l.date, l.subject) end
print(json_write(git_status("")))
local t = 0
for p in git_ls_files("") do if starts_with(p, "notes/") then t = t + 1 end end
print("tracked notes/: " .. t)
local ns = glob("repos:essay-repl-server/notes/*.md")
print("essay-repl-server notes: " .. #ns)
```

```text
4e93f6d	2026-11-14T13:40:41+00:00	journal: Part Two's first prompt, types for a format of our own
...
[]
tracked notes/: 12
essay-repl-server notes: 522
```

A first try at the git log of `notes/` stopped on `git_log(..., {fields = nil})`: a map can't hold nil. I ran `git log` through `run` instead, which showed the 12 came in with the first commit.

I read `README.md`, `markdown.go` (whole), `journal/links.md`, roc-apps `CLAUDE.md` and `docs/roc-style.md` with the Read tool.

## Reading the notes

I read seven of the twelve samples whole (`arrays-that-are-values`, `basic-machine-design`, `three-repos-one-library`, `the-on-ramp`, `opening-game-tie-ins-2`, `roc-bird-stills`, `sunset-eye-test`) and the heads of the rest (`review-queue`, `dog-physics-and-the-report`, `gopher-roc-demo`). What they hold beyond paragraphs: an italic line under the title that says when and for whom, bold-led paragraphs and bullets, tables, `dot` fences, a `raw` fence of HTML, images beside the note, four-space indented commands, links to other notes.

`essay-surface` draws none of the dot: nothing in `comments.js` or the Go mentions it. essay-repl-server does, so I read its renderer too:

```shale
for p in {"comments.js", "frontend.go", "server.go", "comments.css"} do
  for k in {"dot", "graphviz", "viz", "raw", "mermaid", "language-"} do
    for m in grep(p, k) do print(p .. ":" .. m.line, k, sub(m.text, 1, 160)) end
  end
end
```

That found nothing. In essay-repl-server, `renderer/render_note.roc` passes a `raw` fence through as HTML and leaves every other fence as code, and `assets/dot.js` turns each `code.language-dot` into an SVG in the browser with viz.js.

## Counting what the real notes use

Twelve notes are too few to count by, so I wrote `scripts/features.shale`: it walks a folder's notes line by line, tracks fences as `markdown.go` does, and counts each feature by the notes that use it, the times, and the first place. `show=<feature>` prints every place. It's kept because the counts in the report are the argument, and a reply that questions one can be answered by running it again.

The first version used `{0,3}` and `{1,6}`, which Shale's regular expressions refuse. I took them out with an `edit` and a `re_replace` over the script before it ever ran. It ran on the 12 samples, then on essay-repl-server's 522 notes (91,472 lines, 4.8 MB):

```text
feature                     notes  times  first
paragraph                     522   9164  2026-11-01-a-day-of-judges.md:3
heading h1                    519    526  2026-11-01-a-day-of-judges.md:1
code_span                     509  12113  ...
strong_star                   483   8737  ...
...
```

Images counted as links of an unknown kind (`link_other`), since an image's `](` looks like a link's. I fixed that before reading the numbers. Then I added the nesting checks (bold inside italics, a link inside bold, code inside each) and the run went over its step budget at 40 s. Each regex search costs steps a byte, so I put each pattern behind a cheap `find` of its literal. That brought it to 19.5 s with the extra checks in. That's slow for a loop, but this is a survey run a handful of times, not a gate, so I left it.

Then I made a mistake that cost two minutes. To see examples of six features I launched the survey six times, once per `show=`, which took 117 s in all. One run that printed all six would have taken 20 s.

The examples corrected two readings:

- **The five "link inside italics" lines are all subtitles**, like `*2026-11-05. A short look at [Pixelsprout/rocco-engine](...)...*`. So a subtitle with its own type keeps them.
- **"Two deep" nested lists** also matched lines of a diff inside an indented code block. 9 notes is a ceiling, not a count.

Three counts the survey lacked, done by hand. The first was the notes that open with an italic paragraph under the title, wrapped or not. The survey's single-line test had found 168:

```shale
local paras = split(read(p), "\n\n")
if #paras >= 2 and starts_with(paras[1], "# ") then
  local q = trim(paras[2])
  if starts_with(q, "*") and not starts_with(q, "**") and ends_with(q, "*") then ...
```

```text
296 notes open with an italic paragraph under the title, 152 of them wrapped over lines
```

Next, headings that hold a style or a link. A first scan without fences counted 126 styled h2s, but every one of them was a Roc `##` doc comment inside a code fence in `gopher-roc-demo.md`. So I added the check to the survey, which skips fences: 7 notes (h2 in 4, h3 in 3), and no title.

Last, numbered sub-lists and images inside a sentence. A combined scan ran out of steps, because its image pattern started with `[^ ].*`. A `grep` for `![` was cheap. Every image not at a line's start was inside code, except two in a list item in `odin-webgl-spike.md`. Numbered sub-lists are in 4 notes (`gates-reorg`, `gopher-as-a-platform`, `roc-chat-host-design`, `shapes-first-principles`). The fifth hit was a wrapped line that starts `506.`.

## The types

They're in `format/`, one module per type, beside a `roc-version` that pins the box's nightly. They're in a folder of their own because the Go renderer stays at the root for the old notes, and the new format's parser and renderer will join these types there. Each file is only a type and its `##` doc. There are no functions, so the opaque types can't be built yet. Their makers come with the parser.

The first check was a single file, to see if the pin takes an opaque type with a nested alias and no functions:

```shale
local r = run({real("tools:roc-nightly-2026-11-03-34bd006/roc"), "check", "format/Phrase.roc"}, {timeout_ms = 60000, tail = 40})
```

```text
exit 0, 99 ms
No errors found in 88ms for format/Phrase.roc
```

Then all ten, each checked by name. `Note.roc` imports the rest, so its check reaches them all. Every check said `No errors found` in a few milliseconds, which is fast enough to doubt. So I planted two faults under `--undo-on-stop`, each put back before the next:

```shale
edit("format/Block.roc", "\tTable(Table),", "\tTable(Tabel),")
local r = run({roc, "check", "format/Note.roc"}, {timeout_ms = 60000, tail = 20})
edit("format/Block.roc", "\tTable(Tabel),", "\tTable(Table),")
...
edit("format/Text.roc", "target : Text.Target", "target : Text.Destination")
```

```text
plant 1, Tabel in Block: exit 1
── ✗ undeclared type ───────────────────────────────────── format/Block.roc:22:8
plant 2, Text.Destination: exit 1
── ✗ missing nested type ───────────────────────────────── format/Text.roc:19:35
after: exit 0: No errors found in 7ms for format/Note.roc
fmt --check: exit 0
All formatting valid.
```

So the check from `Note.roc` does read the nested types of the modules it imports.

The name, layout and error hunts from Shale's `cookbook/roc/` found five things on their first pass:

```text
format/Block.roc:19: container: the field `items` is only container words ...
format/Block.roc:43: container: the type `Item` is only container words ...
format/Table.roc:9: singular_list: the field `header` holds `List(Text)` but reads as one thing ...
format/Table.roc:8: unstack: 7 tokens in Table :: { header : List(Text), rows : List(List(Text)) }.{}
format/Widget.roc:11: unstack: 4 tokens in Widget :: { name : Str, description : Text }.{}
```

A list's item became a `Point`, with `subpoints`. A numbered list's `first` became `first_number`, because `first` read as its first point. The header became `column_headings`, and the two two-field records went onto one line each. My edit of `Table.roc` stopped on a doc line I'd quoted wrong (the sentence wraps differently), and nothing in that file was written. I ran it again with the right text. After that the check, `roc fmt --check` and the four hunts were all clean: name_hunt, plain_names, stack, and errors.

## Time

By the files' times: the prompt was committed at 13:40:41 and this journal started at 13:41:47. The first type file is from 13:51:39 and the last from 13:52:59. So about 12 minutes went to reading the docs and the notes and counting, and about 2 to writing and checking the types. Most of the design was settled while the counts came in, and the report took longer than either.

## The report, and the commit

The report is `essay-repl-server/notes/essay-format-types.md`: the types with what each is for and what it leaves out, the table of what is dropped by notes, the file name, and three questions. Writing it, I went back over each count against the survey's output rather than my memory of it. The orchestrator publishes it; I don't commit it.

The README names `scripts/features.shale` and `format/` in two sentences. The commit holds `format/` (ten type modules and `roc-version`), `scripts/features.shale`, the README and this journal.

## Steve's reply

Steve read the proposal on the surface, along with the orchestrator's short read of it. He replied:

> Looks good! The honest truth is that I jumped right to the file with the Roc types. I skimmed everything, especially the prose. The Roc types look solid enough to start building out a quick demo with.

He accepted the orchestrator's three recommendations: Note and Block stay nominal, a list point holds one paragraph, and the divider stays.

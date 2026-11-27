# scripts

Each script's header says the rest. Run them from the repo root, `shale scripts/NAME.shale`.

- `check_before_commit.shale`: run before every commit. gofmt, vet, the JS, the tests, the comment flow, the book review flow and the `.note` path, in a few seconds.
- `check_before_release.shale`: run before anything ships. Everything the commit check does, the race detector, and the anchoring, rendering, dates and speed on all the real notes; its last line is `SAFE TO SHIP` or `NOT SAFE`.
- `release.shale`: swaps the running server for a new build, checks every comment is still in place, and puts the old server back if any check fails; `rollback=1` swaps back.
- `check_rendered_notes.shale`: every note rendered through the real server, checked for markdown left over and for its comment markup. A stage of the release check.
- `measure_server.shale`: the server timed by real requests, held to CPU budgets. A stage of the release check.
- `check_note_dates.shale`: a fresh clone must give every note the same date. A stage of the release check, on the sample notes.
- `prune_notes.shale`: keeps the newest 20 notes in a data folder and removes the rest, with their comments and any image only they named, because notes on the surface are transient. Run it from the folder that holds the data folder after writing a note: `shale repos:essay-surface/scripts/prune_notes.shale data=DATA`. Its self-test is a stage of the commit check.
- `check_note.shale`: refuses a `.note` that doesn't parse, or whose diagram can't be read at a glance: wider than the 640px column, taller than one screen (600px), or past 10 nodes, 12 edges, 2 clusters or a 24-character edge label. One PASS or FAIL line a diagram, with what it measured. Run it after writing a note: `shale repos:essay-surface/scripts/check_note.shale data=DATA note=NAME.note`. Its self-test is a stage of the commit check.
- `check_comment_flow.shale`: the comment flow end to end over HTTP. A stage of the commit check.
- `check_book_review_flow.shale`: the book review end to end over HTTP: a chapter served with its widget, replies with stars and without, bad ones refused, the file read back and `read_book_replies.shale` run on it. A stage of the commit check.
- `read_book_replies.shale`: prints a book's replies by chapter, each with its time, name, stars and text: `shale repos:essay-surface/scripts/read_book_replies.shale book=cook-levin` (`data=` another data folder than `essays:`, `page=` one chapter).
- `check_note_format.shale`: the `.note` path, from the Roc to the page. A stage of the commit check.
- `count_markdown_features.shale`: counts which markdown features a folder of notes uses. Run by hand.
- `draw_diagrams.cjs`: draws dot diagrams with viz.js under node, for `check_note_format.shale`.
- `measure_diagrams.cjs`: draws dot diagrams the same way and prints each one's size and counts, for `check_note.shale`.

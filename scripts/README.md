# scripts

Each script's header says the rest. Run them from the repo root, `shale scripts/NAME.shale`.

- `check_before_commit.shale`: run before every commit. gofmt, vet, the JS, the tests, the comment flow and the `.note` path, in a few seconds.
- `check_before_release.shale`: run before anything ships. Everything the commit check does, the race detector, and the anchoring, rendering, dates and speed on all the real notes; its last line is `SAFE TO SHIP` or `NOT SAFE`.
- `release.shale`: swaps the running server for a new build, checks every comment is still in place, and puts the old server back if not; `rollback=1` swaps back.
- `check_rendered_notes.shale`: every note rendered through the real server, checked for markdown left over and for its comment markup. A stage of the release check.
- `measure_server.shale`: the server timed by real requests, held to CPU budgets. A stage of the release check.
- `check_note_dates.shale`: a fresh clone must give every note the same date. A stage of the release check.
- `check_comment_flow.shale`: the comment flow end to end over HTTP. A stage of the commit check.
- `check_note_format.shale`: the `.note` path, from the Roc to the page. A stage of the commit check.
- `count_markdown_features.shale`: counts which markdown features a folder of notes uses. Run by hand.
- `draw_diagrams.cjs`: draws dot diagrams with viz.js under node, for `check_note_format.shale`.

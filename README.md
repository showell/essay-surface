# essay-surface

A small web app for writing notes in markdown and reading them in a browser, with comments on any paragraph. A Go server, a JavaScript front end.

Phones are out of scope: the pages are made for, and checked in, a desktop browser.

Notes are transient. A live data folder's notes are never committed anywhere, and `scripts/prune_notes.shale` keeps only the newest 20, removing the rest with their comments. A note that drafts something durable is promoted to its own repo before it rolls off. The notes in this repo's `notes/` are samples for the tests.

Two checks, run from the repo root:

- `shale scripts/check_before_commit.shale` after every edit: gofmt, vet, the JS, the tests, the comment flow through the real server, and the `.note` path (`scripts/check_note_format.shale`). A few seconds.
- `shale scripts/check_before_release.shale` before anything ships: everything `check_before_commit` checks, the race detector, and the rendering and the speed on the live notes, and the anchoring and the dates on the sample notes. About half a minute; its last line is `SAFE TO SHIP` or `NOT SAFE`.

A release is `scripts/release.shale`, from the folder that holds the data folder (`bin/essay-surface`, `notes/`, `comments/`): `cd /home/steve && shale repos:essay-surface/scripts/release.shale data=essay-dogfood port=9301`. It runs `check_before_release`, builds beside the old binary, copies the comments to `releases/<when>/`, swaps the servers and checks every comment is still where it was, and puts the old server back if not. `rollback=1` swaps back to the last. Its header says the rest. The checks' own servers listen on 9310, so a check runs beside a server on 9300.

The stages are scripts of their own in `scripts/`, listed in its README: `check_rendered_notes`, `measure_server` and `check_note_dates` take `dir=` for another folder of notes, and `check_comment_flow` makes its own. `count_markdown_features` counts what a folder of notes uses (`dir=`), for deciding what the new format keeps.

`format/` holds the surface's own note format (`.note` files) in Roc, with `format/roc-version` naming the compiler: the types, `Parse.roc` (the syntax, in its header), `Page.roc` (the HTML and the blocks that take replies), and `render_note.roc`, a headerless app on Roc's built-in platform that prints a note's page as JSON. The server runs it once each time a `.note` changes (`note.go`), from `render-note` beside its own binary unless `-note-renderer` names another. `scripts/check_note_format.shale` builds it at the root (`roc build format/render_note.roc --opt=dev --output=render-note`), checks and tests the Roc, draws each diagram with the page's viz.js under node, and runs a `.note` through the real server. `release.shale` builds `render-note` beside the server and moves it with its server through every swap.

A `.note` shows a picture with a line of its own, `![description](file.png)`: the file sits beside the note in its folder, and the description is the picture's alt text and its caption. A reply to the picture is a reply like any other, so a sign-off goes on the picture itself. The server answers `GET /notes/<file>` with the picture only for a name `format/ImageFile.roc` allows (a note's name, then `.png`, `.jpg` or `.svg`): no folder, no `..`, no hidden file, no link, no other extension. It sends it with `Cache-Control: no-store`, so a picture redrawn under its name shows on the next load. A markdown note's `![alt](file.png)` is served the same way.

Pitfall: a `.note` that doesn't parse, or a renderer that fails, makes that note's page say why, and the rest are served as usual; the server logs it. A picture's name the format refuses (`frames/a.png`, `a.gif`, `Shot 1.png`) is such a problem, so rename the file to fit.

Pitfall: a reply anchors to a picture's caption and its file name, not its pixels. A picture redrawn under the same name keeps its sign-off, so a new version of a picture gets a new name (`frame-001.png`), and the old sign-off then shows as made on an earlier version, or among the comments on text that is gone.

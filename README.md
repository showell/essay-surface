# essay-surface

A small web app for writing notes in markdown and reading them in a browser, with comments on any paragraph. A Go server, a JavaScript front end.

Phones are out of scope: the pages are made for, and checked in, a desktop browser.

Two checks, run from the repo root:

- `shale scripts/check_before_commit.shale` after every edit: gofmt, vet, the JS, the tests, the comment flow through the real server, and the `.note` path (`scripts/check_note_format.shale`). A few seconds.
- `shale scripts/check_before_release.shale` before anything ships: everything `check_before_commit` checks, the race detector, and the anchoring, the rendering, the dates and the speed on all the real notes. About half a minute; its last line is `SAFE TO SHIP` or `NOT SAFE`.

A release is `scripts/release.shale`, from the folder that holds the data folder (`bin/essay-surface`, `notes/`, `comments/`): `cd /home/steve && shale repos:essay-surface/scripts/release.shale data=essay-dogfood port=9301`. It runs `check_before_release`, builds beside the old binary, copies the comments to `releases/<when>/`, swaps the servers and checks every comment is still where it was, and puts the old server back if not. `rollback=1` swaps back to the last. Its header says the rest. The checks' own servers listen on 9310, so a check runs beside a server on 9300.

The stages are scripts of their own in `scripts/`, listed in its README: `check_rendered_notes`, `measure_server` and `check_note_dates` take `dir=` for another folder of notes, and `check_comment_flow` makes its own. `count_markdown_features` counts what a folder of notes uses (`dir=`), for deciding what the new format keeps.

`format/` holds the surface's own note format (`.note` files) in Roc, with `format/roc-version` naming the compiler: the types, `Parse.roc` (the syntax, in its header), `Page.roc` (the HTML and the blocks that take replies), and `render_note.roc`, a headerless app on Roc's built-in platform that prints a note's page as JSON. The server runs it once each time a `.note` changes (`note.go`), from `render-note` beside its own binary unless `-note-renderer` names another. `scripts/check_note_format.shale` builds it at the root (`roc build format/render_note.roc --opt=dev --output=render-note`), checks and tests the Roc, draws each diagram with the page's viz.js under node, and runs a `.note` through the real server. `release.shale` builds `render-note` beside the server and moves it with its server through every swap.

Pitfall: a `.note` that doesn't parse, or a renderer that fails, makes that note's page say why, and the rest are served as usual; the server logs it.

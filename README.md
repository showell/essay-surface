# essay-surface

A small web app for writing notes in markdown and reading them in a browser, with comments on any paragraph. A Go server, a JavaScript front end.

Phones are out of scope: the pages are made for, and checked in, a desktop browser.

Two checks, run from the repo root:

- `shale scripts/fast.shale` after every edit: gofmt, vet, the JS, the tests, the comment flow through the real server. A few seconds.
- `shale scripts/full.shale` before anything ships: the fast check, the race detector, and the anchoring, the rendering, the dates and the speed on all the real notes. About half a minute; its last line is `SAFE TO SHIP` or `NOT SAFE`.

A release is `scripts/release.shale`, from the folder that holds the data folder (`bin/essay-surface`, `notes/`, `comments/`): `cd /home/steve && shale repos:essay-surface/scripts/release.shale data=essay-dogfood port=9301`. It runs the full check, builds beside the old binary, copies the comments to `releases/<when>/`, swaps the servers and checks every comment is still where it was, and puts the old server back if not. `rollback=1` swaps back to the last. Its header says the rest. The checks' own servers listen on 9310, so a check runs beside a server on 9300.

The stages are scripts of their own in `scripts/`: `audit`, `bench` and `dates` take `dir=` for another folder of notes, and `comments_smoke` makes its own. `features` counts what a folder of notes uses (`dir=`), for deciding what the new format keeps.

`format/` holds the Roc types proposed for a note format of the surface's own (`.note` files), with `format/roc-version` naming the compiler. They are types only so far: `roc check format/Note.roc` reaches them all.

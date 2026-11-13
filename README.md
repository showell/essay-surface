# essay-surface

A small web app for writing notes in markdown and reading them in a browser, with comments on any paragraph. A Go server, a JavaScript front end.

Two checks, run from the repo root:

- `shale scripts/fast.shale` after every edit: gofmt, vet, the JS, the tests, the comment flow through the real server. A few seconds.
- `shale scripts/full.shale` before anything ships: the fast check, the race detector, and the anchoring, the rendering, the dates and the speed on all the real notes. About half a minute; its last line is `SAFE TO SHIP` or `NOT SAFE`.

The stages are scripts of their own in `scripts/`: `audit`, `bench` and `dates` take `dir=` for another folder of notes, and `comments_smoke` makes its own.

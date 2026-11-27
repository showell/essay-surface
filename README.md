# essay-surface

A small web app for writing notes in markdown and reading them in a browser, with comments on any paragraph. A Go server, a JavaScript front end.

The code here was written by AI coding agents (Claude), with human supervision.
Point an agent at this repo to extend it.

The pages are made for, and checked in, a desktop browser.

The scripts are [Shale](https://github.com/showell/shale), a small scripting language for working on code.

Notes are transient. A live data folder's notes stay out of every repo, and `scripts/prune_notes.shale` keeps only the newest 20, removing the rest with their comments. A note that drafts something durable is promoted to its own repo before it rolls off. The notes in this repo's `notes/` are samples for the tests.

Two checks, run from the repo root:

- `shale scripts/check_before_commit.shale` after every edit: gofmt, vet, the JS, the tests, the comment flow through the real server, and the `.note` path (`scripts/check_note_format.shale`). A few seconds.
- `shale scripts/check_before_release.shale` before anything ships: everything `check_before_commit` checks, the race detector, and the rendering and the speed on the live notes, and the anchoring and the dates on the sample notes. About half a minute; its last line is `SAFE TO SHIP` or `NOT SAFE`.

A release is `scripts/release.shale`, given the data folder (`bin/essay-surface`, `notes/`, `comments/`) as a Shale path, a mount's name (`essays:`) or a folder below the launch folder or a mount: `shale repos:essay-surface/scripts/release.shale data=DATA port=PORT`, launched from anywhere but the home folder, which Shale refuses as a run's root. It runs `check_before_release`, builds beside the old binary, copies the comments to `releases/<when>/`, swaps the servers, checks every comment is still where it was, and puts the old server back if any check fails. `rollback=1` swaps back to the last. Its header says the rest. The checks' own servers listen on 9310, so a check runs beside a server on 9300.

The stages are scripts of their own in `scripts/`, listed in its README: `check_rendered_notes`, `measure_server` and `check_note_dates` take `dir=` for another folder of notes, and `check_comment_flow` makes its own. `count_markdown_features` counts what a folder of notes uses (`dir=`), for deciding what the new format keeps.

`format/` holds the surface's own note format (`.note` files) in Roc, with `format/roc-version` naming the compiler: the types, `Parse.roc` (the syntax, in its header), `Page.roc` (the HTML and the blocks that take replies), and `render_note.roc`, a headerless app on Roc's built-in platform that prints a note's page as JSON. The server runs it once each time a `.note` changes (`note.go`), from `render-note` beside its own binary or the program `-note-renderer` names. `scripts/check_note_format.shale` builds it at the root (`roc build format/render_note.roc --opt=dev --output=render-note`), checks and tests the Roc, draws each diagram with the page's viz.js under node, and runs a `.note` through the real server. `release.shale` builds `render-note` beside the server and moves it with its server through every swap.

**Accounts** (`accounts.go`): with `-accounts <folder>`, every page and every comment needs a log-in. `/signup` takes a username and a password on an ordinary form, so the browser offers to generate a strong password and saves it; it is open only until the first account exists. `/login` and `/logout` do the rest, and a session lasts a year. A logged-in reply is signed by its account, whatever name it sends, and the reply form takes the name from `/whoami`; over loopback the form asks for a name. The folder holds `users.json` (PBKDF2-SHA256 hashes of the passwords) and `session_key`, which the server makes on first use; deleting it logs everyone out. A failed log-in waits a second. A request over loopback is trusted, so the scripts here read the server directly. Without the folder the server listens only on loopback: any other `-addr` stops it at the start. `release.shale` starts it on `0.0.0.0` with its accounts when the data folder holds an `accounts` folder. It is plain HTTP, light on purpose: the notes are private, not secret.

Pitfall: because loopback is trusted, never put a proxy on the same box in front of the server; every visitor would look local.

A `.note` shows a picture with a line of its own, `![description](file.png)`: the file sits beside the note in its folder, and the description is the picture's alt text and its caption. A reply to the picture is a reply like any other, so a sign-off goes on the picture itself. The server answers `GET /notes/<file>` with the picture only for a name `format/ImageFile.roc` allows (a note's name, then `.png`, `.jpg` or `.svg`): no folder, no `..`, no hidden file, no link, no other extension. It sends it with `Cache-Control: no-store`, so a picture redrawn under its name shows on the next load. A markdown note's `![alt](file.png)` is served the same way.

A table on either kind of note sorts by a click on a column's heading, and a second click reverses it (`tables.js`, tested by `tables_test.cjs` in the commit check). A column whose every filled cell is a number or a date sorts as one, and any other sorts as text. The order lives only in the page: the table is one commentable block, so its replies stay with it whatever order its rows show.

Pitfall: a `.note` that doesn't parse, or a renderer that fails, makes that note's page say why, and the rest are served as usual; the server logs it. A picture's name the format refuses (`frames/a.png`, `a.gif`, `Shot 1.png`) is such a problem, so rename the file to fit.

Pitfall: a reply anchors to a picture's caption and its file name, not its pixels. A picture redrawn under the same name keeps its sign-off, so a new version of a picture gets a new name (`frame-001.png`), and the old sign-off then shows as made on an earlier version, or among the comments on text that is gone.
## Books under review

The server can also show a book the book builder staged (roc-apps' `book_builder/`), so its reader can reply to each chapter as they read. It is for review on this server only: the books' published copies never carry it.

- **Which books.** `-books FILE` names them, one a line, `<book> <absolute folder>`, the folder the book's build wrote (the dev channel's `<book>/`); `#` lines and blank ones are skipped. `release.shale` passes `-books <data>/books.txt` when the data folder holds that file. A book not in the file is not served, and adding one takes a restart (a release).
- **Reading.** `/books/` lists them, and `/books/<book>/` is the book's first page. The pages come from the folder as they are, behind the accounts like every other page. A path is plain names only, opened through `os.Root`, so neither `..` nor a link leads out of the book's folder, and only a page, a stylesheet, a script, a picture or a few data types are sent.
- **Replying.** Each page at the top of the book gets, as it is sent, a small Reply button in the bottom right corner. It opens a box with a text area and Send, so Tab then Enter sends the reply to the whole chapter. The page's Next link (`a[rel="next"]`) asks for the chapter's stars before it leaves, naming the chapter: a star, or a key from 1 to 5, posts the stars alone and goes on; Skip, or Enter, goes on with no opinion given; Escape stays. The button, its box and its script carry `data-essay-book-review` (`books.go`'s `bookReviewMark`), and no file of the book holds them, so a published book can't carry them; roc-apps' publish scripts look for the mark.
- **Storing.** `POST /books/<book>/replies` appends one line to `book-replies/<book>.jsonl`, beside `notes/` and `comments/` in the data folder (`-book-replies` names another): `id`, `time`, `book`, `page` (the chapter's file without `.html`), `title` (its first heading, read from the page when the reply comes, not from the browser), `stars` (1 to 5, or `null`), `text` and `name`. A logged-in reply is signed by its account. A reply needs a text or stars; stars outside 1 to 5, a chapter the book lacks and an unknown field are refused.
- **Why one file a book, in its own folder.** A book's chapters are renamed and reordered as it is rewritten, so a reply keeps its chapter's slug and title as they were, and the book's whole review reads from one file, in order. They are kept out of `comments/` because `prune_notes.shale` removes every comment file whose note is gone, and a book is no note. `release.shale` copies `book-replies/` into each release's folder with the comments.
- **Acting on them.** `shale repos:essay-surface/scripts/read_book_replies.shale book=<book>` prints a book's replies by chapter.

Pitfall: a book's page runs on this server's origin, so its own scripts could post here as the reader. The books are our own builds, written by the same box's scripts that write the notes, so they get the notes' trust; never list a book built from someone else's text.

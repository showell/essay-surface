We're building a new essay surface: a small web app where I write notes in markdown, read them in a browser, and let readers comment on any paragraph. Today we start it.

Write a Go server, using only the standard library, that serves every `notes/*.md` file as an HTML page at `/notes/<name>.md`, with an index of all the notes at `/`, newest first. Render the markdown in Go for now: headings, paragraphs, lists, code blocks, inline code, emphasis, links and tables. Pages should look clean and readable on a laptop and a phone.

Speed matters to me. A note should come back in a few milliseconds, and the index too, even with five hundred notes. Measure it, and tell me the numbers.

Copy a dozen of my real notes into `notes/` as sample data. Write Go tests for the renderer, and commit as you go. When you're done, tell me how to run it, what you measured, and anything you'd do differently tomorrow.

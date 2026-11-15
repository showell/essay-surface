package main

// The comment front end, plain CSS and JavaScript with no build step,
// kept in their own files and put inline in each note's page.

import _ "embed"

//go:embed comments.css
var commentCSS string

//go:embed comments.js
var commentJS string

// tablesJS makes every table's rows sortable by a click on its heading.
//
//go:embed tables.js
var tablesJS string

var (
	commentStyle  = "<style>\n" + commentCSS + "</style>\n"
	commentScript = "<script>\n" + commentJS + "</script>\n"
	tablesScript  = "<script>\n" + tablesJS + "</script>\n"
)

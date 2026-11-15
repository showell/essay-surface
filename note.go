package main

// Notes in the surface's own format, .note files. The Go server doesn't read
// them: the Roc program format/render_note.roc does, built as render-note
// beside the server, and this runs it on a note's text and takes the page it
// prints as JSON. Each block a reader can reply to comes back marked
// data-c="K" with its end in the body, as RenderBlocks gives a markdown note's,
// so the store, the comments and the page's script treat both alike.

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"
)

// The kinds of note, by the file's extension.
const (
	markdownExt = ".md"
	formatExt   = ".note"
)

// renderArgBytes is the most of a note's text one argument carries. Linux
// refuses an argument over 128 KiB; the parts are cut at a character's start.
const renderArgBytes = 32000

// renderTimeout bounds one render. A note renders in a few milliseconds.
const renderTimeout = 10 * time.Second

// renderedNote is what render-note prints (format/Page.roc's to_json): the
// title as escaped text, the body as HTML, each block a reply can attach to,
// and how many diagrams the body holds.
type renderedNote struct {
	Title  string `json:"title"`
	Body   string `json:"body"`
	Blocks []struct {
		Text string `json:"text"`
		End  int    `json:"end"`
	} `json:"blocks"`
	Diagrams int `json:"diagrams"`
}

// renderNote runs the renderer on a note's text and reads its page.
func renderNote(renderer, src string) (*renderedNote, error) {
	if renderer == "" {
		return nil, fmt.Errorf("no renderer for .note files: pass -note-renderer")
	}
	ctx, cancel := context.WithTimeout(context.Background(), renderTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, renderer, renderParts(src)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%s: %v: %s", renderer, err, strings.TrimSpace(stderr.String()))
	}
	return readRendering(out)
}

// renderParts cuts a note's text into arguments of at most renderArgBytes,
// never inside a character.
func renderParts(src string) []string {
	parts := []string{}
	for len(src) > renderArgBytes {
		cut := renderArgBytes
		for cut > 0 && !utf8.RuneStart(src[cut]) {
			cut--
		}
		parts = append(parts, src[:cut])
		src = src[cut:]
	}
	return append(parts, src)
}

// readRendering reads the renderer's JSON, and refuses a page whose blocks
// don't end in order inside its body: the store would cut the body there.
func readRendering(out []byte) (*renderedNote, error) {
	var r renderedNote
	dec := json.NewDecoder(bytes.NewReader(out))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("the renderer's answer is not a page: %v", err)
	}
	at := 0
	for k, b := range r.Blocks {
		if b.End < at || b.End > len(r.Body) {
			return nil, fmt.Errorf("the renderer's block %d ends at %d, outside %d..%d", k, b.End, at, len(r.Body))
		}
		at = b.End
	}
	return &r, nil
}

// loadFormat fills in a .note's title, body and blocks from its rendering. A
// note the renderer can't render is shown as a page that says why, with no
// blocks, so one broken note never stops the rest being served.
func (n *note) loadFormat(renderer, text string) {
	n.style = formatStyle
	r, err := renderNote(renderer, text)
	if err != nil {
		log.Printf("%s: %v", n.name, err)
		n.title = html.EscapeString(strings.TrimSuffix(n.name, formatExt))
		n.body = `<p class="note-problem">This note can't be shown: ` + html.EscapeString(err.Error()) + "</p>\n"
		return
	}
	n.title = r.Title
	if n.title == "" {
		n.title = html.EscapeString(strings.TrimSuffix(n.name, formatExt))
	}
	n.body = r.Body
	for _, b := range r.Blocks {
		n.blocks = append(n.blocks, Block{Text: b.Text, End: b.End})
	}
	if r.Diagrams > 0 {
		n.scripts = diagramScripts
	}
}

// What a .note's page adds to a markdown note's: the subtitle's look, a
// diagram's frame, a picture scaled to the column with its caption under it,
// and the line that says a note can't be shown.
const formatStyle = `<style>
.subtitle{color:var(--muted);font-style:italic}
figure.dot{margin:0 0 1rem;overflow-x:auto}
figure.dot svg{max-width:100%;height:auto}
figure.image{margin:0 0 1rem}
figure.image img{display:block;max-width:100%;height:auto;border:1px solid var(--rule)}
figure.image figcaption{color:var(--muted);font:.9rem system-ui,sans-serif;margin-top:.4rem}
.dot-error,.note-problem{color:#b3261e;font:.9rem system-ui,sans-serif}
</style>
`

// viz.js draws a diagram from its dot source in the browser: the copy
// essay-repl-server serves, so a diagram looks as it does there.
//
//go:embed assets/viz-standalone.js
var vizJS []byte

//go:embed assets/diagrams.js
var diagramsJS []byte

const diagramScripts = `<script src="/assets/viz-standalone.js"></script>
<script src="/assets/diagrams.js"></script>
`

// serveAsset serves an embedded script, which changes only with the server:
// a browser keeps it and asks again by its ETag.
func serveAsset(content []byte) http.HandlerFunc {
	sum := sha256.Sum256(content)
	etag := `"` + hex.EncodeToString(sum[:8]) + `"`
	return func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("ETag", etag)
		h.Set("Cache-Control", "no-cache")
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		h.Set("Content-Type", "text/javascript; charset=utf-8")
		w.Write(content)
	}
}

package main

// The server: the handlers and the pages. The store (store.go) keeps every
// page made, so a request is a drain of the kernel's news and a write.

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
)

// validName is a note's file name: one path element ending in .md, or in
// .note for a note in the surface's own format.
func validName(name string) bool {
	return (strings.HasSuffix(name, markdownExt) || strings.HasSuffix(name, formatExt)) &&
		!strings.ContainsAny(name, "/\\") && !strings.HasPrefix(name, ".")
}

// noteName is a note's name with no extension, as a link in a .note names
// it: lowercase letters, digits and hyphens (format/NoteName.roc).
func noteName(name string) bool {
	if name == "" || name[0] == '-' {
		return false
	}
	for k := 0; k < len(name); k++ {
		if c := name[k]; !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

// imageTypes are the pictures a note may show, by extension, each with the
// type it is served as.
var imageTypes = map[string]string{".png": "image/png", ".jpg": "image/jpeg", ".svg": "image/svg+xml"}

// imageType is a picture's content type, if its file name is one a note may
// show (format/ImageFile.roc): a note's name, then .png, .jpg or .svg. So no
// folder, no "..", no hidden file and no other kind of file is served.
func imageType(name string) (string, bool) {
	ext := filepath.Ext(name)
	typ, ok := imageTypes[ext]
	if !ok || !noteName(strings.TrimSuffix(name, ext)) {
		return "", false
	}
	return typ, true
}

// serveImage sends a picture from the notes folder. It opens the file without
// following a link, and sends only a regular file, so a name can't reach a
// file outside the folder; the open doesn't wait, so a pipe by a picture's
// name can't hold the request. The browser keeps no copy, so a picture redrawn
// under its name shows on the next load. The policy stops an SVG opened by
// itself from running a script on the surface's origin.
func (s *store) serveImage(w http.ResponseWriter, r *http.Request, name, typ string) {
	f, err := os.OpenFile(filepath.Join(s.dir, name), os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	h := w.Header()
	h.Set("Content-Type", typ)
	h.Set("Content-Length", strconv.FormatInt(info.Size(), 10))
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; sandbox")
	if _, err := io.Copy(w, f); err != nil {
		log.Printf("%s: %v", name, err)
	}
}

func (s *store) serveIndex(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	page, err := s.indexPage()
	if err != nil {
		http.Error(w, "cannot read the notes folder: "+err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	writePage(w, page, start)
}

func (s *store) serveNote(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := strings.TrimPrefix(r.URL.Path, "/notes/")
	if noteName(name) {
		s.redirectToNote(w, r, name)
		return
	}
	if typ, ok := imageType(name); ok {
		s.serveImage(w, r, name, typ)
		return
	}
	if !validName(name) {
		http.NotFound(w, r)
		return
	}
	n, err := s.page(name)
	if err != nil {
		// A comments file that does not read is said, not skipped.
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	if n == nil {
		http.NotFound(w, r)
		return
	}
	writePage(w, n.page, start)
}

// redirectToNote sends a link to a note by its name to its file: the .note
// if there is one, else the .md. A .note's links name notes this way, since
// the note that writes the link can't know which kind the other is.
func (s *store) redirectToNote(w http.ResponseWriter, r *http.Request, name string) {
	for _, ext := range []string{formatExt, markdownExt} {
		n, err := s.note(name + ext)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			log.Print(err)
			return
		}
		if n != nil {
			http.Redirect(w, r, "/notes/"+name+ext, http.StatusFound)
			return
		}
	}
	http.NotFound(w, r)
}

// commentRequest is what the page posts: the block by its number and its
// fingerprint, as the page had them, and the comment.
type commentRequest struct {
	Block int    `json:"block"`
	Hash  string `json:"hash"`
	Name  string `json:"name"`
	Text  string `json:"text"`
}

// postComment adds a comment to a block. The fingerprint must match the
// block now at that number, or another block of the note: a page loaded
// before an edit can still comment on a paragraph that moved, but not on
// one that changed.
func (s *store) postComment(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if !validName(name) {
		http.NotFound(w, r)
		return
	}
	// JSON only: a form from another site cannot send it without asking first.
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "comments are posted as JSON", http.StatusUnsupportedMediaType)
		return
	}
	var req commentRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "not a comment: "+err.Error(), http.StatusBadRequest)
		return
	}
	// A logged-in reply is signed by its account, whatever name it sends.
	if user := accountOf(r); user != "" {
		req.Name = user
	}
	cname, ctext, err := cleanComment(req.Name, req.Text)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	n, err := s.note(name)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	if n == nil {
		http.NotFound(w, r)
		return
	}
	k := req.Block
	if k < 0 || k >= len(n.blocks) || n.blocks[k].hash != req.Hash {
		var at []int
		for j, b := range n.blocks {
			if b.hash == req.Hash {
				at = append(at, j)
			}
		}
		if len(at) == 0 {
			http.Error(w, "that paragraph has changed since the page loaded: reload to comment on it", http.StatusConflict)
			return
		}
		k = nearest(at, req.Block)
	}
	s.write.Lock()
	defer s.write.Unlock()
	path := commentsPath(s.comments, name)
	cs, err := readComments(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Print(err)
		return
	}
	c := newComment(n.blocks, k, newID(), time.Now().UTC().Format(time.RFC3339), cname, ctext)
	if err := writeComments(path, append(cs, c)); err != nil {
		http.Error(w, "the comment was not saved", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{"id": c.ID, "time": c.Time, "name": c.Name, "text": c.Text, "block": k})
}

// writePage sends a page with the server's own time in Server-Timing, so a
// measurement can tell the server's part from the network's.
func writePage(w http.ResponseWriter, page []byte, start time.Time) {
	h := w.Header()
	h.Set("Content-Type", "text/html; charset=utf-8")
	h.Set("Content-Length", strconv.Itoa(len(page)))
	h.Set("Server-Timing", fmt.Sprintf("app;dur=%.3f", float64(time.Since(start).Microseconds())/1000))
	w.Write(page)
}

func (s *store) handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.serveIndex)
	mux.HandleFunc("GET /notes/", s.serveNote)
	mux.HandleFunc("POST /notes/{name}/comments", s.postComment)
	mux.HandleFunc("GET /assets/viz-standalone.js", serveAsset(vizJS))
	mux.HandleFunc("GET /assets/diagrams.js", serveAsset(diagramsJS))
	return mux
}

const style = `<style>
:root{--bg:#fdfdfb;--fg:#1d1d1b;--muted:#6b6b66;--rule:#e3e2dc;--code-bg:#f3f2ed;--link:#1f5fa8;--reply-bg:#eef3fa}
@media (prefers-color-scheme:dark){:root{--bg:#161615;--fg:#e6e5e0;--muted:#9a9993;--rule:#33332f;--code-bg:#22221f;--link:#7fb0ea;--reply-bg:#1d242e}}
*{box-sizing:border-box}
body{margin:0;background:var(--bg);color:var(--fg);font:18px/1.6 Charter,"Bitstream Charter","Sitka Text",Cambria,Georgia,serif;-webkit-text-size-adjust:100%}
main{max-width:42rem;margin:0 auto;padding:1.5rem 1rem 4rem}
a{color:var(--link)}
h1,h2,h3,h4,h5,h6{font-family:system-ui,-apple-system,"Segoe UI",sans-serif;line-height:1.25;margin:2rem 0 .75rem}
h1{font-size:1.9rem;margin-top:.5rem}h2{font-size:1.45rem}h3{font-size:1.2rem}h4,h5,h6{font-size:1rem}
p,ul,ol,blockquote,pre,.table-wrap{margin:0 0 1rem}
ul,ol{padding-left:1.5rem}li>ul,li>ol{margin:.25rem 0}
blockquote{border-left:3px solid var(--rule);padding:0 0 0 1rem;color:var(--muted)}
code{font:.85em/1.5 ui-monospace,SFMono-Regular,Menlo,Consolas,monospace;background:var(--code-bg);padding:.1em .3em;border-radius:3px}
pre{background:var(--code-bg);padding:.75rem 1rem;border-radius:6px;overflow-x:auto}
pre code{background:none;padding:0;font-size:.8rem}
.table-wrap{overflow-x:auto}
table{border-collapse:collapse;font-size:.9rem}
th,td{border:1px solid var(--rule);padding:.35rem .6rem;vertical-align:top}
th{background:var(--code-bg);text-align:left}
th.sortable{cursor:pointer;user-select:none}
th.sortable:hover,th.sortable:focus-visible{color:var(--link)}
.sort-mark{margin-left:.35em;font-size:.75em;color:var(--muted)}
th.sorted .sort-mark{color:var(--link)}
hr{border:0;border-top:1px solid var(--rule);margin:2rem 0}
img{max-width:100%}
nav{font:.9rem system-ui,sans-serif;margin-bottom:1rem}
.date{color:var(--muted);font:.85rem system-ui,sans-serif}
.index{list-style:none;padding:0}
.index li{padding:.6rem 0;border-bottom:1px solid var(--rule);display:flex;gap:1rem;align-items:baseline}
.index a{text-decoration:none;flex:1}
.index .date{white-space:nowrap}
@media (max-width:30rem){body{font-size:17px}main{padding:1rem 1rem 3rem}.index li{flex-direction:column;gap:.1rem}}
</style>`

func pageHead(b *bytes.Buffer, title string) {
	b.WriteString("<!doctype html>\n<html lang=\"en\"><head><meta charset=\"utf-8\">\n")
	b.WriteString("<meta name=\"viewport\" content=\"width=device-width, initial-scale=1\">\n<title>")
	b.WriteString(title)
	b.WriteString("</title>\n")
	b.WriteString(style)
	b.WriteString("\n</head><body><main>\n")
}

// notePage is a note's page: its body with each block's comments after
// the block, the orphans at the end, and the script that adds comments.
func notePage(n *note, cs []*Comment, places []placed) []byte {
	on := make(map[int][]int) // block -> its comments, in file order
	var orphans []int
	for i, p := range places {
		if p.block < 0 {
			orphans = append(orphans, i)
		} else {
			on[p.block] = append(on[p.block], i)
		}
	}
	var b bytes.Buffer
	b.Grow(len(n.body) + len(style) + len(commentStyle) + len(n.style) + len(commentScript) + len(tablesScript) + len(n.scripts) + 300*len(cs) + 12*len(n.blocks) + 1024)
	pageHead(&b, n.title)
	b.WriteString(commentStyle)
	b.WriteString(n.style)
	b.WriteString(`<nav><a href="/">All notes</a> · <span class="date">`)
	b.WriteString(n.date.Format("2006-01-02"))
	b.WriteString("</span></nav>\n<article>\n")
	at := 0
	for k, blk := range n.blocks {
		if len(on[k]) == 0 {
			continue
		}
		b.WriteString(n.body[at:blk.End])
		at = blk.End
		fmt.Fprintf(&b, `<div class="cmts" data-for="%d">`, k)
		for _, i := range on[k] {
			writeComment(&b, cs[i], blk.norm != cs[i].Quote)
		}
		b.WriteString("</div>")
	}
	b.WriteString(n.body[at:])
	b.WriteString("</article>\n")
	if len(orphans) > 0 {
		b.WriteString(`<section class="orphans"><h2>Comments on text that is gone</h2>` + "\n")
		for _, i := range orphans {
			writeComment(&b, cs[i], true)
		}
		b.WriteString("</section>\n")
	}
	// The blocks' fingerprints, for posting a comment: see postComment.
	b.WriteString(`<script id="blocks" type="application/json">[`)
	for k, blk := range n.blocks {
		if k > 0 {
			b.WriteByte(',')
		}
		b.WriteString(`"` + blk.hash + `"`)
	}
	b.WriteString("]</script>\n<script>const NOTE = ")
	name, _ := json.Marshal(n.name)
	b.Write(name)
	b.WriteString(";</script>\n")
	b.WriteString(commentScript)
	b.WriteString(tablesScript)
	b.WriteString(n.scripts)
	b.WriteString("</main></body></html>\n")
	return b.Bytes()
}

// writeComment writes one comment; was shows the text it was made on,
// for a comment whose block has changed since or is gone.
func writeComment(b *bytes.Buffer, c *Comment, was bool) {
	b.WriteString(`<div class="cmt" id="c-` + html.EscapeString(c.ID) + `">`)
	if was {
		b.WriteString(`<details class="was"><summary>on an earlier version</summary><blockquote>`)
		b.WriteString(html.EscapeString(c.Quote))
		b.WriteString("</blockquote></details>")
	}
	b.WriteString(`<div class="who"><b>`)
	b.WriteString(html.EscapeString(c.Name))
	b.WriteString(`</b> <span class="date">`)
	b.WriteString(html.EscapeString(strings.SplitN(c.Time, "T", 2)[0]))
	b.WriteString(`</span></div><div class="said">`)
	b.WriteString(html.EscapeString(c.Text))
	b.WriteString("</div></div>")
}

func indexPage(list []*note) []byte {
	var b bytes.Buffer
	b.Grow(len(style) + 200*len(list) + 512)
	pageHead(&b, "Notes")
	b.WriteString("<h1>Notes</h1>\n<p class=\"date\">")
	b.WriteString(strconv.Itoa(len(list)))
	b.WriteString(" notes, newest first</p>\n<ul class=\"index\">\n")
	for _, n := range list {
		b.WriteString(`<li><a href="/notes/`)
		b.WriteString(html.EscapeString(n.name))
		b.WriteString(`">`)
		b.WriteString(n.title)
		b.WriteString(`</a><span class="date">`)
		b.WriteString(n.date.Format("2006-01-02"))
		b.WriteString("</span></li>\n")
	}
	b.WriteString("</ul>\n</main></body></html>\n")
	return b.Bytes()
}

func main() {
	dir := flag.String("notes", "notes", "the folder of .md and .note notes")
	renderer := flag.String("note-renderer", "", "the program that renders a .note (default: render-note beside this server)")
	comments := flag.String("comments", "", "the folder of comment files (default: comments beside the notes folder)")
	addr := flag.String("addr", "127.0.0.1:9300", "the address to listen on")
	accountsDir := flag.String("accounts", "", "the folder of accounts, which every page then needs (needed to listen past loopback; see accounts.go)")
	flag.Parse()
	if err := checkOpen(*addr, *accountsDir); err != nil {
		log.Fatal(err)
	}
	var gate *accounts
	if *accountsDir != "" {
		g, err := openAccounts(*accountsDir)
		if err != nil {
			log.Fatal(err)
		}
		gate = g
	}
	if *comments == "" {
		*comments = filepath.Join(filepath.Dir(filepath.Clean(*dir)), "comments")
	}
	if *renderer == "" {
		self, err := os.Executable()
		if err != nil {
			log.Fatal(err)
		}
		*renderer = filepath.Join(filepath.Dir(self), "render-note")
	}
	start := time.Now()
	s, err := newStore(*dir, *comments, *renderer)
	if err != nil {
		log.Fatal(err)
	}
	from := s.dateSources()
	log.Printf("rendered %d notes from %s in %s; comments in %s", len(s.list()), *dir, time.Since(start).Round(time.Millisecond), *comments)
	git := fmt.Sprintf("git in %s", s.gitTook.Round(time.Millisecond))
	if s.gitErr != nil {
		git = s.gitErr.Error()
	}
	log.Printf("dates: %d from the notes, %d from git, %d from their files (%s)", from[dateFromNote], from[dateFromGit], from[dateFromFile], git)
	started := cpuTime()
	log.Printf("cpu: %d ms to start", started.Milliseconds())
	var served atomic.Int64
	h := s.handler()
	if gate != nil {
		h = gate.wrap(h)
		log.Printf("accounts: %d, in %s; loopback needs none", gate.count(), *accountsDir)
	}
	counted := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		served.Add(1)
		h.ServeHTTP(w, r)
	})
	// A bench stops the server with SIGTERM and reads this line: CPU time
	// is what a busy box does not inflate.
	go func() {
		stop := make(chan os.Signal, 1)
		signal.Notify(stop, syscall.SIGTERM, os.Interrupt)
		<-stop
		log.Printf("stopping: %d requests served in %d ms of cpu after the start", served.Load(), (cpuTime() - started).Milliseconds())
		os.Exit(0)
	}()
	// "listening" is said once the port is ours: a script that waits for it
	// must never go on to talk to whatever else holds the port.
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on http://%s/", *addr)
	log.Fatal(http.Serve(ln, counted))
}

// cpuTime is the CPU time this process has used, user and system.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		log.Fatal(err)
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

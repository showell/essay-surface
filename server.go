package main

// The server: each note is rendered once and kept, keyed by its modified
// time and size, so a request is a stat and a write. Editing a note on disk
// re-renders it on the next request that needs it.

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type note struct {
	name     string
	title    string // HTML-escaped text, no tags
	modified time.Time
	size     int64
	page     []byte // the whole HTML page
}

type store struct {
	dir   string
	mu    sync.RWMutex
	notes map[string]*note
}

func newStore(dir string) *store {
	return &store{dir: dir, notes: map[string]*note{}}
}

// validName is a note's file name: one path element ending in .md.
func validName(name string) bool {
	return strings.HasSuffix(name, ".md") && len(name) > 3 && !strings.ContainsAny(name, "/\\") &&
		!strings.HasPrefix(name, ".")
}

// get gives the note's current rendering, rendering it again if the file
// changed since. A missing file is (nil, nil).
func (s *store) get(name string, info os.FileInfo) (*note, error) {
	s.mu.RLock()
	n := s.notes[name]
	s.mu.RUnlock()
	if n != nil && n.modified.Equal(info.ModTime()) && n.size == info.Size() {
		return n, nil
	}
	src, err := os.ReadFile(filepath.Join(s.dir, name))
	if err != nil {
		return nil, err
	}
	n = &note{name: name, modified: info.ModTime(), size: info.Size()}
	text := string(src)
	n.title = TitleHTML(Title(text))
	if n.title == "" {
		n.title = html.EscapeString(strings.TrimSuffix(name, ".md"))
	}
	n.page = notePage(n, Render(text))
	s.mu.Lock()
	s.notes[name] = n
	s.mu.Unlock()
	return n, nil
}

// all gives every note in the folder, newest first, then by name.
func (s *store) all() ([]*note, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	list := make([]*note, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if e.IsDir() || !validName(e.Name()) {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue // removed between the listing and the stat
		}
		n, err := s.get(e.Name(), info)
		if err != nil {
			continue
		}
		seen[n.name] = true
		list = append(list, n)
	}
	s.mu.Lock()
	for name := range s.notes {
		if !seen[name] {
			delete(s.notes, name)
		}
	}
	s.mu.Unlock()
	sort.Slice(list, func(a, b int) bool {
		if !list[a].modified.Equal(list[b].modified) {
			return list[a].modified.After(list[b].modified)
		}
		return list[a].name < list[b].name
	})
	return list, nil
}

func (s *store) serveIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	start := time.Now()
	list, err := s.all()
	if err != nil {
		http.Error(w, "cannot read the notes folder", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	page := indexPage(list)
	writePage(w, page, start)
}

func (s *store) serveNote(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	name := strings.TrimPrefix(r.URL.Path, "/notes/")
	if !validName(name) {
		http.NotFound(w, r)
		return
	}
	info, err := os.Stat(filepath.Join(s.dir, name))
	if err != nil || !info.Mode().IsRegular() {
		http.NotFound(w, r)
		return
	}
	n, err := s.get(name, info)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	writePage(w, n.page, start)
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
	return mux
}

const style = `<style>
:root{--bg:#fdfdfb;--fg:#1d1d1b;--muted:#6b6b66;--rule:#e3e2dc;--code-bg:#f3f2ed;--link:#1f5fa8}
@media (prefers-color-scheme:dark){:root{--bg:#161615;--fg:#e6e5e0;--muted:#9a9993;--rule:#33332f;--code-bg:#22221f;--link:#7fb0ea}}
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

func notePage(n *note, body string) []byte {
	var b bytes.Buffer
	b.Grow(len(body) + len(style) + 512)
	pageHead(&b, n.title)
	b.WriteString(`<nav><a href="/">All notes</a> · <span class="date">`)
	b.WriteString(n.modified.Format("2006-01-02"))
	b.WriteString("</span></nav>\n<article>\n")
	b.WriteString(body)
	b.WriteString("</article>\n</main></body></html>\n")
	return b.Bytes()
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
		b.WriteString(n.modified.Format("2006-01-02"))
		b.WriteString("</span></li>\n")
	}
	b.WriteString("</ul>\n</main></body></html>\n")
	return b.Bytes()
}

func main() {
	dir := flag.String("notes", "notes", "the folder of .md notes")
	addr := flag.String("addr", "127.0.0.1:9300", "the address to listen on")
	flag.Parse()
	s := newStore(*dir)
	start := time.Now()
	list, err := s.all()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("rendered %d notes from %s in %s", len(list), *dir, time.Since(start).Round(time.Millisecond))
	log.Printf("listening on http://%s/", *addr)
	log.Fatal(http.ListenAndServe(*addr, s.handler()))
}

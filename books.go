package main

// Books under review: the server shows a staged book's pages, as the book
// builder wrote them, under /books/<book>/, and adds to each page a quiet
// Reply button that opens a box for a reply to the whole chapter, with five
// stars that may be left unset. The pages are served behind the accounts
// like everything else here, and the button exists only on this server:
// no book file holds it, so a published book never can.
//
// The books are named in the file -books gives, one a line, `<book>
// <absolute folder>`; `#` lines and blank ones are skipped. A book not in
// it is not served. Replies go to -book-replies, one file a book,
// <book>.jsonl, one reply a line in the order they were made.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "embed"
)

//go:embed book_review.css
var bookReviewCSS string

//go:embed book_review.js
var bookReviewJS string

// bookReviewMark is on the widget's element, its style and its script. A
// published book must never hold it: the publish scripts look for it.
const bookReviewMark = "data-essay-book-review"

// A Reply is one line of a book's replies file.
type Reply struct {
	ID    string `json:"id"`
	Time  string `json:"time"`
	Book  string `json:"book"`
	Page  string `json:"page"`  // the chapter's file, without .html
	Title string `json:"title"` // the chapter's heading, as text
	Stars *int   `json:"stars"` // 1 to 5, or null: no opinion given
	Text  string `json:"text"`
	Name  string `json:"name"`
}

type books struct {
	folders map[string]string // book -> its absolute folder
	replies string            // the folder of replies files
}

// readBooks reads the list of books; an empty path is no books.
func readBooks(list, replies string) (*books, error) {
	b := &books{folders: map[string]string{}, replies: replies}
	if list == "" {
		return b, nil
	}
	src, err := os.ReadFile(list)
	if err != nil {
		return nil, err
	}
	for n, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Fields(line)
		if len(f) != 2 || !noteName(f[0]) || !filepath.IsAbs(f[1]) {
			return nil, fmt.Errorf("%s:%d: a line is `<book> <absolute folder>`, the book's name lowercase letters, digits and hyphens", list, n+1)
		}
		if _, dup := b.folders[f[0]]; dup {
			return nil, fmt.Errorf("%s:%d: %s is named twice", list, n+1, f[0])
		}
		b.folders[f[0]] = filepath.Clean(f[1])
	}
	return b, nil
}

// bookTypes are the files a book may hold, by extension, with the type each
// is served as. Any other file is not served.
var bookTypes = map[string]string{
	".html": "text/html; charset=utf-8",
	".css":  "text/css; charset=utf-8",
	".js":   "text/javascript; charset=utf-8",
	".svg":  "image/svg+xml",
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".webp": "image/webp",
	".json": "application/json",
	".txt":  "text/plain; charset=utf-8",
	".cnf":  "text/plain; charset=utf-8",
	".wasm": "application/wasm",
}

// pathPart is one folder or file name of a book's path: no hidden name, no
// "..", nothing a shell or a URL would read twice.
var pathPart = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

// bookPath is a request's path inside a book, if every part of it is a
// plain name; "" stands for index.html.
func bookPath(rest string) (string, bool) {
	if rest == "" {
		return "index.html", true
	}
	parts := strings.Split(rest, "/")
	for _, p := range parts {
		if !pathPart.MatchString(p) || strings.Contains(p, "..") {
			return "", false
		}
	}
	return rest, true
}

// pageSlug is a chapter's name as a reply carries it: a page's file name
// without .html, at the top of the book.
var pageSlug = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// open opens a file of a book without leaving its folder: os.Root refuses a
// link or a ".." that would. It sends only a regular file.
func (b *books) open(book, rel string) (*os.File, fs.FileInfo, error) {
	dir, ok := b.folders[book]
	if !ok {
		return nil, nil, fs.ErrNotExist
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, nil, err
	}
	defer root.Close()
	f, err := root.Open(rel)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		f.Close()
		return nil, nil, fs.ErrNotExist
	}
	return f, info, nil
}

// serveBookList lists the books, each linked to its first page.
func (b *books) serveBookList(w http.ResponseWriter, r *http.Request) {
	var page bytes.Buffer
	pageHead(&page, "Books under review")
	page.WriteString("<nav><a href=\"/\">All notes</a></nav>\n<h1>Books under review</h1>\n<ul class=\"index\">\n")
	for _, name := range sortedKeys(b.folders) {
		page.WriteString(`<li><a href="/books/` + name + `/">` + name + "</a></li>\n")
	}
	page.WriteString("</ul>\n</main></body></html>\n")
	writePage(w, page.Bytes(), time.Now())
}

func sortedKeys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// serveBook sends a file of a book; a page gets the review widget.
func (b *books) serveBook(w http.ResponseWriter, r *http.Request) {
	book := r.PathValue("book")
	if _, ok := b.folders[book]; !ok {
		http.NotFound(w, r)
		return
	}
	rest := strings.TrimPrefix(r.URL.Path, "/books/"+book)
	if rest == "" {
		http.Redirect(w, r, "/books/"+book+"/", http.StatusFound)
		return
	}
	rel, ok := bookPath(strings.TrimPrefix(rest, "/"))
	typ := bookTypes[filepath.Ext(rel)]
	if !ok || typ == "" {
		http.NotFound(w, r)
		return
	}
	f, info, err := b.open(book, rel)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	h := w.Header()
	h.Set("Content-Type", typ)
	h.Set("Cache-Control", "no-store")
	h.Set("X-Content-Type-Options", "nosniff")
	if filepath.Ext(rel) != ".html" {
		h.Set("Content-Length", strconv.FormatInt(info.Size(), 10))
		if _, err := io.Copy(w, f); err != nil {
			log.Printf("books/%s/%s: %v", book, rel, err)
		}
		return
	}
	src, err := io.ReadAll(io.LimitReader(f, 64<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	page := withReview(src, book, strings.TrimSuffix(rel, ".html"))
	h.Set("Content-Length", strconv.Itoa(len(page)))
	w.Write(page)
}

// withReview puts the widget into a page, just before its </body>, or at
// its end when it has none. A page in a folder of the book is not a
// chapter and gets none.
func withReview(src []byte, book, page string) []byte {
	if !pageSlug.MatchString(page) {
		return src
	}
	bk, _ := json.Marshal(book)
	pg, _ := json.Marshal(page)
	widget := "<style " + bookReviewMark + ">\n" + bookReviewCSS + "</style>\n" +
		"<script " + bookReviewMark + ">const BOOK_REVIEW = {book: " + string(bk) + ", page: " + string(pg) + "};\n" +
		strings.ReplaceAll(bookReviewJS, "</", "<\\/") + "</script>\n"
	at := bytes.LastIndex(src, []byte("</body>"))
	if at < 0 {
		at = len(src)
	}
	out := make([]byte, 0, len(src)+len(widget))
	out = append(out, src[:at]...)
	out = append(out, widget...)
	return append(out, src[at:]...)
}

var (
	h1RE    = regexp.MustCompile(`(?s)<h1[^>]*>(.*?)</h1>`)
	titleRE = regexp.MustCompile(`(?s)<title>(.*?)</title>`)
	tagRE   = regexp.MustCompile(`<[^>]*>`)
)

// chapterTitle is a page's first heading as text, else its <title>, else
// its slug.
func chapterTitle(src []byte, slug string) string {
	for _, re := range []*regexp.Regexp{h1RE, titleRE} {
		if m := re.FindSubmatch(src); m != nil {
			t := strings.Join(strings.Fields(html.UnescapeString(tagRE.ReplaceAllString(string(m[1]), ""))), " ")
			if t != "" {
				return t
			}
		}
	}
	return slug
}

// replyRequest is what the widget posts. Stars is null when the reader
// left them unset.
type replyRequest struct {
	Page  string `json:"page"`
	Stars *int   `json:"stars"`
	Text  string `json:"text"`
	Name  string `json:"name"`
}

// cleanReply checks a reply; the error is for the reader. A reply says
// something: a text, stars, or both.
func cleanReply(req replyRequest) (replyRequest, error) {
	req.Name = strings.TrimSpace(req.Name)
	req.Text = strings.TrimSpace(strings.ReplaceAll(req.Text, "\r\n", "\n"))
	switch {
	case !pageSlug.MatchString(req.Page):
		return req, errors.New("no such chapter")
	case req.Stars != nil && (*req.Stars < 1 || *req.Stars > 5):
		return req, errors.New("stars are 1 to 5, or none")
	case req.Text == "" && req.Stars == nil:
		return req, errors.New("a reply needs a text or stars")
	case req.Name == "":
		return req, errors.New("a reply needs a name")
	case len(req.Name) > maxName || strings.ContainsAny(req.Name, "\n\t"):
		return req, fmt.Errorf("a name is one line of at most %d bytes", maxName)
	case len(req.Text) > maxText:
		return req, fmt.Errorf("a reply is at most %d bytes", maxText)
	}
	return req, nil
}

// postReply adds a reply to a book's replies file, tied to its chapter: the
// page must be one of the book's, and its title is read from the page.
func (s *store) postReply(w http.ResponseWriter, r *http.Request) {
	b := s.books
	book := r.PathValue("book")
	if _, ok := b.folders[book]; !ok {
		http.NotFound(w, r)
		return
	}
	// JSON only: a form from another site cannot send it without asking first.
	if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		http.Error(w, "replies are posted as JSON", http.StatusUnsupportedMediaType)
		return
	}
	var req replyRequest
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "not a reply: "+err.Error(), http.StatusBadRequest)
		return
	}
	// A logged-in reply is signed by its account, whatever name it sends.
	if user := accountOf(r); user != "" {
		req.Name = user
	}
	req, err := cleanReply(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	f, _, err := b.open(book, req.Page+".html")
	if err != nil {
		http.Error(w, "no such chapter: "+req.Page, http.StatusNotFound)
		return
	}
	src, err := io.ReadAll(io.LimitReader(f, 64<<20))
	f.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rep := Reply{ID: newID(), Time: time.Now().UTC().Format(time.RFC3339), Book: book, Page: req.Page,
		Title: chapterTitle(src, req.Page), Stars: req.Stars, Text: req.Text, Name: req.Name}
	s.write.Lock()
	defer s.write.Unlock()
	if err := appendReply(filepath.Join(b.replies, book+".jsonl"), rep); err != nil {
		http.Error(w, "the reply was not saved", http.StatusInternalServerError)
		log.Print(err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(rep)
}

// appendReply adds one line to a replies file, in one write, making the
// folder when it is missing.
func appendReply(path string, rep Reply) error {
	var line bytes.Buffer
	enc := json.NewEncoder(&line)
	enc.SetEscapeHTML(false) // the file is for reading: keep < and & as they are
	if err := enc.Encode(rep); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(line.Bytes()); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// readReplies reads a replies file; a missing file is none, and a line
// that is not a reply stops the read, naming the file and line.
func readReplies(path string) ([]Reply, error) {
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Reply
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		var rep Reply
		if err := dec.Decode(&rep); err != nil {
			return nil, fmt.Errorf("%s:%d: %v", path, n, err)
		}
		out = append(out, rep)
	}
	return out, sc.Err()
}

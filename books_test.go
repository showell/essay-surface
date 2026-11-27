package main

import (
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// bookStore serves a notes folder and one book, "demo", of two chapters and
// a stylesheet, with a picture in a folder of its own and a link that leads
// out of the book. Gives the handler, the book's folder and the replies'.
func bookStore(t *testing.T) (*store, string, string) {
	dir := t.TempDir()
	notes := filepath.Join(dir, "notes")
	book := filepath.Join(dir, "next", "demo")
	for _, d := range []string{notes, filepath.Join(book, "shots")} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	files := map[string]string{
		"index.html":       "<html><head><title>Demo</title></head><body><h1>Demo book</h1></body></html>\n",
		"the-machine.html": "<html><head><title>The machine · Demo</title></head><body><h1 id=\"m\">The <em>machine</em> &amp; you</h1></body></html>\n",
		"book.css":         "body{}\n",
		"shots/a.png":      "png",
		"notes.md":         "not served\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(book, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	secret := filepath.Join(dir, "secret.html")
	os.WriteFile(secret, []byte("<body>secret</body>"), 0o644)
	if err := os.Symlink(secret, filepath.Join(book, "out.html")); err != nil {
		t.Fatal(err)
	}
	list := filepath.Join(dir, "books.txt")
	os.WriteFile(list, []byte("# the books\n\ndemo "+book+"\n"), 0o644)
	s := testStore(t, notes)
	replies := filepath.Join(dir, "book-replies")
	b, err := readBooks(list, replies)
	if err != nil {
		t.Fatal(err)
	}
	s.books = b
	return s, book, replies
}

func postBook(t *testing.T, s *store, path, body string) (int, string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	s.handler().ServeHTTP(rec, req)
	out, _ := io.ReadAll(rec.Body)
	return rec.Code, string(out)
}

func TestReadBooksRefusesBadLines(t *testing.T) {
	dir := t.TempDir()
	for _, bad := range []string{"demo relative/folder\n", "Demo /abs\n", "demo\n", "demo /a\ndemo /b\n", "../x /abs\n"} {
		p := filepath.Join(dir, "books.txt")
		os.WriteFile(p, []byte(bad), 0o644)
		if _, err := readBooks(p, dir); err == nil {
			t.Errorf("readBooks took %q", bad)
		}
	}
	p := filepath.Join(dir, "books.txt")
	os.WriteFile(p, []byte("cook-levin /a/b\njs-book /c\n"), 0o644)
	b, err := readBooks(p, dir)
	if err != nil || len(b.folders) != 2 || b.folders["js-book"] != "/c" {
		t.Errorf("readBooks: %v %v", b, err)
	}
	if b, err := readBooks("", dir); err != nil || len(b.folders) != 0 {
		t.Errorf("no list is no books: %v %v", b, err)
	}
}

func TestServeBookPages(t *testing.T) {
	s, _, _ := bookStore(t)
	h := s.handler()
	code, body := get(t, h, "/books/demo/the-machine.html")
	if code != 200 || !strings.Contains(body, bookReviewMark) || !strings.Contains(body, `page: "the-machine"`) {
		t.Fatalf("a chapter: %d, widget %v", code, strings.Contains(body, bookReviewMark))
	}
	if strings.Index(body, bookReviewMark) > strings.LastIndex(body, "</body>") {
		t.Error("the widget is after </body>")
	}
	code, body = get(t, h, "/books/demo/")
	if code != 200 || !strings.Contains(body, `page: "index"`) {
		t.Errorf("the book's first page: %d", code)
	}
	code, body = get(t, h, "/books/demo/book.css")
	if code != 200 || strings.Contains(body, bookReviewMark) {
		t.Errorf("the stylesheet: %d", code)
	}
	if code, _ := get(t, h, "/books/demo/shots/a.png"); code != 200 {
		t.Errorf("a picture in a folder: %d", code)
	}
	if code, _ := get(t, h, "/books/demo"); code != 302 {
		t.Errorf("the book without its slash: %d", code)
	}
	code, body = get(t, h, "/books/")
	if code != 200 || !strings.Contains(body, `href="/books/demo/"`) {
		t.Errorf("the list of books: %d", code)
	}
	for _, path := range []string{"/books/other/index.html", "/books/demo/notes.md", "/books/demo/out.html",
		"/books/demo/../secret.html", "/books/demo/.hidden.html", "/books/demo/missing.html", "/books/demo/replies"} {
		// The mux sends a path with ".." to its cleaned form (307), which is
		// outside the book; everything else is not found.
		if code, body := get(t, h, path); code != 404 && code != 307 || strings.Contains(body, "<body>secret") {
			t.Errorf("%s: %d, want 404 (or the mux's 307)", path, code)
		}
	}
}

func TestPostReply(t *testing.T) {
	s, _, replies := bookStore(t)
	code, body := postBook(t, s, "/books/demo/replies", `{"page":"the-machine","stars":4,"text":"Clear.","name":"Steve"}`)
	if code != 201 {
		t.Fatalf("a reply with stars: %d %s", code, body)
	}
	code, body = postBook(t, s, "/books/demo/replies", `{"page":"index","stars":null,"text":"No opinion on stars yet.","name":"Steve"}`)
	if code != 201 {
		t.Fatalf("a reply without stars: %d %s", code, body)
	}
	if code, body = postBook(t, s, "/books/demo/replies", `{"page":"index","stars":2,"text":"","name":"Steve"}`); code != 201 {
		t.Fatalf("stars alone: %d %s", code, body)
	}
	got, err := readReplies(filepath.Join(replies, "demo.jsonl"))
	if err != nil || len(got) != 3 {
		t.Fatalf("read back: %d replies, %v", len(got), err)
	}
	if got[0].Page != "the-machine" || got[0].Title != "The machine & you" || got[0].Stars == nil || *got[0].Stars != 4 || got[0].Book != "demo" || got[0].Name != "Steve" {
		t.Errorf("the first reply: %+v", got[0])
	}
	if got[1].Stars != nil || got[1].Title != "Demo book" || got[1].Text != "No opinion on stars yet." {
		t.Errorf("the second reply: %+v", got[1])
	}
	raw, _ := os.ReadFile(filepath.Join(replies, "demo.jsonl"))
	if !strings.Contains(string(raw), `"stars":null`) {
		t.Error("an unset star is not written as null")
	}
	for _, bad := range []string{
		`{"page":"index","stars":0,"text":"x","name":"S"}`,
		`{"page":"index","stars":6,"text":"x","name":"S"}`,
		`{"page":"index","stars":null,"text":"  ","name":"S"}`,
		`{"page":"index","text":"x","name":""}`,
		`{"page":"../secret","text":"x","name":"S"}`,
		`{"page":"missing","text":"x","name":"S"}`,
		`{"page":"out","text":"x","name":"S"}`,
		`{"page":"index","text":"x","name":"S","block":1}`,
	} {
		if code, _ := postBook(t, s, "/books/demo/replies", bad); code == 201 {
			t.Errorf("took %s", bad)
		}
	}
	if code, _ := postBook(t, s, "/books/other/replies", `{"page":"index","text":"x","name":"S"}`); code != 404 {
		t.Errorf("a book not listed: %d", code)
	}
	if got, _ := readReplies(filepath.Join(replies, "demo.jsonl")); len(got) != 3 {
		t.Errorf("a refused reply was written: %d replies", len(got))
	}
}

// A reply to a book needs a log-in like a comment, and is signed by the
// account, whatever name it sends.
func TestReplyNeedsLogin(t *testing.T) {
	s, _, replies := bookStore(t)
	hashRounds = 1000
	a, err := openAccounts(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := a.addFirst("steve", "a long enough password"); err != nil {
		t.Fatal(err)
	}
	h := a.wrap(s.handler())
	body := `{"page":"index","stars":5,"text":"x","name":"someone else"}`
	req := httptest.NewRequest("POST", "/books/demo/replies", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:4000"
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 401 {
		t.Errorf("a reply while logged out: %d, want 401", rec.Code)
	}
	req = httptest.NewRequest("GET", "/books/demo/index.html", nil)
	req.RemoteAddr = "203.0.113.9:4000"
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 303 {
		t.Errorf("a page while logged out: %d, want the log-in's 303", rec.Code)
	}
	req = httptest.NewRequest("POST", "/books/demo/replies", strings.NewReader(body))
	req.RemoteAddr = "203.0.113.9:4000"
	req.Header.Set("Content-Type", "application/json")
	rec0 := httptest.NewRecorder()
	a.startSession(rec0, "steve")
	for _, c := range rec0.Result().Cookies() {
		req.AddCookie(c)
	}
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("a logged-in reply: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := readReplies(filepath.Join(replies, "demo.jsonl"))
	if len(got) != 1 || got[0].Name != "steve" {
		t.Errorf("the reply is not signed by its account: %+v", got)
	}
}

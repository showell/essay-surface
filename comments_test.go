package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

func post(t testing.TB, h http.Handler, path, contentType, body string) (int, string) {
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

// hashesOf is the block fingerprints a page carries.
func hashesOf(t testing.TB, page string) []string {
	m := regexp.MustCompile(`<script id="blocks" type="application/json">(.*?)</script>`).FindStringSubmatch(page)
	if m == nil {
		t.Fatal("the page has no block fingerprints")
	}
	var hs []string
	if err := json.Unmarshal([]byte(m[1]), &hs); err != nil {
		t.Fatal(err)
	}
	return hs
}

func commentJSON(block int, hash, name, text string) string {
	b, _ := json.Marshal(commentRequest{Block: block, Hash: hash, Name: name, Text: text})
	return string(b)
}

const commentNote = "# Note\n\nFirst paragraph here.\n\nSecond paragraph, the one with a claim.\n\nThird paragraph to end.\n"

// setup serves a folder with one note, a.md, and gives its first page's
// fingerprints. Its blocks: 0 the title, 1-3 the paragraphs.
func setup(t *testing.T) (string, http.Handler, []string) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", commentNote, time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC))
	h := testStore(t, dir).handler()
	_, page := get(t, h, "/notes/a.md")
	hs := hashesOf(t, page)
	if len(hs) != 4 {
		t.Fatalf("%d blocks, want 4", len(hs))
	}
	return dir, h, hs
}

func TestPostComment(t *testing.T) {
	dir, h, hs := setup(t)
	code, body := post(t, h, "/notes/a.md/comments", "application/json", commentJSON(2, hs[2], " Ann ", "I <b>doubt</b> this & that.\nSecond line."))
	if code != 201 {
		t.Fatalf("post: %d %s", code, body)
	}
	var got map[string]any
	json.Unmarshal([]byte(body), &got)
	if got["name"] != "Ann" || got["block"] != float64(2) || got["id"] == "" {
		t.Errorf("the answer: %s", body)
	}
	src, err := os.ReadFile(filepath.Join(dir, "comments", "a.md.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(src)), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], `"quote":"Second paragraph, the one with a claim."`) ||
		!strings.Contains(lines[0], `"text":"I <b>doubt</b> this & that.\nSecond line."`) {
		t.Errorf("the file:\n%s", src)
	}
	_, page := get(t, h, "/notes/a.md")
	want := `with a claim.</p><div class="cmts" data-for="2"><div class="cmt" id="c-`
	if !strings.Contains(page, want) {
		t.Errorf("the comment is not after its paragraph")
	}
	if !strings.Contains(page, "I &lt;b&gt;doubt&lt;/b&gt; this &amp; that.\nSecond line.") || strings.Contains(page, "<b>doubt") {
		t.Error("the comment's text is not escaped")
	}
	if strings.Contains(page, `<details class="was">`) || strings.Contains(page, `<section class="orphans">`) {
		t.Error("an unchanged paragraph's comment is marked as on an earlier version")
	}
	// A second comment on the same block follows the first.
	post(t, h, "/notes/a.md/comments", "application/json", commentJSON(2, hs[2], "Bo", "Agreed."))
	_, page = get(t, h, "/notes/a.md")
	if i, j := strings.Index(page, "doubt"), strings.Index(page, "Agreed."); i < 0 || j < i || strings.Count(page, `class="cmts"`) != 1 {
		t.Error("two comments on a block are not one thread, in order")
	}
}

// Every field of a comment that reaches the page is the commenter's or the
// file's, so each is escaped: a name of markup is shown, not run.
func TestEveryCommentFieldEscaped(t *testing.T) {
	const m = `<i x="1">&`
	var b bytes.Buffer
	writeComment(&b, &Comment{ID: m, Time: m, Name: m, Text: m, Quote: m}, true)
	if got := b.String(); strings.Contains(got, "<i") || strings.Count(got, "&lt;i x=&#34;1&#34;&gt;&amp;") != 5 {
		t.Errorf("a field is not escaped: %s", got)
	}
}

func TestPostCommentRefusals(t *testing.T) {
	_, h, hs := setup(t)
	cases := []struct {
		name, path, ct, body string
		code                 int
	}{
		{"not JSON", "/notes/a.md/comments", "application/x-www-form-urlencoded", "name=a&text=b", 415},
		{"no content type", "/notes/a.md/comments", "", commentJSON(1, hs[1], "a", "b"), 415},
		{"bad JSON", "/notes/a.md/comments", "application/json", "{", 400},
		{"an unknown field", "/notes/a.md/comments", "application/json", `{"block":1,"hash":"` + hs[1] + `","name":"a","text":"b","admin":true}`, 400},
		{"no name", "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], "  ", "b"), 400},
		{"no text", "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], "a", "\n"), 400},
		{"a long name", "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], strings.Repeat("n", maxName+1), "b"), 400},
		{"a long text", "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], "a", strings.Repeat("t", maxText+1)), 400},
		{"a name of two lines", "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], "a\nb", "c"), 400},
		{"a fingerprint of no block", "/notes/a.md/comments", "application/json", commentJSON(1, "nope", "a", "b"), 409},
		{"no such note", "/notes/b.md/comments", "application/json", commentJSON(1, hs[1], "a", "b"), 404},
		{"not a note's name", "/notes/..%2Fx.md/comments", "application/json", commentJSON(1, hs[1], "a", "b"), 404},
		{"a GET", "", "", "", 404},
	}
	for _, c := range cases {
		var code int
		if c.path == "" {
			code, _ = get(t, h, "/notes/a.md/comments")
		} else {
			code, _ = post(t, h, c.path, c.ct, c.body)
		}
		if code != c.code {
			t.Errorf("%s: %d, want %d", c.name, code, c.code)
		}
	}
	// Exactly at the limits is fine.
	if code, body := post(t, h, "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], strings.Repeat("n", maxName), strings.Repeat("t", maxText))); code != 201 {
		t.Errorf("at the limits: %d %s", code, body)
	}
}

// A page loaded before an edit can comment on a paragraph that moved, by
// its fingerprint, but not on one whose text changed.
func TestPostFromAStalePage(t *testing.T) {
	dir, h, hs := setup(t)
	later := time.Date(2026, 11, 13, 10, 0, 0, 0, time.UTC)
	writeNote(t, dir, "a.md", strings.Replace(commentNote, "First paragraph here.", "New opening.\n\nFirst paragraph here.", 1), later)
	code, body := post(t, h, "/notes/a.md/comments", "application/json", commentJSON(2, hs[2], "Ann", "moved"))
	if code != 201 || !strings.Contains(body, `"block":3`) {
		t.Errorf("a moved paragraph: %d %s", code, body)
	}
	writeNote(t, dir, "a.md", strings.Replace(commentNote, "with a claim", "with a bold claim", 1), later.Add(time.Minute))
	if code, _ := post(t, h, "/notes/a.md/comments", "application/json", commentJSON(2, hs[2], "Ann", "changed")); code != 409 {
		t.Errorf("a changed paragraph: %d, want 409", code)
	}
}

func TestCommentsFollowEdits(t *testing.T) {
	dir, h, hs := setup(t)
	post(t, h, "/notes/a.md/comments", "application/json", commentJSON(2, hs[2], "Ann", "on the claim"))
	post(t, h, "/notes/a.md/comments", "application/json", commentJSON(3, hs[3], "Bo", "on the end"))
	path := filepath.Join(dir, "comments", "a.md.jsonl")
	day := time.Date(2026, 11, 13, 11, 0, 0, 0, time.UTC)

	// A typo fixed in the claim, a paragraph added above it.
	writeNote(t, dir, "a.md", "# Note\n\nAn added paragraph.\n\nFirst paragraph here.\n\nSecond paragraph, the one with a clam.\n\nThird paragraph to end.\n", day)
	_, page := get(t, h, "/notes/a.md")
	if !strings.Contains(page, `the one with a clam.</p><div class="cmts" data-for="3">`) || !strings.Contains(page, "on the claim") {
		t.Error("the comment did not follow its edited paragraph")
	}
	if !strings.Contains(page, `<details class="was"><summary>on an earlier version</summary><blockquote>Second paragraph, the one with a claim.</blockquote>`) {
		t.Error("a comment on an edited paragraph does not show what it was made on")
	}
	src, _ := os.ReadFile(path)
	if !strings.Contains(string(src), `"anchor":"Second paragraph, the one with a clam."`) {
		t.Errorf("the file was not healed:\n%s", src)
	}
	healed, _ := os.Stat(path)

	// Asking again changes nothing on disk.
	get(t, h, "/notes/a.md")
	if again, _ := os.Stat(path); !again.ModTime().Equal(healed.ModTime()) {
		t.Error("an unchanged note rewrote its comments file")
	}

	// The claim deleted: its comment is shown apart, and kept.
	writeNote(t, dir, "a.md", "# Note\n\nAn added paragraph.\n\nFirst paragraph here.\n\nThird paragraph to end.\n", day.Add(time.Minute))
	_, page = get(t, h, "/notes/a.md")
	i := strings.Index(page, `<section class="orphans">`)
	if i < 0 {
		t.Fatal("a deleted paragraph's comment left no orphans section")
	}
	if !strings.Contains(page[i:], "on the claim") || strings.Contains(page[:i], "on the claim") {
		t.Error("a deleted paragraph's comment is not with the orphans")
	}
	if !strings.Contains(page[:i], `end.</p><div class="cmts" data-for="3">`) || !strings.Contains(page[:i], "on the end") {
		t.Error("the other comment lost its paragraph")
	}
	src, _ = os.ReadFile(path)
	if strings.Count(string(src), "\n") != 2 {
		t.Errorf("a comment was lost from the file:\n%s", src)
	}
}

func TestCommentsFileChangedOnDisk(t *testing.T) {
	dir, h, hs := setup(t)
	path := filepath.Join(dir, "comments", "a.md.jsonl")
	get(t, h, "/notes/a.md") // the page is made and kept
	// A pull brings a comment: the next page has it.
	line := fmt.Sprintf(`{"id":"abc","time":"2026-11-12T08:00:00Z","name":"Cy","text":"from elsewhere","ord":1,"prev":"%s","next":"%s","quote":"First paragraph here."}`+"\n", hs[0], hs[2])
	os.MkdirAll(filepath.Dir(path), 0o755)
	os.WriteFile(path, []byte(line), 0o644)
	if _, page := get(t, h, "/notes/a.md"); !strings.Contains(page, `here.</p><div class="cmts" data-for="1"><div class="cmt" id="c-abc">`) {
		t.Error("a comment written to disk is not on the page")
	}
	// A broken line is said, with where it is.
	os.WriteFile(path, []byte(line+"{not json\n"), 0o644)
	code, body := get(t, h, "/notes/a.md")
	if code != 500 || !strings.Contains(body, "a.md.jsonl:2:") {
		t.Errorf("a broken comments file: %d %q", code, body)
	}
	if code, _ := post(t, h, "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], "a", "b")); code != 500 {
		t.Errorf("posting to a broken comments file: %d, want 500", code)
	}
	src, _ := os.ReadFile(path)
	if string(src) != line+"{not json\n" {
		t.Error("a post rewrote a comments file it could not read")
	}
}

func TestCommentsFileRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x", "n.md.jsonl")
	cs := []*Comment{
		{ID: "1", Time: "t", Name: "A", Text: "a <tag> & \"q\"\nline", Ord: 0, Quote: "q1"},
		{ID: "2", Time: "t", Name: "B", Text: "b", Ord: 3, Prev: "p", Next: "n", Quote: "q2", Anchor: "a2"},
	}
	if err := writeComments(path, cs); err != nil {
		t.Fatal(err)
	}
	src, _ := os.ReadFile(path)
	if !strings.Contains(string(src), `"text":"a <tag> & \"q\"\nline"`) {
		t.Errorf("the file escapes what it need not:\n%s", src)
	}
	if strings.Contains(string(src), `"anchor":""`) {
		t.Error("an empty anchor is written")
	}
	back, err := readComments(path)
	if err != nil || len(back) != 2 || *back[0] != *cs[0] || *back[1] != *cs[1] {
		t.Errorf("read back %v, %v", back, err)
	}
	if cs, err := readComments(filepath.Join(t.TempDir(), "none.jsonl")); cs != nil || err != nil {
		t.Errorf("a missing file: %v, %v", cs, err)
	}
	os.WriteFile(path, []byte(`{"id":"","quote":"q"}`+"\n"), 0o644)
	if _, err := readComments(path); err == nil {
		t.Error("a comment with no id was read")
	}
}

// Posts and reads at once: every comment is kept, and the page has them all.
func TestConcurrentPosts(t *testing.T) {
	dir, h, hs := setup(t)
	const n = 40
	done := make(chan int, 2*n)
	for i := 0; i < n; i++ {
		go func() {
			code, _ := post(t, h, "/notes/a.md/comments", "application/json", commentJSON(1+i%3, hs[1+i%3], "R", fmt.Sprintf("comment %d", i)))
			done <- code
		}()
		go func() {
			code, _ := get(t, h, "/notes/a.md")
			done <- code
		}()
	}
	for i := 0; i < 2*n; i++ {
		if code := <-done; code != 201 && code != 200 {
			t.Errorf("a request: %d", code)
		}
	}
	cs, err := readComments(filepath.Join(dir, "comments", "a.md.jsonl"))
	if err != nil || len(cs) != n {
		t.Fatalf("%d comments kept of %d, %v", len(cs), n, err)
	}
	_, page := get(t, h, "/notes/a.md")
	for i := 0; i < n; i++ {
		if !strings.Contains(page, fmt.Sprintf(">comment %d<", i)) {
			t.Errorf("comment %d is not on the page", i)
		}
	}
}

// comments.js finds a block's thread as the element right after it. On
// every kind of block, a comment's thread follows the block's closing tag.
func TestThreadFollowsEveryKindOfBlock(t *testing.T) {
	dir := t.TempDir()
	md := "## A heading\n\n- an item\n- another\n  - nested\n\n1. loose\n\n2. items\n\n```go\ncode\n```\n\n| a |\n|---|\n| 1 |\n\n> quoted\n"
	writeNote(t, dir, "k.md", md, time.Now())
	h := testStore(t, dir).handler()
	_, page := get(t, h, "/notes/k.md")
	hs := hashesOf(t, page)
	for k := range hs {
		if code, body := post(t, h, "/notes/k.md/comments", "application/json", commentJSON(k, hs[k], "A", fmt.Sprintf("on %d", k))); code != 201 {
			t.Fatalf("block %d: %d %s", k, code, body)
		}
	}
	_, page = get(t, h, "/notes/k.md")
	ends := []string{"</h2>", "</span>", "</span>", "</span>", "</p>", "</p>", "</pre>", "</div>", "</p>"}
	if len(hs) != len(ends) {
		t.Fatalf("%d blocks, want %d", len(hs), len(ends))
	}
	for k, end := range ends {
		want := fmt.Sprintf(`%s<div class="cmts" data-for="%d">`, end, k)
		if !strings.Contains(page, want) {
			t.Errorf("block %d: no %q", k, want)
		}
	}
}

// The index carries no comment machinery.
func TestIndexHasNoComments(t *testing.T) {
	_, h, _ := setup(t)
	_, page := get(t, h, "/")
	if strings.Contains(page, "cbtn") || strings.Contains(page, "data-c=") {
		t.Error("the index has comment markup")
	}
}

// benchComments posts n comments on a note, spread across its blocks.
func benchComments(b *testing.B, h http.Handler, note string, n int) {
	_, page := get(b, h, "/notes/"+note)
	hs := hashesOf(b, page)
	for k := 0; k < n; k++ {
		blk := k * len(hs) / n
		if code, body := post(b, h, "/notes/"+note+"/comments", "application/json", commentJSON(blk, hs[blk], "Reader", "A comment of a sentence or two, about as long as a real one is.")); code != 201 {
			b.Fatalf("%d %s", code, body)
		}
	}
}

func BenchmarkNoteWith100Comments(b *testing.B) {
	dir := benchDir(b, 500)
	h := testStore(b, dir).handler()
	benchComments(b, h, "note-000.md", 100)
	get(b, h, "/notes/note-000.md")
	b.ResetTimer()
	for b.Loop() {
		get(b, h, "/notes/note-000.md")
	}
}

// BenchmarkPageRebuild100 is the cost of the first request after the note
// or its comments change: placing 100 comments and making the page.
func BenchmarkPageRebuild100(b *testing.B) {
	dir := benchDir(b, 500)
	s := testStore(b, dir)
	h := s.handler()
	benchComments(b, h, "note-000.md", 100)
	n, _ := s.note("note-000.md")
	cs, _ := readComments(commentsPath(s.comments, "note-000.md"))
	b.ResetTimer()
	for b.Loop() {
		notePage(n, cs, resolve(n.blocks, cs))
	}
}

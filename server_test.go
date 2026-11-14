package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func writeNote(t testing.TB, dir, name, text string, modified time.Time) {
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(p, modified, modified); err != nil {
		t.Fatal(err)
	}
}

// testStore serves dir, with its comments in a folder inside it. Its
// watch is closed after the test: a user has 128 inotify instances.
func testStore(t testing.TB, dir string) *store {
	s, err := newStore(dir, filepath.Join(dir, "comments"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.close() })
	return s
}

func get(t testing.TB, h http.Handler, path string) (int, string) {
	req := httptest.NewRequest("GET", path, nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	body, _ := io.ReadAll(rec.Body)
	return rec.Code, string(body)
}

func TestIndexNewestFirst(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	writeNote(t, dir, "old.md", "# The old one\n", day)
	writeNote(t, dir, "new.md", "# The new one\n", day.Add(48*time.Hour))
	writeNote(t, dir, "untitled.md", "no heading here\n", day.Add(24*time.Hour))
	writeNote(t, dir, "skip.txt", "# not a note\n", day.Add(72*time.Hour))
	h := testStore(t, dir).handler()
	code, body := get(t, h, "/")
	if code != 200 {
		t.Fatalf("index: %d", code)
	}
	iNew := strings.Index(body, "The new one")
	iUntitled := strings.Index(body, ">untitled<")
	iOld := strings.Index(body, "The old one")
	if iNew < 0 || iUntitled < 0 || iOld < 0 || !(iNew < iUntitled && iUntitled < iOld) {
		t.Errorf("want new, untitled, old in that order; at %d, %d, %d", iNew, iUntitled, iOld)
	}
	if strings.Contains(body, "skip") {
		t.Error("the index lists a file that is not .md")
	}
	if !strings.Contains(body, "3 notes") || !strings.Contains(body, "2026-11-12") {
		t.Error("the index lacks its count or a date")
	}
}

func TestNotePage(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", "# Hello\n\nsome *text*\n", time.Now())
	h := testStore(t, dir).handler()
	code, body := get(t, h, "/notes/a.md")
	if code != 200 {
		t.Fatalf("note: %d", code)
	}
	for _, want := range []string{"<title>Hello</title>", `<h1 id="hello" data-c="0">Hello</h1>`, `<em>text</em></p>`, `data-c="1"`, "<em>text</em>", `name="viewport"`, `href="/"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the page lacks %q", want)
		}
	}
}

func TestNotFound(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", "# A\n", time.Now())
	os.Mkdir(filepath.Join(dir, "sub.md"), 0o755)
	outside := filepath.Join(filepath.Dir(dir), "outside.md")
	os.WriteFile(outside, []byte("# secret\n"), 0o644)
	defer os.Remove(outside)
	h := testStore(t, dir).handler()
	for _, path := range []string{"/notes/missing.md", "/notes/a.txt", "/notes/", "/notes/sub.md",
		"/notes/..%2Foutside.md", "/notes/../outside.md", "/notes/.md", "/other", "/notes/a.md/x"} {
		code, body := get(t, h, path)
		if code == 200 {
			t.Errorf("%s: got 200, want an error", path)
		}
		if strings.Contains(body, "secret") {
			t.Errorf("%s: served a file outside the folder", path)
		}
	}
	if code, _ := get(t, h, "/notes/a.md"); code != 200 {
		t.Errorf("the one real note: %d", code)
	}
}

func TestEditedNoteRerenders(t *testing.T) {
	dir := t.TempDir()
	day := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	writeNote(t, dir, "a.md", "# First\n", day)
	h := testStore(t, dir).handler()
	if _, body := get(t, h, "/notes/a.md"); !strings.Contains(body, "First") {
		t.Fatal("first render")
	}
	writeNote(t, dir, "a.md", "# Second\n", day.Add(time.Minute))
	if _, body := get(t, h, "/notes/a.md"); !strings.Contains(body, "Second") || strings.Contains(body, "First") {
		t.Error("an edited note was served from the old rendering")
	}
	if _, body := get(t, h, "/"); !strings.Contains(body, "Second") {
		t.Error("the index kept the old title")
	}
	os.Remove(filepath.Join(dir, "a.md"))
	if code, _ := get(t, h, "/notes/a.md"); code != 404 {
		t.Errorf("a removed note: %d", code)
	}
	if _, body := get(t, h, "/"); strings.Contains(body, "Second") {
		t.Error("the index still lists a removed note")
	}
}

// benchDir copies the sample notes until there are n of them, each with
// its own name and time, so the benchmarks see a folder of real size.
func benchDir(b *testing.B, n int) string {
	paths, _ := filepath.Glob("notes/*.md")
	if len(paths) == 0 {
		b.Fatal("no sample notes")
	}
	dir := b.TempDir()
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for k := 0; k < n; k++ {
		src, _ := os.ReadFile(paths[k%len(paths)])
		writeNote(b, dir, fmt.Sprintf("note-%03d.md", k), string(src), base.Add(time.Duration(k)*time.Hour))
	}
	return dir
}

func BenchmarkIndex500(b *testing.B) {
	h := testStore(b, benchDir(b, 500)).handler()
	get(b, h, "/") // render every note once, as the server does at start
	b.ResetTimer()
	for b.Loop() {
		get(b, h, "/")
	}
}

// The first request for the index after a note changed: the note read
// and rendered again, and the page made. The change is a touch, which is
// heard as one.
func BenchmarkIndexAfterTouch500(b *testing.B) {
	dir := benchDir(b, 500)
	h := testStore(b, dir).handler()
	get(b, h, "/")
	p := filepath.Join(dir, "note-001.md")
	b.ResetTimer()
	for b.Loop() {
		now := time.Now()
		os.Chtimes(p, now, now)
		get(b, h, "/")
	}
}

// Writing the index page alone.
func BenchmarkIndexPage500(b *testing.B) {
	s := testStore(b, benchDir(b, 500))
	list := s.list()
	b.ResetTimer()
	for b.Loop() {
		indexPage(list)
	}
}

// Everything read again, as at the start and after the kernel's queue
// overflows.
func BenchmarkRescan500(b *testing.B) {
	s := testStore(b, benchDir(b, 500))
	b.ResetTimer()
	for b.Loop() {
		s.syncMu.Lock()
		s.rescan()
		s.syncMu.Unlock()
	}
}

func BenchmarkNoteCached(b *testing.B) {
	h := testStore(b, benchDir(b, 500)).handler()
	get(b, h, "/")
	b.ResetTimer()
	for b.Loop() {
		get(b, h, "/notes/note-000.md")
	}
}

func largestSample(b *testing.B) string {
	paths, _ := filepath.Glob("notes/*.md")
	var biggest []byte
	for _, p := range paths {
		src, _ := os.ReadFile(p)
		if len(src) > len(biggest) {
			biggest = src
		}
	}
	b.SetBytes(int64(len(biggest)))
	return string(biggest)
}

// BenchmarkRenderLargest renders the biggest sample cold: the cost of the
// first request after the note changes.
func BenchmarkRenderLargest(b *testing.B) {
	text := largestSample(b)
	for b.Loop() {
		Render(text)
	}
}

// The same with the blocks marked, and then indexed, as the server does.
func BenchmarkRenderBlocksLargest(b *testing.B) {
	text := largestSample(b)
	for b.Loop() {
		RenderBlocks(text)
	}
}

func BenchmarkRenderIndexLargest(b *testing.B) {
	text := largestSample(b)
	for b.Loop() {
		_, bs := RenderBlocks(text)
		indexBlocks(bs)
	}
}

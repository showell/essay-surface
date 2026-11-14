package main

import (
	"encoding/json"
	"html"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The test binary stands in for render-note when ESSAY_FAKE_RENDERER says
// how: "echo" prints a page of one paragraph holding the arguments joined,
// as render-note joins them; "fail" exits 1 saying why; "junk" prints what
// isn't a page. The real renderer is tested by format/'s expects and run
// through the server by scripts/note_check.shale.
func TestMain(m *testing.M) {
	switch os.Getenv("ESSAY_FAKE_RENDERER") {
	case "":
		os.Exit(m.Run())
	case "echo":
		text := strings.Join(os.Args[1:], "")
		body := `<h1 data-c="0">Fake</h1>` + "\n" + `<p data-c="1">` + html.EscapeString(text) + "</p>\n"
		out, _ := json.Marshal(map[string]any{
			"title":    "Fake &amp; title",
			"body":     body,
			"blocks":   []map[string]any{{"text": "Fake", "end": strings.Index(body, "\n")}, {"text": text, "end": len(body) - 1}},
			"diagrams": strings.Count(text, "digraph"),
		})
		os.Stdout.Write(out)
	case "fail":
		os.Stderr.WriteString("Roc crashed: a planted crash\n")
		os.Exit(1)
	case "junk":
		os.Stdout.WriteString("<html>not json</html>")
	}
	os.Exit(0)
}

func fakeStore(t *testing.T, dir, how string) *store {
	t.Setenv("ESSAY_FAKE_RENDERER", how)
	s, err := newStore(dir, filepath.Join(dir, "comments"), os.Args[0])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.close() })
	return s
}

func TestNoteNames(t *testing.T) {
	for _, name := range []string{"a.md", "a.note", "format-demo.note", "x.y.md"} {
		if !validName(name) {
			t.Errorf("%q is a note's file name", name)
		}
	}
	for _, name := range []string{".md", ".note", "a.txt", "a.notes", "a", "a/b.note", `a\b.note`, ".a.note", "a.note.jsonl"} {
		if validName(name) {
			t.Errorf("%q is no note's file name", name)
		}
	}
	for _, name := range []string{"a", "format-demo", "9-lives"} {
		if !noteName(name) {
			t.Errorf("%q is a note's name", name)
		}
	}
	for _, name := range []string{"", "-a", "A", "a.md", "a_b", "a/b", "é"} {
		if noteName(name) {
			t.Errorf("%q is no note's name", name)
		}
	}
}

// A note's text is cut below Linux's limit on one argument, never inside a
// character, and the renderer joins the parts back to the text.
func TestRenderPartsJoinToTheText(t *testing.T) {
	for _, src := range []string{"", "short", strings.Repeat("a", renderArgBytes), strings.Repeat("a", renderArgBytes+1),
		strings.Repeat("é", renderArgBytes), strings.Repeat("ab€", renderArgBytes)} {
		parts := renderParts(src)
		if strings.Join(parts, "") != src {
			t.Errorf("%d bytes: the parts don't join back to the text", len(src))
		}
		for _, p := range parts {
			if len(p) > renderArgBytes {
				t.Errorf("%d bytes: a part of %d bytes", len(src), len(p))
			}
			if strings.ToValidUTF8(p, "�") != p {
				t.Errorf("%d bytes: a part cut inside a character", len(src))
			}
		}
	}
	if n := len(renderParts(strings.Repeat("a", 3*renderArgBytes+1))); n != 4 {
		t.Errorf("3 parts and a byte make %d parts", n)
	}
}

func TestReadRendering(t *testing.T) {
	good := `{"blocks":[{"end":5,"text":"a"},{"end":9,"text":"b"}],"body":"<p>a</p><p>b</p>","diagrams":0,"title":"T"}`
	r, err := readRendering([]byte(good))
	if err != nil || r.Title != "T" || len(r.Blocks) != 2 || r.Blocks[1].End != 9 {
		t.Fatalf("a good page: %+v, %v", r, err)
	}
	for what, bad := range map[string]string{
		"not json":             `<p>`,
		"an unknown field":     `{"blocks":[],"body":"","diagrams":0,"title":"T","extra":1}`,
		"ends out of order":    `{"blocks":[{"end":9,"text":"a"},{"end":5,"text":"b"}],"body":"<p>a</p><p>b</p>","diagrams":0,"title":"T"}`,
		"an end past the body": `{"blocks":[{"end":17,"text":"a"}],"body":"<p>a</p><p>b</p>","diagrams":0,"title":"T"}`,
		"a negative end":       `{"blocks":[{"end":-1,"text":"a"}],"body":"<p>a</p>","diagrams":0,"title":"T"}`,
	} {
		if _, err := readRendering([]byte(bad)); err == nil {
			t.Errorf("%s: read as a page", what)
		}
	}
	// The end of the body itself is inside it.
	if _, err := readRendering([]byte(`{"blocks":[{"end":16,"text":"a"}],"body":"<p>a</p><p>b</p>","diagrams":0,"title":"T"}`)); err != nil {
		t.Errorf("an end at the body's end: %v", err)
	}
}

// A .note is rendered by the renderer, its text passed in parts, and takes
// comments like any note; a note too big for one argument arrives whole.
func TestNoteRenderedAndCommented(t *testing.T) {
	dir := t.TempDir()
	big := "# Big\n\n" + strings.Repeat("word ", renderArgBytes/2) + "digraph"
	writeNote(t, dir, "big.note", big, time.Now())
	writeNote(t, dir, "plain.md", "# Plain\n\nA paragraph.\n", time.Now())
	h := fakeStore(t, dir, "echo").handler()
	_, index := get(t, h, "/")
	if !strings.Contains(index, `<a href="/notes/big.note">Fake &amp; title</a>`) || !strings.Contains(index, "2 notes") {
		t.Errorf("the index doesn't list the .note by its title:\n%s", index)
	}
	code, page := get(t, h, "/notes/big.note")
	if code != 200 || !strings.Contains(page, html.EscapeString(big)) {
		t.Fatalf("the page doesn't hold the whole text: %d", code)
	}
	if !strings.Contains(page, formatStyle) || !strings.Contains(page, diagramScripts) {
		t.Error("a .note with a diagram lacks its style or its scripts")
	}
	_, md := get(t, h, "/notes/plain.md")
	if strings.Contains(md, formatStyle) || strings.Contains(md, "viz-standalone") {
		t.Error("a markdown note's page gained a .note's style or scripts")
	}
	hashes := blockHashes(t, page)
	if len(hashes) != 2 {
		t.Fatalf("%d blocks, want the fake's 2", len(hashes))
	}
	req := httptest.NewRequest("POST", "/notes/big.note/comments", strings.NewReader(`{"block":0,"hash":"`+hashes[0]+`","name":"Steve","text":"On the title."}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != 201 {
		t.Fatalf("a comment on a .note: %d %s", rec.Code, rec.Body.String())
	}
	if _, page := get(t, h, "/notes/big.note"); !strings.Contains(page, `Fake</h1><div class="cmts" data-for="0">`) {
		t.Error("the comment doesn't follow its block")
	}
}

// A .note with no scripts to add (no diagram) gets none.
func TestNoteWithoutDiagramHasNoScripts(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.note", "# A\n\nWords.\n", time.Now())
	_, page := get(t, fakeStore(t, dir, "echo").handler(), "/notes/a.note")
	if !strings.Contains(page, formatStyle) || strings.Contains(page, "viz-standalone") {
		t.Error("a .note without a diagram: want its style and no viz.js")
	}
}

// A renderer that fails, or answers what isn't a page, or isn't there, fails
// that note's page alone: it says why, takes no comments, and the rest serve.
func TestNoteThatCannotRenderFailsAlone(t *testing.T) {
	for how, why := range map[string]string{
		"fail": "Roc crashed: a planted crash",
		"junk": "the renderer&#39;s answer is not a page",
	} {
		dir := t.TempDir()
		writeNote(t, dir, "broken.note", "# Broken\n", time.Now())
		writeNote(t, dir, "plain.md", "# Plain\n", time.Now())
		h := fakeStore(t, dir, how).handler()
		code, page := get(t, h, "/notes/broken.note")
		if code != 200 || !strings.Contains(page, `<p class="note-problem">This note can't be shown: `) || !strings.Contains(page, why) {
			t.Errorf("%s: the page doesn't say why: %d\n%s", how, code, page)
		}
		if len(blockHashes(t, page)) != 0 {
			t.Errorf("%s: a page that can't be shown has blocks", how)
		}
		if _, index := get(t, h, "/"); !strings.Contains(index, `href="/notes/broken.note">broken<`) || !strings.Contains(index, "Plain") {
			t.Errorf("%s: the index lacks a note", how)
		}
	}
	dir := t.TempDir()
	writeNote(t, dir, "a.note", "# A\n", time.Now())
	if _, page := get(t, testStore(t, dir).handler(), "/notes/a.note"); !strings.Contains(page, "no renderer for .note files") {
		t.Error("with no renderer, the page doesn't say so")
	}
}

// A link by a note's name goes to its .note when there is one, else its .md.
func TestLinkByNameFindsTheNote(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "both.md", "# Both, md\n", time.Now())
	writeNote(t, dir, "both.note", "# Both\n", time.Now())
	writeNote(t, dir, "only-md.md", "# Only md\n", time.Now())
	h := fakeStore(t, dir, "echo").handler()
	for path, want := range map[string]string{"/notes/both": "/notes/both.note", "/notes/only-md": "/notes/only-md.md"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 302 || rec.Header().Get("Location") != want {
			t.Errorf("%s: %d to %q, want 302 to %q", path, rec.Code, rec.Header().Get("Location"), want)
		}
	}
	if code, _ := get(t, h, "/notes/missing"); code != 404 {
		t.Errorf("a link to no note: %d", code)
	}
}

func TestAssetsAskedAgainByETag(t *testing.T) {
	h := testStore(t, t.TempDir()).handler()
	for path, content := range map[string][]byte{"/assets/viz-standalone.js": vizJS, "/assets/diagrams.js": diagramsJS} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != 200 || rec.Body.String() != string(content) || len(content) < 1000 || rec.Header().Get("ETag") == "" {
			t.Fatalf("%s: %d, %d bytes", path, rec.Code, rec.Body.Len())
		}
		req := httptest.NewRequest("GET", path, nil)
		req.Header.Set("If-None-Match", rec.Header().Get("ETag"))
		again := httptest.NewRecorder()
		h.ServeHTTP(again, req)
		if again.Code != 304 || again.Body.Len() != 0 {
			t.Errorf("%s asked again by its ETag: %d", path, again.Code)
		}
	}
}

func blockHashes(t *testing.T, page string) []string {
	start := strings.Index(page, `<script id="blocks" type="application/json">`)
	if start < 0 {
		t.Fatal("the page has no block fingerprints")
	}
	rest := page[start+len(`<script id="blocks" type="application/json">`):]
	var hashes []string
	if err := json.Unmarshal([]byte(rest[:strings.Index(rest, "</script>")]), &hashes); err != nil {
		t.Fatal(err)
	}
	return hashes
}

package main

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The comments folder may be missing at the start, but its parent must be
// a folder: the store refuses to start, naming it, and makes nothing.
func TestCommentsParentMustExist(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", "# A\n", time.Now())
	file := filepath.Join(dir, "a.md")
	for _, parent := range []string{filepath.Join(dir, "missing"), file} {
		s, err := newStore(dir, filepath.Join(parent, "comments"), "")
		if err == nil {
			s.close()
			t.Fatalf("%s: the store started", parent)
		}
		if want := "the comments folder's parent " + parent + " is not a folder"; !strings.Contains(err.Error(), want) {
			t.Errorf("%s: said %q, want it to say %q", parent, err, want)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "missing")); !os.IsNotExist(err) {
		t.Error("the missing parent was made")
	}
	s, err := newStore(dir, filepath.Join(dir, "comments"), "")
	if err != nil {
		t.Fatalf("a parent that is there, its comments folder not yet: %v", err)
	}
	s.close()
}

// The index page is made once and kept until a note changes. A comment
// changes a note's page but never the index.
func TestIndexKeptUntilANoteChanges(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", commentNote, time.Now())
	s := testStore(t, dir)
	h := s.handler()
	_, page := get(t, h, "/notes/a.md")
	hs := hashesOf(t, page)
	for range 3 {
		if code, _ := get(t, h, "/"); code != 200 {
			t.Fatalf("index: %d", code)
		}
	}
	if n := s.indexBuilds.Load(); n != 1 {
		t.Errorf("three requests made the index %d times, want 1", n)
	}
	post(t, h, "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], "Ann", "a comment"))
	get(t, h, "/notes/a.md")
	get(t, h, "/")
	if n := s.indexBuilds.Load(); n != 1 {
		t.Errorf("a comment made the index again (%d times)", n)
	}
	writeNote(t, dir, "a.md", strings.Replace(commentNote, "# Note", "# Renamed", 1), time.Now())
	if _, body := get(t, h, "/"); !strings.Contains(body, "Renamed") {
		t.Error("the index kept a changed title")
	}
	if n := s.indexBuilds.Load(); n != 2 {
		t.Errorf("after an edit the index was made %d times, want 2", n)
	}
}

// An edit that keeps a file's size and modified time is still heard. The
// kernel's clock for file times moves in ticks of a few milliseconds, so
// two saves in one tick look the same to stat.
func TestEditInOneClockTick(t *testing.T) {
	dir := t.TempDir()
	when := time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)
	writeNote(t, dir, "a.md", "# AAAA\n", when)
	h := testStore(t, dir).handler()
	if _, body := get(t, h, "/notes/a.md"); !strings.Contains(body, "AAAA") {
		t.Fatal("first version")
	}
	writeNote(t, dir, "a.md", "# BBBB\n", when)
	if _, body := get(t, h, "/notes/a.md"); !strings.Contains(body, "BBBB") || strings.Contains(body, "AAAA") {
		t.Error("an edit of the same size and time was not heard")
	}
}

// A note added, renamed and removed under the running store.
func TestNotesComeAndGo(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", "# A\n", time.Now())
	h := testStore(t, dir).handler()
	get(t, h, "/")
	writeNote(t, dir, "c.md", "# See\n", time.Now())
	if _, body := get(t, h, "/"); !strings.Contains(body, "See") || !strings.Contains(body, "2 notes") {
		t.Error("an added note is not in the index")
	}
	if code, _ := get(t, h, "/notes/c.md"); code != 200 {
		t.Errorf("an added note: %d", code)
	}
	if err := os.Rename(filepath.Join(dir, "c.md"), filepath.Join(dir, "d.md")); err != nil {
		t.Fatal(err)
	}
	if code, _ := get(t, h, "/notes/c.md"); code != 404 {
		t.Errorf("a note renamed away: %d", code)
	}
	if code, _ := get(t, h, "/notes/d.md"); code != 200 {
		t.Errorf("a note renamed in: %d", code)
	}
	if _, body := get(t, h, "/"); !strings.Contains(body, `href="/notes/d.md"`) || strings.Contains(body, `href="/notes/c.md"`) {
		t.Error("the index did not follow a rename")
	}
	os.Remove(filepath.Join(dir, "d.md"))
	if _, body := get(t, h, "/"); strings.Contains(body, "d.md") || !strings.Contains(body, "1 notes") {
		t.Error("the index still lists a removed note")
	}
	// A folder named like a note is not one.
	os.Mkdir(filepath.Join(dir, "e.md"), 0o755)
	if code, _ := get(t, h, "/notes/e.md"); code != 404 {
		t.Errorf("a folder named e.md: %d", code)
	}
}

// The comments folder removed and made again under the running store: its
// watch is dropped with it and made again, and the pages follow.
func TestCommentsFolderRemovedAndMadeAgain(t *testing.T) {
	dir, h, hs := setup(t)
	post(t, h, "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], "Ann", "first"))
	if _, page := get(t, h, "/notes/a.md"); !strings.Contains(page, ">first<") {
		t.Fatal("the posted comment is not on the page")
	}
	cdir := filepath.Join(dir, "comments")
	if err := os.RemoveAll(cdir); err != nil {
		t.Fatal(err)
	}
	if _, page := get(t, h, "/notes/a.md"); strings.Contains(page, ">first<") {
		t.Error("a removed comments folder's comment is still on the page")
	}
	os.Mkdir(cdir, 0o755)
	if code, body := post(t, h, "/notes/a.md/comments", "application/json", commentJSON(1, hs[1], "Bo", "second")); code != 201 {
		t.Fatalf("a post to the new folder: %d %s", code, body)
	}
	if _, page := get(t, h, "/notes/a.md"); !strings.Contains(page, ">second<") {
		t.Error("a comment in the folder made again is not on the page")
	}
	// A comment written straight into the new folder is heard too, so its
	// watch was made again.
	src, _ := os.ReadFile(filepath.Join(cdir, "a.md.jsonl"))
	os.WriteFile(filepath.Join(cdir, "a.md.jsonl"), []byte(strings.Replace(string(src), "second", "edited by hand", 1)), 0o644)
	if _, page := get(t, h, "/notes/a.md"); !strings.Contains(page, ">edited by hand<") {
		t.Error("a hand edit in the folder made again was not heard")
	}
}

// When more changes come than the kernel will queue, it drops the rest and
// says so; the store then reads everything again. The edit here comes
// after the queue is full, so the rescan is the only way to see it.
func TestQueueOverflowRescans(t *testing.T) {
	src, err := os.ReadFile("/proc/sys/fs/inotify/max_queued_events")
	if err != nil {
		t.Fatal(err)
	}
	most, err := strconv.Atoi(strings.TrimSpace(string(src)))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	writeNote(t, dir, "a.md", "# A\n", time.Now())
	writeNote(t, dir, "b.md", "# B\n", time.Now())
	writeNote(t, dir, "c.md", "# Before\n", time.Now())
	s := testStore(t, dir)
	h := s.handler()
	get(t, h, "/")
	// The kernel folds an event into the one before it when they are the
	// same, so the touches take turns between two files.
	pa, pb := filepath.Join(dir, "a.md"), filepath.Join(dir, "b.md")
	for k := 0; k < most/2+100; k++ {
		now := time.Now()
		os.Chtimes(pa, now, now)
		os.Chtimes(pb, now, now)
	}
	writeNote(t, dir, "c.md", "# After the flood\n", time.Now())
	if _, body := get(t, h, "/"); !strings.Contains(body, "After the flood") {
		t.Error("an edit made after the queue overflowed was not seen")
	}
	if n := s.rescans.Load(); n != 2 {
		t.Errorf("%d rescans, want 2: the start and the overflow", n)
	}
}

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNoteDate(t *testing.T) {
	cases := []struct {
		name, src, want string // want "" for no date
	}{
		{"an italic line after the title", "# T\n\n*2026-11-07. Branch x.*\n\nBody 2025-01-01.\n", "2026-11-07"},
		{"a byline with words first", "# T\n\n*For Steve, 2026-11-10. Six chapters.*\n", "2026-11-10"},
		{"in the title", "# Catch-up, late night 2026-10-10\n\n*Claude, 2026-10-11.*\n", "2026-10-10"},
		{"the paragraph's later line", "# T\n\nWritten for the book,\non 2026-09-01, before any code.\n", "2026-09-01"},
		{"no title", "*2026-08-27, near midnight.*\n\nText.\n", "2026-08-27"},
		{"blank lines first", "\n\n# T\n\n\n2026-05-06 notes\n", "2026-05-06"},
		{"only in the second paragraph", "# T\n\nNo date here.\n\nOn 2026-01-02 we met.\n", ""},
		{"none at all", "# T\n\nText.\n", ""},
		{"not a real date, then one", "# T\n\n*2026-13-01 is no month; 2026-02-03 is.*\n", "2026-02-03"},
		{"part of a longer number", "# T\n\nBuild 12026-11-07 and 2026-11-079.\n", ""},
		{"only a title", "# 2026-11-02: the day", "2026-11-02"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		d, ok := noteDate(c.src)
		got := ""
		if ok {
			got = d.Format("2006-01-02")
		}
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.name, got, c.want)
		}
	}
	// A date past the bytes looked at is not seen.
	long := "# T\n\n" + strings.Repeat("word ", dateLookBytes/5) + "2026-01-01\n"
	if _, ok := noteDate(long); ok {
		t.Error("a date past dateLookBytes was taken")
	}
}

// git runs git in dir for a test, its config and dates fixed.
func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.org", "-c", "commit.gpgsign=false"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_DATE=2026-01-02T03:04:05Z", "GIT_COMMITTER_DATE=2026-01-02T03:04:05Z")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

// Each of the three sources, and the order they make. Every file's time is
// set to mislead, as a fresh clone's would: only the untracked note may
// use it.
func TestDatesFromNoteGitAndFile(t *testing.T) {
	repo := t.TempDir()
	dir := filepath.Join(repo, "notes")
	os.Mkdir(dir, 0o755)
	misleading := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	writeNote(t, dir, "dated.md", "# Dated\n\n*2026-03-04. A byline.*\n", misleading)
	writeNote(t, dir, "committed.md", "# Committed\n\nNo date.\n", misleading)
	writeNote(t, dir, "later.md", "# Later\n\nNo date.\n", misleading)
	git(t, repo, "init", "-q")
	git(t, repo, "add", ".")
	git(t, repo, "commit", "-q", "-m", "notes")
	// later.md is in git but not in the folder when the store starts: it
	// comes back after, as a checkout brings a note.
	os.Remove(filepath.Join(dir, "later.md"))
	writeNote(t, dir, "untracked.md", "# Untracked\n\nNo date.\n", time.Date(2025, 6, 7, 8, 0, 0, 0, time.UTC))

	s := testStore(t, dir)
	h := s.handler()
	git(t, repo, "checkout", "--", "notes/later.md")
	_, body := get(t, h, "/")
	want := []struct{ name, date string }{{"dated.md", "2026-03-04"}, {"committed.md", "2026-01-02"}, {"later.md", "2026-01-02"}, {"untracked.md", "2025-06-07"}}
	at := -1
	for _, w := range want {
		i := strings.Index(body, `href="/notes/`+w.name+`"`)
		if i < 0 {
			t.Fatalf("%s is not in the index", w.name)
		}
		if i < at {
			t.Errorf("%s is out of order", w.name)
		}
		at = i
		// The day leads the date; a minute follows it when the file changed that day.
		if line := body[i:min(len(body), i+200)]; !strings.Contains(line, `<span class="date">`+w.date) {
			t.Errorf("%s: want %s in %q", w.name, w.date, line)
		}
	}
	from := map[string]dateFrom{"dated.md": dateFromNote, "committed.md": dateFromGit, "later.md": dateFromGit, "untracked.md": dateFromFile}
	for name, f := range from {
		if n := s.lookup(name); n == nil {
			t.Errorf("%s is not in the store", name)
		} else if n.from != f {
			t.Errorf("%s: dated from its %s, want its %s", name, n.from, f)
		}
	}
	if _, page := get(t, h, "/notes/committed.md"); !strings.Contains(page, `<span class="date">2026-01-02</span>`) {
		t.Error("a note's page does not carry its git date")
	}
}

// Outside a repository every undated note is dated by its file, and git
// is asked only once.
func TestDatesWithoutGit(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", "# A\n", time.Date(2026, 2, 3, 0, 0, 0, 0, time.UTC))
	s := testStore(t, dir)
	if n := s.lookup("a.md"); n == nil || n.from != dateFromFile {
		t.Fatalf("a.md: %v", n)
	}
	if s.gitErr == nil {
		t.Error("git's refusal was not kept")
	}
	took := s.gitTook
	writeNote(t, dir, "b.md", "# B\n", time.Now())
	get(t, s.handler(), "/")
	if s.gitTook != took {
		t.Error("git was asked again after it said there is no repository")
	}
}

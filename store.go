package main

// The store holds every note of the folder rendered, each note's page with
// its comments once asked for, and the index page. It hears of changes
// from the kernel (watch.go) instead of looking for them: a request first
// drains what the kernel queued since the last one, which is one read when
// nothing changed, and brings up to date only what changed. A note's file
// is read when it changes, never to check whether it has.

import (
	"errors"
	"fmt"
	"html"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
)

// A note is never changed once in the map: a new rendering, a page made or
// a page dropped is a new note in the map, so a reader holding the old one
// is safe.
type note struct {
	name     string
	title    string // HTML-escaped text, no tags
	date     time.Time
	from     dateFrom
	modified time.Time // the file's, the last of the three dates
	body     string    // the rendered note, blocks marked
	blocks   []Block   // its commentable blocks
	style    string    // what its page adds to the style: a .note's, or nothing
	scripts  string    // what its page adds to the scripts: a .note's diagrams, or nothing
	page     []byte    // the whole page with its comments, or nil until made
}

type store struct {
	dir      string // the notes
	comments string // the comments, one file a note
	renderer string // the program that renders a .note (note.go)
	watch    *watcher

	// The watches: the notes folder, the comments folder (-1 while it is
	// missing), and the folder that holds the comments folder, which says
	// when it is made. Two may be one watch.
	notesWd, commentsWd, parentWd int32

	// syncMu is held while changes are drained and applied, and while the
	// index is made. What is drained waits in pending until applied, so a
	// change whose note cannot be read yet is tried again next time.
	syncMu       sync.Mutex
	pendingNotes map[string]bool
	pendingPages map[string]bool
	pendingAll   bool // every page, after the comments folder came or went
	index        []byte
	gitWhen      map[string]time.Time // each name git was asked about: its last commit, or zero
	gitErr       error                // errNoGit once git has said so
	gitTook      time.Duration

	mu    sync.RWMutex // the map, for readers
	notes map[string]*note

	write sync.Mutex // one change to the comment files at a time

	// How often the index page was made and the folder read whole, for the
	// tests.
	indexBuilds, rescans atomic.Int64
}

// newStore watches the folders and renders every note, a .note with the
// renderer. The comments folder may be missing, but not the folder it goes in.
func newStore(dir, comments, renderer string) (*store, error) {
	// The parent is checked, never made: a mistyped -comments would
	// otherwise grow a folder of comments somewhere nobody looks.
	parent := filepath.Dir(comments)
	if info, err := os.Stat(parent); err != nil || !info.IsDir() {
		return nil, fmt.Errorf("the comments folder's parent %s is not a folder; make it, or pass another -comments", parent)
	}
	w, err := newWatcher()
	if err != nil {
		return nil, err
	}
	s := &store{dir: dir, comments: comments, renderer: renderer, watch: w, commentsWd: -1,
		pendingNotes: map[string]bool{}, pendingPages: map[string]bool{},
		gitWhen: map[string]time.Time{}, notes: map[string]*note{}}
	if s.parentWd, err = w.add(filepath.Dir(comments)); err != nil {
		w.close()
		return nil, fmt.Errorf("the comments folder's parent: %w", err)
	}
	if err := s.rescan(); err != nil {
		w.close()
		return nil, err
	}
	return s, nil
}

func (s *store) close() error { return s.watch.close() }

// rescan reads the whole folder again, as at the start, and after the
// kernel's queue overflowed. The watches come before the reading, so a
// change made while the folder is read is heard after.
func (s *store) rescan() error {
	s.rescans.Add(1)
	var err error
	if s.notesWd, err = s.watch.add(s.dir); err != nil {
		return err
	}
	if err := s.watchComments(); err != nil {
		return err
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return err
	}
	fresh := make([]*note, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !validName(e.Name()) {
			continue
		}
		n, err := s.load(e.Name())
		if err != nil {
			return err
		}
		if n != nil {
			fresh = append(fresh, n)
		}
	}
	s.datesFromGit(fresh)
	notes := make(map[string]*note, len(fresh))
	for _, n := range fresh {
		notes[n.name] = n
	}
	s.mu.Lock()
	s.notes = notes
	s.mu.Unlock()
	s.index = nil
	clear(s.pendingNotes)
	clear(s.pendingPages)
	s.pendingAll = false
	return nil
}

// watchComments watches the comments folder if it is there.
func (s *store) watchComments() error {
	if s.commentsWd >= 0 {
		return nil
	}
	wd, err := s.watch.add(s.comments)
	if errors.Is(err, syscall.ENOENT) {
		return nil
	}
	if err != nil {
		return err
	}
	s.commentsWd = wd
	return nil
}

// load reads and renders a note; a note no longer there is nil.
func (s *store) load(name string) (*note, error) {
	path := filepath.Join(s.dir, name)
	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) || err == nil && !info.Mode().IsRegular() {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	text := string(src)
	n := &note{name: name, modified: info.ModTime()}
	if d, ok := noteDate(text); ok {
		n.date, n.from = d, dateFromNote
	} else {
		n.date, n.from = n.modified, dateFromFile
	}
	if strings.HasSuffix(name, formatExt) {
		n.loadFormat(s.renderer, text)
	} else {
		n.title = TitleHTML(Title(text))
		if n.title == "" {
			n.title = html.EscapeString(strings.TrimSuffix(name, markdownExt))
		}
		n.body, n.blocks = RenderBlocks(text)
	}
	indexBlocks(n.blocks)
	return n, nil
}

// datesFromGit dates the notes that do not date themselves by their last
// commit. Git is asked once for every name it has not been asked about.
func (s *store) datesFromGit(fresh []*note) {
	var ask []string
	for _, n := range fresh {
		if _, asked := s.gitWhen[n.name]; n.from != dateFromNote && !asked {
			ask = append(ask, n.name)
		}
	}
	if len(ask) > 0 && s.gitErr == nil {
		start := time.Now()
		got, err := gitDates(s.dir, ask)
		s.gitTook += time.Since(start)
		if errors.Is(err, errNoGit) {
			s.gitErr = err
		} else if err != nil {
			log.Printf("dates: %v", err)
		}
		for _, name := range ask {
			s.gitWhen[name] = got[name]
		}
	}
	for _, n := range fresh {
		if t := s.gitWhen[n.name]; n.from != dateFromNote && !t.IsZero() {
			n.date, n.from = t, dateFromGit
		}
	}
}

// sync brings the store up to date with what the kernel says changed.
func (s *store) sync() error {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	return s.syncLocked()
}

func (s *store) syncLocked() error {
	changes, overflow, err := s.watch.drain()
	if err != nil {
		return err
	}
	if overflow {
		return s.rescan()
	}
	base := filepath.Base(s.comments)
	for _, c := range changes {
		self := c.mask&selfMask != 0
		if c.wd == s.notesWd {
			if self {
				return s.rescan()
			}
			if validName(c.name) {
				s.pendingNotes[c.name] = true
			}
		}
		if c.wd == s.commentsWd {
			if self {
				s.commentsWd = -1
				s.pendingAll = true
			} else if name, ok := strings.CutSuffix(c.name, ".jsonl"); ok {
				s.pendingPages[name] = true
			}
		}
		if c.wd == s.parentWd && c.name == base {
			// The comments folder came, went or was renamed: its files
			// may have changed without a word from its own watch.
			if c.mask&(syscall.IN_DELETE|syscall.IN_MOVED_FROM) != 0 && s.commentsWd >= 0 {
				s.watch.remove(s.commentsWd)
				s.commentsWd = -1
			}
			if err := s.watchComments(); err != nil {
				return err
			}
			s.pendingAll = true
		}
	}
	return s.apply()
}

// apply brings in what is pending: the notes that changed read again, and
// the pages whose comments changed dropped, to be made on their next
// request.
func (s *store) apply() error {
	if len(s.pendingNotes) == 0 && len(s.pendingPages) == 0 && !s.pendingAll {
		return nil
	}
	var fresh []*note
	var gone []string
	for name := range s.pendingNotes {
		n, err := s.load(name)
		if err != nil {
			return err
		}
		if n == nil {
			gone = append(gone, name)
		} else {
			fresh = append(fresh, n)
		}
	}
	s.datesFromGit(fresh)
	s.mu.Lock()
	for _, name := range gone {
		delete(s.notes, name)
	}
	for _, n := range fresh {
		s.notes[n.name] = n
	}
	// A page dropped is always a new note in the map, even one with no page
	// yet: a page being made from the old comments is then not kept.
	for name, n := range s.notes {
		if s.pendingAll || s.pendingPages[name] {
			m := *n
			m.page = nil
			s.notes[name] = &m
		}
	}
	s.mu.Unlock()
	if len(fresh)+len(gone) > 0 {
		s.index = nil
	}
	clear(s.pendingNotes)
	clear(s.pendingPages)
	s.pendingAll = false
	return nil
}

func (s *store) lookup(name string) *note {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.notes[name]
}

// note gives a note as it is now, or nil for none.
func (s *store) note(name string) (*note, error) {
	if err := s.sync(); err != nil {
		return nil, err
	}
	return s.lookup(name), nil
}

// page gives the note with its page made from its comments as they are
// now. Placing the comments may heal some (anchor.go), and then the
// comments file is written: this happens only after the note changed.
func (s *store) page(name string) (*note, error) {
	for {
		n, err := s.note(name)
		if n == nil || err != nil || n.page != nil {
			return n, err
		}
		if m, err := s.makePage(name, n); m != nil || err != nil {
			return m, err
		}
	}
}

// makePage makes n's page, or gives nil when n is no longer the store's
// (a page was made, or the note changed, while this waited for the lock).
func (s *store) makePage(name string, n *note) (*note, error) {
	s.write.Lock()
	defer s.write.Unlock()
	if s.lookup(name) != n {
		return nil, nil
	}
	path := commentsPath(s.comments, name)
	cs, err := readComments(path)
	if err != nil {
		return nil, err
	}
	places := resolve(n.blocks, cs)
	healed := false
	for i, c := range cs {
		if heal(c, n.blocks, places[i]) {
			healed = true
		}
	}
	if healed {
		if err := writeComments(path, cs); err != nil {
			return nil, err
		}
	}
	m := *n
	m.page = notePage(&m, cs, places)
	s.mu.Lock()
	if s.notes[name] == n {
		s.notes[name] = &m
	}
	s.mu.Unlock()
	return &m, nil
}

// list gives every note, newest first, then by name.
func (s *store) list() []*note {
	s.mu.RLock()
	list := make([]*note, 0, len(s.notes))
	for _, n := range s.notes {
		list = append(list, n)
	}
	s.mu.RUnlock()
	sort.Slice(list, func(a, b int) bool {
		if !list[a].date.Equal(list[b].date) {
			return list[a].date.After(list[b].date)
		}
		return list[a].name < list[b].name
	})
	return list
}

// indexPage gives the index page, made again only after a note changed.
func (s *store) indexPage() ([]byte, error) {
	s.syncMu.Lock()
	defer s.syncMu.Unlock()
	if err := s.syncLocked(); err != nil {
		return nil, err
	}
	if s.index == nil {
		s.index = indexPage(s.list())
		s.indexBuilds.Add(1)
	}
	return s.index, nil
}

// dateSources counts the notes by where their dates came from.
func (s *store) dateSources() map[dateFrom]int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := map[dateFrom]int{}
	for _, n := range s.notes {
		out[n.from]++
	}
	return out
}

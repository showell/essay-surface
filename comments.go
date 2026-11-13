package main

// Comments on disk: one file a note, comments/<note>.jsonl, one comment a
// line in the order they were made. A line is one comment, so git's diff
// of the file is a list of comments added or moved; nothing else is in it.

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// commentsPath is the file that holds a note's comments.
func commentsPath(dir, note string) string { return filepath.Join(dir, note+".jsonl") }

// readComments reads a note's comments; a missing file is none. A line that
// is not a comment stops the read, naming the file and line: a comment file
// is hand-editable, and a mistake in one must be seen, not skipped.
func readComments(path string) ([]*Comment, error) {
	src, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []*Comment
	sc := bufio.NewScanner(bytes.NewReader(src))
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for n := 1; sc.Scan(); n++ {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		c := &Comment{}
		if err := dec.Decode(c); err != nil {
			return nil, fmt.Errorf("%s:%d: %v", path, n, err)
		}
		if c.ID == "" || c.Quote == "" {
			return nil, fmt.Errorf("%s:%d: a comment needs an id and a quote", path, n)
		}
		out = append(out, c)
	}
	return out, sc.Err()
}

// writeComments replaces a note's comments file whole: written beside it,
// then renamed over it, so a reader never sees half a file.
func writeComments(path string, cs []*Comment) error {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false) // the file is for reading: keep < and & as they are
	for _, c := range cs {
		if err := enc.Encode(c); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b.Bytes(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func newID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand does not fail on Linux
	}
	return hex.EncodeToString(b[:])
}

// The limits on a comment, in bytes after trimming.
const (
	maxName = 80
	maxText = 10000
)

// cleanComment trims a name and a text and checks them; the error is for
// the reader.
func cleanComment(name, text string) (string, string, error) {
	name, text = strings.TrimSpace(name), strings.TrimSpace(strings.ReplaceAll(text, "\r\n", "\n"))
	switch {
	case name == "" || text == "":
		return "", "", fmt.Errorf("a comment needs a name and a text")
	case len(name) > maxName:
		return "", "", fmt.Errorf("a name is at most %d bytes", maxName)
	case len(text) > maxText:
		return "", "", fmt.Errorf("a comment is at most %d bytes", maxText)
	case strings.ContainsAny(name, "\n\t"):
		return "", "", fmt.Errorf("a name is one line")
	}
	return name, text, nil
}

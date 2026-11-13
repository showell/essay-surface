package main

// A note's date orders the index and heads its page. It comes from the
// first of:
//
//  1. the note itself: the first date (YYYY-MM-DD) in its title or in the
//     paragraph right after the title, where most notes say when they were
//     written ("*2026-11-07. Branch sunset-rule, not merged yet.*"). It
//     counts as the start of that day.
//  2. git: the time of the last commit that touched the file.
//  3. the file's modified time, for a note git does not know.
//
// The file's time comes last because a fresh clone or a copy resets it.

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// dateFrom says which of the three a note's date came from.
type dateFrom string

const (
	dateFromNote dateFrom = "note"
	dateFromGit  dateFrom = "git"
	dateFromFile dateFrom = "file"
)

// The title and the paragraph after it are near the top, so a note's date
// is looked for in its first bytes only.
const dateLookBytes = 4096

// noteDate gives the first date in a note's title or in the paragraph that
// follows it.
func noteDate(src string) (time.Time, bool) {
	lines := strings.Split(src[:min(len(src), dateLookBytes)], "\n")
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i < len(lines) && strings.HasPrefix(lines[i], "#") {
		if d, ok := firstDate(lines[i]); ok {
			return d, true
		}
		i++
		for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
			i++
		}
	}
	for ; i < len(lines) && strings.TrimSpace(lines[i]) != ""; i++ {
		if d, ok := firstDate(lines[i]); ok {
			return d, true
		}
	}
	return time.Time{}, false
}

// firstDate gives the first real date written YYYY-MM-DD in s, not part of
// a longer run of digits. "2026-13-01" is not a date, and is passed over.
func firstDate(s string) (time.Time, bool) {
	for j := 0; j+10 <= len(s); j++ {
		if !dateShaped(s[j:j+10]) || (j > 0 && isDigit(s[j-1])) || (j+10 < len(s) && isDigit(s[j+10])) {
			continue
		}
		if d, err := time.ParseInLocation("2006-01-02", s[j:j+10], time.Local); err == nil {
			return d, true
		}
	}
	return time.Time{}, false
}

func dateShaped(s string) bool {
	for k := 0; k < 10; k++ {
		if k == 4 || k == 7 {
			if s[k] != '-' {
				return false
			}
		} else if !isDigit(s[k]) {
			return false
		}
	}
	return true
}

func isDigit(c byte) bool { return '0' <= c && c <= '9' }

// errNoGit is gitDates's error when dir is in no repository or git is not
// installed: every note there is dated by its file, and git is not asked
// again.
var errNoGit = errors.New("no git")

// gitDates gives the time of the last commit that touched each named file
// in dir, in one git log. A name git does not know is left out.
func gitDates(dir string, names []string) (map[string]time.Time, error) {
	out := make(map[string]time.Time, len(names))
	if len(names) == 0 {
		return out, nil
	}
	args := append([]string{"log", "--no-renames", "--relative", "--format=%x00%ct", "--name-only", "--"}, names...)
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_LITERAL_PATHSPECS=1")
	src, err := cmd.Output()
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 128 || errors.Is(err, exec.ErrNotFound) {
			return out, fmt.Errorf("%w: %v", errNoGit, err)
		}
		return out, fmt.Errorf("git log in %s: %v", dir, err)
	}
	var when time.Time
	for _, line := range strings.Split(string(src), "\n") {
		if t, ok := strings.CutPrefix(line, "\x00"); ok {
			sec, err := strconv.ParseInt(t, 10, 64)
			if err != nil {
				return out, fmt.Errorf("git log in %s: a commit time %q", dir, t)
			}
			when = time.Unix(sec, 0)
			continue
		}
		// The log is newest first, so a name's first commit is its last.
		if _, seen := out[line]; line != "" && !seen {
			out[line] = when
		}
	}
	return out, nil
}

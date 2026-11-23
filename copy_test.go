package main

import (
	"strings"
	"testing"
	"time"
)

// A note's page carries the copy buttons: copy.js after the article, so its
// code blocks are there when it runs, and the buttons' style. What the script
// does is copy_test.cjs's (under node) and the headless check's.
func TestCopyButtonsServed(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", "# A\n\n```go\nx := 1\n```\n", time.Now())
	_, page := get(t, testStore(t, dir).handler(), "/notes/a.md")
	// The places copy.js looks: a renamed tag or class would leave every
	// block without a button and every check green.
	for _, s := range []string{`querySelectorAll("article pre")`, `closest("figure.dot")`, `btn.className = "copybtn"`, `pin.className = "copypin"`} {
		if !strings.Contains(copyJS, s) {
			t.Errorf("copy.js no longer has %s", s)
		}
	}
	at := strings.Index(page, copyScript)
	end := strings.Index(page, "</article>")
	if at < 0 || end < 0 || at < end {
		t.Fatalf("the copy script is not after the article (script at %d, article ends at %d)", at, end)
	}
	if !strings.Contains(page[:end], `<pre`) {
		t.Error("the article has no pre for a button")
	}
	css := servedCSS(page)
	for _, rule := range []string{".copypin{", ".copybtn{"} {
		if !strings.Contains(css, rule) {
			t.Errorf("the page's CSS has no %s rule", rule)
		}
	}
}

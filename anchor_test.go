package main

import (
	"strconv"
	"strings"
	"testing"
)

func blocksOf(md string) []Block {
	_, bs := RenderBlocks(md)
	return bs
}

// on makes a comment on the block of md whose text starts with prefix.
func on(t *testing.T, md, prefix string) *Comment {
	t.Helper()
	bs := blocksOf(md)
	for k, b := range bs {
		if strings.HasPrefix(b.Text, prefix) {
			return newComment(bs, k, "c1", "2026-11-13T12:00:00Z", "Ann", "a comment")
		}
	}
	t.Fatalf("no block starts %q", prefix)
	return nil
}

// where resolves one comment against md and gives its block's text ("" for
// an orphan) and how it was found.
func where(c *Comment, md string) (string, how) {
	bs := blocksOf(md)
	p := resolve(bs, []*Comment{c})[0]
	if p.block < 0 {
		return "", p.how
	}
	return bs[p.block].Text, p.how
}

const draft = `# Title

The first paragraph says what the note is about.

The second paragraph has the claim a reader will argue with.

The third paragraph closes the argument.
`

func TestAnchorUnchanged(t *testing.T) {
	c := on(t, draft, "The second")
	if got, h := where(c, draft); !strings.HasPrefix(got, "The second") || h != exact {
		t.Errorf("got %q, %v", got, h)
	}
}

func TestAnchorParagraphAddedAbove(t *testing.T) {
	c := on(t, draft, "The second")
	edited := strings.Replace(draft, "The first", "A brand new opening paragraph.\n\nThe first", 1)
	if got, h := where(c, edited); !strings.HasPrefix(got, "The second") || h != exact {
		t.Errorf("got %q, %v", got, h)
	}
}

func TestAnchorTypoFixed(t *testing.T) {
	c := on(t, draft, "The second")
	edited := strings.Replace(draft, "will argue with", "will argue against", 1)
	got, h := where(c, edited)
	if got != "The second paragraph has the claim a reader will argue against." || h != inPlace {
		t.Errorf("got %q, %v", got, h)
	}
}

func TestAnchorRewrapped(t *testing.T) {
	c := on(t, draft, "The second")
	edited := strings.Replace(draft, "has the claim ", "has the claim\n", 1)
	if got, h := where(c, edited); !strings.HasPrefix(got, "The second") || h != exact {
		t.Errorf("rewrapping is no edit: got %q, %v", got, h)
	}
}

func TestAnchorRewrittenInPlace(t *testing.T) {
	// Every word new, but between the same neighbors: the same paragraph,
	// rewritten. The page marks it as on an earlier version.
	c := on(t, draft, "The second")
	edited := strings.Replace(draft, "The second paragraph has the claim a reader will argue with.", "Completely different words now.", 1)
	if got, h := where(c, edited); got != "Completely different words now." || h != inPlace {
		t.Errorf("got %q, %v", got, h)
	}
}

func TestAnchorMoved(t *testing.T) {
	c := on(t, draft, "The second")
	moved := "# Title\n\nThe first paragraph says what the note is about.\n\nThe third paragraph closes the argument.\n\n" +
		"The second paragraph has the claim a reader will argue with.\n"
	if got, h := where(c, moved); !strings.HasPrefix(got, "The second") || h != exact {
		t.Errorf("got %q, %v", got, h)
	}
}

func TestAnchorMovedAndEdited(t *testing.T) {
	c := on(t, draft, "The second")
	moved := "# Title\n\nThe first paragraph says what the note is about.\n\nThe third paragraph closes the argument.\n\n" +
		"The second paragraph has the claim a reader will surely argue with.\n"
	got, h := where(c, moved)
	if got != "The second paragraph has the claim a reader will surely argue with." || h != similar {
		t.Errorf("got %q, %v", got, h)
	}
}

func TestAnchorSplit(t *testing.T) {
	md := "# T\n\nOne two three four five six seven eight nine ten. Eleven twelve thirteen.\n\nAfter.\n"
	c := on(t, md, "One two")
	// The comment goes with the half that holds more of the old text.
	split := "# T\n\nOne two three four five six seven eight nine ten.\n\nEleven twelve thirteen.\n\nAfter.\n"
	if got, h := where(c, split); got != "One two three four five six seven eight nine ten." || h != similar {
		t.Errorf("got %q, %v", got, h)
	}
	other := "# T\n\nOne two three.\n\nFour five six seven eight nine ten. Eleven twelve thirteen.\n\nAfter.\n"
	if got, h := where(c, other); got != "Four five six seven eight nine ten. Eleven twelve thirteen." || h != similar {
		t.Errorf("got %q, %v", got, h)
	}
}

func TestAnchorDeleted(t *testing.T) {
	c := on(t, draft, "The second")
	deleted := strings.Replace(draft, "The second paragraph has the claim a reader will argue with.\n\n", "", 1)
	if got, h := where(c, deleted); got != "" || h != orphaned {
		t.Errorf("a deleted paragraph's comment went to %q, %v", got, h)
	}
	// Put back, it finds its paragraph again.
	if got, h := where(c, draft); !strings.HasPrefix(got, "The second") || h != exact {
		t.Errorf("restored: got %q, %v", got, h)
	}
}

func TestAnchorDeletedNextToLookalike(t *testing.T) {
	// A paragraph much like the deleted one is not it: below the bar.
	md := "Intro.\n\nThe build passes on Linux today.\n\nThe build fails on macOS today.\n\nEnd.\n"
	c := on(t, md, "The build passes")
	deleted := "Intro.\n\nThe build fails on macOS today.\n\nEnd.\n"
	if got, h := where(c, deleted); got != "" || h != orphaned {
		t.Errorf("got %q, %v", got, h)
	}
}

func TestAnchorTwins(t *testing.T) {
	md := "Intro.\n\nSame words.\n\nMiddle.\n\nSame words.\n\nEnd.\n"
	bs := blocksOf(md)
	c := newComment(bs, 3, "c", "", "n", "t") // the second "Same words."
	p := resolve(blocksOf(md), []*Comment{c})[0]
	if p.block != 3 {
		t.Errorf("the second twin's comment went to block %d", p.block)
	}
	first := newComment(bs, 1, "c", "", "n", "t")
	if p := resolve(blocksOf(md), []*Comment{first})[0]; p.block != 1 {
		t.Errorf("the first twin's comment went to block %d", p.block)
	}
}

func TestAnchorFirstAndLastBlocks(t *testing.T) {
	md := "Alpha beta gamma.\n\nMiddle.\n\nOmega psi chi.\n"
	first := on(t, md, "Alpha")
	last := on(t, md, "Omega")
	edited := "Alpha delta gamma.\n\nMiddle.\n\nOmega phi chi.\n"
	if got, h := where(first, edited); got != "Alpha delta gamma." || h != inPlace {
		t.Errorf("first: got %q, %v", got, h)
	}
	if got, h := where(last, edited); got != "Omega phi chi." || h != inPlace {
		t.Errorf("last: got %q, %v", got, h)
	}
	if got, h := where(on(t, "Only.\n", "Only"), "Changed.\n"); got != "Changed." || h != inPlace {
		t.Errorf("a one-block note: got %q, %v", got, h)
	}
	if got, h := where(on(t, "Only.\n", "Only"), ""); got != "" || h != orphaned {
		t.Errorf("an emptied note: got %q, %v", got, h)
	}
}

func TestAnchorEveryKindOfBlock(t *testing.T) {
	md := "## A heading\n\n- an item\n- another item\n\n```\ncode line\n```\n\n| a | b |\n|---|---|\n| 1 | 2 |\n\n> quoted words\n"
	bs := blocksOf(md)
	var got []string
	for _, b := range bs {
		got = append(got, normalize(b.Text))
	}
	want := []string{"A heading", "an item", "another item", "code line", "| a | b | |---|---| | 1 | 2 |", "quoted words"}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("blocks:\n%q\nwant\n%q", got, want)
	}
	html, _ := RenderBlocks(md)
	for k := range want {
		if !strings.Contains(html, ` data-c="`+strconv.Itoa(k)+`"`) {
			t.Errorf("block %d has no data-c", k)
		}
	}
	// Each block's End is just past its element.
	for k, b := range bs {
		before := html[:b.End]
		if !(strings.HasSuffix(before, "</h2>") || strings.HasSuffix(before, "</span>") || strings.HasSuffix(before, "</pre>") ||
			strings.HasSuffix(before, "</div>") || strings.HasSuffix(before, "</p>")) {
			t.Errorf("block %d ends at %q", k, before[max(0, len(before)-20):])
		}
	}
}

// Healing is what lets a comment survive a block and its neighbors all
// changing, one edit at a time. Without it, the same chain orphans it.
func TestAnchorChainOfEditsNeedsHealing(t *testing.T) {
	v1 := "Intro here.\n\nShort one.\n\nOutro here.\n"
	v2 := "Intro here.\n\nShort two.\n\nOutro here.\n"              // the block, in place
	v3 := "Intro rewritten entirely.\n\nShort two.\n\nOutro here.\n" // a neighbor
	v4 := "Intro rewritten entirely.\n\nBrief.\n\nOutro here.\n"     // the block again
	healed := on(t, v1, "Short")
	unhealed := *healed
	for _, v := range []string{v2, v3} {
		bs := blocksOf(v)
		p := resolve(bs, []*Comment{healed})[0]
		heal(healed, bs, p)
	}
	if got, h := where(healed, v4); got != "Brief." || h != inPlace {
		t.Errorf("healed: got %q, %v", got, h)
	}
	if got, h := where(&unhealed, v4); got != "" || h != orphaned {
		t.Errorf("unhealed: got %q, %v; healing would not be needed", got, h)
	}
	if healed.Quote != "Short one." || healed.Anchor != "Brief." && healed.Anchor != "Short two." {
		t.Errorf("quote %q, anchor %q", healed.Quote, healed.Anchor)
	}
}

func TestHeal(t *testing.T) {
	bs := blocksOf(draft)
	c := newComment(bs, 2, "c", "", "n", "t")
	if c.Anchor != "" || c.Quote != normalize(bs[2].Text) || c.Prev != blockHash(bs[1].Text) || c.Next != blockHash(bs[3].Text) {
		t.Errorf("a new comment: %+v", c)
	}
	if heal(c, bs, placed{2, exact}) {
		t.Error("healing in place changed something")
	}
	edited := blocksOf(strings.Replace(draft, "argue with", "argue against", 1))
	if !heal(c, edited, placed{2, inPlace}) || c.Anchor != normalize(edited[2].Text) || c.Quote != normalize(bs[2].Text) {
		t.Errorf("healed: %+v", c)
	}
	// Back to the quote's text, the anchor is dropped again.
	if !heal(c, bs, placed{2, exact}) || c.Anchor != "" {
		t.Errorf("restored: %+v", c)
	}
	before := *c
	if heal(c, bs, placed{-1, orphaned}) || *c != before {
		t.Error("an orphan was healed")
	}
}

func TestDice(t *testing.T) {
	a := wordFeatures("the quick brown fox")
	if d := dice(a, a); d != 1000 {
		t.Errorf("same: %d", d)
	}
	if d := dice(a, wordFeatures("lorem ipsum")); d != 0 {
		t.Errorf("nothing shared: %d", d)
	}
	if d := dice(nil, nil); d != 1000 {
		t.Errorf("both empty: %d", d)
	}
	if d := dice(a, wordFeatures("The QUICK, brown fox!")); d != 1000 {
		t.Errorf("case and punctuation: %d", d)
	}
	// 4 words and 3 pairs against 3 words and 2 pairs, sharing 3 and 2.
	if d := dice(a, wordFeatures("the quick brown")); d != 2000*5/12 {
		t.Errorf("a prefix: %d", d)
	}
	if b := diceBound(a, wordFeatures("the")); b != 2000*1/8 {
		t.Errorf("bound: %d", b)
	}
}

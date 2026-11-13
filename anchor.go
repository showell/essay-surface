package main

// Anchoring: which block of a note's current text a comment belongs to.
// A comment keeps the text of the block it was made on (quote) and the
// text it was last found at (anchor, when that differs). On each render it
// goes to the block with that exact text, else to the block most like it,
// else nowhere: an orphan, shown apart and never dropped.

import (
	"hash/fnv"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// normalize is a block's text with every run of white space one space, so
// rewrapping a paragraph changes nothing.
func normalize(s string) string { return strings.Join(strings.Fields(s), " ") }

// blockHash is a short fingerprint of a block's normalized text: the page
// carries it, and a new comment names its block by number and fingerprint.
func blockHash(text string) string {
	h := fnv.New64a()
	h.Write([]byte(normalize(text)))
	return strconv.FormatUint(h.Sum64()&0xffffffffff, 36)
}

// words is a text's words, lowercased: runs of letters and digits.
func words(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(c rune) bool {
		return !unicode.IsLetter(c) && !unicode.IsDigit(c)
	})
}

func hashOf(s string) uint64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return h.Sum64()
}

// wordFeatures is a text's words and pairs of adjacent words, hashed and
// sorted: a multiset, for dice.
func wordFeatures(s string) []uint64 {
	ws := words(s)
	f := make([]uint64, 0, 2*len(ws))
	for k, w := range ws {
		f = append(f, hashOf(w))
		if k > 0 {
			f = append(f, hashOf(ws[k-1]+" "+w))
		}
	}
	sort.Slice(f, func(a, b int) bool { return f[a] < f[b] })
	return f
}

// diceBound is the most dice could give two multisets of these sizes.
func diceBound(a, b []uint64) int {
	if len(a)+len(b) == 0 {
		return 1000
	}
	return 2000 * min(len(a), len(b)) / (len(a) + len(b))
}

// dice is the Sørensen-Dice similarity of two sorted multisets, in
// thousandths: 1000 for the same, 0 for nothing shared.
func dice(a, b []uint64) int {
	if len(a)+len(b) == 0 {
		return 1000
	}
	shared := 0
	for i, j := 0, 0; i < len(a) && j < len(b); {
		switch {
		case a[i] == b[j]:
			shared++
			i++
			j++
		case a[i] < b[j]:
			i++
		default:
			j++
		}
	}
	return 2000 * shared / (len(a) + len(b))
}

// Comment is one reader's comment, one line of a note's comments file.
// Quote is the block's text when the comment was made, and never changes;
// Anchor is the text it was last found at, when that is not Quote. Prev
// and Next are the fingerprints of the blocks around it then ("" at the
// start or end of the note), and Ord its place.
type Comment struct {
	ID     string `json:"id"`
	Time   string `json:"time"`
	Name   string `json:"name"`
	Text   string `json:"text"`
	Ord    int    `json:"ord"`
	Prev   string `json:"prev"`
	Next   string `json:"next"`
	Quote  string `json:"quote"`
	Anchor string `json:"anchor,omitempty"`
}

func (c *Comment) anchorText() string {
	if c.Anchor != "" {
		return c.Anchor
	}
	return c.Quote
}

// How a comment found its block.
type how int

const (
	orphaned how = iota
	exact        // the same text
	inPlace      // edited, between the same two neighbors
	similar      // edited and moved: the most similar block
)

// similarEnough is the least dice score, in thousandths, at which an
// edited block that also moved keeps its comments. TestSimulateAnchoring,
// run on the real notes, chose it and the features (journal/tuesday.md).
const similarEnough = 600

// features is the similarity's view of a text. A variable so that
// TestSimulateAnchoring can compare another.
var features = wordFeatures

// placed is where resolve put one comment: a block's index or -1.
type placed struct {
	block int
	how   how
}

// resolve places each comment on a block of the note as it is now.
func resolve(blocks []Block, cs []*Comment) []placed {
	ix := indexBlocks(blocks)
	out := make([]placed, len(cs))
	for i, c := range cs {
		p, score := ix.place(c)
		if p.how == similar && score < similarEnough {
			p = placed{-1, orphaned}
		}
		out[i] = p
	}
	return out
}

// blockIndex finds blocks by text and by fingerprint.
type blockIndex struct {
	blocks []Block
	byText map[string][]int
	byHash map[string][]int
}

// indexBlocks fills in each block's normal form and fingerprint, where it
// has none yet, and indexes them.
func indexBlocks(blocks []Block) *blockIndex {
	ix := &blockIndex{blocks, map[string][]int{}, map[string][]int{}}
	for k := range blocks {
		b := &blocks[k]
		if b.hash == "" {
			b.norm = normalize(b.Text)
			b.hash = blockHash(b.Text)
		}
		ix.byText[b.norm] = append(ix.byText[b.norm], k)
		ix.byHash[b.hash] = append(ix.byHash[b.hash], k)
	}
	return ix
}

func (ix *blockIndex) neighbor(k int) string {
	if k < 0 || k >= len(ix.blocks) {
		return ""
	}
	return ix.blocks[k].hash
}

// place finds a comment's block. When it is only the most similar block,
// how is similar and score says how similar: the caller decides whether
// that is enough.
func (ix *blockIndex) place(c *Comment) (placed, int) {
	blocks := ix.blocks
	// The same text: the nearest such block to where it was.
	for _, t := range []string{normalize(c.anchorText()), normalize(c.Quote)} {
		if at := ix.byText[t]; len(at) > 0 {
			return placed{nearest(at, c.Ord), exact}, 1000
		}
	}
	// Between the same two neighbors: the block was edited in place. A
	// deleted block leaves its neighbors side by side, with nothing between.
	var between []int
	if c.Prev == "" {
		if len(blocks) > 0 && ix.neighbor(1) == c.Next {
			between = append(between, 0)
		}
	} else {
		for _, p := range ix.byHash[c.Prev] {
			if p+1 < len(blocks) && ix.neighbor(p+2) == c.Next {
				between = append(between, p+1)
			}
		}
	}
	if len(between) > 0 {
		return placed{nearest(between, c.Ord), inPlace}, 1000
	}
	// Edited and moved, or its neighbors changed too: the most similar block.
	fa, fq := features(c.anchorText()), features(c.Quote)
	best, bestAt := -1, -1
	for k := range blocks {
		b := &blocks[k]
		if b.feat == nil {
			b.feat = features(b.Text)
		}
		// Dice can be no more than its sizes allow: skip a block too
		// much larger or smaller than the comment's text to win.
		if diceBound(fa, b.feat) < best && diceBound(fq, b.feat) < best {
			continue
		}
		s := max(dice(fa, b.feat), dice(fq, b.feat))
		if s > best || (s == best && abs(k-c.Ord) < abs(bestAt-c.Ord)) {
			best, bestAt = s, k
		}
	}
	if bestAt < 0 {
		return placed{-1, orphaned}, 0
	}
	return placed{bestAt, similar}, best
}

// nearest is the item of ks closest to ord, the first on a tie.
func nearest(ks []int, ord int) int {
	best := ks[0]
	for _, k := range ks[1:] {
		if abs(k-ord) < abs(best-ord) {
			best = k
		}
	}
	return best
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

// heal moves a placed comment's remembered text, neighbors and place to
// where it was found, so a chain of small edits never drifts it away. It
// says whether anything changed.
func heal(c *Comment, blocks []Block, p placed) bool {
	if p.block < 0 {
		return false // an orphan keeps what it knew, in case the text comes back
	}
	text := normalize(blocks[p.block].Text)
	anchor := text
	if anchor == normalize(c.Quote) {
		anchor = ""
	}
	prev, next := "", ""
	if p.block > 0 {
		prev = blockHash(blocks[p.block-1].Text)
	}
	if p.block+1 < len(blocks) {
		next = blockHash(blocks[p.block+1].Text)
	}
	if normalize(c.Anchor) == anchor && c.Prev == prev && c.Next == next {
		return false
	}
	c.Anchor, c.Prev, c.Next, c.Ord = anchor, prev, next, p.block
	return true
}

// newComment is a comment on block k of blocks, as the note is now.
func newComment(blocks []Block, k int, id, time, name, text string) *Comment {
	c := &Comment{ID: id, Time: time, Name: name, Text: text, Quote: normalize(blocks[k].Text)}
	heal(c, blocks, placed{k, exact})
	c.Ord = k
	return c
}

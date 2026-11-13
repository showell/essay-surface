package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// notesForAnchoring is the folder the anchoring simulation reads:
// ESSAY_NOTES when it is set (the real notes), else the samples.
func notesForAnchoring(t testing.TB) map[string][]Block {
	dir := os.Getenv("ESSAY_NOTES")
	if dir == "" {
		dir = "notes"
	}
	paths, _ := filepath.Glob(filepath.Join(dir, "*.md"))
	if len(paths) == 0 {
		t.Fatalf("no notes in %s", dir)
	}
	out := map[string][]Block{}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		_, blocks := RenderBlocks(string(src))
		out[filepath.Base(p)] = blocks
	}
	return out
}

// trigramFeatures is a text's overlapping three-character pieces, of its
// lowercased words joined by single spaces.
func trigramFeatures(s string) []uint64 {
	t := []rune(" " + strings.Join(words(s), " ") + " ")
	f := make([]uint64, 0, len(t))
	for k := 0; k+3 <= len(t); k++ {
		f = append(f, hashOf(string(t[k:k+3])))
	}
	sort.Slice(f, func(a, b int) bool { return f[a] < f[b] })
	return f
}

// A simulated edit changes block k of a note and says which blocks of the
// result the comment on k should be on: none means orphaned.
type simEdit struct {
	name string
	do   func(bs []Block, k int) ([]Block, []int)
}

func texts(ts ...string) []Block {
	out := make([]Block, len(ts))
	for k, t := range ts {
		out[k] = Block{Text: t}
	}
	return out
}

func with(bs []Block, k int, repl ...Block) []Block {
	out := append([]Block{}, bs[:k]...)
	out = append(out, repl...)
	return append(out, bs[k+1:]...)
}

func typo(s string) string {
	ws := strings.Fields(s)
	long := -1
	for i, w := range ws {
		if len(w) >= 4 && (long < 0 || len(w) > len(ws[long])) {
			long = i
		}
	}
	if long < 0 {
		return ""
	}
	r := []byte(ws[long])
	r[1], r[2] = r[2], r[1]
	ws[long] = string(r)
	return strings.Join(ws, " ")
}

var simEdits = []simEdit{
	{"typo", func(bs []Block, k int) ([]Block, []int) {
		t := typo(bs[k].Text)
		if t == "" {
			return nil, nil
		}
		return with(bs, k, Block{Text: t}), []int{k}
	}},
	{"one word", func(bs []Block, k int) ([]Block, []int) {
		ws := strings.Fields(bs[k].Text)
		ws[len(ws)/2] = "zebra"
		return with(bs, k, Block{Text: strings.Join(ws, " ")}), []int{k}
	}},
	{"a sentence", func(bs []Block, k int) ([]Block, []int) {
		return with(bs, k, Block{Text: bs[k].Text + " And then one more sentence goes here."}), []int{k}
	}},
	{"rewrapped", func(bs []Block, k int) ([]Block, []int) {
		return with(bs, k, Block{Text: strings.ReplaceAll(bs[k].Text, " ", "\n")}), []int{k}
	}},
	{"insert above", func(bs []Block, k int) ([]Block, []int) {
		return with(bs, k, Block{Text: "A new paragraph, written above."}, bs[k]), []int{k + 1}
	}},
	{"split", func(bs []Block, k int) ([]Block, []int) {
		ws := strings.Fields(bs[k].Text)
		if len(ws) < 8 {
			return nil, nil
		}
		h := len(ws) / 2
		return with(bs, k, Block{Text: strings.Join(ws[:h], " ")}, Block{Text: strings.Join(ws[h:], " ")}), []int{k, k + 1}
	}},
	{"merged", func(bs []Block, k int) ([]Block, []int) {
		if k+1 >= len(bs) {
			return nil, nil
		}
		out := with(bs, k, Block{Text: bs[k].Text + " " + bs[k+1].Text})
		return append(out[:k+1], out[k+2:]...), []int{k}
	}},
	{"moved, typo", func(bs []Block, k int) ([]Block, []int) {
		t := typo(bs[k].Text)
		if t == "" || len(bs) < 3 {
			return nil, nil
		}
		out := with(bs, k)
		return append(out, Block{Text: t}), []int{len(out)}
	}},
	{"deleted", func(bs []Block, k int) ([]Block, []int) {
		return with(bs, k), nil
	}},
}

// tally is how one simulated edit came out, at one threshold.
type tally struct{ right, wrong, lost, cases int }

var simThresholds = []int{400, 500, 600, 700}

var simShown []string

func cut(s string) string {
	if len(s) > 90 {
		return s[:90] + "..."
	}
	return s
}

// simulate comments on up to 150 blocks of each note, edits each block
// every way, and counts where resolve puts the comment. A block with an
// exact twin elsewhere is left out: the twin takes its comments, by design.
func simulate(notes map[string][]Block) map[string][]tally {
	out := map[string][]tally{}
	for _, e := range simEdits {
		out[e.name] = make([]tally, len(simThresholds))
	}
	for _, blocks := range notes {
		count := map[string]int{}
		for _, b := range blocks {
			count[normalize(b.Text)]++
		}
		indexBlocks(blocks)
		for k := range blocks {
			blocks[k].feat = features(blocks[k].Text)
		}
		step := max(1, len(blocks)/150)
		for k := 0; k < len(blocks); k += step {
			if len(strings.Fields(blocks[k].Text)) == 0 || count[normalize(blocks[k].Text)] > 1 {
				continue
			}
			c := newComment(blocks, k, "x", "", "n", "t")
			for _, e := range simEdits {
				after, want := e.do(blocks, k)
				if after == nil {
					continue
				}
				cc := *c
				found, score := indexBlocks(after).place(&cc)
				for ti, th := range simThresholds {
					p := found
					if p.how == similar && score < th {
						p = placed{-1, orphaned}
					}
					t := &out[e.name][ti]
					t.cases++
					switch {
					case p.block < 0 && len(want) == 0:
						t.right++
					case p.block < 0:
						t.lost++
					case slicesContains(want, p.block):
						t.right++
					default:
						t.wrong++
						if e.name == "deleted" && th == 600 && len(simShown) < 6 {
							simShown = append(simShown, "  "+cut(blocks[k].norm)+"\n    -> "+cut(after[p.block].norm))
						}
					}
				}
			}
		}
	}
	return out
}

func slicesContains(ks []int, k int) bool {
	for _, x := range ks {
		if x == k {
			return true
		}
	}
	return false
}

func TestSimulateAnchoring(t *testing.T) {
	for _, kind := range []string{"words", "trigrams"} {
		if kind == "words" {
			features = wordFeatures
		} else {
			features = trigramFeatures
		}
		res := simulate(notesForAnchoring(t))
		for _, e := range simEdits {
			for ti, th := range simThresholds {
				r := res[e.name][ti]
				t.Logf("%-8s T=%d %-12s cases %5d: right %5d, on another block %4d, orphaned %4d", kind, th, e.name, r.cases, r.right, r.wrong, r.lost)
			}
		}
		t.Logf("%s: deleted, then on another block at T=600:\n%s", kind, strings.Join(simShown, "\n"))
		simShown = nil
	}
	features = wordFeatures
}

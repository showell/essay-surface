package main

// On 2026-11-11 every check passed and a page had no way to reply: the
// reply buttons sat at opacity 0 until hovered. These tests read the CSS a
// note's page is served with and work the cascade out by hand for a resting
// reply button (not hovered, not focused), with comments and without, in
// every desktop state the CSS tells apart: each width its media queries
// name, light and dark. A button is hidden when it or a box around it has
// display:none, when it inherits or has visibility:hidden, or when the
// opacities from it out to the page multiply to less than leastOpacity.
//
// What this does not see, said plainly: a script that stops making the
// buttons (TestReplyButtonsShow ties the class names here to comments.js
// and the renderer, and fast parses the script, but nothing runs it), a
// button clipped or pushed off the page, and colours too faint to read.
// CSS it cannot reason about (an at-rule other than @media, !important, a
// pseudo-class past the four of a resting button, an unknown media
// feature) stops the test rather than being skipped.

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The least opacity a resting reply button may show at. comments.css has
// .6 on a wide screen and .5 on a narrow one; Session 2's had 0.
const leastOpacity = 0.5

type cssRule struct {
	media    string // the @media condition, or ""
	selector string
	decls    map[string]string
	order    int
}

// parseCSS reads the plain CSS these pages use: rules, and @media blocks
// of rules one level deep.
func parseCSS(src string) ([]cssRule, error) {
	for {
		i := strings.Index(src, "/*")
		if i < 0 {
			break
		}
		j := strings.Index(src[i:], "*/")
		if j < 0 {
			return nil, fmt.Errorf("a comment is not closed")
		}
		src = src[:i] + src[i+j+2:]
	}
	var out []cssRule
	var rules func(s, media string) error
	rules = func(s, media string) error {
		for {
			s = strings.TrimSpace(s)
			if s == "" {
				return nil
			}
			open := strings.IndexByte(s, '{')
			if open < 0 {
				return fmt.Errorf("no { after %q", s)
			}
			head := strings.TrimSpace(s[:open])
			if strings.HasPrefix(head, "@") {
				cond, ok := strings.CutPrefix(head, "@media")
				if !ok || media != "" {
					return fmt.Errorf("cannot judge %q", head)
				}
				depth, end := 0, -1
				for k := open; k < len(s); k++ {
					if s[k] == '{' {
						depth++
					} else if s[k] == '}' {
						if depth--; depth == 0 {
							end = k
							break
						}
					}
				}
				if end < 0 {
					return fmt.Errorf("%q is not closed", head)
				}
				if err := rules(s[open+1:end], strings.TrimSpace(cond)); err != nil {
					return err
				}
				s = s[end+1:]
				continue
			}
			close := strings.IndexByte(s, '}')
			if close < open {
				return fmt.Errorf("no } after %q", head)
			}
			decls := map[string]string{}
			for _, d := range strings.Split(s[open+1:close], ";") {
				name, value, ok := strings.Cut(d, ":")
				if !ok {
					if strings.TrimSpace(d) != "" {
						return fmt.Errorf("%q in %q is not a declaration", d, head)
					}
					continue
				}
				value = strings.TrimSpace(value)
				if strings.Contains(value, "!important") {
					return fmt.Errorf("cannot judge !important in %q", head)
				}
				decls[strings.TrimSpace(name)] = value
			}
			for _, sel := range strings.Split(head, ",") {
				out = append(out, cssRule{media: media, selector: strings.TrimSpace(sel), decls: decls, order: len(out)})
			}
			s = s[close+1:]
		}
	}
	return out, rules(src, "")
}

// view is one state of the page: its width and colour scheme.
type view struct {
	widthRem int
	dark     bool
}

func (v view) String() string {
	scheme := "light"
	if v.dark {
		scheme = "dark"
	}
	return fmt.Sprintf("%drem wide, %s", v.widthRem, scheme)
}

var mediaFeature = regexp.MustCompile(`^\(\s*([a-z-]+)\s*:\s*([a-z0-9.]+)\s*\)$`)

// matches says whether an @media condition holds in v. Hover is a
// desktop's: phones are out of scope.
func (v view) matches(cond string) (bool, error) {
	if cond == "" {
		return true, nil
	}
	for _, part := range strings.Split(cond, " and ") {
		m := mediaFeature.FindStringSubmatch(strings.TrimSpace(part))
		if m == nil {
			return false, fmt.Errorf("cannot judge @media %s", cond)
		}
		holds := false
		switch m[1] {
		case "max-width", "min-width":
			n, err := strconv.ParseFloat(strings.TrimSuffix(m[2], "rem"), 64)
			if err != nil || !strings.HasSuffix(m[2], "rem") {
				return false, fmt.Errorf("cannot judge @media %s", cond)
			}
			holds = float64(v.widthRem) <= n
			if m[1] == "min-width" {
				holds = float64(v.widthRem) >= n
			}
		case "hover":
			holds = m[2] == "hover"
		case "prefers-color-scheme":
			holds = (m[2] == "dark") == v.dark
		default:
			return false, fmt.Errorf("cannot judge @media %s", cond)
		}
		if !holds {
			return false, nil
		}
	}
	return true, nil
}

// views is every desktop state the rules tell apart: one width on each
// side of every width a query names, light and dark.
func views(rules []cssRule) []view {
	widths := map[int]bool{80: true}
	for _, r := range rules {
		for _, m := range regexp.MustCompile(`(?:max|min)-width\s*:\s*([0-9.]+)rem`).FindAllStringSubmatch(r.media, -1) {
			n, _ := strconv.ParseFloat(m[1], 64)
			widths[int(n)] = true
			widths[int(n)+1] = true
		}
	}
	var out []view
	for w := range widths {
		out = append(out, view{w, false}, view{w, true})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].widthRem != out[b].widthRem {
			return out[a].widthRem < out[b].widthRem
		}
		return !out[a].dark && out[b].dark
	})
	return out
}

type elem struct {
	tag     string
	classes []string
	attrs   map[string]string
}

func el(tag string, classes ...string) elem { return elem{tag: tag, classes: classes} }

func (e elem) has(class string) bool {
	for _, c := range e.classes {
		if c == class {
			return true
		}
	}
	return false
}

// Where comments.js puts a reply button, from the page out: a paragraph or
// heading holds it, a list item's text span holds it in a .chost li, and a
// code block or table has it in a .cpin after it.
func buttonPlaces(hasComments bool) map[string][]elem {
	btn := elem{tag: "button", classes: []string{"cbtn"}, attrs: map[string]string{"type": "button"}}
	if hasComments {
		btn.classes = append(btn.classes, "has")
	}
	top := []elem{el("html"), el("body"), el("main"), el("article")}
	with := func(es ...elem) []elem { return append(append([]elem{}, top...), es...) }
	return map[string][]elem{
		"a paragraph's":  with(el("p", "chost"), btn),
		"a heading's":    with(el("h2", "chost"), btn),
		"a list item's":  with(el("ul"), el("li", "chost"), el("span", "ct"), btn),
		"a code block's": with(el("div", "cpin"), btn),
	}
}

type compound struct {
	tag     string
	classes []string
	attrs   map[string]string
	never   bool // a state a resting button is not in
	root    bool
	spec    int
}

var compoundPart = regexp.MustCompile(`^(?:\.([\w-]+)|:([\w-]+)|\[([\w-]+)=([\w-]+)\]|#([\w-]+))`)

// parseSelector gives a selector's compounds, the subject last, and the
// combinator before each (" " or ">").
func parseSelector(sel string) ([]compound, []string, error) {
	sel = regexp.MustCompile(`\s*>\s*`).ReplaceAllString(strings.TrimSpace(sel), ">")
	var cs []compound
	var combs []string
	for _, word := range strings.Fields(sel) {
		for k, part := range strings.Split(word, ">") {
			if part == "" {
				return nil, nil, fmt.Errorf("cannot judge selector %q", sel)
			}
			if len(cs) > 0 {
				comb := " "
				if k > 0 {
					comb = ">"
				}
				combs = append(combs, comb)
			}
			c := compound{attrs: map[string]string{}}
			rest := part
			if m := regexp.MustCompile(`^(\*|[a-z][a-z0-9]*)`).FindString(rest); m != "" {
				if m != "*" {
					c.tag = m
					c.spec++
				}
				rest = rest[len(m):]
			}
			for rest != "" {
				m := compoundPart.FindStringSubmatch(rest)
				if m == nil {
					return nil, nil, fmt.Errorf("cannot judge selector %q", sel)
				}
				switch {
				case m[1] != "":
					c.classes = append(c.classes, m[1])
					c.spec += 100
				case m[2] == "root":
					c.root = true
					c.spec += 100
				case m[2] == "hover" || m[2] == "focus" || m[2] == "focus-visible" || m[2] == "active":
					c.never = true
					c.spec += 100
				case m[2] != "":
					return nil, nil, fmt.Errorf("cannot judge :%s in %q", m[2], sel)
				case m[3] != "":
					c.attrs[m[3]] = m[4]
					c.spec += 100
				default:
					c.never = true // these pages give no element an id
					c.spec += 10000
				}
				rest = rest[len(m[0]):]
			}
			cs = append(cs, c)
		}
	}
	return cs, combs, nil
}

func (c compound) matches(e elem, isRoot bool) bool {
	if c.never || c.root && !isRoot || c.tag != "" && c.tag != e.tag {
		return false
	}
	for _, k := range c.classes {
		if !e.has(k) {
			return false
		}
	}
	for k, v := range c.attrs {
		if e.attrs[k] != v {
			return false
		}
	}
	return true
}

// selects says whether compounds cs[:n] match path[:at+1], cs[n-1] at
// path[at].
func selects(cs []compound, combs []string, n int, path []elem, at int) bool {
	if !cs[n-1].matches(path[at], at == 0) {
		return false
	}
	if n == 1 {
		return true
	}
	if combs[n-2] == ">" {
		return at > 0 && selects(cs, combs, n-1, path, at-1)
	}
	for up := at - 1; up >= 0; up-- {
		if selects(cs, combs, n-1, path, up) {
			return true
		}
	}
	return false
}

// buttonsHidden says, for each place and state where a resting reply
// button would not show, why.
func buttonsHidden(css string) ([]string, error) {
	rules, err := parseCSS(css)
	if err != nil {
		return nil, err
	}
	type parsed struct {
		cs    []compound
		combs []string
	}
	sels := make([]parsed, len(rules))
	for i, r := range rules {
		cs, combs, err := parseSelector(r.selector)
		if err != nil {
			return nil, err
		}
		sels[i] = parsed{cs, combs}
	}
	var hidden []string
	for _, v := range views(rules) {
		for _, hasComments := range []bool{false, true} {
			places := buttonPlaces(hasComments)
			names := make([]string, 0, len(places))
			for name := range places {
				names = append(names, name)
			}
			sort.Strings(names)
			for _, name := range names {
				path := places[name]
				// The winning value of a property on path[at], or "".
				value := func(at int, prop string) (string, error) {
					best, bestSpec, bestOrder := "", -1, -1
					for i, r := range rules {
						val, ok := r.decls[prop]
						if !ok {
							continue
						}
						holds, err := v.matches(r.media)
						if err != nil {
							return "", err
						}
						p := sels[i]
						if !holds || !selects(p.cs, p.combs, len(p.cs), path, at) {
							continue
						}
						spec := 0
						for _, c := range p.cs {
							spec += c.spec
						}
						if spec > bestSpec || spec == bestSpec && r.order > bestOrder {
							best, bestSpec, bestOrder = val, spec, r.order
						}
					}
					return best, nil
				}
				opacity, visibility, why := 1.0, "visible", ""
				for at := range path {
					d, err := value(at, "display")
					if err != nil {
						return nil, err
					}
					if d == "none" {
						why = fmt.Sprintf("display:none on its %s", path[at].tag)
					}
					o, err := value(at, "opacity")
					if err != nil {
						return nil, err
					}
					if o != "" {
						f, err := strconv.ParseFloat(strings.TrimSuffix(o, "%"), 64)
						if err != nil {
							return nil, fmt.Errorf("cannot judge opacity %q", o)
						}
						if strings.HasSuffix(o, "%") {
							f /= 100
						}
						opacity *= f
					}
					vis, err := value(at, "visibility")
					if err != nil {
						return nil, err
					}
					if vis != "" {
						visibility = vis
					}
				}
				if why == "" && visibility != "visible" {
					why = "visibility:" + visibility
				}
				if why == "" && opacity < leastOpacity {
					why = fmt.Sprintf("opacity %g, under %g", opacity, leastOpacity)
				}
				if why != "" {
					state := "with no comments"
					if hasComments {
						state = "with comments"
					}
					hidden = append(hidden, fmt.Sprintf("%s reply button %s, %s: %s", name, state, v, why))
				}
			}
		}
	}
	return hidden, nil
}

// servedCSS is every style sheet a page carries, in order.
func servedCSS(page string) string {
	var b strings.Builder
	for _, m := range regexp.MustCompile(`(?s)<style>(.*?)</style>`).FindAllStringSubmatch(page, -1) {
		b.WriteString(m[1])
		b.WriteByte('\n')
	}
	return b.String()
}

// The page as served, with a paragraph, a heading, a list item and a code
// block, shows a resting reply button at each.
func TestReplyButtonsShow(t *testing.T) {
	dir := t.TempDir()
	writeNote(t, dir, "a.md", "# A\n\nA paragraph.\n\n## A heading\n\n- an item\n\n```\ncode\n```\n", time.Now())
	_, page := get(t, testStore(t, dir).handler(), "/notes/a.md")
	// The places above are where comments.js and the renderer put a button;
	// a renamed class would make this test pass by matching nothing.
	for _, s := range []string{`btn.className = "cbtn"`, `classList.add("chost")`, `pin.className = "cpin"`, `el.tagName === "PRE"`} {
		if !strings.Contains(commentJS, s) {
			t.Errorf("comments.js no longer has %s: the places this test judges are out of date", s)
		}
	}
	if !strings.Contains(page, `<span class="ct"`) || !strings.Contains(page, commentScript) {
		t.Error("the page has no list item text span, or not the comment script")
	}
	css := servedCSS(page)
	if !strings.Contains(css, ".cbtn{") {
		t.Fatal("the page's CSS has no .cbtn rule")
	}
	hidden, err := buttonsHidden(css)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range hidden {
		t.Error(h)
	}
}

// Known-bad and known-good style sheets (testdata/buttons), each read with
// the page's own style before it. Every file there is named here, so none
// is skipped.
func TestReplyButtonFixtures(t *testing.T) {
	want := map[string]string{
		"hidden-at-rest.css":   "a paragraph's reply button with no comments, 80rem wide, light: opacity 0, under 0.5",
		"narrow-none.css":      "a code block's reply button with no comments, 50rem wide, light: display:none on its button",
		"host-hidden.css":      "a list item's reply button with comments, 80rem wide, dark: visibility:hidden",
		"faint-article.css":    "a heading's reply button with no comments, 80rem wide, light: opacity 0.4",
		"important.css":        "cannot judge !important",
		"specific-shows.css":   "",
		"hover-only-hides.css": "",
	}
	files, err := filepath.Glob(filepath.Join("testdata", "buttons", "*.css"))
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(want) {
		t.Errorf("%d fixtures in testdata/buttons, %d named here", len(files), len(want))
	}
	page := strings.TrimSuffix(strings.TrimPrefix(style, "<style>"), "</style>")
	for _, f := range files {
		name := filepath.Base(f)
		w, ok := want[name]
		if !ok {
			t.Errorf("%s: not named in this test", name)
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		hidden, err := buttonsHidden(page + "\n" + string(src))
		said := strings.Join(hidden, "\n")
		if err != nil {
			said = err.Error()
		}
		t.Logf("%s: %d hidden; %s", name, len(hidden), strings.SplitN(said, "\n", 2)[0])
		switch {
		case w == "" && said != "":
			t.Errorf("%s: a known-good sheet was refused:\n%s", name, said)
		case w != "" && !strings.Contains(said, w):
			t.Errorf("%s: want %q among:\n%s", name, w, said)
		}
	}
}

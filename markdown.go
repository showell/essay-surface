package main

// The markdown renderer: the subset the notes use, in one pass over the
// lines for blocks and one pass over each block's text for inline markup.
// Block-level raw HTML passes through, since a note is its author's own;
// inline text is always escaped.

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Render turns a markdown document into an HTML fragment.
func Render(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	r := &renderer{slugs: map[string]int{}}
	r.blocks(strings.Split(src, "\n"), false)
	return r.out.String()
}

// Block is one commentable block of a note: a paragraph, a heading, a
// tight list item's text, a code fence or a table. Text is its markdown
// source; End is the offset in the HTML just past the block's element,
// where its comments go.
type Block struct {
	Text string
	End  int
	norm string   // normalize(Text), once prepared
	hash string   // blockHash(Text)
	feat []uint64 // features(Text), made only when a comment needs it
}

// RenderBlocks is Render with each commentable block marked data-c="K",
// K counting from 0 in document order, and the blocks returned.
func RenderBlocks(src string) (string, []Block) {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	r := &renderer{slugs: map[string]int{}, mark: true}
	r.blocks(strings.Split(src, "\n"), false)
	return r.out.String(), r.marked
}

// open writes a commentable block's data-c attribute and records the
// block; close records where its element ended.
func (r *renderer) open(text string) {
	if !r.mark {
		return
	}
	r.out.WriteString(` data-c="` + strconv.Itoa(len(r.marked)) + `"`)
	r.marked = append(r.marked, Block{Text: text})
}

// openLines is open for a block of several source lines, joined only when
// the render is marked.
func (r *renderer) openLines(lines []string) {
	if r.mark {
		r.open(strings.Join(lines, "\n"))
	}
}

func (r *renderer) close() {
	if r.mark {
		r.marked[len(r.marked)-1].End = r.out.Len()
	}
}

// Title is the text of a document's first level-one heading outside a
// code fence, or "".
func Title(src string) string {
	var fenceCh byte
	fenceN := 0
	for _, line := range strings.Split(src, "\n") {
		if fenceN > 0 {
			if isFenceClose(line, fenceCh, fenceN) {
				fenceN = 0
			}
			continue
		}
		if ch, n, _, _, ok := fenceOpen(line); ok {
			fenceCh, fenceN = ch, n
			continue
		}
		if level, text, ok := atxHeading(line); ok && level == 1 {
			return text
		}
	}
	return ""
}

// TitleHTML is a title's text for <title> and the index: its inline markup
// rendered and the tags dropped, so `code` reads as code. Already escaped.
func TitleHTML(title string) string {
	h := inline(title)
	var b strings.Builder
	inTag := false
	for k := 0; k < len(h); k++ {
		switch {
		case h[k] == '<':
			inTag = true
		case h[k] == '>' && inTag:
			inTag = false
		case !inTag:
			b.WriteByte(h[k])
		}
	}
	return b.String()
}

type renderer struct {
	out        strings.Builder
	slugs      map[string]int
	paragraphs int
	mark       bool
	marked     []Block
}

// blocks renders a run of lines. In a tight list item, a paragraph is its
// text alone, with no <p>.
func (r *renderer) blocks(lines []string, tight bool) {
	i := 0
	for i < len(lines) {
		line := lines[i]
		if isBlank(line) {
			i++
			continue
		}
		if ch, n, indent, info, ok := fenceOpen(line); ok {
			i = r.fence(lines, i+1, ch, n, indent, info)
			continue
		}
		if level, text, ok := atxHeading(line); ok {
			r.heading(level, text)
			i++
			continue
		}
		if isRule(line) {
			r.out.WriteString("<hr>\n")
			i++
			continue
		}
		if isQuote(line) {
			i = r.quote(lines, i)
			continue
		}
		if isHTMLBlock(line) {
			for i < len(lines) && !isBlank(lines[i]) {
				r.out.WriteString(lines[i])
				r.out.WriteByte('\n')
				i++
			}
			continue
		}
		if i+1 < len(lines) && isTableStart(line, lines[i+1]) {
			i = r.table(lines, i)
			continue
		}
		if _, ok := listMarker(line); ok {
			i = r.list(lines, i)
			continue
		}
		i = r.paragraph(lines, i, tight)
	}
}

func isBlank(s string) bool { return strings.TrimSpace(s) == "" }

func leadingSpaces(s string) int {
	n := 0
	for n < len(s) && s[n] == ' ' {
		n++
	}
	return n
}

// stripIndent removes up to n leading spaces; a tab counts as four.
func stripIndent(s string, n int) string {
	col := 0
	i := 0
	for i < len(s) && col < n {
		switch s[i] {
		case ' ':
			col++
		case '\t':
			col += 4
		default:
			return s[i:]
		}
		i++
	}
	return s[i:]
}

// fenceOpen recognizes ``` or ~~~ (three or more), indented at most three.
func fenceOpen(line string) (ch byte, n int, indent int, info string, ok bool) {
	indent = leadingSpaces(line)
	if indent > 3 || indent >= len(line) {
		return
	}
	ch = line[indent]
	if ch != '`' && ch != '~' {
		return
	}
	j := indent
	for j < len(line) && line[j] == ch {
		j++
	}
	n = j - indent
	if n < 3 {
		return
	}
	info = strings.TrimSpace(line[j:])
	if ch == '`' && strings.Contains(info, "`") {
		return
	}
	return ch, n, indent, info, true
}

func isFenceClose(line string, ch byte, n int) bool {
	t := strings.TrimSpace(line)
	if len(t) < n || leadingSpaces(line) > 3 {
		return false
	}
	for k := 0; k < len(t); k++ {
		if t[k] != ch {
			return false
		}
	}
	return true
}

// fence renders a code block from line i, the one after the opener, and
// returns the index after its closer. An unclosed fence runs to the end.
func (r *renderer) fence(lines []string, i int, ch byte, n, indent int, info string) int {
	lang := info
	if k := strings.IndexAny(lang, " \t"); k >= 0 {
		lang = lang[:k]
	}
	start := i
	for i < len(lines) && !isFenceClose(lines[i], ch, n) {
		i++
	}
	body := lines[start:i]
	if i < len(lines) {
		i++ // the closer
	}
	r.out.WriteString("<pre")
	r.openLines(body)
	if lang != "" {
		r.out.WriteString(`><code class="language-`)
		r.out.WriteString(escapeAttr(lang))
		r.out.WriteString(`">`)
	} else {
		r.out.WriteString("><code>")
	}
	for _, line := range body {
		r.out.WriteString(escapeHTML(stripIndent(line, indent)))
		r.out.WriteByte('\n')
	}
	r.out.WriteString("</code></pre>")
	r.close()
	r.out.WriteByte('\n')
	return i
}

func atxHeading(line string) (level int, text string, ok bool) {
	s := line[min(leadingSpaces(line), len(line)):]
	if leadingSpaces(line) > 3 {
		return
	}
	for level < len(s) && s[level] == '#' {
		level++
	}
	if level == 0 || level > 6 {
		return 0, "", false
	}
	rest := s[level:]
	if rest != "" && rest[0] != ' ' && rest[0] != '\t' {
		return 0, "", false
	}
	rest = strings.TrimSpace(rest)
	// A closing run of #s goes, if a space stands before it.
	if t := strings.TrimRight(rest, "#"); t != rest && (t == "" || strings.HasSuffix(t, " ")) {
		rest = strings.TrimSpace(t)
	}
	return level, rest, true
}

func (r *renderer) heading(level int, text string) {
	slug := r.slug(text)
	tag := "h" + strconv.Itoa(level)
	r.out.WriteString("<" + tag + ` id="` + slug + `"`)
	r.open(text)
	r.out.WriteString(">")
	r.out.WriteString(inline(text))
	r.out.WriteString("</" + tag + ">")
	r.close()
	r.out.WriteByte('\n')
}

// slug makes a heading's id: lowercase letters and digits, words joined by
// hyphens, and a number added for a repeat.
func (r *renderer) slug(text string) string {
	var b strings.Builder
	dash := false
	for _, c := range strings.ToLower(text) {
		switch {
		case unicode.IsLetter(c) || unicode.IsDigit(c):
			if dash && b.Len() > 0 {
				b.WriteByte('-')
			}
			dash = false
			b.WriteRune(c)
		case c == ' ' || c == '-' || c == '_':
			dash = true
		}
	}
	s := b.String()
	if s == "" {
		s = "section"
	}
	r.slugs[s]++
	if n := r.slugs[s]; n > 1 {
		s += "-" + strconv.Itoa(n)
	}
	return s
}

func isRule(line string) bool {
	t := strings.TrimSpace(line)
	if leadingSpaces(line) > 3 || len(t) < 3 {
		return false
	}
	ch := t[0]
	if ch != '-' && ch != '*' && ch != '_' {
		return false
	}
	count := 0
	for k := 0; k < len(t); k++ {
		switch t[k] {
		case ch:
			count++
		case ' ', '\t':
		default:
			return false
		}
	}
	return count >= 3
}

func isQuote(line string) bool {
	return leadingSpaces(line) <= 3 && strings.HasPrefix(strings.TrimLeft(line, " "), ">")
}

func (r *renderer) quote(lines []string, i int) int {
	var inner []string
	for i < len(lines) && isQuote(lines[i]) {
		s := strings.TrimLeft(lines[i], " ")[1:]
		s = strings.TrimPrefix(s, " ")
		inner = append(inner, s)
		i++
	}
	r.out.WriteString("<blockquote>\n")
	r.blocks(inner, false)
	r.out.WriteString("</blockquote>\n")
	return i
}

var htmlBlockTags = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true, "details": true,
	"div": true, "dl": true, "fieldset": true, "figcaption": true, "figure": true,
	"footer": true, "form": true, "h1": true, "h2": true, "h3": true, "h4": true,
	"h5": true, "h6": true, "header": true, "hr": true, "iframe": true, "li": true,
	"nav": true, "ol": true, "p": true, "pre": true, "section": true, "summary": true,
	"table": true, "tbody": true, "td": true, "tfoot": true, "th": true, "thead": true,
	"tr": true, "ul": true, "img": true, "svg": true, "canvas": true, "video": true,
	"audio": true, "picture": true, "style": true, "script": true,
}

// isHTMLBlock is a line that opens with a block-level tag or a comment.
func isHTMLBlock(line string) bool {
	if leadingSpaces(line) > 3 {
		return false
	}
	s := strings.TrimLeft(line, " ")
	if strings.HasPrefix(s, "<!--") {
		return true
	}
	if !strings.HasPrefix(s, "<") {
		return false
	}
	s = strings.TrimPrefix(s[1:], "/")
	j := 0
	for j < len(s) && (isASCIILetter(s[j]) || (j > 0 && s[j] >= '0' && s[j] <= '9')) {
		j++
	}
	if j == 0 {
		return false
	}
	if j < len(s) && s[j] != ' ' && s[j] != '>' && s[j] != '/' && s[j] != '\t' {
		return false
	}
	return htmlBlockTags[strings.ToLower(s[:j])]
}

func isASCIILetter(c byte) bool { return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') }

// splitRow splits a table row on its pipes, leaving a pipe inside a code
// span or escaped with a backslash in its cell.
func splitRow(line string) []string {
	s := strings.TrimSpace(line)
	s = strings.TrimPrefix(s, "|")
	if strings.HasSuffix(s, "|") && !strings.HasSuffix(s, `\|`) {
		s = s[:len(s)-1]
	}
	var cells []string
	var cur strings.Builder
	for k := 0; k < len(s); k++ {
		c := s[k]
		switch {
		case c == '\\' && k+1 < len(s) && s[k+1] == '|':
			cur.WriteByte('|')
			k++
		case c == '`':
			// A code span keeps its pipes; a run with no partner is text.
			end := k
			if _, e, ok := codeSpan(s, k); ok {
				end = e
			} else {
				for end < len(s) && s[end] == '`' {
					end++
				}
			}
			cur.WriteString(s[k:end])
			k = end - 1
		case c == '|':
			cells = append(cells, strings.TrimSpace(cur.String()))
			cur.Reset()
		default:
			cur.WriteByte(c)
		}
	}
	cells = append(cells, strings.TrimSpace(cur.String()))
	return cells
}

// delimiterAligns reads a table's second row (|---|:--:|) into each
// column's alignment, or false if the line is not one.
func delimiterAligns(line string) ([]string, bool) {
	if !strings.Contains(line, "-") {
		return nil, false
	}
	cells := splitRow(line)
	aligns := make([]string, len(cells))
	for k, c := range cells {
		if c == "" {
			return nil, false
		}
		left := strings.HasPrefix(c, ":")
		right := strings.HasSuffix(c, ":")
		core := strings.Trim(c, ":")
		if core == "" || strings.Trim(core, "-") != "" {
			return nil, false
		}
		switch {
		case left && right:
			aligns[k] = "center"
		case right:
			aligns[k] = "right"
		case left:
			aligns[k] = "left"
		}
	}
	return aligns, true
}

func isTableStart(line, next string) bool {
	if !strings.Contains(line, "|") || !strings.Contains(next, "|") {
		return false
	}
	aligns, ok := delimiterAligns(next)
	return ok && len(aligns) == len(splitRow(line))
}

func (r *renderer) table(lines []string, i int) int {
	header := splitRow(lines[i])
	aligns, _ := delimiterAligns(lines[i+1])
	i += 2
	start := i - 2
	end := i
	for end < len(lines) && !isBlank(lines[end]) && strings.Contains(lines[end], "|") {
		end++
	}
	r.out.WriteString(`<div class="table-wrap"`)
	r.openLines(lines[start:end])
	r.out.WriteString("><table>\n<thead><tr>")
	r.cells("th", header, aligns)
	r.out.WriteString("</tr></thead>\n<tbody>\n")
	for ; i < end; i++ {
		r.out.WriteString("<tr>")
		r.cells("td", splitRow(lines[i]), aligns)
		r.out.WriteString("</tr>\n")
	}
	r.out.WriteString("</tbody></table></div>")
	r.close()
	r.out.WriteByte('\n')
	return i
}

// cells writes one row's cells, as many as the header has.
func (r *renderer) cells(tag string, cells, aligns []string) {
	for k := range aligns {
		r.out.WriteString("<" + tag)
		if aligns[k] != "" {
			r.out.WriteString(` style="text-align:` + aligns[k] + `"`)
		}
		r.out.WriteString(">")
		if k < len(cells) {
			r.out.WriteString(inline(cells[k]))
		}
		r.out.WriteString("</" + tag + ">")
	}
}

type marker struct {
	indent  int // spaces before the marker
	content int // the column where the item's text starts
	ordered bool
	start   int  // an ordered item's number
	ch      byte // '-', '*', '+', or the '.' or ')' after a number
}

// listMarker recognizes "- ", "* ", "+ ", "1. " or "1) " at the start of a
// line, indented at most three spaces more than nothing.
func listMarker(line string) (marker, bool) {
	indent := leadingSpaces(line)
	s := line[indent:]
	if s == "" {
		return marker{}, false
	}
	m := marker{indent: indent}
	width := 0
	switch {
	case s[0] == '-' || s[0] == '*' || s[0] == '+':
		m.ch = s[0]
		width = 1
	case s[0] >= '0' && s[0] <= '9':
		j := 0
		for j < len(s) && j < 9 && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j >= len(s) || (s[j] != '.' && s[j] != ')') {
			return marker{}, false
		}
		m.ordered = true
		m.start, _ = strconv.Atoi(s[:j])
		m.ch = s[j]
		width = j + 1
	default:
		return marker{}, false
	}
	rest := s[width:]
	if rest == "" {
		m.content = indent + width + 1
		return m, true
	}
	if rest[0] != ' ' && rest[0] != '\t' {
		return marker{}, false
	}
	gap := leadingSpaces(rest)
	if gap > 4 || gap == 0 {
		gap = 1
	}
	m.content = indent + width + gap
	return m, true
}

func sameList(a, b marker) bool { return a.ordered == b.ordered && a.ch == b.ch }

// list renders the list starting at line i, and returns the index after it.
// An item holds every following line indented past its marker; a blank
// line between items or inside one makes the list loose (<p> in items).
func (r *renderer) list(lines []string, i int) int {
	first, _ := listMarker(lines[i])
	var items [][]string
	loose := false
	for i < len(lines) {
		m, ok := listMarker(lines[i])
		if !ok || !sameList(first, m) || m.indent > first.indent+1 || m.indent+1 < first.indent {
			break
		}
		item := []string{lines[i][min(m.content, len(lines[i])):]}
		i++
		for i < len(lines) {
			line := lines[i]
			if isBlank(line) {
				// A blank line stays in the item if the item goes on after it.
				j := i
				for j < len(lines) && isBlank(lines[j]) {
					j++
				}
				if j < len(lines) && leadingSpaces(lines[j]) > m.indent {
					for ; i < j; i++ {
						item = append(item, "")
					}
					loose = true
					continue
				}
				break
			}
			if leadingSpaces(line) > m.indent {
				item = append(item, stripIndent(line, m.content))
				i++
				continue
			}
			// A lazy line continues the item's paragraph; any line that
			// could start a block ends the item.
			if _, isMarker := listMarker(line); isMarker || startsBlock(line) || isBlank(item[len(item)-1]) {
				break
			}
			item = append(item, line)
			i++
		}
		items = append(items, item)
		// A blank line between this item and the next makes the list loose.
		if i < len(lines) && isBlank(lines[i]) {
			j := i
			for j < len(lines) && isBlank(lines[j]) {
				j++
			}
			if next, ok := listMarker(lines[min(j, len(lines)-1)]); j < len(lines) && ok && sameList(first, next) && next.indent <= first.indent+1 {
				loose = true
				i = j
			}
		}
	}
	tag := "ul"
	if first.ordered {
		tag = "ol"
	}
	r.out.WriteString("<" + tag)
	if first.ordered && first.start != 1 {
		r.out.WriteString(` start="` + strconv.Itoa(first.start) + `"`)
	}
	r.out.WriteString(">\n")
	for _, item := range items {
		r.out.WriteString("<li>")
		r.blocks(item, !loose)
		trimNewline(&r.out)
		r.out.WriteString("</li>\n")
	}
	r.out.WriteString("</" + tag + ">\n")
	return i
}

// trimNewline drops a trailing newline, so a tight item reads <li>text</li>.
func trimNewline(b *strings.Builder) {
	s := b.String()
	if strings.HasSuffix(s, "\n") {
		b.Reset()
		b.WriteString(s[:len(s)-1])
	}
}

// startsBlock is a line that ends a paragraph: anything but plain text.
func startsBlock(line string) bool {
	if _, _, _, _, ok := fenceOpen(line); ok {
		return true
	}
	if _, _, ok := atxHeading(line); ok {
		return true
	}
	return isRule(line) || isQuote(line) || isHTMLBlock(line)
}

func (r *renderer) paragraph(lines []string, i int, tight bool) int {
	start := i
	i++
	for i < len(lines) {
		line := lines[i]
		if isBlank(line) || startsBlock(line) {
			break
		}
		if m, ok := listMarker(line); ok && (!m.ordered || m.start == 1) {
			break
		}
		if i+1 < len(lines) && isTableStart(line, lines[i+1]) {
			break
		}
		i++
	}
	text := strings.Join(lines[start:i], "\n")
	text = strings.TrimSpace(text)
	if tight {
		// A tight item's text has no element of its own; marked, it gets a span.
		if r.mark {
			r.out.WriteString(`<span class="ct"`)
			r.open(text)
			r.out.WriteString(">")
		}
		r.out.WriteString(inline(text))
		if r.mark {
			r.out.WriteString("</span>")
			r.close()
		}
		r.out.WriteByte('\n')
		return i
	}
	r.paragraphs++
	r.out.WriteString(`<p id="p` + strconv.Itoa(r.paragraphs) + `"`)
	r.open(text)
	r.out.WriteString(">")
	r.out.WriteString(inline(text))
	r.out.WriteString("</p>")
	r.close()
	r.out.WriteByte('\n')
	return i
}

func escapeHTML(s string) string {
	if !strings.ContainsAny(s, "<>&\"") {
		return s
	}
	var b strings.Builder
	b.Grow(len(s) + 16)
	for k := 0; k < len(s); k++ {
		switch s[k] {
		case '<':
			b.WriteString("&lt;")
		case '>':
			b.WriteString("&gt;")
		case '&':
			b.WriteString("&amp;")
		case '"':
			b.WriteString("&quot;")
		default:
			b.WriteByte(s[k])
		}
	}
	return b.String()
}

func escapeAttr(s string) string { return escapeHTML(s) }

// The inline pass. Text becomes a list of pieces: finished HTML, or a run
// of * or _ that may open or close emphasis. Runs are matched afterwards,
// as CommonMark does, so "**a *b* c**" nests.

type piece struct {
	html     string
	ch       byte // '*' or '_' for a delimiter run, 0 for html
	n        int  // the run's characters not yet used
	orig     int
	canOpen  bool
	canClose bool
	opens    []string // tags this run opens, innermost last matched first
	closes   []string
}

func inline(s string) string {
	var ps []piece
	var text strings.Builder
	flush := func() {
		if text.Len() > 0 {
			ps = append(ps, piece{html: text.String()})
			text.Reset()
		}
	}
	for k := 0; k < len(s); {
		c := s[k]
		switch {
		case c == '\\' && k+1 < len(s) && s[k+1] == '\n':
			text.WriteString("<br>\n")
			k += 2
		case c == '\\' && k+1 < len(s) && isASCIIPunct(s[k+1]):
			text.WriteString(escapeHTML(s[k+1 : k+2]))
			k += 2
		case c == '`':
			if html, end, ok := codeSpan(s, k); ok {
				text.WriteString(html)
				k = end
			} else {
				j := k
				for j < len(s) && s[j] == '`' {
					j++
				}
				text.WriteString(s[k:j])
				k = j
			}
		case c == '!' && k+1 < len(s) && s[k+1] == '[':
			if html, end, ok := link(s, k+1, true); ok {
				text.WriteString(html)
				k = end
			} else {
				text.WriteByte('!')
				k++
			}
		case c == '[':
			if html, end, ok := link(s, k, false); ok {
				text.WriteString(html)
				k = end
			} else {
				text.WriteByte('[')
				k++
			}
		case c == '<':
			if html, end, ok := autolink(s, k); ok {
				text.WriteString(html)
				k = end
			} else {
				text.WriteString("&lt;")
				k++
			}
		case c == '*' || c == '_':
			j := k
			for j < len(s) && s[j] == c {
				j++
			}
			before, _ := utf8.DecodeLastRuneInString(s[:k])
			after, _ := utf8.DecodeRuneInString(s[j:])
			if k == 0 {
				before = ' '
			}
			if j == len(s) {
				after = ' '
			}
			left := !isSpace(after) && (!isPunct(after) || isSpace(before) || isPunct(before))
			right := !isSpace(before) && (!isPunct(before) || isSpace(after) || isPunct(after))
			p := piece{ch: c, n: j - k, orig: j - k}
			if c == '*' {
				p.canOpen, p.canClose = left, right
			} else {
				p.canOpen = left && (!right || isPunct(before))
				p.canClose = right && (!left || isPunct(after))
			}
			flush()
			ps = append(ps, p)
			k = j
		case c == ' ' && strings.HasPrefix(s[k:], "  \n"):
			j := k
			for j < len(s) && s[j] == ' ' {
				j++
			}
			text.WriteString("<br>\n")
			k = j + 1
		case c == '&' && isEntity(s[k:]):
			// An entity the author wrote (&mdash;, &#39;) stays one.
			e := strings.IndexByte(s[k:], ';')
			text.WriteString(s[k : k+e+1])
			k += e + 1
		case c == '&' || c == '>' || c == '"':
			text.WriteString(escapeHTML(s[k : k+1]))
			k++
		default:
			text.WriteByte(c)
			k++
		}
	}
	flush()
	matchEmphasis(ps)
	var b strings.Builder
	for _, p := range ps {
		if p.ch == 0 {
			b.WriteString(p.html)
			continue
		}
		for _, t := range p.closes {
			b.WriteString(t)
		}
		b.WriteString(strings.Repeat(string(p.ch), p.n))
		for k := len(p.opens) - 1; k >= 0; k-- {
			b.WriteString(p.opens[k])
		}
	}
	return b.String()
}

// matchEmphasis pairs each closing run with the nearest opener of its
// character before it: two characters each make <strong>, one makes <em>.
func matchEmphasis(ps []piece) {
	for c := range ps {
		closer := &ps[c]
		if closer.ch == 0 || !closer.canClose {
			continue
		}
		for closer.n > 0 {
			o := -1
			for k := c - 1; k >= 0; k-- {
				op := &ps[k]
				if op.ch != closer.ch || op.n == 0 || !op.canOpen {
					continue
				}
				// CommonMark's rule of three: a run that could both open and
				// close does not pair when the lengths sum to a multiple of 3.
				if (op.canClose || closer.canOpen) && (op.orig+closer.orig)%3 == 0 && !(op.orig%3 == 0 && closer.orig%3 == 0) {
					continue
				}
				o = k
				break
			}
			if o < 0 {
				break
			}
			opener := &ps[o]
			use := 1
			if opener.n >= 2 && closer.n >= 2 {
				use = 2
			}
			tag := "em"
			if use == 2 {
				tag = "strong"
			}
			opener.opens = append(opener.opens, "<"+tag+">")
			closer.closes = append(closer.closes, "</"+tag+">")
			opener.n -= use
			closer.n -= use
			// Runs between the two can no longer pair across them.
			for k := o + 1; k < c; k++ {
				if ps[k].ch != 0 {
					ps[k].canOpen = false
					ps[k].canClose = false
				}
			}
		}
	}
}

// isEntity is "&name;" or "&#123;" or "&#x1F;" at the start of s.
func isEntity(s string) bool {
	e := strings.IndexByte(s, ';')
	if e < 2 || e > 32 {
		return false
	}
	body := s[1:e]
	if body[0] == '#' {
		body = body[1:]
		if body != "" && (body[0] == 'x' || body[0] == 'X') {
			body = body[1:]
		}
	}
	if body == "" {
		return false
	}
	for k := 0; k < len(body); k++ {
		if !isASCIILetter(body[k]) && !(body[k] >= '0' && body[k] <= '9') {
			return false
		}
	}
	return true
}

func isSpace(r rune) bool { return unicode.IsSpace(r) }

func isPunct(r rune) bool { return unicode.IsPunct(r) || unicode.IsSymbol(r) }

func isASCIIPunct(c byte) bool {
	return strings.IndexByte("!\"#$%&'()*+,-./:;<=>?@[\\]^_`{|}~", c) >= 0
}

// codeSpan matches a run of backticks at k with the next run of the same
// length, and gives the span's HTML and the index after it.
func codeSpan(s string, k int) (string, int, bool) {
	j := k
	for j < len(s) && s[j] == '`' {
		j++
	}
	n := j - k
	for m := j; m < len(s); {
		if s[m] != '`' {
			m++
			continue
		}
		e := m
		for e < len(s) && s[e] == '`' {
			e++
		}
		if e-m == n {
			body := strings.ReplaceAll(s[j:m], "\n", " ")
			if len(body) >= 2 && body[0] == ' ' && body[len(body)-1] == ' ' && strings.TrimSpace(body) != "" {
				body = body[1 : len(body)-1]
			}
			return "<code>" + escapeHTML(body) + "</code>", e, true
		}
		m = e
	}
	return "", 0, false
}

// link matches [text](dest "title") at k, the '['. Brackets nest, and a
// code span inside the text hides its brackets.
func link(s string, k int, image bool) (string, int, bool) {
	depth := 0
	end := -1
	for m := k; m < len(s); m++ {
		switch s[m] {
		case '\\':
			m++
		case '`':
			if _, e, ok := codeSpan(s, m); ok {
				m = e - 1
			}
		case '[':
			depth++
		case ']':
			depth--
			if depth == 0 {
				end = m
			}
		}
		if end >= 0 {
			break
		}
	}
	if end < 0 || end+1 >= len(s) || s[end+1] != '(' {
		return "", 0, false
	}
	label := s[k+1 : end]
	m := end + 2
	for m < len(s) && s[m] == ' ' {
		m++
	}
	var dest string
	if m < len(s) && s[m] == '<' {
		close := strings.IndexByte(s[m:], '>')
		if close < 0 {
			return "", 0, false
		}
		dest = s[m+1 : m+close]
		m += close + 1
	} else {
		parens := 0
		st := m
		for m < len(s) && s[m] != ' ' && s[m] != '\n' {
			if s[m] == '(' {
				parens++
			} else if s[m] == ')' {
				if parens == 0 {
					break
				}
				parens--
			}
			m++
		}
		dest = s[st:m]
	}
	for m < len(s) && (s[m] == ' ' || s[m] == '\n') {
		m++
	}
	title := ""
	if m < len(s) && (s[m] == '"' || s[m] == '\'') {
		q := s[m]
		close := strings.IndexByte(s[m+1:], q)
		if close < 0 {
			return "", 0, false
		}
		title = s[m+1 : m+1+close]
		m += close + 2
		for m < len(s) && s[m] == ' ' {
			m++
		}
	}
	if m >= len(s) || s[m] != ')' {
		return "", 0, false
	}
	href := escapeAttr(safeURL(dest))
	var b strings.Builder
	if image {
		b.WriteString(`<img src="` + href + `" alt="` + escapeAttr(label) + `"`)
		if title != "" {
			b.WriteString(` title="` + escapeAttr(title) + `"`)
		}
		b.WriteString(">")
	} else {
		b.WriteString(`<a href="` + href + `"`)
		if title != "" {
			b.WriteString(` title="` + escapeAttr(title) + `"`)
		}
		b.WriteString(">" + inline(label) + "</a>")
	}
	return b.String(), m + 1, true
}

// safeURL refuses a script URL; anything else is the author's.
func safeURL(u string) string {
	l := strings.ToLower(strings.TrimSpace(u))
	if strings.HasPrefix(l, "javascript:") || strings.HasPrefix(l, "vbscript:") || strings.HasPrefix(l, "data:text/html") {
		return "#"
	}
	return u
}

// autolink matches <https://...> or <mailto:...> at k.
func autolink(s string, k int) (string, int, bool) {
	close := strings.IndexByte(s[k:], '>')
	if close < 0 {
		return "", 0, false
	}
	u := s[k+1 : k+close]
	if strings.ContainsAny(u, " <\n") {
		return "", 0, false
	}
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") && !strings.HasPrefix(u, "mailto:") {
		return "", 0, false
	}
	return `<a href="` + escapeAttr(u) + `">` + escapeHTML(u) + "</a>", k + close + 1, true
}

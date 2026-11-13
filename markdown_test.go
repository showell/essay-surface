package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRender(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"heading levels and ids", "# One\n## Two words\n###### Six",
			"<h1 id=\"one\">One</h1>\n<h2 id=\"two-words\">Two words</h2>\n<h6 id=\"six\">Six</h6>\n"},
		{"seven hashes is a paragraph", "####### no", "<p id=\"p1\">####### no</p>\n"},
		{"a hash with no space is a paragraph", "#tag", "<p id=\"p1\">#tag</p>\n"},
		{"closing hashes go", "## Title ##", "<h2 id=\"title\">Title</h2>\n"},
		{"repeated heading ids are numbered", "## A\n## A", "<h2 id=\"a\">A</h2>\n<h2 id=\"a-2\">A</h2>\n"},
		{"paragraphs split on blank lines and are numbered", "one\ntwo\n\nthree",
			"<p id=\"p1\">one\ntwo</p>\n<p id=\"p2\">three</p>\n"},
		{"html in text is escaped", "a <b> & \"c\"", "<p id=\"p1\">a &lt;b&gt; &amp; &quot;c&quot;</p>\n"},
		{"an entity stays", "a &mdash; b &#39; c &x", "<p id=\"p1\">a &mdash; b &#39; c &amp;x</p>\n"},
		{"fence with language", "```go\nx := <-ch\n```",
			"<pre><code class=\"language-go\">x := &lt;-ch\n</code></pre>\n"},
		{"fence without language", "```\na\n\nb\n```", "<pre><code>a\n\nb\n</code></pre>\n"},
		{"tilde fence holds backticks", "~~~~\n```\n~~~\n~~~~", "<pre><code>```\n~~~\n</code></pre>\n"},
		{"unclosed fence runs to the end", "```\na", "<pre><code>a\n</code></pre>\n"},
		{"markdown inside a fence is left alone", "```\n# no *em*\n```", "<pre><code># no *em*\n</code></pre>\n"},
		{"inline code", "use `a < b` here", "<p id=\"p1\">use <code>a &lt; b</code> here</p>\n"},
		{"double backticks hold one", "``a ` b``", "<p id=\"p1\"><code>a ` b</code></p>\n"},
		{"an unmatched backtick is literal", "a ` b", "<p id=\"p1\">a ` b</p>\n"},
		{"emphasis inside code is literal", "`*a*`", "<p id=\"p1\"><code>*a*</code></p>\n"},
		{"em and strong", "*a* _b_ **c** __d__", "<p id=\"p1\"><em>a</em> <em>b</em> <strong>c</strong> <strong>d</strong></p>\n"},
		{"strong around em", "**a *b* c**", "<p id=\"p1\"><strong>a <em>b</em> c</strong></p>\n"},
		{"triple is both", "***a***", "<p id=\"p1\"><em><strong>a</strong></em></p>\n"},
		{"underscores inside a word stay", "machine__4 and snake_case_name", "<p id=\"p1\">machine__4 and snake_case_name</p>\n"},
		{"star inside a word is emphasis", "un*frigging*believable", "<p id=\"p1\">un<em>frigging</em>believable</p>\n"},
		{"a lone star is literal", "2 * 3 = 6", "<p id=\"p1\">2 * 3 = 6</p>\n"},
		{"unbalanced stars", "**a*", "<p id=\"p1\">*<em>a</em></p>\n"},
		{"escaped star", `\*not\*`, "<p id=\"p1\">*not*</p>\n"},
		{"link", "see [the docs](https://x.org/a_b) now", "<p id=\"p1\">see <a href=\"https://x.org/a_b\">the docs</a> now</p>\n"},
		{"link with title and markup", "[**b** `c`](/p \"T\")", "<p id=\"p1\"><a href=\"/p\" title=\"T\"><strong>b</strong> <code>c</code></a></p>\n"},
		{"link with parens in url", "[w](https://en.wikipedia.org/wiki/Go_(language))",
			"<p id=\"p1\"><a href=\"https://en.wikipedia.org/wiki/Go_(language)\">w</a></p>\n"},
		{"brackets without a url are text", "[not a link] and [x]", "<p id=\"p1\">[not a link] and [x]</p>\n"},
		{"a javascript url is refused", "[x](javascript:alert(1))", "<p id=\"p1\"><a href=\"#\">x</a></p>\n"},
		{"image", "![a cat](cat.png)", "<p id=\"p1\"><img src=\"cat.png\" alt=\"a cat\"></p>\n"},
		{"autolink", "<https://x.org>", "<p id=\"p1\"><a href=\"https://x.org\">https://x.org</a></p>\n"},
		{"hard break", "a  \nb", "<p id=\"p1\">a<br>\nb</p>\n"},
		{"tight bullet list", "- a\n- *b*\n- c", "<ul>\n<li>a</li>\n<li><em>b</em></li>\n<li>c</li>\n</ul>\n"},
		{"star and plus bullets", "* a\n+ b", "<ul>\n<li>a</li>\n</ul>\n<ul>\n<li>b</li>\n</ul>\n"},
		{"loose list has paragraphs", "- a\n\n- b", "<ul>\n<li><p id=\"p1\">a</p></li>\n<li><p id=\"p2\">b</p></li>\n</ul>\n"},
		{"ordered list", "1. a\n2. b", "<ol>\n<li>a</li>\n<li>b</li>\n</ol>\n"},
		{"ordered list from three", "3. a\n4. b", "<ol start=\"3\">\n<li>a</li>\n<li>b</li>\n</ol>\n"},
		{"nested list", "- a\n  - b\n  - c\n- d",
			"<ul>\n<li>a\n<ul>\n<li>b</li>\n<li>c</li>\n</ul></li>\n<li>d</li>\n</ul>\n"},
		{"deeply nested", "- a\n  - b\n    - c",
			"<ul>\n<li>a\n<ul>\n<li>b\n<ul>\n<li>c</li>\n</ul></li>\n</ul></li>\n</ul>\n"},
		{"ordered holding bullets", "1. a\n   - b\n2. c",
			"<ol>\n<li>a\n<ul>\n<li>b</li>\n</ul></li>\n<li>c</li>\n</ol>\n"},
		{"lazy continuation", "- a\nstill a\n- b", "<ul>\n<li>a\nstill a</li>\n<li>b</li>\n</ul>\n"},
		{"code fence in a list item", "- a\n  ```\n  x\n  ```\n- b",
			"<ul>\n<li>a\n<pre><code>x\n</code></pre></li>\n<li>b</li>\n</ul>\n"},
		{"list ends at an unindented paragraph", "- a\n\nafter", "<ul>\n<li>a</li>\n</ul>\n<p id=\"p1\">after</p>\n"},
		{"a list interrupts a paragraph", "text\n- a", "<p id=\"p1\">text</p>\n<ul>\n<li>a</li>\n</ul>\n"},
		{"a number other than 1 does not interrupt", "in\n2014. a year", "<p id=\"p1\">in\n2014. a year</p>\n"},
		{"dash without space is text", "-a", "<p id=\"p1\">-a</p>\n"},
		{"table with alignment", "| a | b | c |\n|:--|:-:|--:|\n| 1 | `x|y` | **z** |",
			"<div class=\"table-wrap\"><table>\n<thead><tr><th style=\"text-align:left\">a</th><th style=\"text-align:center\">b</th><th style=\"text-align:right\">c</th></tr></thead>\n" +
				"<tbody>\n<tr><td style=\"text-align:left\">1</td><td style=\"text-align:center\"><code>x|y</code></td><td style=\"text-align:right\"><strong>z</strong></td></tr>\n</tbody></table></div>\n"},
		{"table without outer pipes, short and long rows", "a | b\n---|---\n1 |\n1 | 2 | 3",
			"<div class=\"table-wrap\"><table>\n<thead><tr><th>a</th><th>b</th></tr></thead>\n<tbody>\n<tr><td>1</td><td></td></tr>\n<tr><td>1</td><td>2</td></tr>\n</tbody></table></div>\n"},
		// GFM would take a pipe-less line as a one-cell row; here it ends the table.
		{"a line without a pipe ends a table", "| a |\n|---|\n| 1 |\nplain",
			"<div class=\"table-wrap\"><table>\n<thead><tr><th>a</th></tr></thead>\n<tbody>\n<tr><td>1</td></tr>\n</tbody></table></div>\n<p id=\"p1\">plain</p>\n"},
		{"table escaped pipe", "| a |\n|---|\n| x \\| y |",
			"<div class=\"table-wrap\"><table>\n<thead><tr><th>a</th></tr></thead>\n<tbody>\n<tr><td>x | y</td></tr>\n</tbody></table></div>\n"},
		// From essay-surface-quality.md: an unpaired ``` once swallowed the row's later pipes.
		{"an unpaired backtick run in a cell keeps the pipes after it", "| a | b |\n|---|---|\n| ```dot x | `c` |",
			"<div class=\"table-wrap\"><table>\n<thead><tr><th>a</th><th>b</th></tr></thead>\n<tbody>\n<tr><td>```dot x</td><td><code>c</code></td></tr>\n</tbody></table></div>\n"},
		{"pipes without a delimiter row are a paragraph", "a | b\nc | d", "<p id=\"p1\">a | b\nc | d</p>\n"},
		{"a column count mismatch is not a table", "| a | b |\n|---|", "<p id=\"p1\">| a | b |\n|---|</p>\n"},
		{"a table ends at a blank line", "| a |\n|---|\n| 1 |\n\nafter",
			"<div class=\"table-wrap\"><table>\n<thead><tr><th>a</th></tr></thead>\n<tbody>\n<tr><td>1</td></tr>\n</tbody></table></div>\n<p id=\"p1\">after</p>\n"},
		{"rules", "---\n***\n_ _ _", "<hr>\n<hr>\n<hr>\n"},
		{"two dashes are text", "--", "<p id=\"p1\">--</p>\n"},
		{"blockquote", "> a *b*\n> c", "<blockquote>\n<p id=\"p1\">a <em>b</em>\nc</p>\n</blockquote>\n"},
		{"html block passes through", "<div class=x>\n<p>hi</p>\n</div>\n\n*after*",
			"<div class=x>\n<p>hi</p>\n</div>\n<p id=\"p1\"><em>after</em></p>\n"},
		{"inline html is escaped", "a <span>b</span>", "<p id=\"p1\">a &lt;span&gt;b&lt;/span&gt;</p>\n"},
		{"a heading interrupts a paragraph", "a\n# B", "<p id=\"p1\">a</p>\n<h1 id=\"b\">B</h1>\n"},
		{"crlf line ends", "# A\r\n\r\nb\r\n", "<h1 id=\"a\">A</h1>\n<p id=\"p1\">b</p>\n"},
		{"empty", "", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Render(c.in)
			if got != c.want {
				t.Errorf("Render(%q)\n got: %q\nwant: %q", c.in, got, c.want)
			}
		})
	}
}

func TestTitle(t *testing.T) {
	cases := map[string]string{
		"# Hello\n\ntext":              "Hello",
		"intro\n# Later #":             "Later",
		"## Only a two":                "",
		"":                             "",
		"#NoSpace\n# Real":             "Real",
		"```\n# in code\n```\n# After": "After",
	}
	for in, want := range cases {
		if got := Title(in); got != want {
			t.Errorf("Title(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestTitleHTML(t *testing.T) {
	cases := map[string]string{
		"Review: gopher-roc `app/DateTime.roc`": "Review: gopher-roc app/DateTime.roc",
		"A <b> & *em* [link](x)":                "A &lt;b&gt; &amp; em link",
		"plain":                                 "plain",
	}
	for in, want := range cases {
		if got := TitleHTML(in); got != want {
			t.Errorf("TitleHTML(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestSamplesRender renders every sample note and checks the shape of what
// comes out: tags balanced, nothing from a fence leaking into markup.
func TestSamplesRender(t *testing.T) {
	paths, _ := filepath.Glob("notes/*.md")
	if len(paths) == 0 {
		t.Fatal("no sample notes in notes/")
	}
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		out := Render(string(src))
		for _, tag := range []string{"pre", "code", "table", "ul", "ol", "li", "blockquote", "em", "strong", "a"} {
			opens := strings.Count(out, "<"+tag+">") + strings.Count(out, "<"+tag+" ")
			closes := strings.Count(out, "</"+tag+">")
			// Raw HTML blocks in a note may carry their own; count only ours.
			if strings.Contains(string(src), "<"+tag) {
				continue
			}
			if opens != closes {
				t.Errorf("%s: <%s> opened %d times, closed %d", p, tag, opens, closes)
			}
		}
	}
}

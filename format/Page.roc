## A note's page as the server shows it: its title as escaped text, its body as
## HTML, and the blocks a reader can reply to. Each such block's element is
## marked `data-c="K"`, K counting from 0 in reading order, as the markdown
## renderer marks its own (markdown.go), so the server and the page's script
## treat both kinds of note alike. For each one the page keeps the text a reply
## anchors to, as a reader reads it, and the byte in the body just past its
## element, where the server puts its replies.
import Block
import Diagram
import Note
import NoteName
import Parse
import Phrase
import Table
import Text
import WebAddress

Page :: {
	title : Str,
	body : Str,
	blocks : List(Page.Commentable),
	diagram_count : U64,
}.{

	## A block a reader can reply to: the text a reply anchors to, and where its
	## element ends in the body.
	Commentable : { text : Str, end : U64 }

	## The page of a note: the title and the subtitle as blocks of their own,
	## then each block in order.
	from_note : Note -> Page
	from_note = |note| {
		title = note.title
		titled = with_commentable(
			empty_page,
			{
				opening: "<h1",
				inner: phrase_html(title),
				closing: "</h1>",
				text: title.reading(),
			},
		)
			|> with_html("\n")
		subtitled =
			match note.subtitle {
				NoSubtitle => titled
				Subtitle(subtitle) =>
					with_commentable(
						titled,
						{
							opening: "<p class=\"subtitle\"",
							inner: text_html(subtitle),
							closing: "</p>",
							text: subtitle.reading(),
						},
					)
						|> with_html("\n")
			}
		written = note.blocks.fold(subtitled, with_block)
		Page.({ ..written, title: escaped_text(title.reading()) })
	}

	## The page that says why a note can't be shown, and nothing more: no
	## block of it can take a reply.
	refusal : Parse.Problem -> Page
	refusal = |problem|
		Page.(
			{
				title: "",
				body: "<p class=\"note-problem\">This note can't be shown. ${escaped_text(Parse.problem_message(problem))}.</p>\n",
				blocks: [],
				diagram_count: 0,
			},
		)

	## The page as the JSON the server reads (store.go's renderedNote).
	to_json : Page -> Str
	to_json = |Page.(page)|
		Json.to_str({
			title: page.title,
			body: page.body,
			blocks: page.blocks,
			diagrams: page.diagram_count,
		})
}

## A page being written: what `Page` holds, before its title is set.
Written : {
	title : Str,
	body : Str,
	blocks : List(Page.Commentable),
	diagram_count : U64,
}

empty_page : Written
empty_page = {
	title: "",
	body: "",
	blocks: [],
	diagram_count: 0,
}

## The page with a block a reader can reply to written onto it: its opening tag
## up to its attributes' end, its mark, its inner HTML and its closing tag.
with_commentable : Written,
{
	opening : Str,
	inner : Str,
	closing : Str,
	text : Str,
} -> Written
with_commentable = |page, { opening, inner, closing, text }| {
	mark = page.blocks.len().to_str()
	body = "${page.body}${opening} data-c=\"${mark}\">${inner}${closing}"
	{
		..page,
		body,
		blocks: page.blocks.append({ text, end: body.count_utf8_bytes() }),
	}
}

with_html : Written, Str -> Written
with_html = |page, html| { ..page, body: "${page.body}${html}" }

with_block : Written, Block -> Written
with_block = |page, block|
	match block {
		Paragraph(prose) =>
			page
				|> with_commentable({
					opening: "<p",
					inner: prose_html(prose),
					closing: "</p>",
					text: prose_reading(prose),
				})
				|> with_html("\n")
		Heading({ level, title }) => {
			tag =
				match level {
					Section => "h2"
					Subsection => "h3"
				}
			page
				|> with_commentable({
					opening: "<${tag}",
					inner: phrase_html(title),
					closing: "</${tag}>",
					text: title.reading(),
				})
				|> with_html("\n")
		}
		Bullets(points) =>
			points.fold(with_html(page, "<ul>\n"), with_point)
				|> with_html("</ul>\n")
		Numbered({ first_number, points }) => {
			start = if (first_number == 1) "" else " start=\"${first_number.to_str()}\""
			points.fold(with_html(page, "<ol${start}>\n"), with_point)
				|> with_html("</ol>\n")
		}
		Quote(proses) =>
			proses.fold(
				with_html(page, "<blockquote>\n"),
				|quoted, prose|
					quoted
						|> with_commentable({
							opening: "<p",
							inner: prose_html(prose),
							closing: "</p>",
							text: prose_reading(prose),
						})
						|> with_html("\n"),
			)
				|> with_html("</blockquote>\n")
		Code({ language, body }) => {
			class =
				match language {
					NoLanguage => ""
					Language(name) => " class=\"language-${escaped_text(name)}\""
				}
			page
				|> with_commentable({
					opening: "<pre",
					inner: "<code${class}>${escaped_text(body)}\n</code>",
					closing: "</pre>",
					text: body,
				})
				|> with_html("\n")
		}
		Diagram(diagram) => {
			source = diagram.source()
			drawn =
				page
					|> with_commentable({
						opening: "<figure class=\"dot\"",
						inner: "<pre><code class=\"language-dot\">${escaped_text(source)}\n</code></pre>",
						closing: "</figure>",
						text: source,
					})
					|> with_html("\n")
			{ ..drawn, diagram_count: drawn.diagram_count + 1 }
		}
		Divider => with_html(page, "<hr>\n")
		Table(table) => {
			headings = table.column_headings()
			rows = table.rows()
			head = Str.join_with(headings.map(|heading| "<th>${text_html(heading)}</th>"), "")
			body_rows = Str.join_with(rows.map(row_html), "")
			page
				|> with_commentable({
					opening: "<div class=\"table-wrap\"",
					inner: "<table>\n<thead><tr>${head}</tr></thead>\n<tbody>\n${body_rows}</tbody></table>",
					closing: "</div>",
					text: Str.join_with([headings].concat(rows).map(row_reading), "\n"),
				})
				|> with_html("\n")
		}
		Image(_) => crash "Page.with_block: ImageFile has no maker yet, so no note holds an image"
		Widget(_) => crash "Page.with_block: Widget has no maker yet, so no note holds one"
	}

## A list's point: its text a block of its own, in a span as the markdown
## renderer writes a tight list's item, then its subpoints as a list inside it.
with_point : Written, Block.Point -> Written
with_point = |page, { prose, subpoints }| {
	opened =
		with_html(page, "<li>")
			|> with_commentable({
				opening: "<span class=\"ct\"",
				inner: prose_html(prose),
				closing: "</span>",
				text: prose_reading(prose),
			})
	with_subpoints =
		if subpoints.is_empty() {
			opened
		} else {
			subpoints.fold(
				with_html(opened, "\n<ul>\n"),
				|listed, subpoint|
					with_html(listed, "<li>")
						|> with_commentable({
							opening: "<span class=\"ct\"",
							inner: prose_html(subpoint),
							closing: "</span>",
							text: prose_reading(subpoint),
						})
						|> with_html("</li>\n"),
			)
				|> with_html("</ul>")
		}
	with_html(with_subpoints, "</li>\n")
}

## A table's row as the markdown renderer writes one (markdown.go's table).
row_html : List(Text) -> Str
row_html = |cells| "<tr>${Str.join_with(cells.map(|cell| "<td>${text_html(cell)}</td>"), "")}</tr>\n"

## A table's row as a reader reads it: its cells, a `|` between them.
row_reading : List(Text) -> Str
row_reading = |cells| Str.join_with(cells.map(|cell| cell.reading()), " | ")

prose_html : Block.Prose -> Str
prose_html = |{ lead, text }|
	match lead {
		NoLead => text_html(text)
		Lead(phrase) => "<strong>${phrase_html(phrase)}</strong> ${text_html(text)}"
	}

prose_reading : Block.Prose -> Str
prose_reading = |{ lead, text }|
	match lead {
		NoLead => text.reading()
		Lead(phrase) => "${phrase.reading()} ${text.reading()}"
	}

text_html : Text -> Str
text_html = |text| Str.join_with(text.spans().map(span_html), "")

span_html : Text.Span -> Str
span_html = |span|
	match span {
		Unstyled(phrase) => phrase_html(phrase)
		Strong(phrase) => "<strong>${phrase_html(phrase)}</strong>"
		Emphasis(phrase) => "<em>${phrase_html(phrase)}</em>"
		Link({ label, target }) => {
			href =
				match target {
					Note(name) => "/notes/${name.to_str()}"
					Web(address) => address.to_str()
				}
			"<a href=\"${escaped_text(href)}\">${phrase_html(label)}</a>"
		}
	}

phrase_html : Phrase -> Str
phrase_html = |phrase| Str.join_with(phrase.pieces().map(piece_html), "")

piece_html : Phrase.Piece -> Str
piece_html = |piece|
	match piece {
		Plain(words) => escaped_text(words)
		Code(code) => "<code>${escaped_text(code)}</code>"
	}

## Text made safe between tags and inside a quoted attribute, as markdown.go's
## escapeHTML makes it.
escaped_text : Str -> Str
escaped_text = |text|
	text
		.replace_each("&", "&amp;")
		.replace_each("<", "&lt;")
		.replace_each(">", "&gt;")
		.replace_each("\"", "&quot;")

# Every kind of block the parser reads, drawn byte for byte, and each block's
# end just past its element.

page_of : Str -> Written
page_of = |source|
	match Parse.note(source) {
		Ok(note) => {
			Page.(page) = Page.from_note(note)
			page
		}
		Err(problem) => crash "Page's tests: the sources here all parse, not ${Parse.problem_message(problem)}"
	}

every_kind : Str
every_kind =
	\\# A `t` & <u>
	\\
	\\*2026-11-14. For [you](https://a.org/?x=1&y=2).*
	\\
	\\**Lead.** Then *words* and [a note](other-note).
	\\
	\\## Section
	\\
	\\### Subsection
	\\
	\\- **One.** point
	\\  going on
	\\  - sub
	\\
	\\- two
	\\
	\\3. three
	\\
	\\> quoted
	\\>
	\\> again
	\\
	\\```
	\\a < b
	\\```
	\\
	\\```go
	\\x := 1
	\\```
	\\
	\\```dot
	\\digraph { a -> "b" }
	\\```
	\\
	\\| `a|b` | **B** |
	\\|---|---|
	\\| 1 & 2 | [x](https://x.org) |
	\\
	\\---
	\\
	\\End.

expect page_of(every_kind).title == "A t &amp; &lt;u&gt;"
expect
	page_of(every_kind).body
		==
		\\<h1 data-c="0">A <code>t</code> &amp; &lt;u&gt;</h1>
		\\<p class="subtitle" data-c="1">2026-11-14. For <a href="https://a.org/?x=1&amp;y=2">you</a>.</p>
		\\<p data-c="2"><strong>Lead.</strong> Then <em>words</em> and <a href="/notes/other-note">a note</a>.</p>
		\\<h2 data-c="3">Section</h2>
		\\<h3 data-c="4">Subsection</h3>
		\\<ul>
		\\<li><span class="ct" data-c="5"><strong>One.</strong> point going on</span>
		\\<ul>
		\\<li><span class="ct" data-c="6">sub</span></li>
		\\</ul></li>
		\\<li><span class="ct" data-c="7">two</span></li>
		\\</ul>
		\\<ol start="3">
		\\<li><span class="ct" data-c="8">three</span></li>
		\\</ol>
		\\<blockquote>
		\\<p data-c="9">quoted</p>
		\\<p data-c="10">again</p>
		\\</blockquote>
		\\<pre data-c="11"><code>a &lt; b
		\\</code></pre>
		\\<pre data-c="12"><code class="language-go">x := 1
		\\</code></pre>
		\\<figure class="dot" data-c="13"><pre><code class="language-dot">digraph { a -&gt; &quot;b&quot; }
		\\</code></pre></figure>
		\\<div class="table-wrap" data-c="14"><table>
		\\<thead><tr><th><code>a|b</code></th><th><strong>B</strong></th></tr></thead>
		\\<tbody>
		\\<tr><td>1 &amp; 2</td><td><a href="https://x.org">x</a></td></tr>
		\\</tbody></table></div>
		\\<hr>
		\\<p data-c="15">End.</p>
		\\
expect page_of(every_kind).blocks.map(|block| block.text) == [
	"A t & <u>",
	"2026-11-14. For you.",
	"Lead. Then words and a note.",
	"Section",
	"Subsection",
	"One. point going on",
	"sub",
	"two",
	"three",
	"quoted",
	"again",
	"a < b",
	"x := 1",
	"digraph { a -> \"b\" }",
	"a|b | B\n1 & 2 | x",
	"End.",
]
expect page_of(every_kind).diagram_count == 1

# Each end is just past its element's closing tag, in bytes.
expect {
	page = page_of(every_kind)
	bytes = page.body.to_utf8()
	page.blocks.all(
		|block| {
			before =
				match Str.from_utf8(bytes.take_first(block.end)) {
					Ok(text) => text
					Err(_) => crash "Page's tests: a block ends at a tag's `>`, never inside a character"
				}
			closings = ["</h1>", "</h2>", "</h3>", "</p>", "</span>", "</pre>", "</figure>", "</div>"]
			closings.any(|closing| before.ends_with(closing))
		},
	)
}
expect {
	page = page_of("# Ü\n\nÉ.\n")
	page.blocks.map(|block| block.end) == [
		"<h1 data-c=\"0\">Ü</h1>".count_utf8_bytes(),
		"<h1 data-c=\"0\">Ü</h1>\n<p data-c=\"1\">É.</p>".count_utf8_bytes(),
	]
}

expect {
	Page.(page) = Page.refusal({ line: 3, fault: UnclosedFence })
	page.body == "<p class=\"note-problem\">This note can't be shown. line 3: this code fence has no closing fence.</p>\n" and page.blocks == [] and page.title == ""
}
expect Page.from_note({
	title: one_word("T"),
	subtitle: NoSubtitle,
	blocks: [],
}).to_json() == "{\"blocks\":[{\"end\":21,\"text\":\"T\"}],\"body\":\"<h1 data-c=\\\"0\\\">T</h1>\\n\",\"diagrams\":0,\"title\":\"T\"}"

one_word : Str -> Phrase
one_word = |word|
	match Phrase.from_pieces([Plain(word)]) {
		Ok(phrase) => phrase
		Err(_) => crash "Page's tests: a phrase of one word"
	}

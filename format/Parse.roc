## Reads a note written in the surface's own format, a `.note` file, into a
## `Note`, or says the first line it can't read and why.
##
## The syntax is the smallest one the types need, and looks like the markdown
## the notes are written in now. The first line is the title, `# ` and a
## phrase. A paragraph right after it, wrapped in single `*`s, is the
## subtitle. After that, blocks are separated by blank lines:
##
## - `## ` and `### ` start a section and a subsection heading;
## - `- ` starts a bulleted point and `1. ` a numbered one, a point's
##   subpoints are indented `- ` lines, and an indented line goes on with the
##   point or subpoint above it;
## - `>` starts each line of a quote, and a `>` line alone parts its
##   paragraphs;
## - a fence of three backticks holds code, its language after the fence, and a
##   fence named `dot` holds a diagram;
## - `|` starts each line of a table: its headings, then a run of dashes for
##   each column (`|---|---|`), then its rows, each with a `|` before, between
##   and after its cells. Every cell holds text, and every row has a cell for
##   each heading. A `|` inside code stays in its cell;
## - `![description](file.png)` alone on its line is an image: a picture
##   beside the note in its folder, named as `ImageFile` says (a note's name,
##   then `.png`, `.jpg` or `.svg`, no folder), and a description in plain
##   words, which is the picture's alt text and its caption;
## - `---` alone is a divider;
## - anything else is a paragraph, its lines joined by spaces. One that opens
##   in bold has that bold as its lead.
##
## In text, `**` marks bold, `*` italics, a backtick code, and
## `[label](target)` a link to a note by its name or to an `http://` or
## `https://` address. A style holds only words and code. There are no
## escapes, so a lone `*` is an unclosed style. A line that would start a
## block, written straight after a paragraph's line, is refused rather than
## read as words: it needs a blank line before it.
import Block
import Diagram
import ImageFile
import Note
import NoteName
import Phrase
import Table
import Text
import WebAddress

Parse := [].{

	## Where a note can't be read, and why.
	Problem : { line : U64, fault : Parse.Fault }

	## What is wrong at a problem's line.
	Fault : [
		NoTitle,
		SecondTitle,
		HeadingTooDeep,
		StyleInHeading,
		NoBlankLineBefore,
		StrayListLine,
		UnclosedFence,
		UnclosedCode,
		UnclosedStyle,
		StyleInStyle,
		EmptyStyle,
		EmptyCode,
		UnclosedLink,
		NotALinkTarget(Str),
		NotADiagram,
		NotATableLine,
		NoTableDivider,
		TableRowWidth,
		EmptyCell,
		NotAnImageLine,
		NoImageDescription,
		NotAnImageFile({ file : Str, problem : ImageFile.Problem }),
	]

	## The note this source holds.
	note : Str -> Try(Note, Parse.Problem)
	note = |source| {
		lines = numbered_lines(source)
		title_at = next_filled(lines, 0)
		title_line =
			match lines.get(title_at) {
				Ok(line) => line
				Err(_) => return Err({ line: 1, fault: NoTitle })
			}
		if !title_line.text.starts_with("# ") {
			return Err({ line: title_line.number, fault: NoTitle })
		}
		title = phrase_in_heading(title_line.text.drop_prefix("# ").trim(), title_line.number)?
		opening = subtitle_at(lines, title_at + 1)?
		var $blocks = []
		var $at = next_filled(lines, opening.after)
		while $at < lines.len() {
			found = block_at(lines, $at)?
			$blocks = $blocks.append(found.block)
			$at = next_filled(lines, found.after)
		}
		Ok({ title, subtitle: opening.subtitle, blocks: $blocks })
	}

	## The problem in words, for the page that shows it.
	problem_message : Parse.Problem -> Str
	problem_message = |{ line, fault }| {
		why =
			match fault {
				NoTitle => "a note starts with its title, a line that starts `# `"
				SecondTitle => "a note has one title; a section heading starts `## `"
				HeadingTooDeep => "headings go two deep, `##` and `###`"
				StyleInHeading => "a title or a heading holds words and code, with no bold, italics or link"
				NoBlankLineBefore => "this line starts a block, so it needs a blank line before it"
				StrayListLine => "a list's line is a point, an indented subpoint, or an indented line that goes on with one"
				UnclosedFence => "this code fence has no closing fence"
				UnclosedCode => "this code span has no closing backtick"
				UnclosedStyle => "a `*` or `**` here has no partner; there are no escapes, so a lone `*` can't be written"
				StyleInStyle => "a style holds only words and code: no bold in italics, and no link in either"
				EmptyStyle => "a style here holds nothing"
				EmptyCode => "a code span here holds nothing"
				UnclosedLink => "a link is `[label](target)`, all on one paragraph"
				NotALinkTarget(target) => "`${target}` is neither a note's name (lowercase letters, digits and hyphens) nor an http:// or https:// address"
				NotADiagram => "a dot fence's source starts `digraph`, `graph` or `strict`"
				NotATableLine => "every line of a table starts and ends with `|`"
				NoTableDivider => "a table's second line is a run of dashes for each column, `|---|---|`"
				TableRowWidth => "this line has a different number of cells than the table has headings"
				EmptyCell => "a table cell holds words; an empty one can't be written"
				NotAnImageLine => "an image is a line of its own, `![description](file.png)`, with a blank line after it"
				NoImageDescription => "an image's description is its alt text and its caption, so it can't be empty"
				NotAnImageFile({ file, problem: InAFolder }) => "`${file}` names a folder; an image is a file beside the note, named with no `/`"
				NotAnImageFile({ file, problem: NotAPicture }) => "`${file}` isn't a picture: an image's file ends `.png`, `.jpg` or `.svg`"
				NotAnImageFile({ file, problem: NotAName }) => "`${file}` isn't a picture's name: before its extension come lowercase letters, digits and hyphens, starting with a letter or a digit"
			}
		"line ${line.to_str()}: ${why}"
	}
}

## One line of the source, with its number from 1.
Line : { number : U64, text : Str }

## A run of a span's contents before it is checked: words and code.
RawPiece : [Words(Str), CodeText(Str)]

## A span as the scanner finds it, before its phrase is checked.
RawSpan : [
	Unstyled(List(RawPiece)),
	Bold(List(RawPiece)),
	Italic(List(RawPiece)),
	LinkTo({ label_pieces : List(RawPiece), target : Str }),
]

numbered_lines : Str -> List(Line)
numbered_lines = |source| {
	var $lines = []
	var $number = 1
	for text in source.split_on("\n") {
		$lines = $lines.append({ number: $number, text: text.drop_suffix("\r") })
		$number = $number + 1
	}
	$lines
}

is_blank : Line -> Bool
is_blank = |line| line.text.trim().is_empty()

## The index of the first line at or after `at` that isn't blank, or the
## number of lines when none is.
next_filled : List(Line), U64 -> U64
next_filled = |lines, at| {
	var $i = at
	while $i < lines.len() and lines.get($i).map_ok(is_blank) == Ok(True) {
		$i = $i + 1
	}
	$i
}

line_at : List(Line), U64 -> Line
line_at = |lines, at|
	match lines.get(at) {
		Ok(line) => line
		Err(_) => crash "Parse.line_at: callers ask only for lines before the end"
	}

## The lines from `at` up to the next blank line or the end.
run_from : List(Line), U64 -> List(Line)
run_from = |lines, at| {
	var $run = []
	var $i = at
	while $i < lines.len() and !is_blank(line_at(lines, $i)) {
		$run = $run.append(line_at(lines, $i))
		$i = $i + 1
	}
	$run
}

## The subtitle, if the first block after the title is a paragraph wrapped in
## single `*`s, and the index of the line after what was read.
subtitle_at : List(Line), U64 -> Try({ subtitle : Note.Subtitle, after : U64 }, Parse.Problem)
subtitle_at = |lines, from| {
	at = next_filled(lines, from)
	run = run_from(lines, at)
	joined = joined_text(run)
	opens_in_italics = joined.starts_with("*") and !joined.starts_with("**")
	closes_in_italics = joined.ends_with("*") and !joined.ends_with("**")
	is_wrapped = opens_in_italics and closes_in_italics
	if run.is_empty() or !is_wrapped {
		Ok({ subtitle: NoSubtitle, after: from })
	} else {
		first = line_at(run, 0).number
		inner = joined.drop_prefix("*").drop_suffix("*")
		subtitle = text_of(scanned_spans(inner, first)?, first)?
		Ok({ subtitle: Subtitle(subtitle), after: at + run.len() })
	}
}

joined_text : List(Line) -> Str
joined_text = |run_lines| Str.join_with(run_lines.map(|line| line.text.trim()), " ")

## Whether a line, as the first of a block, would start something other than a
## paragraph.
starts_block : Str -> Bool
starts_block = |text| {
	is_marked = ["```", "#", ">", "- ", "|", "!["].any(|opening| text.starts_with(opening))
	is_marked or text.trim() == "---" or numbered_marker(text).is_ok()
}

## The block that starts at line `at`, which isn't blank, and the index of the
## line after it.
block_at : List(Line), U64 -> Try({ block : Block, after : U64 }, Parse.Problem)
block_at = |lines, at| {
	line = line_at(lines, at)
	text = line.text
	if text.starts_with("```") {
		fence_at(lines, at)
	} else if text.starts_with("# ") {
		Err({ line: line.number, fault: SecondTitle })
	} else if text.starts_with("## ") {
		title = phrase_in_heading(text.drop_prefix("## ").trim(), line.number)?
		Ok({ block: Block.Heading({ level: Section, title }), after: at + 1 })
	} else if text.starts_with("### ") {
		title = phrase_in_heading(text.drop_prefix("### ").trim(), line.number)?
		Ok({ block: Block.Heading({ level: Subsection, title }), after: at + 1 })
	} else if text.starts_with("####") {
		Err({ line: line.number, fault: HeadingTooDeep })
	} else if text.trim() == "---" {
		Ok({ block: Block.Divider, after: at + 1 })
	} else if text.starts_with(">") {
		quote_at(lines, at)
	} else if text.starts_with("|") {
		table_at(lines, at)
	} else if text.starts_with("![") {
		image_at(lines, at)
	} else if text.starts_with("- ") or numbered_marker(text).is_ok() {
		list_at(lines, at)
	} else {
		paragraph_at(lines, at)
	}
}

fence_at : List(Line), U64 -> Try({ block : Block, after : U64 }, Parse.Problem)
fence_at = |lines, at| {
	opener = line_at(lines, at)
	language = opener.text.drop_prefix("```").trim()
	var $body = []
	var $i = at + 1
	while $i < lines.len() {
		line = line_at(lines, $i)
		if line.text.trim() == "```" {
			source = Str.join_with($body, "\n")
			block =
				if language == "dot" {
					match Diagram.from_source(source) {
						Ok(diagram) => Block.Diagram(diagram)
						Err(NotDot) => return Err({ line: opener.number, fault: NotADiagram })
					}
				} else if language.is_empty() {
					Block.Code({ language: NoLanguage, body: source })
				} else {
					Block.Code({ language: Language(language), body: source })
				}
			return Ok({ block, after: $i + 1 })
		}
		$body = $body.append(line.text)
		$i = $i + 1
	}
	Err({ line: opener.number, fault: UnclosedFence })
}

paragraph_at : List(Line), U64 -> Try({ block : Block, after : U64 }, Parse.Problem)
paragraph_at = |lines, at| {
	run = run_from(lines, at)
	for line in run.drop_first(1) {
		if starts_block(line.text) {
			return Err({ line: line.number, fault: NoBlankLineBefore })
		}
	}
	prose = prose_of(joined_text(run), line_at(lines, at).number)?
	Ok({ block: Block.Paragraph(prose), after: at + run.len() })
}

## A quote: the `>` lines from `at`, each stripped of its `>` and the space
## after it, in paragraphs parted by lines with nothing after the `>`.
quote_at : List(Line), U64 -> Try({ block : Block, after : U64 }, Parse.Problem)
quote_at = |lines, at| {
	var $paragraphs = []
	var $current = []
	var $i = at
	while $i < lines.len() and line_at(lines, $i).text.starts_with(">") {
		line = line_at(lines, $i)
		inner = line.text.drop_prefix(">").trim()
		if inner.is_empty() {
			$paragraphs = $paragraphs.append($current)
			$current = []
		} else {
			$current = $current.append({ number: line.number, text: inner })
		}
		$i = $i + 1
	}
	var $proses = []
	for run in $paragraphs.append($current) {
		if !run.is_empty() {
			$proses = $proses.append(prose_of(joined_text(run), line_at(run, 0).number)?)
		}
	}
	Ok({ block: Block.Quote($proses), after: $i })
}

## A table: the lines from `at` to the next blank line, its headings first,
## then its divider, then its rows.
table_at : List(Line), U64 -> Try({ block : Block, after : U64 }, Parse.Problem)
table_at = |lines, at| {
	run = run_from(lines, at)
	for line in run {
		if !is_table_line(line.text) {
			return Err({ line: line.number, fault: NotATableLine })
		}
	}
	heading_line = line_at(run, 0)
	column_headings = cells_of(heading_line)?
	divider =
		match run.get(1) {
			Ok(line) => line
			Err(_) => return Err({ line: heading_line.number, fault: NoTableDivider })
		}
	divider_cells = raw_cells(divider.text)
	if !divider_cells.all(is_dashes) {
		return Err({ line: divider.number, fault: NoTableDivider })
	}
	if divider_cells.len() != column_headings.len() {
		return Err({ line: divider.number, fault: TableRowWidth })
	}
	var $rows = []
	for line in run.drop_first(2) {
		$rows = $rows.append(cells_of(line)?)
	}
	match Table.from_rows({ column_headings, rows: $rows }) {
		Ok(table) => Ok({ block: Block.Table(table), after: at + run.len() })
		Err(NoColumns) => crash "Parse.table_at: a table's line holds at least one cell"
		Err(RowWidth(index)) => Err({ line: line_at(run, index + 2).number, fault: TableRowWidth })
	}
}

## Whether a line can be a table's: `|` first and last, with room for a cell
## between.
is_table_line : Str -> Bool
is_table_line = |text| {
	trimmed = text.trim()
	trimmed.count_utf8_bytes() >= 2 and trimmed.starts_with("|") and trimmed.ends_with("|")
}

is_dashes : Str -> Bool
is_dashes = |cell| !cell.is_empty() and cell.to_utf8().all(|b| b == '-')

## A table line's cells as written, each trimmed: split at each `|` outside
## code, the first and last `|` taken off.
raw_cells : Str -> List(Str)
raw_cells = |text| {
	inner = text.trim().drop_prefix("|").drop_suffix("|")
	var $cells = []
	var $current = []
	var $in_code = False
	for b in inner.to_utf8() {
		if b == '`' {
			$in_code = !$in_code
			$current = $current.append(b)
		} else if b == '|' and !$in_code {
			$cells = $cells.append(utf8($current).trim())
			$current = []
		} else {
			$current = $current.append(b)
		}
	}
	$cells.append(utf8($current).trim())
}

## A table line's cells, each read as text.
cells_of : Line -> Try(List(Text), Parse.Problem)
cells_of = |line| {
	var $cells = []
	for raw in raw_cells(line.text) {
		if raw.is_empty() {
			return Err({ line: line.number, fault: EmptyCell })
		}
		$cells = $cells.append(text_of(scanned_spans(raw, line.number)?, line.number)?)
	}
	Ok($cells)
}

## An image: the one line `![description](file)` and nothing after it on the
## line or below it before a blank line. The description is plain words, read
## as written, since it is an attribute's text as well as the caption's.
image_at : List(Line), U64 -> Try({ block : Block, after : U64 }, Parse.Problem)
image_at = |lines, at| {
	line = line_at(lines, at)
	run = run_from(lines, at)
	if run.len() > 1 {
		return Err({ line: line_at(run, 1).number, fault: NotAnImageLine })
	}
	bytes = line.text.trim().to_utf8()
	close = find_byte(bytes, { byte: ']', at: 2 }) ? |NotFound| { line: line.number, fault: NotAnImageLine }
	description = utf8(bytes.sublist({ start: 2, len: close - 2 })).trim()
	target = utf8(bytes.drop_first(close + 1))
	inner = target.drop_prefix("(").drop_suffix(")")
	is_wrapped = target.starts_with("(") and target.ends_with(")")
	if !is_wrapped or inner.to_utf8().any(|b| b == '(' or b == ')') {
		return Err({ line: line.number, fault: NotAnImageLine })
	}
	if description.is_empty() {
		return Err({ line: line.number, fault: NoImageDescription })
	}
	file = ImageFile.from_str(inner) ? |problem| { line: line.number, fault: NotAnImageFile({ file: inner, problem }) }
	Ok({ block: Block.Image({ file, description }), after: at + 1 })
}

## A point as the list's lines give it, before its text is read: its first
## line's number and its lines, and its subpoints the same way.
RawPoint : { lines : List(Line), subpoints : List(List(Line)) }

## A list from `at`: bulleted or numbered by its first line. Blank lines between
## its points don't end it.
list_at : List(Line), U64 -> Try({ block : Block, after : U64 }, Parse.Problem)
list_at = |lines, at| {
	first = line_at(lines, at)
	numbering = numbered_marker(first.text)
	var $points = []
	var $i = at
	var $going = True
	while $going {
		line = line_at(lines, $i)
		$points =
			if is_point_line(line.text, numbering) {
				$points.append({
					lines: [{ number: line.number, text: without_marker(line.text) }],
					subpoints: [],
				})
			} else if line.text.starts_with(" ") and line.text.trim().starts_with("- ") {
				subpoint = [{ number: line.number, text: line.text.trim().drop_prefix("- ") }]
				match $points.last() {
					Ok(point) => $points.drop_last(1).append({
						..point,
						subpoints: point.subpoints.append(subpoint),
					})
					Err(_) => crash "Parse.list_at: a list starts with a point"
				}
			} else if line.text.starts_with(" ") {
				match $points.last() {
					Ok(point) => $points.drop_last(1).append(going_on(point, line))
					Err(_) => crash "Parse.list_at: a list starts with a point"
				}
			} else {
				return Err({ line: line.number, fault: StrayListLine })
			}
		next = $i + 1
		after_blanks = next_filled(lines, next)
		if next < lines.len() and !is_blank(line_at(lines, next)) {
			$i = next
		} else if after_blanks < lines.len() and is_point_line(
			line_at(lines, after_blanks).text,
			numbering,
		) {
			$i = after_blanks
		} else {
			$i = next
			$going = False
		}
	}
	var $read = []
	for point in $points {
		var $subpoints = []
		for subpoint in point.subpoints {
			$subpoints = $subpoints.append(
				prose_of(joined_text(subpoint), line_at(subpoint, 0).number)?,
			)
		}
		prose = prose_of(joined_text(point.lines), line_at(point.lines, 0).number)?
		$read = $read.append({ prose, subpoints: $subpoints })
	}
	block =
		match numbering {
			Ok({ number, .. }) => Block.Numbered({ first_number: number, points: $read })
			Err(NotNumbered) => Block.Bullets($read)
		}
	Ok({ block, after: $i })
}

## A line of the point or subpoint above it: the last subpoint if it has one.
going_on : RawPoint, Line -> RawPoint
going_on = |point, line|
	match point.subpoints.last() {
		Ok(subpoint) => {
			..point,
			subpoints: point.subpoints.drop_last(1).append(subpoint.append(line)),
		}
		Err(_) => { ..point, lines: point.lines.append(line) }
	}

## Whether a line starts a point of this list: a `- ` in a bulleted list, a
## number in a numbered one.
is_point_line : Str, Try({ number : U64, rest : Str }, [NotNumbered]) -> Bool
is_point_line = |text, numbering|
	match numbering {
		Ok(_) => numbered_marker(text).is_ok()
		Err(NotNumbered) => text.starts_with("- ")
	}

without_marker : Str -> Str
without_marker = |text|
	match numbered_marker(text) {
		Ok({ rest, .. }) => rest
		Err(NotNumbered) => text.drop_prefix("- ")
	}

## The number a numbered point's line starts with, and the line after its
## `. `.
numbered_marker : Str -> Try({ number : U64, rest : Str }, [NotNumbered])
numbered_marker = |text| {
	bytes = text.to_utf8()
	digit_count = bytes.find_first_index(|b| b < '0' or b > '9') ?? bytes.len()
	digits = bytes.take_first(digit_count)
	after = bytes.drop_first(digit_count)
	if digits.is_empty() or digits.len() > 9 or after.take_first(2) != ['.', ' '] {
		Err(NotNumbered)
	} else {
		number = digits.fold(0, |n, digit| n * 10 + (digit - '0').to_u64())
		Ok({ number, rest: utf8(after.drop_first(2)) })
	}
}

## The prose of a paragraph's words: a lead when it opens in bold and goes on
## after.
prose_of : Str, U64 -> Try(Block.Prose, Parse.Problem)
prose_of = |words, line| {
	spans = scanned_spans(words, line)?
	match spans {
		[Bold(lead_pieces), .. as rest] if !rest.is_empty() => {
			lead = phrase_of(lead_pieces, line)?
			text = text_of(without_leading_space(rest), line)?
			Ok({ lead: Lead(lead), text })
		}
		_ => {
			text = text_of(spans, line)?
			Ok({ lead: NoLead, text })
		}
	}
}

## The spans with the space a lead leaves before them taken off, and the
## first span dropped if that was all it held.
without_leading_space : List(RawSpan) -> List(RawSpan)
without_leading_space = |spans|
	match spans {
		[Unstyled([Words(words), .. as pieces]), .. as rest] => {
			trimmed = words.trim_start()
			kept = if (trimmed.is_empty()) pieces else [Words(trimmed)].concat(pieces)
			if kept.is_empty() {
				rest
			} else {
				[Unstyled(kept)].concat(rest)
			}
		}
		_ => spans
	}

phrase_in_heading : Str, U64 -> Try(Phrase, Parse.Problem)
phrase_in_heading = |words, line|
	match scanned_spans(words, line)? {
		[Unstyled(pieces)] => phrase_of(pieces, line)
		[] => Err({ line, fault: NoTitle })
		_ => Err({ line, fault: StyleInHeading })
	}

phrase_of : List(RawPiece), U64 -> Try(Phrase, Parse.Problem)
phrase_of = |raw_pieces, line| {
	pieces = raw_pieces.map(
		|piece|
			match piece {
				Words(words) => Plain(words)
				CodeText(code) => Code(code)
			},
	)
	match Phrase.from_pieces(pieces) {
		Ok(phrase) => Ok(phrase)
		Err(EmptyPhrase) => Err({ line, fault: EmptyStyle })
		Err(EmptyCode) => Err({ line, fault: EmptyCode })
		Err(LineBreak) => crash "Parse.phrase_of: a paragraph's lines are joined by spaces before its spans are read"
	}
}

text_of : List(RawSpan), U64 -> Try(Text, Parse.Problem)
text_of = |raw_spans, line| {
	var $spans = []
	for span in raw_spans {
		read =
			match span {
				Unstyled(pieces) => Unstyled(phrase_of(pieces, line)?)
				Bold(pieces) => Strong(phrase_of(pieces, line)?)
				Italic(pieces) => Emphasis(phrase_of(pieces, line)?)
				LinkTo({ label_pieces, target }) => Link({
					label: phrase_of(label_pieces, line)?,
					target: target_of(target, line)?,
				})
			}
		$spans = $spans.append(read)
	}
	match Text.from_spans($spans) {
		Ok(text) => Ok(text)
		Err(EmptyText) => Err({ line, fault: EmptyStyle })
		Err(UnstyledTwice) => crash "Parse.text_of: the scanner never puts two runs of words side by side"
	}
}

target_of : Str, U64 -> Try(Text.Target, Parse.Problem)
target_of = |target, line|
	if target.starts_with("http://") or target.starts_with("https://") {
		match WebAddress.from_str(target) {
			Ok(address) => Ok(Web(address))
			Err(NotAWebAddress) => Err({ line, fault: NotALinkTarget(target) })
		}
	} else {
		match NoteName.from_str(target) {
			Ok(name) => Ok(Note(name))
			Err(NotANoteName) => Err({ line, fault: NotALinkTarget(target) })
		}
	}

## Where the scanner is: in plain words, or inside a style or a link's label.
Inside : [InWords, InBold, InItalic, InLabel]

## A paragraph's words as spans, read left to right: a backtick opens code to
## the next backtick, `**` and `*` open and close bold and italics, and `[`
## opens a link's label, which `](target)` closes.
scanned_spans : Str, U64 -> Try(List(RawSpan), Parse.Problem)
scanned_spans = |words, line| {
	bytes = words.to_utf8()
	var $spans = []
	var $pieces = []
	var $plain = []
	var $inside = InWords
	var $i = 0
	while $i < bytes.len() {
		c = byte_at(bytes, $i)
		if c == '`' {
			close = find_byte(
				bytes,
				{ byte: '`', at: $i + 1 },
			) ? |NotFound| { line, fault: UnclosedCode }
			$pieces = with_plain($pieces, $plain).append(
				CodeText(utf8(bytes.sublist({ start: $i + 1, len: close - $i - 1 }))),
			)
			$plain = []
			$i = close + 1
		} else if c == '*' {
			is_double = bytes.get($i + 1) == Ok('*')
			pieces = with_plain($pieces, $plain)
			$plain = []
			$pieces = []
			match ($inside, is_double) {
				(InWords, True) => {
					$spans = with_unstyled($spans, pieces)
					$inside = InBold
				}
				(InWords, False) => {
					$spans = with_unstyled($spans, pieces)
					$inside = InItalic
				}
				(InBold, True) => {
					if pieces.is_empty() {
						return Err({ line, fault: EmptyStyle })
					}
					$spans = $spans.append(Bold(pieces))
					$inside = InWords
				}
				(InItalic, False) => {
					if pieces.is_empty() {
						return Err({ line, fault: EmptyStyle })
					}
					$spans = $spans.append(Italic(pieces))
					$inside = InWords
				}
				(InBold, False) => return Err({ line, fault: StyleInStyle })
				(InItalic, True) => return Err({ line, fault: StyleInStyle })
				(InLabel, _) => return Err({ line, fault: StyleInStyle })
			}
			$i = if (is_double) $i + 2 else $i + 1
		} else if c == '[' {
			match $inside {
				InWords => {
					$spans = with_unstyled($spans, with_plain($pieces, $plain))
					$plain = []
					$pieces = []
					$inside = InLabel
				}
				_ => return Err({ line, fault: StyleInStyle })
			}
			$i = $i + 1
		} else if c == ']' and $inside == InLabel {
			if bytes.get($i + 1) != Ok('(') {
				return Err({ line, fault: UnclosedLink })
			}
			close = find_byte(
				bytes,
				{ byte: ')', at: $i + 2 },
			) ? |NotFound| { line, fault: UnclosedLink }
			label = with_plain($pieces, $plain)
			if label.is_empty() {
				return Err({ line, fault: EmptyStyle })
			}
			target = utf8(bytes.sublist({ start: $i + 2, len: close - $i - 2 }))
			$spans = $spans.append(LinkTo({ label_pieces: label, target }))
			$plain = []
			$pieces = []
			$inside = InWords
			$i = close + 1
		} else {
			$plain = $plain.append(c)
			$i = $i + 1
		}
	}
	match $inside {
		InWords => Ok(with_unstyled($spans, with_plain($pieces, $plain)))
		InBold => Err({ line, fault: UnclosedStyle })
		InItalic => Err({ line, fault: UnclosedStyle })
		InLabel => Err({ line, fault: UnclosedLink })
	}
}

with_plain : List(RawPiece), List(U8) -> List(RawPiece)
with_plain = |pieces, plain|
	if plain.is_empty() {
		pieces
	} else {
		pieces.append(Words(utf8(plain)))
	}

with_unstyled : List(RawSpan), List(RawPiece) -> List(RawSpan)
with_unstyled = |spans, pieces|
	if pieces.is_empty() {
		spans
	} else {
		spans.append(Unstyled(pieces))
	}

byte_at : List(U8), U64 -> U8
byte_at = |bytes, i|
	match bytes.get(i) {
		Ok(b) => b
		Err(_) => crash "Parse.byte_at: the scanner reads only inside the paragraph"
	}

find_byte : List(U8), { byte : U8, at : U64 } -> Try(U64, [NotFound])
find_byte = |bytes, { byte, at }| {
	var $i = at
	while $i < bytes.len() {
		if byte_at(bytes, $i) == byte {
			return Ok($i)
		}
		$i = $i + 1
	}
	Err(NotFound)
}

## Bytes cut from a `Str` at ASCII bytes, which UTF-8 never splits a character
## at, back as a `Str`.
utf8 : List(U8) -> Str
utf8 = |bytes|
	match Str.from_utf8(bytes) {
		Ok(text) => text
		Err(_) => crash "Parse.utf8: bytes cut at ASCII bytes are still UTF-8"
	}

# The problems: each fault at the line it names, beside a near miss that reads.

fault_of : Str -> Try({}, Parse.Problem)
fault_of = |source| Parse.note(source).map_ok(|_| {})

expect fault_of("") == Err({ line: 1, fault: NoTitle })
expect fault_of("\n\nNo title.\n") == Err({ line: 3, fault: NoTitle })
expect fault_of("# T\n") == Ok({})
expect fault_of("# T\n\n# Again\n") == Err({ line: 3, fault: SecondTitle })
expect fault_of("# T\n\n#### Deep\n") == Err({ line: 3, fault: HeadingTooDeep })
expect fault_of("# T\n\n### Not deep\n") == Ok({})
expect fault_of("# A **b**\n") == Err({ line: 1, fault: StyleInHeading })
expect fault_of("# T\n\n## A [b](c)\n") == Err({ line: 3, fault: StyleInHeading })
expect fault_of("# A `b`\n") == Ok({})
expect fault_of("# T\n\nWords.\n- a point\n") == Err({ line: 4, fault: NoBlankLineBefore })
expect fault_of("# T\n\nWords.\n-not a point\n") == Ok({})
expect fault_of("# T\n\n- a\nstray\n") == Err({ line: 4, fault: StrayListLine })
expect fault_of("# T\n\n- a\n  going on\n") == Ok({})
expect fault_of("# T\n\n1. a\n- b\n") == Err({ line: 4, fault: StrayListLine })
expect fault_of("# T\n\n```\ncode\n") == Err({ line: 3, fault: UnclosedFence })
expect fault_of("# T\n\nA `b.\n") == Err({ line: 3, fault: UnclosedCode })
expect fault_of("# T\n\nA ``.\n") == Err({ line: 3, fault: EmptyCode })
expect fault_of("# T\n\nA **b.\n") == Err({ line: 3, fault: UnclosedStyle })
expect fault_of("# T\n\nA 2 * 3.\n") == Err({ line: 3, fault: UnclosedStyle })
expect fault_of("# T\n\nA **b *c* d**.\n") == Err({ line: 3, fault: StyleInStyle })
expect fault_of("# T\n\nA *b **c** d*.\n") == Err({ line: 3, fault: StyleInStyle })
expect fault_of("# T\n\nA [*b*](c).\n") == Err({ line: 3, fault: StyleInStyle })
expect fault_of("# T\n\nA **[b](c)**.\n") == Err({ line: 3, fault: StyleInStyle })
expect fault_of("# T\n\nA ****.\n") == Err({ line: 3, fault: EmptyStyle })
expect fault_of("# T\n\nA [b] c.\n") == Err({ line: 3, fault: UnclosedLink })
expect fault_of("# T\n\nA [b](c.\n") == Err({ line: 3, fault: UnclosedLink })
expect fault_of("# T\n\nA [b.\n") == Err({ line: 3, fault: UnclosedLink })
expect fault_of("# T\n\nA [b](c.md).\n") == Err({ line: 3, fault: NotALinkTarget("c.md") })
expect fault_of("# T\n\nA [b](javascript:x).\n") == Err({
	line: 3,
	fault: NotALinkTarget("javascript:x"),
})
expect fault_of("# T\n\nA [b](https://a b).\n") == Err({
	line: 3,
	fault: NotALinkTarget("https://a b"),
})
expect fault_of("# T\n\nA [b](c-2) and [d](https://e.org/f).\n") == Ok({})
expect fault_of("# T\n\n```dot\na -> b\n```\n") == Err({ line: 3, fault: NotADiagram })
expect fault_of("# T\n\n```dot\ndigraph { a -> b }\n```\n") == Ok({})
expect fault_of("# T\n\n| a | b |\n|---|---|\n| 1 | 2 |\n") == Ok({})
expect fault_of("# T\n\n| a |\n|---|\n") == Ok({})
expect fault_of("# T\n\n| `x|y` |\n|---|\n| `|` |\n") == Ok({})
expect fault_of("# T\n\nWords.\n| a |\n") == Err({ line: 4, fault: NoBlankLineBefore })
expect fault_of("# T\n\n| a |\n|---|\n| 1\n") == Err({ line: 5, fault: NotATableLine })
expect fault_of("# T\n\n| a |\n|---|\nafter\n") == Err({ line: 5, fault: NotATableLine })
expect fault_of("# T\n\n| a |\n") == Err({ line: 3, fault: NoTableDivider })
expect fault_of("# T\n\n| a |\n| 1 |\n") == Err({ line: 4, fault: NoTableDivider })
expect fault_of("# T\n\n| a |\n|:--|\n") == Err({ line: 4, fault: NoTableDivider })
expect fault_of("# T\n\n| a | b |\n|---|\n") == Err({ line: 4, fault: TableRowWidth })
expect fault_of("# T\n\n| a | b |\n|---|---|\n| 1 | 2 |\n| 3 |\n") == Err({ line: 6, fault: TableRowWidth })
expect fault_of("# T\n\n| a | b |\n|---|---|\n| 1 | 2 | 3 |\n") == Err({ line: 5, fault: TableRowWidth })
expect fault_of("# T\n\n| a | b |\n|---|---|\n| 1 |  |\n") == Err({ line: 5, fault: EmptyCell })
expect fault_of("# T\n\n| a | **b |\n|---|---|\n") == Err({ line: 3, fault: UnclosedStyle })

expect fault_of("# T\n\n![A sun](sun.png)\n") == Ok({})
expect fault_of("# T\n\n![A sun](sun-2.jpg)\n\n![A plan](plan.svg)\n\nAfter.\n") == Ok({})
expect fault_of("# T\n\n![A sun](sun.png) and more\n") == Err({ line: 3, fault: NotAnImageLine })
expect fault_of("# T\n\n![A sun](sun.png)\nmore\n") == Err({ line: 4, fault: NotAnImageLine })
expect fault_of("# T\n\n![A sun](sun.png\n") == Err({ line: 3, fault: NotAnImageLine })
expect fault_of("# T\n\n![A sun]\n") == Err({ line: 3, fault: NotAnImageLine })
expect fault_of("# T\n\n![A sun(sun.png)\n") == Err({ line: 3, fault: NotAnImageLine })
expect fault_of("# T\n\n![A](b.png) (c)\n") == Err({ line: 3, fault: NotAnImageLine })
expect fault_of("# T\n\nWords.\n![A sun](sun.png)\n") == Err({ line: 4, fault: NoBlankLineBefore })
expect fault_of("# T\n\n![ ](sun.png)\n") == Err({ line: 3, fault: NoImageDescription })
expect fault_of("# T\n\n![A](notes/sun.png)\n") == Err({
	line: 3,
	fault: NotAnImageFile({ file: "notes/sun.png", problem: InAFolder }),
})
expect fault_of("# T\n\n![A](../sun.png)\n") == Err({
	line: 3,
	fault: NotAnImageFile({ file: "../sun.png", problem: InAFolder }),
})
expect fault_of("# T\n\n![A](sun.gif)\n") == Err({
	line: 3,
	fault: NotAnImageFile({ file: "sun.gif", problem: NotAPicture }),
})
expect fault_of("# T\n\n![A](..png)\n") == Err({
	line: 3,
	fault: NotAnImageFile({ file: "..png", problem: NotAName }),
})

# The image block holds the file and the description as written, trimmed.
expect
	Parse.note("# T\n\n![ A sun, *low* ](sun.png)\n").map_ok(
		|note|
			note.blocks.map(
				|block|
					match block {
						Image({ file, description }) => "${file.to_str()}|${description}"
						_ => "another kind"
					},
			),
	) == Ok(["sun.png|A sun, *low*"])

expect Parse.problem_message({
	line: 4,
	fault: NotAnImageFile({ file: "a/b.png", problem: InAFolder }),
}) == "line 4: `a/b.png` names a folder; an image is a file beside the note, named with no `/`"

expect Parse.problem_message({
	line: 7,
	fault: NotALinkTarget("x.md"),
}) == "line 7: `x.md` is neither a note's name (lowercase letters, digits and hyphens) nor an http:// or https:// address"

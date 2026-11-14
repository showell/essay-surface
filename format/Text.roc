## The text of a paragraph, a list item, a table cell or a subtitle: phrases in
## order, each plain, strong, emphasized or a link. A style holds a phrase, and a
## phrase holds no style, so styles never nest: there is no bold inside italics
## and no link inside bold.
##
## It is opaque because the maker keeps two rules: a text is never empty, and
## two unstyled phrases never stand side by side.
import NoteName
import Phrase
import WebAddress

Text :: List(Text.Span).{

	## One phrase of a text, with the one thing done to it.
	Span : [
		Unstyled(Phrase),
		Strong(Phrase),
		Emphasis(Phrase),
		Link({ label : Phrase, target : Text.Target }),
	]

	## Where a link goes: another note on the surface, or a page on the web.
	Target : [Note(NoteName), Web(WebAddress)]

	## The text of these spans, or why it can't be one: it is empty, or two
	## unstyled spans stand side by side, which a reader can't tell from one.
	from_spans : List(Text.Span) -> Try(Text, [EmptyText, UnstyledTwice])
	from_spans = |given| {
		var $after_unstyled = False
		for span in given {
			is_unstyled =
				match span {
					Unstyled(_) => True
					Strong(_) => False
					Emphasis(_) => False
					Link(_) => False
				}
			if is_unstyled and $after_unstyled {
				return Err(UnstyledTwice)
			}
			$after_unstyled = is_unstyled
		}
		if given.is_empty() {
			Err(EmptyText)
		} else {
			Ok(Text.(given))
		}
	}

	## The text's spans, in order.
	spans : Text -> List(Text.Span)
	spans = |Text.(given)| given

	## The text as a reader reads it, with no styles: a link is its label.
	reading : Text -> Str
	reading = |Text.(given)|
		Str.join_with(given.map(span_reading), "")

	is_eq : _
}

span_reading : Text.Span -> Str
span_reading = |span|
	match span {
		Unstyled(phrase) => phrase.reading()
		Strong(phrase) => phrase.reading()
		Emphasis(phrase) => phrase.reading()
		Link({ label, .. }) => label.reading()
	}

words : Str -> Phrase
words = |plain|
	match Phrase.from_pieces([Plain(plain)]) {
		Ok(phrase) => phrase
		Err(_) => crash "Text's tests: a phrase of plain words"
	}

expect Text.from_spans([
	Unstyled(words("a ")),
	Strong(words("b")),
	Unstyled(words(" c")),
]).map_ok(Text.reading) == Ok("a b c")
expect Text.from_spans([Strong(words("b")), Emphasis(words("c"))]).map_ok(Text.reading) == Ok("bc")
expect Text.from_spans([]) == Err(EmptyText)
expect Text.from_spans([Unstyled(words("a")), Unstyled(words("b"))]) == Err(UnstyledTwice)

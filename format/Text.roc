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
}

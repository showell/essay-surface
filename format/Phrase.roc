## A phrase is text with no style of its own: plain words and inline code, in
## order. It is what a style or a link holds, and what a title, a heading and
## a lead are made of, so a style inside a style can't be written: bold inside
## a link, or a link inside a heading, has no type.
##
## It is opaque because its pieces keep rules that the maker checks: no piece
## is empty, two plain pieces never stand side by side, and no piece holds a
## line break. A paragraph's line breaks are spaces by the time a phrase holds
## them.
Phrase :: List(Phrase.Piece).{

	## One run of a phrase: words, or code shown as written.
	Piece : [Plain(Str), Code(Str)]
}

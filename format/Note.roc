## A note in the surface's own format: its title, the line under the title if
## it has one, and its blocks in reading order. A note has one title and nothing
## above it, so the title is a field and never a block.
##
## It is nominal, not opaque: every note built from valid parts is a valid note,
## and its readers (the page, the index, the comments) read its fields.
import Block
import Phrase
import Text

Note := {
	title : Phrase,
	subtitle : Note.Subtitle,
	blocks : List(Block),
}.{

	## The line under the title that says when, for whom or what for:
	## "2026-11-05. Five flight drawings for safari's Roc bird...". It is drawn
	## as the subtitle, so it is written as plain text, links and all.
	Subtitle : [NoSubtitle, Subtitle(Text)]
}

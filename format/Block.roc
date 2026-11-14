## One block of a note, in reading order: what a reader sees as one piece of the
## page, and what a reply attaches to.
##
## It is nominal, not opaque: it is a closed set of kinds, and every value built
## from valid parts is a valid block, so there is nothing for a maker to guard.
## The parts that have rules (`Text`, `Phrase`, `Table` and the rest) are opaque
## and keep them.
import Diagram
import ImageFile
import Phrase
import Table
import Text
import Widget

Block := [
	Paragraph(Block.Prose),
	Heading({ level : Block.HeadingLevel, title : Phrase }),
	Bullets(List(Block.Point)),
	Numbered({ first_number : U64, points : List(Block.Point) }),
	Quote(List(Block.Prose)),
	Code({ language : Block.Language, body : Str }),
	Table(Table),
	Image({ file : ImageFile, description : Str }),
	Diagram(Diagram),
	Widget(Widget),
	Divider,
].{

	## A paragraph's words, which may open with a bold lead: "**The short
	## version.** Each tick is a pure function...". The lead is its own part
	## because most of the bold in the notes is a lead, and a lead names its
	## paragraph.
	Prose : { lead : Block.Lead, text : Text }

	## A paragraph's bold opening phrase, or none.
	Lead : [NoLead, Lead(Phrase)]

	## A heading under the note's title: a section, or a part of one.
	HeadingLevel : [Section, Subsection]

	## One point of a list, and the points under it, which hold no list of
	## their own.
	Point : { prose : Block.Prose, subpoints : List(Block.Prose) }

	## The language a code block names, for the reader; nothing is drawn
	## differently for it.
	Language : [NoLanguage, Language(Str)]
}

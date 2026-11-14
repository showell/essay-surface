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

	## The phrase of these pieces. Empty plain pieces are dropped and plain
	## pieces side by side are joined, since a reader can't tell them apart;
	## an empty code piece, a line break, or nothing left at all is refused.
	from_pieces : List(Phrase.Piece) -> Try(Phrase, [EmptyPhrase, EmptyCode, LineBreak])
	from_pieces = |given| {
		var $kept = []
		for piece in given {
			match piece {
				Plain(words) =>
					if words.contains("\n") {
						return Err(LineBreak)
					} else if !words.is_empty() {
						$kept =
							match $kept.last() {
								Ok(Plain(before)) => {
									joined = Plain("${before}${words}")
									$kept.drop_last(1).append(joined)
								}
								_ => $kept.append(Plain(words))
							}
					}
				Code(code) =>
					if code.contains("\n") {
						return Err(LineBreak)
					} else if code.is_empty() {
						return Err(EmptyCode)
					} else {
						$kept = $kept.append(Code(code))
					}
			}
		}
		if $kept.is_empty() {
			Err(EmptyPhrase)
		} else {
			Ok(Phrase.($kept))
		}
	}

	## The phrase's pieces, in order.
	pieces : Phrase -> List(Phrase.Piece)
	pieces = |Phrase.(kept)| kept

	## The phrase as a reader reads it: its words and its code, as written.
	reading : Phrase -> Str
	reading = |Phrase.(kept)|
		Str.join_with(kept.map(piece_reading), "")

	is_eq : _
}

piece_reading : Phrase.Piece -> Str
piece_reading = |piece|
	match piece {
		Plain(words) => words
		Code(code) => code
	}

expect Phrase.from_pieces([
	Plain("a "),
	Plain(""),
	Plain("b"),
	Code("x"),
]).map_ok(Phrase.pieces) == Ok([Plain("a b"), Code("x")])
expect Phrase.from_pieces([Code("x"), Plain(" y")]).map_ok(Phrase.reading) == Ok("x y")
expect Phrase.from_pieces([]) == Err(EmptyPhrase)
expect Phrase.from_pieces([Plain("")]) == Err(EmptyPhrase)
expect Phrase.from_pieces([Plain("a"), Code("")]) == Err(EmptyCode)
expect Phrase.from_pieces([Plain("a\nb")]) == Err(LineBreak)
expect Phrase.from_pieces([Code("a\nb")]) == Err(LineBreak)

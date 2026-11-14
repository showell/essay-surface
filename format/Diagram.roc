## A diagram, drawn from its description in Graphviz's dot language. The note
## holds the dot source as written; drawing it is Graphviz's job, wherever that
## runs, and the format never reads inside the source.
##
## It is opaque so that a diagram can later carry more (a caption, a choice of
## layout) without any reader of a block changing. The maker checks only that
## the source is not empty and starts as a dot graph does (`digraph`, `graph`,
## or `strict`).
Diagram :: Str.{

	## The diagram drawn from this dot source, if it starts as a dot graph
	## does.
	from_source : Str -> Try(Diagram, [NotDot])
	from_source = |source| {
		opening = source.trim()
		if ["digraph", "graph", "strict"].any(|word| opening.starts_with(word)) {
			Ok(Diagram.(source))
		} else {
			Err(NotDot)
		}
	}

	## The dot source, as written.
	source : Diagram -> Str
	source = |Diagram.(dot)| dot

	is_eq : _
}

expect Diagram.from_source("digraph { a -> b }").map_ok(Diagram.source) == Ok("digraph { a -> b }")
expect ["  graph { a -- b }", "strict digraph { a }"].all(|dot| Diagram.from_source(dot).is_ok())
expect ["", "   ", "a -> b", "{ digraph }"].all(|dot| Diagram.from_source(dot) == Err(NotDot))

## A diagram, drawn from its description in Graphviz's dot language. The note
## holds the dot source as written; drawing it is Graphviz's job, wherever that
## runs, and the format never reads inside the source.
##
## It is opaque so that a diagram can later carry more (a caption, a choice of
## layout) without any reader of a block changing. The maker checks only that
## the source is not empty and starts as a dot graph does (`digraph`, `graph`,
## or `strict`).
Diagram :: Str.{}

## The name of a note on the surface, as its file is named without the
## extension: `basic-machine-design`. A link to another note holds one, so a
## link names a note and never a path or a URL of the server's.
##
## It is opaque because the maker checks the name: lowercase letters, digits
## and hyphens, starting with a letter or a digit. Whether such a note exists
## is the server's to know, not the name's.
NoteName :: Str.{}

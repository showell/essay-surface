## A page on the web that a link opens: an `http://` or `https://` address.
##
## It is opaque because the maker checks the address: its scheme is one of the
## two, a host follows, and it holds no space and no `<`, `>` or `"`. So no other
## scheme reaches a link (no `javascript:`, no `file:`), and an address never
## needs escaping to stay inside an attribute.
WebAddress :: Str.{}

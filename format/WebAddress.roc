## A page on the web that a link opens: an `http://` or `https://` address.
##
## It is opaque because the maker checks the address: its scheme is one of the
## two, a host follows, and it holds no space and no `<`, `>` or `"`. So no other
## scheme reaches a link (no `javascript:`, no `file:`), and an address never
## needs escaping to stay inside an attribute.
WebAddress :: Str.{

	## The address, if it is one: `http://` or `https://`, then a host, and
	## no space, control byte, `<`, `>` or `"` anywhere.
	from_str : Str -> Try(WebAddress, [NotAWebAddress])
	from_str = |given| {
		after_scheme =
			if given.starts_with("https://") {
				given.drop_prefix("https://")
			} else {
				given.drop_prefix("http://")
			}
		has_scheme = after_scheme != given
		has_host =
			match after_scheme.to_utf8().first() {
				Ok(first) => first != '/'
				Err(_) => False
			}
		if has_scheme and has_host and given.to_utf8().all(is_allowed) {
			Ok(WebAddress.(given))
		} else {
			Err(NotAWebAddress)
		}
	}

	to_str : WebAddress -> Str
	to_str = |WebAddress.(address)| address

	is_eq : _
}

is_allowed : U8 -> Bool
is_allowed = |b| b > ' ' and b != '<' and b != '>' and b != '"' and b != 127

expect [
	"https://roc-lang.org/",
	"http://143.244.172.148:9100/notes/x.md",
	"https://a",
].all(|address| WebAddress.from_str(address).map_ok(WebAddress.to_str) == Ok(address))
expect [
	"",
	"https://",
	"http:///x",
	"javascript:alert(1)",
	"file:///etc",
	"ftp://a",
	"https://a b",
	"https://a\"b",
	"https://a<b",
	"https://a\tb",
	"roc-lang.org",
].all(|address| WebAddress.from_str(address) == Err(NotAWebAddress))

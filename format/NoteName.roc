## The name of a note on the surface, as its file is named without the
## extension: `basic-machine-design`. A link to another note holds one, so a
## link names a note and never a path or a URL of the server's.
##
## It is opaque because the maker checks the name: lowercase letters, digits
## and hyphens, starting with a letter or a digit. Whether such a note exists
## is the server's to know, not the name's.
NoteName :: Str.{

	## The name, if it is one: lowercase letters, digits and hyphens,
	## starting with a letter or a digit.
	from_str : Str -> Try(NoteName, [NotANoteName])
	from_str = |given| {
		bytes = given.to_utf8()
		starts_well =
			match bytes.first() {
				Ok(first) => is_lowercase_or_digit(first)
				Err(_) => False
			}
		if starts_well and bytes.all(|b| is_lowercase_or_digit(b) or b == '-') {
			Ok(NoteName.(given))
		} else {
			Err(NotANoteName)
		}
	}

	to_str : NoteName -> Str
	to_str = |NoteName.(name)| name

	is_eq : _
}

is_lowercase_or_digit : U8 -> Bool
is_lowercase_or_digit = |b| (b >= 'a' and b <= 'z') or (b >= '0' and b <= '9')

expect [
	"basic-machine-design",
	"a",
	"9-lives",
].all(|name| NoteName.from_str(name).map_ok(NoteName.to_str) == Ok(name))
expect [
	"",
	"-a",
	"A",
	"a_b",
	"a.md",
	"a/b",
	"notes/a",
	"é",
].all(|name| NoteName.from_str(name) == Err(NotANoteName))

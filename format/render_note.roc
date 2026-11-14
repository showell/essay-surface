## Prints one `.note` file's page as JSON, for the server to show (store.go's
## renderNote). Roc's built-in platform gives a headerless app its arguments
## and a way to print, nothing else, so the note arrives as arguments: Linux
## stops one argument at 128 KiB, and the server cuts the text into parts
## below that, which this joins.
import Page
import Parse

main! = |parts| {
	source = Str.join_with(parts, "")
	page =
		match Parse.note(source) {
			Ok(note) => Page.from_note(note)
			Err(problem) => Page.refusal(problem)
		}
	echo!(page.to_json())
	Ok({})
}

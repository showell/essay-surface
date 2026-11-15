## A picture file kept beside the note in its folder: `sunset-before-5700.png`.
##
## It is opaque because the maker checks the name: a note's name (lowercase
## letters, digits and hyphens, as `NoteName` has it), then `.png`, `.jpg` or
## `.svg`, and no folder. So an image is always the note's own, never a file
## elsewhere on the box or a page on the web, and the server serves a picture
## only by such a name (server.go's imageType).
import NoteName

ImageFile :: Str.{

	## Why a name isn't a picture's: it names a folder, it doesn't end in one
	## of the three extensions, or what comes before the extension isn't a
	## note's name (a hidden file's `.`, a `..`, a capital or a space).
	Problem : [InAFolder, NotAPicture, NotAName]

	## The file, if its name is one: a note's name, then `.png`, `.jpg` or
	## `.svg`, and no `/` or `\` anywhere.
	from_str : Str -> Try(ImageFile, ImageFile.Problem)
	from_str = |given|
		if given.to_utf8().any(|b| b == '/' or b == '\\') {
			Err(InAFolder)
		} else {
			match [".png", ".jpg", ".svg"].find_first(|ext| given.ends_with(ext)) {
				Err(_) => Err(NotAPicture)
				Ok(ext) =>
					match NoteName.from_str(given.drop_suffix(ext)) {
						Ok(_) => Ok(ImageFile.(given))
						Err(NotANoteName) => Err(NotAName)
					}
			}
		}

	to_str : ImageFile -> Str
	to_str = |ImageFile.(name)| name

	is_eq : _
}

expect [
	"sunset-before-5700.png",
	"a.jpg",
	"9-lives.svg",
].all(|name| ImageFile.from_str(name).map_ok(ImageFile.to_str) == Ok(name))
expect [
	"notes/a.png",
	"../a.png",
	"a\\b.png",
	"/a.png",
].all(|name| ImageFile.from_str(name) == Err(InAFolder))
expect [
	"a.gif",
	"a.jpeg",
	"a.PNG",
	"a.png.md",
	"a",
	"..",
	"",
].all(|name| ImageFile.from_str(name) == Err(NotAPicture))
expect [
	".png",
	"..png",
	".hidden.png",
	"a..png",
	"A.png",
	"a b.png",
	"a_b.jpg",
	"-a.svg",
].all(|name| ImageFile.from_str(name) == Err(NotAName))

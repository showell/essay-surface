## A place in a note where a small live program will run: which widget it is,
## and a description of what it shows. The description is what a reader sees
## where the widget can't run, and what an author writes before the widget
## exists.
##
## Widgets are not designed yet. The type is opaque so that their inputs, their
## size and how they run can be added inside it without any reader of a block
## changing.
import Text

Widget :: { name : Str, description : Text }.{}

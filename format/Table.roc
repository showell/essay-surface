## A table: a heading for each column, then rows of cells, each cell a text.
##
## It is opaque because the maker keeps the table's shape: at least one column,
## and every row exactly as wide as the headings. A reader can then take a row's
## cells by column without checking the row.
import Text

Table :: { column_headings : List(Text), rows : List(List(Text)) }.{}

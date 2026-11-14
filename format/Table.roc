## A table: a heading for each column, then rows of cells, each cell a text.
##
## It is opaque because the maker keeps the table's shape: at least one column,
## and every row exactly as wide as the headings. A reader can then take a row's
## cells by column without checking the row.
import Phrase
import Text

Table :: { column_headings : List(Text), rows : List(List(Text)) }.{

	## The table of these headings and rows, or why it can't be one: it has no
	## column, or a row (counted from 0) is wider or narrower than the headings.
	from_rows : { column_headings : List(Text), rows : List(List(Text)) } -> Try(Table, [NoColumns, RowWidth(U64)])
	from_rows = |{ column_headings, rows }| {
		if column_headings.is_empty() {
			return Err(NoColumns)
		}
		var $index = 0
		for row in rows {
			if row.len() != column_headings.len() {
				return Err(RowWidth($index))
			}
			$index = $index + 1
		}
		Ok(Table.({ column_headings, rows }))
	}

	## The column headings, in order.
	column_headings : Table -> List(Text)
	column_headings = |Table.(table)| table.column_headings

	## The rows, in order, each as wide as the headings.
	rows : Table -> List(List(Text))
	rows = |Table.(table)| table.rows

	is_eq : _
}

cell : Str -> Text
cell = |plain|
	match Phrase.from_pieces([Plain(plain)]) {
		Ok(phrase) =>
			match Text.from_spans([Unstyled(phrase)]) {
				Ok(text) => text
				Err(_) => crash "Table's tests: a text of one phrase"
			}
		Err(_) => crash "Table's tests: a phrase of plain words"
	}

expect Table.from_rows({ column_headings: [cell("a"), cell("b")], rows: [[cell("1"), cell("2")]] }).map_ok(|t| t.rows().len()) == Ok(1)
expect Table.from_rows({ column_headings: [cell("a")], rows: [] }).map_ok(|t| t.column_headings().len()) == Ok(1)
expect Table.from_rows({ column_headings: [], rows: [] }) == Err(NoColumns)
expect Table.from_rows({ column_headings: [cell("a"), cell("b")], rows: [[cell("1"), cell("2")], [cell("3")]] }) == Err(RowWidth(1))
expect Table.from_rows({ column_headings: [cell("a")], rows: [[cell("1"), cell("2")]] }) == Err(RowWidth(0))

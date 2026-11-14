# Session 10: writing notes in the .note format

The orchestrator writes its notes to Steve as `.note` files, to find where the format pushes back on real use. Each finding: what happened, and whether the format or the writer should change.

- **2026-11-14, a list straight after a paragraph.** `a-language-in-a-week.note` started a bullet list on the line after a paragraph, as markdown allows. The surface showed the note's problem page: "line 24: this line starts a block, so it needs a blank line before it." The message named the line and the fix, and the fix took one blank line. It's a habit carried over from markdown; the rule is deliberate (MISSION PB: none of CommonMark's rules), so the writer changes. Worth watching how often it happens.
- **2026-11-14, tables.** The artifacts inventory needed a four-column table, and the format has no table maker yet, so it went out as markdown with a raw HTML table. A real requirement for `Table`'s maker.

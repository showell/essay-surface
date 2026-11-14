The essay surface works, and I use it every day. Now I want notes that can do more than markdown does. A note should be able to hold a diagram, drawn from a dot description, and later a small widget. Markdown isn't a good fit for that, so we're going to build our own format, inspired by markdown but designed from first principles for exactly what I do. It needs nothing like markdown's full power, and none of CommonMark's rules.

The features will be written in Roc. Go keeps doing the plumbing it does now, and the front end stays plain JavaScript. Where Roc runs, on the server or in the browser, is for the requirements to decide, so don't decide it yet.

Start with the types, not the code. Read my real notes in `notes/` to see what I actually write. Then propose the Roc types for a note in the new format: its blocks, the text inside them, and a diagram block. Leave room for widgets without designing them yet. For each type, say in a sentence what it's for, and what it deliberately leaves out. Tell me which things in my real notes the new format would drop, and how many notes use each. Propose a name for the new format's files, too.

Write no Roc functions and no Go yet. Your report is the types, as a note I can reply to. We'll argue about them before anything is built on them.

My existing notes keep rendering exactly as they do now. The new format is for new notes.

Today, comments. A reader should be able to comment on any paragraph of a note, and I read those comments to steer my next draft.

Here's what I want, from first principles:

- **A comment belongs to a paragraph, not a position.** I edit notes all the time. When I add a paragraph above, or fix a typo in the paragraph itself, its comments should stay with it. When a paragraph is deleted, its comments should still be shown, marked as orphaned. A comment is never lost.
- **The front end is plain JavaScript**, with no framework and no build step. Each paragraph gets a quiet way to comment, and its comments show beside it or under it. It should work on a phone.
- **The server stores comments as files** beside the notes, in a format I can read and diff with git. Pick it, and tell me why.
- **It stays fast.** A page with comments should come back about as fast as one without. Measure both.
- **There's no login yet.** A comment has a name and a text. It runs only on the dev port for now.

Use yesterday's scripts where they fit, and improve them when they don't. Write Go tests for the anchoring: what happens to comments when a paragraph is edited, moved, split or deleted. Commit as you go. When you're done, tell me how comments stay attached, what you measured, and what you'd want to check every time from now on.

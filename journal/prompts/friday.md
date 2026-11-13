Today we ship, and shipping should be boring.

I've been using the new surface for real since Tuesday, replying to essays on it, and every time we've changed the server I've swapped the running one by hand: build, stop the old one, start the new one, check a page. That's exactly the kind of chore I want a script for. Write `scripts/release.shale`, which does a release the same way every time:

- It runs the full check first, and stops unless it says SAFE TO SHIP.
- It builds the new server next to the old one and keeps the old binary.
- Before it stops anything, it copies the comments somewhere safe. Those are my real replies, and a release must never lose one.
- It stops the old server, starts the new one, and checks it: the index, a note, a note with comments, and that every comment from before is still on the page.
- If any check fails, it puts the old server back and says so. A failed release leaves me where I started.
- Its last line says what's running now, and how to roll back.

Prove it on the dev port with a copy of my notes and comments, including one release that fails on purpose and rolls back. Don't touch the server I'm using. When it works on dev, we'll run the real release ourselves.

When you're done, show me the output of a good release, a failed one and its rollback, and what each costs in time.

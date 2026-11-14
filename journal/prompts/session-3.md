Two jobs today: a faster index, and a way of working we can keep.

**The index.** Session 1's numbers showed where its time goes: checking every note's file on every request. Make the server hear about changes instead of looking for them. While you're there, take a note's date from the note or from git, not from the file's modified time, since a fresh clone resets those. Measure before and after.

**The checks.** Over two days we've collected scripts: `check.shale`, `bench.shale`, `audit.shale` and `comments_smoke.shale`, and yesterday's report lists what to check every time. Now make them easy to run at the right moment, so nobody has to remember the list:

- a **fast** check for each edit, a few seconds, that I'd run without thinking;
- a **full** check before anything ships, on all my real notes, that says plainly whether it's safe;
- and the timing kept out of the full check's verdict when the box is busy, or held to a budget that a busy box can't break.

I read the scripts' code, not their comments, so make the code say what it does. Clean up anything you wouldn't want to read twice. When you're done, show me each check's last lines from a real run, how long each takes, and anything that's still checked by hand.

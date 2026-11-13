Yesterday's full check says SAFE TO SHIP. Today, make me believe it.

A check I can trust is one I've seen fail. For each stage of `full.shale`, show me that it can: put in the one fault it exists to catch, run it, watch it go red, then take the fault out. Do that once per stage, and write down what you planted and what it said. Wednesday's planted fault already found a test whose panic hid twelve others, so this is worth the time.

Some of those faults are worth keeping as permanent fixtures: small inputs the check must reject, and small inputs it must accept, run every time. Keep only the ones that are cheap. I've been burned by fixtures that rebuild half the project and turn a ten-second gate into a two-minute one. A fixture here should be data the check reads, never something that needs a build of its own. Tell me what each one costs, and keep `full.shale` under thirty seconds.

One more. On Tuesday every check passed, and I opened a page and found no way to reply, because the reply buttons were invisible until you hovered. Make that something a check catches: a page that hides its reply buttons should fail. Do it without a browser, if you can do it honestly.

When you're done, show me the table: each stage, the fault you planted, what it said, and which fixtures you kept and what they cost.

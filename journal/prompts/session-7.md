A small feature, and a careful one: bare URLs in a note should be links. Right now I see `http://...` as text and have to copy it.

Do the minimal thing, and do it safely:

- Only `http://` and `https://` URLs written bare in prose become links. Markdown's own `[text](url)` links already work; leave them as they are.
- Nothing inside code changes, inline or fenced. A URL in a code block is code.
- A URL ends where a reader would say it ends. A period, comma, colon or closing parenthesis at its end belongs to the sentence, unless the URL opened that parenthesis itself.
- The link's text and its `href` are escaped like everything else on the page. A note must never be able to put markup into a page through a URL.
- Links to other sites open the way they do now.

This is a classic place for a renderer to get subtly wrong, so show me the cases: the ones that must link, the ones that must not, and the punctuation cases. Keep them as fixtures the tests read. Seed a fault once to prove the escaping test catches it. Then run the full check, and tell me what it found on my real notes: how many bare URLs it linked, and anything it left alone that you think it shouldn't have.

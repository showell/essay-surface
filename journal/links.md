# Links: bare URLs in a note become links

The prompt is `journal/prompts/links.md`: bare `http://` and `https://` URLs in prose become links, nothing in code changes, a URL ends where a reader would say it ends, the link is escaped like the rest of the page, and links open as they do now. Show the cases as fixtures, seed one fault into escaping, run the full check, and say what it did on the real notes.

## Orienting

My first command was a bare shell `cat` of the prompt. The gate refused it:

```text
shale-gate: `cat` is not `shale`; only a launch, ...
```

So I read files with the Read tool from then on, and used `shale --help`, then `SHALE-AGENTS.md`, for the rest. Next, a look at the repo:

```shale
for p in glob("**/*") do print(p, count_lines(p)) end
for l in git_log("", {n = 15}) do print(l.hash, l.subject) end
print(git_status(""))
```

```text
README.md	14
anchor.go	324
...
markdown.go	1106
markdown_test.go	160
...
312aa7c	Journal: the prompt for bare URLs as links
...
── stopped at stdin:3:7: type: `print` takes scalars, not an empty table: ...
```

The stop was on the last line: `git_status` gives a list, and here it was empty, so the tree was clean. I listed the journals' headings and read each day's report, then `markdown.go`, `markdown_test.go`, `TestEveryCommentFieldEscaped`, the button fixtures test (the pattern for "every file named, so none is skipped"), `fast.shale`, `full.shale`, `audit.shale` and the repo prelude.

```shale
for p in glob("journal/*.md") do
  for m in grep(p, "## ") do if starts_with(m.text, "## ") then print(p .. ":" .. m.line, m.text) end end
end
print(has_item(git_ls_files(""), "essay-surface"))
```

```shale
for p in glob("*.go") do
  for m in grep(p, "func Test") do print(p .. ":" .. m.line, m.text) end
  for m in grep(p, "testdata") do print(p .. ":" .. m.line, m.text) end
  for m in grep(p, "target") do print(p .. ":" .. m.line, m.text) end
end
for m in grep("comments.js", "href") do print("js:" .. m.line, m.text) end
for m in grep("comments.js", "target") do print("js:" .. m.line, m.text) end
```

Nothing in the Go code or `comments.js` sets a `target`. A link is a plain `<a href>` and opens in the same tab. "Links to other sites open the way they do now" means a bare URL's link is a plain `<a href>` too.

```shale
for p in glob(".shale/*", {hidden = true}) do print(p, count_lines(p)) end
for m in grep(".shale/prelude.shale", "function ") do print(m.line, m.text) end
print(read("README.md"))
```

What the renderer does now: inline text is escaped, `[text](url)` and `<https://...>` are links, and a bare URL is text. Its `_` and `*` can also start emphasis in the middle of a URL.

## The URLs in the real notes, first

```shale
print(read(".gitignore"))
local n = 0
local files = 0
local ctx = {}
for p in glob("repos:essay-repl-server/notes/*.md") do
  files = files + 1
  for m in grep(p, "://") do
    for h in re_find_all(m.text, "https?://") do n = n + 1 end
  end
end
print(files .. " notes, " .. n .. " http(s):// occurrences")
```

```text
/essay-surface
/.run/

514 notes, 125 http(s):// occurrences
```

All 125 with their context:

```shale
for p in glob("repos:essay-repl-server/notes/*.md") do
  for m in grep(p, "://") do
    for h in re_find_all(m.text, "https?://") do
      local a = h.at - 30
      if a < 1 then a = 1 end
      print(basename(p) .. ":" .. m.line, "[" .. sub(m.text, a, h.at + 70) .. "]")
    end
  end
end
```

Here is what they hold, from the 125 lines (a few of them below). Most are markdown links. The bare ones end in every way the prompt names:

```text
autonomous-run-2026-11-12.md:3	[n master, and the dev book is http://143.244.172.148:9210/tutorial.fresh/]
book-draft-deploy.md:23	[Preview: **http://143.244.172.148:9210/draft/billboard-critters.html**]
day-wrap-2026-11-12.md:7	[aster and on the dev channel (http://143.244.172.148:9210/tutorial.fresh/). None of it is deployed ye]
metal-lynrummy-com.md:5	[tal now has a real address: **https://metal.lynrummy.com**.]
review-queue.md:531	[...; fork draft PR https://github.com/showell/roc/pull/2, CI Manager run ...]
the-browser-that-was-already-signed-in.md:139	[| prod | https://chat.lynrummy.com runs 203059c |]
roc-web-umbrella.md:44	[live   http://<box>:9200/            the landing page]
home-app-block.md:20	[code: https://github.com/...]
machine-batch-demo.md:9	[**The page is built: `http://143.244.172.148:9203/machine/batch/`.**]
```

These gave me fixture cases: the bold, the parenthesis and then a period, the comma, the table cell, a placeholder host (`<box>`), and a URL written as code.

## A "before" for the real notes

I wanted to say exactly what the change did to every real page, and not just count URLs in the source. So before any edit I rendered every real note through today's server (the audit fetches each page) and kept the pages:

```shale
-- before any change: every real note's page from today's renderer, kept as the "before" to compare against
-- budget: steps=50000000
local r = run({"shale", "scripts/audit.shale", "dir=/home/steve/showell_repos/essay-repl-server/notes"}, {timeout_ms = 100000, tail = 6})
print("audit: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
essay_fresh(".run/links/before")
local n = 0
for p in glob(".run/audit/pages/*.html") do
  create(".run/links/before/" .. basename(p), read(p))
  n = n + 1
  yield()
end
print(n .. " pages kept in .run/links/before")
```

```text
audit: exit 0, 2195 ms, cpu 1794
...
audit: PASS
514 pages kept in .run/links/before
```

## The design

One new branch in the inline pass, `bareURL(s, k)` in `markdown.go`. At an `h`:

- **It starts a word:** the character before it is not a letter or a digit, so `xhttps://` is text.
- **The scheme is `http://` or `https://`, lowercase**, and a letter or a digit follows it, so `https://` alone, `https:///path` and `http://<box>` are text.
- **It runs to** a space, or a character that ends a URL in a sentence: `<`, `>`, `"`, a backtick, a backslash, or punctuation outside ASCII (an em dash, a curly quote). A URL can't hold `<`, `>` or `"`, so markup can never get into its href, and both the href and the text are escaped as well.
- **Punctuation at its end is the sentence's:** `. , : ; ! ? ' * _ ~`, peeled one at a time. A closing `)` or `]` is peeled only when the URL has more closers than openers, so `Go_(language)` keeps its `)`, and `(see https://x.org)` gives its `)` back.
- **Inside a link's own text it stays text** (`inlineText(label, true)`), because a link can't hold another. So `[https://x.org](https://x.org)` is still one link.

Code, fences, block HTML and `<...>` links never reach this branch, because they're handled before it or aren't inline text. The URL is consumed whole, so a `_` or `*` inside it is no longer emphasis.

## The fixtures

`testdata/links/`, one file a kind, in a small format the test reads (`~~ case: <name>`, the markdown, `~~ want`, the HTML). The test reads the files directly, so they need no build:

- `link.txt`, 16 cases that must link: in a sentence, http with an address and port, a path with a query and fragment, `_` and `*` inside, two in a line, a second line's start, after `=`, letters past ASCII, in strong, in `*`/`_` emphasis, in a list item, a heading, a table cell, a quote, and beside a markdown link.
- `not.txt`, 15 cases that must not: inline code, two fences, a markdown link, a link whose text is its URL, a URL in a link's text, `<https://...>`, an image, inside a word, `ftp:`/`mailto:`/`file:`/`www.`, a scheme alone, no host, a `<box>` placeholder, capitals, and an HTML block.
- `punctuation.txt`, 22 cases: period, a period inside, ellipsis, comma, colon, a port's colon, semicolon, `?` and `!`, a query's `=`, a trailing slash, parentheses, parentheses then a period, a parenthesis the URL opened (alone and inside parentheses), square brackets, double, single and curly quotes, an em dash, a trailing `_`, bold then a period, and a backtick.
- `escaping.txt`, 9 hostile notes: a quote and `<script>` after the URL, `&` in a query (`&lt=3` among them), single quotes and a handler, a tag straight after, entities written in the URL, a backslash and a quote, a URL behind `javascript:`, and one each in a heading and a table cell.

`TestBareURLs` renders every case and compares the HTML exactly. It checks that the folder holds exactly the four files it names, so a new file can't be skipped. `TestBareURLsEscaped` holds each hostile note to rules that don't depend on its exact HTML: no tag but the renderer's own, every `&` starts an entity, and each link's href and text are the same URL, escaped exactly once, as the note wrote it.

I wrote the escaping cases into `link.txt` first by mistake, with two stray lines of tool markup at the end. I caught it before any test ran and wrote all four files again.

The tests were appended with one launch:

```shale
-- the bare-URL tests, appended to markdown_test.go
append("markdown_test.go", [==[
... (TestBareURLs, TestBareURLsEscaped and their reader, as committed in e980835)
]==])
```

## The first run, and is that green a claim?

```shale
-- first run of the new tests and the renderer's old ones
local r = run({"go", "test", "-count=1", "-run", "TestBareURL|TestRender|TestTitle|TestSamples", "."}, {timeout_ms = 100000, tail = 60})
print("exit " .. r.code .. ", " .. r.ms .. " ms")
print(r.out)
```

```text
exit 0, 1359 ms
ok  	essay-surface	0.044s
```

A first green makes me ask whether the tests ran at all:

```shale
-- did the new tests run, and on how many cases?
local r = run({"go", "test", "-count=1", "-v", "-run", "TestBareURL", "."}, {timeout_ms = 100000, tail = 20})
print(r.out)
local all = 0
for f in glob("testdata/links/*.txt") do
  local n = #grep(f, "~~ case: ")
  all = all + n
  print(f, n .. " cases")
end
print(all .. " cases in all")
```

```text
--- PASS: TestBareURLs (0.00s)
--- PASS: TestBareURLsEscaped (0.00s)
testdata/links/escaping.txt	9 cases
testdata/links/link.txt	17 cases
testdata/links/not.txt	15 cases
testdata/links/punctuation.txt	22 cases
63 cases in all
```

The grep counts `link.txt`'s header too, which describes the format. That leaves 62 real cases, and the seeded fault below shows the files are really read.

## The seeded fault, and its red

Once, under `--undo-on-stop`: a bare URL's href written as the note wrote it, unescaped.

```shale
-- seed once: a bare URL's href written as the note wrote it, unescaped. The escaping test must go red, by name
const OLD = [=[text.WriteString(`<a href="` + escapeAttr(u) + `">` + escapeHTML(u) + "</a>")]=]
const NEW = [=[text.WriteString(`<a href="` + u + `">` + escapeHTML(u) + "</a>")]=]
local before = git_diff("")
edit("markdown.go", OLD, NEW)
local r = run({"go", "test", "-count=1", "-run", "TestBareURL", "."}, {timeout_ms = 100000, tail = 30})
edit("markdown.go", NEW, OLD)
print("planted: exit " .. r.code .. ", " .. r.ms .. " ms")
print(r.out)
if git_diff("") ~= before then fail("the tree is not back") end
print("git diff: as before the plant")
```

```text
planted: exit 1, 1153 ms
--- FAIL: TestBareURLs (0.00s)
    markdown_test.go:224: link.txt: a path, a query and a fragment: ...
    markdown_test.go:224: escaping.txt: ampersands in the query: ...
    markdown_test.go:224: escaping.txt: entities written in it: ...
--- FAIL: TestBareURLsEscaped (0.00s)
    markdown_test.go:245: ampersands in the query: an & that starts no entity: <p id="p1"><a href="https://x.org/?a=1&b=2&lt=3">...
    markdown_test.go:257: ampersands in the query: not escaped exactly once: href "https://x.org/?a=1&b=2&lt=3", text "https://x.org/?a=1&amp;b=2&amp;lt=3"
    markdown_test.go:259: entities written in it: the href "https://x.org/\"onclick=\"alert(1)" is not the text "https://x.org/&quot;onclick=&quot;alert(1)"
FAIL
git diff: as before the plant
```

The escaping test caught it through three of its own rules, independently of the exact fixtures. One of them matters most. With the fault, `&quot;` written in a URL reaches the href raw, and a browser reads it as a `"` inside the attribute. Only the "href and text are one URL" rule sees that, because a raw `&quot;` is a valid entity and passes the `&` rule.

## The gate, and the commit

```shale
-- the fast gate before the first commit
local r = run({"shale", "scripts/fast.shale"}, {timeout_ms = 110000, tail = 12})
print("fast: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
```

```text
fast: exit 0, 3055 ms, cpu 3735
ok  gofmt, 21 ms, cpu 32 ms
ok  vet, 341 ms, cpu 522 ms
ok  js, 36 ms, cpu 36 ms
ok  test, 1569 ms, cpu 1758 ms: 50 tests passed, 63 subtests
ok  smoke, 1082 ms, cpu 1379 ms: 10 checks through the real server
fast: PASS
```

The test count went from 48 to 50, so both new tests are among them. The commit was `e980835` (`markdown.go`, `markdown_test.go`, `testdata/links`), made with `git_commit` and `{add = {...}}`.

## The real notes, after

```shale
-- after: every real note rendered again, the pages kept beside the before ones
-- budget: steps=50000000
local r = run({"shale", "scripts/audit.shale", "dir=/home/steve/showell_repos/essay-repl-server/notes"}, {timeout_ms = 100000, tail = 6})
print("audit: exit " .. r.code .. ", " .. r.ms .. " ms, cpu " .. r.cpu_ms)
print(r.out)
essay_fresh(".run/links/after")
local n = 0
for p in glob(".run/audit/pages/*.html") do
  create(".run/links/after/" .. basename(p), read(p))
  n = n + 1
  yield()
end
print(n .. " pages kept in .run/links/after")
```

```text
audit: exit 0, 2321 ms, cpu 2031
audit: PASS
514 pages kept in .run/links/after
```

The comparison took two stops. The first used `replace_all`, which Shale doesn't have:

```shale
-- before against after on the 514 real pages: the bare URLs linked, anything else that changed, and every http(s):// left as text outside code
-- budget: steps=200000000
const BARE = "<a href=\"(https?://[^\"]*)\">(https?://[^<]*)</a>"

function cut(s, open, close)
  local parts = split(s, open)
  local out = {parts[1]}
  for i = 2, #parts do
    local e = find(parts[i], close)
    if e ~= nil then push(out, sub(parts[i], e + #close, #parts[i])) end
  end
  return join(out, " ")
end

-- the page with every link whose text is its URL put back as the bare text
function unwrap(page)
  local out = {}
  local at = 1
  for m in re_find_all(page, BARE) do
    if m.groups[1] == m.groups[2] then
      push(out, sub(page, at, m.at - 1))
      push(out, m.groups[2])
      at = m.at + #m.text
    end
  end
  push(out, sub(page, at, #page))
  return join(out, "")
end

function bare_count(page)
  local n = 0
  for m in re_find_all(page, BARE) do if m.groups[1] == m.groups[2] then n = n + 1 end end
  return n
end

function compare(name)
  local b = read(".run/links/before/" .. name)
  local a = read(".run/links/after/" .. name)
  local res = {name = name, linked = bare_count(a) - bare_count(b), anchors = #find_all(a, "<a ") - #find_all(b, "<a "), same = unwrap(a) == unwrap(b), left = {}}
  local text = cut(cut(cut(cut(cut(a, "<script", "</script>"), "<style>", "</style>"), "<pre", "</pre>"), "<code>", "</code>"), "<a ", "</a>")
  text = cut(text, "<", ">")
  for m in re_find_all(text, "https?://") do
    local from = m.at - 25
    if from < 1 then from = 1 end
    push(res.left, sub(text, from, m.at + 45))
  end
  return res
end

local total = 0
local pages = 0
local rows = {}
for p in glob(".run/links/after/*.html") do
  local c = compare(basename(p))
  if c.linked ~= c.anchors then print("MORE THAN BARE URLS CHANGED IN ANCHORS: " .. c.name .. " " .. c.linked .. " vs " .. c.anchors) end
  if not c.same then print("OTHER TEXT CHANGED: " .. c.name) end
  if c.linked > 0 then
    total = total + c.linked
    pages = pages + 1
    push(rows, {linked = c.linked, note = c.name})
  end
  for l in c.left do print("left as text: " .. c.name .. ": [" .. replace_all(l, "\n", " ") .. "]") end
  yield()
end
sort(rows, "linked", "desc")
print(table(rows, {"linked", "note"}))
print(total .. " bare URLs linked on " .. pages .. " pages")
```

```text
── stopped at stdin:62:67: name: no function `replace_all` above this line: a function is called only after it is declared (0 steps)
```

The retry was the same script with `replace_all(l, "\n", " ")` as `join(split(l, "\n"), " ")`. It ran out of steps in `re_find_all`:

```text
── stopped at the prelude:244:15 (in `the prelude's re_find_all`): budget steps: the step budget (200000000 steps) ran out (200000001 steps)
   called from stdin:32:12 (in `bare_count`)
```

Monday's lesson, again: `re_find_all` is the prelude's and costs a step a character, and these are 7 MB of pages read twice. The third version uses `split` and `find`, which are builtins:

```shale
-- before against after on the 514 real pages, by split and find (builtins): the bare URLs linked, anything else that changed, every http(s):// left as text outside code
-- budget: steps=200000000
function cut(s, open, close)
  local parts = split(s, open)
  local out = {parts[1]}
  for i = 2, #parts do
    local e = find(parts[i], close)
    if e ~= nil then push(out, sub(parts[i], e + #close, #parts[i])) end
  end
  return join(out, " ")
end

-- {n, plain}: the links whose text is their http(s) URL, counted, and the
-- page with each put back as its bare text
function bare(page)
  local parts = split(page, "<a href=\"")
  local out = {parts[1]}
  local n = 0
  for i = 2, #parts do
    local p = parts[i]
    local q = find(p, "\">")
    local e = find(p, "</a>")
    local done = false
    if q ~= nil and e ~= nil and q < e and starts_with(p, "http") then
      local href = sub(p, 1, q - 1)
      if sub(p, q + 2, e - 1) == href then
        n = n + 1
        push(out, href .. sub(p, e + 4, #p))
        done = true
      end
    end
    if not done then push(out, "<a href=\"" .. p) end
  end
  return {n = n, plain = join(out, "")}
end

function compare(name)
  local b = bare(read(".run/links/before/" .. name))
  local a_page = read(".run/links/after/" .. name)
  local a = bare(a_page)
  local res = {name = name, linked = a.n - b.n, same = a.plain == b.plain, left = {}}
  local text = cut(cut(cut(cut(cut(a_page, "<script", "</script>"), "<style>", "</style>"), "<pre", "</pre>"), "<code>", "</code>"), "<a ", "</a>")
  text = cut(text, "<", ">")
  for lit in {"http://", "https://"} do
    for at in find_all(text, lit) do
      local from = at - 25
      if from < 1 then from = 1 end
      push(res.left, sub(text, from, at + 45))
    end
  end
  return res
end

local total = 0
local rows = {}
for p in glob(".run/links/after/*.html") do
  local c = compare(basename(p))
  if not c.same then print("OTHER TEXT CHANGED: " .. c.name) end
  if c.linked ~= 0 then
    total = total + c.linked
    push(rows, {linked = c.linked, note = c.name})
  end
  for l in c.left do print("left as text: " .. c.name .. ": [" .. join(split(l, "\n"), " ") .. "]") end
  yield()
end
sort(rows, "linked", "desc")
print(table(rows, {"linked", "note"}))
print(total .. " bare URLs linked on " .. #rows .. " pages")
```

```text
linked  note
     2  dogpark-v9.md.html
     2  safari-eye-tests.md.html
     1  autonomous-run-2026-11-12.md.html
     ... (21 more pages, 1 each)
26 bare URLs linked on 24 pages
── used: 128492696 of 200000000 steps, ...
```

No `OTHER TEXT CHANGED` line came out, and no `left as text` line. Put back as text, every after-page equals its before-page byte for byte. So the only change on all 514 pages is the 26 new links: no emphasis moved and no other text changed. 26 is also the number of bare URLs in prose that I counted by hand in the source survey.

A finder that reports nothing proves nothing until it has found something. So the same finder ran on the before-pages, and I counted the URLs written as a whole code span:

```shale
-- the left-as-text finder, run on the before pages, must find the 26 now linked; then the URLs written as a whole code span, and www. with no scheme
-- budget: steps=200000000
function cut(s, open, close)
  local parts = split(s, open)
  local out = {parts[1]}
  for i = 2, #parts do
    local e = find(parts[i], close)
    if e ~= nil then push(out, sub(parts[i], e + #close, #parts[i])) end
  end
  return join(out, " ")
end

function left_as_text(page)
  local text = cut(cut(cut(cut(cut(page, "<script", "</script>"), "<style>", "</style>"), "<pre", "</pre>"), "<code>", "</code>"), "<a ", "</a>")
  text = cut(text, "<", ">")
  return #find_all(text, "http://") + #find_all(text, "https://")
end

-- inline code spans that hold one http(s) URL and nothing else
function url_spans(page, name)
  local out = {}
  local parts = split(cut(page, "<pre", "</pre>"), "<code>")
  for i = 2, #parts do
    local e = find(parts[i], "</code>")
    if e ~= nil then
      local body = sub(parts[i], 1, e - 1)
      if (starts_with(body, "http://") or starts_with(body, "https://")) and find(body, " ") == nil then push(out, name .. ": " .. body) end
    end
  end
  return out
end

local before = 0
local spans = {}
local www = 0
for p in glob(".run/links/before/*.html") do
  before = before + left_as_text(read(p))
  local a = read(".run/links/after/" .. basename(p))
  for s in url_spans(a, basename(p)) do push(spans, s) end
  local text = cut(cut(cut(a, "<script", "</script>"), "<pre", "</pre>"), "<code>", "</code>")
  for at in find_all(text, "www.") do
    if sub(text, at - 2, at - 1) ~= "//" then
      www = www + 1
      print("www without a scheme: " .. basename(p) .. ": [" .. sub(text, at - 20, at + 40) .. "]")
    end
  end
  yield()
end
print("before pages: " .. before .. " http(s):// left as text outside code")
print(#spans .. " code spans that are one URL:")
for s in spans do print("  " .. s) end
print(www .. " www. with no scheme outside code")
```

```text
before pages: 26 http(s):// left as text outside code
15 code spans that are one URL:
  book-draft-deploy.md.html: https://roc.lynrummy.com/tutorial/billboard-critters.html
  dogpark-v9.md.html: http://143.244.172.148:9301/dogpark-5d8a122bbe0e.img.gz
  humans-without-github.md.html: https://chat.lynrummy.com/join#&lt;code&gt;
  in-flight-2026-09-15.md.html: https://roc.lynrummy.com
  machine-batch-demo.md.html: http://143.244.172.148:9203/machine/batch/
  roc-chat-auth-design.md.html: https://chat.lynrummy.com/login/github
  roc-chat-parity-audit.md.html: https://chat.lynrummy.com/chat.js
  roc-web-umbrella.md.html: http://&lt;box&gt;:9210/
  roc-web-umbrella.md.html: http://&lt;box&gt;:9200/
  roc-web-umbrella.md.html: https://roc.lynrummy.com/
  roc-web-umbrella.md.html: https://roc.lynrummy.com/safari/
  roc-web-umbrella.md.html: https://roc.lynrummy.com/basic/...
  wgsl-naga-where-it-landed.md.html: http://localhost:9202/apps/gpushow/web/index.html
  wgsl-naga-where-it-landed.md.html: http://localhost:9202/apps/globe/web/globe-codex.html
  wgsl-naga-where-it-landed.md.html: http://localhost:9202/apps/gpushow/web/reflect.html
0 www. with no scheme outside code
```

On the before-pages the finder found exactly the 26, so its zero on the after-pages is a real zero.

## The full check

```shale
-- the full check on the real notes, with the bare URLs in
local r = run({"shale", "scripts/full.shale"}, {timeout_ms = 115000, tail = 60, not_re = "^       calibrate|^             "})
print("exit " .. r.code .. ", " .. r.ms .. " ms wall, cpu " .. r.cpu_ms .. " ms")
print(r.out)
```

```text
exit 0, 25650 ms wall, cpu 22375 ms
box: load 0.29 on 2 CPUs; notes: /home/steve/showell_repos/essay-repl-server/notes
== fast: PASS, 2652 ms, cpu 3207 ms
== race: PASS, 3649 ms, cpu 3173 ms
== anchoring: PASS, 11675 ms, cpu 12930 ms
== audit: PASS, 1248 ms, cpu 618 ms
   514 pages, 24220 commentable blocks, 3 pages with leftovers
== dates: PASS, 1691 ms, cpu 444 ms
== bench: PASS, 4722 ms, cpu 1987 ms
   cpu, notes: 167 us a request over 1529 requests (256 ms), budget 400: PASS
   latency: every median within budget (load 0.41 on 2 CPUs before, 0.46 on 2 CPUs after): PASS
by hand still: an eye test of a note and its comments in a desktop browser
SAFE TO SHIP
```

```shale
-- is anything of mine still up? every essay-surface process; the human's on 9301 is left alone
for p in procs() do
  if p.name == "essay-surface" then print("pid " .. p.pid .. ": " .. sub(p.args, 1, 120)) end
end
print(json_write(git_status("")))
```

```text
pid 2545196: /home/steve/essay-dogfood/bin/essay-surface -notes /home/steve/essay-dogfood/notes -addr 127.0.0.1:9301
[]
```

Only your server on 9301 is running, and I didn't touch it.

## Report

**What it does.** A bare `http://` or `https://` URL in prose is now a link, `<a href="...">` with the same text and no `target`, so it opens the way the other links do. Code, fences, HTML blocks, markdown's own links and `<...>` links are unchanged, and a URL inside a link's text stays text. The change is one function in `markdown.go`, `bareURL`, and one branch of the inline pass (`e980835`).

**The cases**, 62 of them in `testdata/links/`, which the tests read directly with no build:

- **Must link (16):** in a sentence, an IP and port, a query and fragment, `_` and `*` inside, two in a line, at a line start, after `=`, letters past ASCII, inside strong and both emphases, and in a list item, heading, table cell and quote.
- **Must not (15):** inline code, fences, markdown links (one whose text is its URL is still one link), `<...>` links, images, inside a word (`xhttps://`), other schemes and `www.`, a scheme with no host, `http://<box>:9200/`, capitals, and HTML blocks.
- **Punctuation (22):** a final `. , : ; ! ? ' _ * ...` goes to the sentence. `)` and `]` stay only when the URL opened them, so `Go_(language)` is kept and `(see https://x.org).` gives both back. Quotes (straight and curly), an em dash, a backtick and a backslash end a URL, and a colon or period inside it stays.
- **Escaping (9 hostile notes):** `TestBareURLsEscaped` holds each to the renderer's tags only, every `&` an entity, and the href and the text the same URL, escaped once, as written.

**The seeded fault:** the href written unescaped. `TestBareURLsEscaped` went red on two hostile cases by three of its rules, and `TestBareURLs` on three exact cases. It ran once, under `--undo-on-stop`, with the tree back after.

**On your real notes (514):** it linked **26 bare URLs on 24 pages**. All 514 pages were rendered before and after. With those 26 links taken back to text, every page is byte for byte what it was, so nothing else moved. No `http(s)://` is left as text outside code; the same finder found those 26 on the old pages.

**Left alone, which you may want otherwise:**

1. **15 URLs written as a whole code span**, such as `` `http://143.244.172.148:9203/machine/batch/` `` in machine-batch-demo.md, the dogpark image in dogpark-v9.md, and three `localhost:9202` pages in wgsl-naga-where-it-landed.md. You said code is code, so they stay text. Some look meant to be clicked. Five of them are placeholders (`<box>`, `<code>`, `...`) that shouldn't be links anyway.
2. **Capitals (`HTTPS://`) and `www.` with no scheme** are not linked. Neither appears in your notes.
3. **A backslash ends a URL**, so a markdown-escaped `https://x.org/a\_b` links only `https://x.org/a`. None of your notes do this.
4. **An entity written inside a URL** (`?a&amp;b`) is shown as written, not decoded. None of your notes do this.

**The full check's last lines:**

```text
== bench: PASS, 4722 ms, cpu 1987 ms
   latency: every median within budget (load 0.41 on 2 CPUs before, 0.46 on 2 CPUs after): PASS
   bench: PASS
by hand still: an eye test of a note and its comments in a desktop browser
SAFE TO SHIP
```

25.7 s wall, 22.4 s CPU, at load 0.3.

**Commits:** `e980835` (the bare URLs, their tests and fixtures), and this journal.

## The release

The orchestrator ran `scripts/release.shale` on Steve's server, and these were its last lines:

```text
full check: SAFE TO SHIP, 24566 ms
comments kept: 4 files, 11 comments, in essay-dogfood/releases/2026-11-14T110736Z/comments, each compared
ok: the index, 514 notes, the same as before
ok: all 11 comments from before on their pages and in their files
RELEASED: running bbd9eae on 127.0.0.1:9301; to roll back to 8218ffc: cd /home/steve && shale repos:essay-surface/scripts/release.shale data=essay-dogfood port=9301 rollback=1
```

# Session 6: the release as a child run

This journal was written by the orchestrator from the agent's report. That agent polished the scripts and made the release's swap a child run (commits `cea25e7` to `8218ffc`). Its trial outputs are quoted as it reported them.

## Why

Shale now ends the jobs a stopped run started. A release that stopped on a budget *after* the swap would end the new server with it, and could never put the old one back. A script can't catch its own stop, so the swap and its checks moved into a child run, `release.shale part=install`: a second Shale process that the parent starts and watches. A child that stops just exits nonzero, and the parent, still running, puts the old server back.

## The good release, on dev

A fresh dev copy (`/tmp/claude-1000/essay-polish`: the real notes plus 2 test comments, port 9300):

```text
stopped the old server; its log kept
started 8218ffc built 2026-11-14T10:19:54Z: release.shale: a stopped child's call stack is not among its lines
ok: the index, 511 notes, the same as before
ok: a note, 2026-11-01-a-day-of-judges.md, 18882 bytes
ok: essay-surface-monday.md, 200, its 2 comments where they were
ok: all 2 comments from before on their pages and in their files
RELEASED: running 8218ffc on 127.0.0.1:9300; to roll back to 4ef30df: cd /tmp/claude-1000 && shale repos:essay-surface/scripts/release.shale data=essay-polish port=9300 rollback=1
```

After the parent exited, the new server was still serving, with both comments on the page.

## The planted stop, on dev

The fault was an endless loop just before `rel_check`, committed only in a throwaway clone (`/tmp/claude-1000/plant-repo`, run with `repo=plant-repo`, then deleted):

```text
stopped the old server; its log kept
started 0d12f14 built 2026-11-14T10:19:20Z: plant: a budget stop in the child's checks
FAILED: plant-repo/scripts/release.shale:248:9 (in `rel_install_and_check`): budget steps: the step budget (50000000 steps) ran out (50000001 steps)
started 4ef30df built 2026-11-14T10:17:16Z: release.shale: the swap and its checks run as a child run
ok: the index, 511 notes, the same as before
ok: a note, 2026-11-01-a-day-of-judges.md, 18882 bytes
ok: essay-surface-monday.md, 200, its 2 comments where they were
ok: all 2 comments from before on their pages and in their files
── stopped at ...: fail: ROLLED BACK: running 4ef30df on 127.0.0.1:9300, as before; the release failed: ...
── left running (keep_on_stop): .../essay-polish/bin/essay-surface ...
```

The old server came back and served both comments, and the comments file still held both lines.

## The real release

The child-run release then shipped to Steve's server on 9301 (8218ffc, 2026-11-14 10:28 UTC): SAFE TO SHIP, 7 comments kept and compared, all 7 where they were afterwards, in 24.3 s.

# Issue tracker: drops

Issues for this repo live in **drops**, the cross-project tracker this repo builds.
There is no GitHub or GitLab remote. `drops` is on `PATH` at `~/.local/bin/drops`.

drops keeps one database for every repository at `~/.drops/drops.db` and works out
which project you mean from the working directory. So **run every command from
inside the repo the work belongs to** and the scoping is automatic. `-P <slug>`
overrides it; `--all-projects` spans every project.

Every command takes `--json` and exits with a stable code. **Prefer `--json` for
anything you parse.** The collection verbs (`list`, `ready`, `blocked`, `search`)
render one terse row per issue. `show` renders a full page: identity, body,
every relation, and the whole comment thread.

## Everyday operations

| Want | Command |
|---|---|
| capture something fast | `drops q "title"` (prints the id, nothing else) |
| create with detail | `drops create "title" -t task -p 2 -l label -d "body"` |
| what can I work on | `drops ready` |
| what is stuck | `drops blocked` |
| read one | `drops show <id>` for a person, `drops show <id> --json` to parse |
| change one | `drops update <id> --title/-d/-p/-t/--status/-A` |
| finish one | `drops close <id> --reason "why"` |
| search | `drops search "term"` (titles, descriptions and comment bodies; add `-a` to reach closed issues) |
| link | `drops dep add <id> <blocker-id>` |
| refile into another project | `drops move <id>... --to <slug> [--subtree] [--dry-run]` |
| comment | `drops comment add <id> "body"` (prints the comment id) |
| read a thread | `drops comment list <id>`, or just `drops show <id>` |
| undo a comment | `drops comment rm <comment-id>` |

Issue types: `task`, `bug`, `feature`, `epic`, `chore`, `research`, `decision`.
Priority is 0 (critical) to 4 (backlog), default 2.

A new issue id is five random characters and no prefix, so `k3f9x`. A child
created with `--parent` takes a `.N` suffix under its parent, starting at `.1`,
so `k3f9x.3` is the third child of `k3f9x`. A comment id is
`<issue-id>:<12 hex>`, so it carries its own issue and `comment rm` needs no
issue argument.

**A memory keeps a `mem-` prefix; an issue has none.** The shared uniqueness
pool is why a store-wide CHECK is needed, not why a prefix is: `IDOwner` spans
both tables whatever an id looks like. The prefix is a label a reader uses, and
it sits on the 149-row side rather than the 979-row side because that is the
side where a marker is worth two characters. Ids minted before 2026-08-28 carry
a project prefix instead
(`beacon-0bl`, `br-6vf`, and beads-era slugs like
`beacon-ci-never-executed-4bhg`). **All of these shapes are permanent and equally
valid.** An id is an opaque key: nothing parses it, and `show` resolves any of
them from any working directory. Do not read a project out of an id, and do not
"fix" one.

The alphabet excludes `0`, `1`, `i`, `l` and `o`, so a hand-typed id has no
confusable characters. A mistyped id does not resolve; nothing normalises it.

**Ids are unique across BOTH tables, and `remember --key` is the one verb that
can test it.** `--key` sets a memory's id directly, so pasting an issue id into
it is a thing you can now do by accident: a bare id looks like an ordinary short
key. It is refused, exit 2, naming what the key already holds. Until
`drops-il6.14` it was not: the check spanned both tables but guarded only the
mint, so `drops remember --key <an issue id>` succeeded at exit 0 and one string
named two rows, `show` answering the issue and `recall` the memory. Nothing at
the database level stops that on its own; `memories.id` is a primary key over
`memories` alone.

`drops doctor` sweeps for it on every run, listing any id held by both tables.
**A collision is reported and never repaired**, including under `--repair`: an
id gets written into commit messages and `@refs` pointers, so renaming either
row breaks a citation nothing can renumber. The sweep is there for the one path
a write-time check cannot cover, a snapshot merged from another machine, which
arrives already written.

## How a long body gets in

Every text-bearing verb takes its body as an **argument**, so a long or multi-line
body comes from the shell:

```sh
drops create "title" -d "$(cat body.md)"
drops update <id> -d "$(cat body.md)"
drops comment add <id> "$(cat note.md)"
drops close <id> --reason "$(cat answer.md)"
```

**That is the convention, not a gap.** There is no `--description-file`, no
`--body-file`, no `-F`, no stdin form and no `$EDITOR` handoff, and the absence is
a choice. Measured against the binary and the live store on 2026-08-28:

- `ARG_MAX` is 1,048,576 bytes here and the longest description in the store is
  11,633, so the headroom is about 90x. A 320,000-byte body written through
  `-d "$(cat …)"` read back byte-identical.
- drops stores a body **verbatim**. Nothing in the service layer or the CLI trims
  it, so leading whitespace, tabs, `"`, `$` and backticks all survive.
- The `show --json`, edit, `update -d` round trip that the wayfinder map body uses
  is **byte-stable** after the first pass.

One real cost remains. **`$(cat …)` strips trailing newlines**, and no drops change
can fix it, because the loss happens in the shell before drops sees the argument.
`jq -r` adds one newline back, so the round trip converges rather than eroding, and
trailing blank lines are the only thing it can ever drop. Write a body with its
trailing blank lines already gone and the round trip is exact.

**Declined, and why**, so nobody re-proposes them:

- **A bare `-` meaning stdin.** `-d -` today stores a literal `-`. Making it mean
  stdin changes shipped behaviour to buy nothing, because nothing is near a size
  limit.
- **`-F/--file`.** To be worth having it would have to land on every text-bearing
  field: description, close reason and comment body. That is three flags to
  replace an idiom that already works.
- **`$EDITOR`.** The consumer is an agent. An agent writes a temp file without
  friction and never opens an editor.

**What would overturn this.** One body above about 1MB. Nothing is within 90x of it.

**Plan 6 does not reopen it.** The MCP server calls the service layer directly and
never goes through a shell, so a body reaches it as a string already.

## Moving an issue between projects

`drops move <id>... --to <slug>`. **The id never changes.** A new-style `k3f9x` says nothing
about a project to begin with. An older id keeps a project prefix that no longer matches:
`br-6vf` in project `drops` is correct, because that prefix records where the issue was
minted. Either way the `project` field is the only answer to where an issue lives. Do not
"fix" a mismatched prefix, and do not use one to infer a project.

The destination is `--to`, never `-P`. `-P` is the persistent scoping flag on every verb, so
`move <id> -P <slug>` scopes the lookup and moves nothing. The destination must already
exist; `move` never auto-creates one.

`--subtree` adds every descendant, by the union of the `<parent>.N` id rule and explicit
`parent-child` edges. Every status moves, tombstones included. An issue already in the
destination is a reported no-op, so re-running a half-finished triage is safe. One lost
compare-and-swap abandons the whole move, so a failure means retry, never repair.

The move prints the relationships it made cross a project boundary, `blocks` apart from
`parent-child`. Read the `blocks` list: `ready` and `blocked` take candidates from one
project but compute blockers across the whole store, so a blocker left behind makes an issue
un-ready with no visible cause where it now lives. `blocked` still names the blocker id, and
`show <id>` resolves it from any working directory.

`--dry-run` runs the real write path and rolls it back, printing what a real move would
print. `--json` emits **one object**, not an array: `moved`, `noop`, `crossing_blocks`,
`crossing_parentage`, `to` and `committed`, with the four lists always present as `[]`.

Issues only. A memory's scope moves with `drops memory edit <id> -P <slug>` or `--global`.

## Comments

`drops comment add <id> "body"` appends a comment and prints its id. The body is
a positional argument, so a long one comes from the shell the same way a
description does. `drops comment add <id> "$(cat note.md)"`. See
[How a long body gets in](#how-a-long-body-gets-in).

**Pass `--author`.** It defaults to the global git `user.name`, else the OS
username, which is right for a human and wrong for an agent. An agent commenting
as the repo's owner is a false record.

Adding or removing a comment stamps the issue's `updated_at`, so a growing
thread counts as activity in anything ordered by it. A closed issue accepts
comments.

**An issue has one body, and the thread holds everything else.** `design`,
`acceptance_criteria` and `notes` were columns inherited from beads that no CLI
verb could ever write. They are gone (`drops-il6.9`). The 38 populated `notes`
rows became comments authored `migration`, because every one of them was a dated
after-the-fact update, which is what a comment is; the 5 `acceptance_criteria`
rows folded into the description under an `## Acceptance criteria` heading,
because that is the spec an issue was written against rather than something that
happened later. Use that split when you are deciding where to put text.

`drops search` covers comment bodies as well as titles and descriptions, so
nothing became less findable in the move. A match in a comment is deliberately
indistinguishable from a match in a description: the result is still one row per
issue, in the same priority order.

**There is no `comment edit`, deliberately.** A comment body is a record of what
someone said, and rewriting it silently changes something another reader may
already have acted on. `comment rm` exists for an obvious mistake; a revision is
a new comment.

## When a skill says "publish to the issue tracker"

`drops create` in the repo the work belongs to. Capture the printed id.

## When a skill says "fetch the relevant ticket"

`drops show <id> --json`. The user normally passes the id directly.

## Scanning verbs and the reading verb

`list`, `ready`, `blocked` and `search` are **scanning verbs**: one row per issue,
always exactly one. `show` is the **reading verb**: one issue in full. The split
decides what each may drop, and the rule is not the same in both:

- A scanning verb **truncates what it displays** (a title is a label you recognise)
  and **never what identifies** (an id is a key you paste). `show` truncates nothing.
- **Truncation happens only when stdout is a terminal.** Piped or redirected, the
  full title prints, so `drops list | grep` sees every byte. This deliberately
  differs from `show`, which wraps at a fixed 80 off-terminal: wrapping is lossless
  and truncation is not, so copying the mechanism would have dropped grep matches
  silently.
- The id column **sizes to the widest id in the result set**, so column positions
  vary between invocations. Do not write a parser against a byte offset; use `--json`.
- A **project column appears only under `--all-projects`.** It has to exist, because
  `move` keeps an id byte-identical across a project change, so the prefix does not
  answer which project a row is in.
- Continuation lines (`ready`'s `unblocks N`, `blocked`'s `blocked by a, b`) are
  indented a fixed four columns, and the blocker list is comma-joined.
- All four page through `$PAGER` like `show`. `--json` never pages.

There is no assignee column, no label column and no `--wide`. Measured 2026-08-28:
10 of 184 open issues carry an assignee. `show` answers both for a row you picked out.

## Wayfinding operations

Used by `/wayfinder`. The **map** is one issue; its **tickets** are that issue's
children. Everything below was verified against the binary on 2026-08-28.

**Map.** An `epic` labelled `wayfinder:map`, created in the repo the effort
belongs to:

```sh
drops create "<effort name>" -t epic -l wayfinder:map -d "$(cat body.md)"
```

The body is the Destination / Notes / Decisions-so-far / Not-yet-specified /
Out-of-scope document. It arrives through `-d "$(cat body.md)"`, per
[How a long body gets in](#how-a-long-body-gets-in). To amend it, read
`drops show <map> --json`, edit the `description`, and write it back with
`drops update <map> -d "$(cat …)"`.

**Ticket.** A child of the map, carrying its type label:

```sh
drops create "<question title>" --parent <map-id> \
  -l wayfinder:research   # or wayfinder:prototype | wayfinder:grilling | wayfinder:task
  -d "## Question

<the decision this ticket resolves>"
```

Leave tickets at the default type `task`. That is load-bearing: it is what keeps
the map itself out of the frontier query.

**`--parent` carries the project, and the working directory is not consulted.**
A ticket lands in its map's project wherever you run `create` from, so a session
working a map in another repo no longer has to stand in the right directory. No
project is resolved at all on that path, which also means `create --parent` can
no longer auto-register a project for an unregistered cwd.

This was the reverse until `drops-il6.13`, and the reversal is why the rule is
worth stating rather than assuming: `beacon-p9d.1` through `.9` were created
against a map in `beacon`, landed in `inbox`, and all nine were deleted and
re-created as `.11` through `.19`, burning ten ids. Those nine are the only
straddling parent-child pairs the store has ever held, they are all `deleted`,
and no repair was made.

Two refusals come with it:

- **A `-P` or `--inbox` that disagrees with `--parent` is an error**, exit 2,
  naming both projects. One that agrees is fine. The cwd is ambient and carries
  no intention, so a parent silently overrides it; `-P` is a typed argument, so
  a contradiction cannot be resolved by guessing. The check runs before any
  resolution, so a refused `--inbox` does not create the inbox project.
- **A `--parent` naming no issue is an error**, exit 4. It used to succeed:
  `NextChildID` only scans for `<parent>.%`, so `create --parent zzzzz` minted
  `zzzzz.1` under an issue that did not exist. Inheriting the project made the
  parent lookup mandatory, which is what turned this into a refusal.

`drops move` is deliberately unchanged and still allows a subtree to be split
across projects, reporting what it crossed (`drops-il6.4`). That asymmetry is
intended: `create --parent` never carried an intention about projects, so
inferring one is free, while `move --to` is nothing but an intention about
projects.

**Blocking.** Native dependencies, so the tracker computes the frontier itself:

```sh
drops dep add <blocked-id> <blocker-id>     # <blocked-id> depends on <blocker-id>
```

`drops dep tree <id>` shows the longest chain of open blockers; `drops dep cycles`
exits non-zero if any cycle exists.

**Frontier.** Open, unblocked, unclaimed children of the map:

```sh
drops ready -t task --json | jq --arg m "<map-id>." \
  '[.[] | select(.id | startswith($m)) | select(.assignee == null)]'
```

`drops ready` already means open and unblocked. `-t task` drops the map's own
epic, which is otherwise unblocked and shows up in its own frontier. The
`assignee` filter is done here because `list` and `ready` have no assignee flag.
First by id order wins.

**Claim.** Before any work, so concurrent sessions skip the ticket:

```sh
drops update <ticket-id> -A "<dev name>"
```

An open, unassigned child is unclaimed. The assignee *is* the claim.

**Resolve.**

```sh
drops close <ticket-id> --reason "<the answer>"
```

The reason is stored as `close_reason` and is readable via
`drops show <id> --json`. Then append a one-line gist plus the ticket id to the
map's Decisions-so-far, using the `update -d` round-trip above.

**A resolution goes in `close --reason`, not in a comment.** This is a choice,
not the absence of an alternative: comments landed on 2026-08-28 (`drops-il6.2`)
and resolutions stayed here anyway. 735 issues already carry a `close_reason`,
and `show` renders it under `closed:` where a reader looks for why an issue
ended. A comment carries no ordering relative to the close, so putting the
answer there makes it harder to find. Comments are for what accumulates while a
ticket is open.

The one cost, so it is not a surprise: `close_reason` is a single column, so
reopening and re-closing a ticket overwrites it, where a thread would
accumulate.

**Rule out of scope.** Close the ticket without a decision reason:

```sh
drops close <ticket-id> --reason "out of scope: <why>"
```

and record it under the map's Out-of-scope section rather than Decisions-so-far.

**Known rough edges**, so a session does not mistake them for its own error:

- **`show --json` carries relations and comments; `list --json` does not.**
  `show` emits `parent`, `blockers`, `blocking`, `children` and `comments`
  alongside the issue's own fields, with a shape that does not vary: `parent` is
  `null` when there is none, and the four lists are `[]` rather than absent, so
  `.blockers[].id` and `.comments[].body` are always safe. No other verb emits
  them. `comments` carries whole bodies, so `show --json` on a long thread is
  large: the worst case in the store is 18.5KB.
- **`list --parent <id>` misses migrated children.** It matches on the id
  (`<parent>.N`) and ignores the `parent-child` dependency rows, and 124 of the
  484 such rows in the store have a child id that does not follow that pattern.
  `drops show <id> --json | jq -r '.children[].id'` takes the union of both and
  is the reliable answer.
- `drops ready` exits 0 with an advisory on stderr when the cwd resolves to no
  project. Check stdout, not just the exit code.
- **`blocked --json` is not a flat array of issues.** Each element is
  `{"issue": {...}, "blocked_by": [...]}`, which is deliberate (it carries the
  blockers), but it means `.[].id` works on `list` and `ready` and returns nothing
  on `blocked`. Use `.[].issue.id` there. **`blocked_by` is a bare array of id
  strings**, not of issue objects, so `.blocked_by[].id` fails with `Cannot index
  string with string`. `.blocked_by | join(", ")` and `| length` are the two useful
  forms. Measured 2026-08-30.
- **`list --json` omits `labels`; `show --json` includes it.** Filtering a `list`
  result by label in your own code therefore matches nothing, silently. Use the
  server-side `-l` / `--label-any` flags, which work correctly on both, or fetch
  the issue with `show --json` when you need its labels.
- **`drops label list` ignores `-P` and returns store-wide counts.** `-P drops`,
  `-P beacon` and `-P global` print the same table byte for byte. `-P` is accepted
  and exits 0, so a per-project label count is another project's numbers with
  nothing to tell them apart. Filed as `i-wnkh7`. Until it is settled, do not read
  a label count as project-scoped.

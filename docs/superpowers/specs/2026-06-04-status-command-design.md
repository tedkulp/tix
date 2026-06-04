# tix status — Design Spec

**Date:** 2026-06-04  
**Status:** Draft

---

## Overview

Add a `tix status` command that answers "where am I in the tix workflow?" for the current branch. It resolves the branch → issue → MR chain and reports what exists, what's missing, and what to do next. It supports `--json` for automation and LLM use.

---

## Goals

- Give a quick, reliable "resume point" when returning to a branch.
- Provide a `--json` output that eliminates guesswork for scripts and LLMs.
- Degrade gracefully: report what's knowable even when pieces are missing.
- Signal "not a ticket branch" with a non-zero exit code so callers can detect it.

---

## Non-Goals

- Pipeline/CI state (requires extra API calls; not requested).
- Interactive prompts (status is purely read-only).
- Modifying any remote state.

---

## Architecture

### Option chosen: Service layer + thin command (Option B)

**`internal/services/status.go`** — contains `GetWorkflowStatus()` and the `WorkflowStatus` struct. Testable independently of Cobra.

**`cmd/status.go`** — thin Cobra command; calls the service, formats output (human-readable or JSON), sets exit code.

**`cmd/repo_selector.go`** (new shared file) — extracts `selectRepository()` out of `setdesc.go` so both `setdesc` and `status` can use it without duplication. The `RepoInfo` struct moves here too.

---

## Data Model

```go
// WorkflowStatus is the result returned by GetWorkflowStatus.
type WorkflowStatus struct {
    Branch        string   // current git branch
    IssueNumber   int      // 0 if not a ticket branch
    IssueTitle    string   // empty if issue lookup failed
    IssueLabels   []string // labels on the issue
    Milestone     string   // milestone title, empty if none
    MRURL         string   // empty if no open MR
    MRNumber      int      // 0 if no open MR
    MRIsDraft     bool
    SuggestedNext string   // e.g. "tix mr", "tix setdesc", "tix ready"
}
```

`GetWorkflowStatus` accepts a `SCMProvider` (and optionally an issue provider for cross-repo) plus the branch name and issue number, so it is fully unit-testable with a mock provider.

**Milestone resolution:** `IssueResult` currently stores only `MilestoneID int`. To surface a human-readable milestone title, `IssueResult` will be extended with a `MilestoneTitle string` field, populated by the GitLab and GitHub provider implementations at fetch time. GitHub exposes the milestone title in the issue response directly; GitLab's `GetIssue` response includes `issue.Milestone.Title` and can be read without an extra API call.

---

## Workflow State Machine

`SuggestedNext` is derived from this precedence:

| Condition | SuggestedNext |
|-----------|--------------|
| Branch doesn't parse as a ticket branch | `""` (not applicable) |
| No open MR exists | `tix mr` |
| MR exists and is a draft | `tix setdesc` |
| MR exists, not draft, ready_label is absent from issue labels | `tix ready` |
| MR exists, not draft, ready_label is present | `""` (workflow complete) |

The ready label is resolved from config (per-repo → global → empty), same as `tix ready` does today.

---

## Command Interface

```
tix status [--json]
```

**Flags:**
- `--json` / `-j` — emit a single JSON object to stdout instead of human-readable text.

**Exit codes:**
- `0` — on a ticket branch and status was retrieved successfully.
- `1` — not a ticket branch, or branch parsing failed.
- `2` — ticket branch detected but API calls failed (provider error).

(Exit code `1` vs `2` lets callers distinguish "wrong branch" from "API problem".)

---

## Human-Readable Output

Normal case:
```
Branch:   123-fix-login-bug
Issue:    #123 "Fix login bug"  [open]
Labels:   bug, in-progress
Milestone: 2026.Q2
MR:       !45 (draft)  https://gitlab.com/.../merge_requests/45
Next:     tix setdesc
```

No MR yet:
```
Branch:   456-add-widget
Issue:    #456 "Add widget"  [open]
Labels:   feature
Milestone: (none)
MR:       (none)
Next:     tix mr
```

Not a ticket branch:
```
Error: branch "main" is not a ticket branch
```
(exits 1)

---

## JSON Output (`--json`)

```json
{
  "branch": "123-fix-login-bug",
  "issue_number": 123,
  "issue_title": "Fix login bug",
  "issue_labels": ["bug", "in-progress"],
  "milestone": "2026.Q2",
  "mr_url": "https://gitlab.com/.../merge_requests/45",
  "mr_number": 45,
  "mr_is_draft": true,
  "suggested_next": "tix setdesc"
}
```

Fields are always present (zero values: `""`, `0`, `false`, `[]`). No field is ever omitted so consumers don't need existence checks.

---

## Refactoring: `selectRepository()` extraction

`selectRepository()` and `RepoInfo` currently live in `cmd/setdesc.go`. They will move to a new `cmd/repo_selector.go`. `setdesc.go` imports nothing extra — it just calls the function from the same package. This is a pure rename/move with no behaviour change, so it carries no regression risk.

---

## Error Handling

| Situation | Behaviour |
|-----------|-----------|
| Not a ticket branch | print error message, exit 1 |
| Config not found | return error, exit 2 |
| No matching repo for current dir, non-interactive | return error, exit 2 |
| Issue API call fails | return error with context, exit 2 |
| MR lookup fails | report issue info only, note MR lookup failed, exit 2 |

MR lookup failure is separated from issue lookup failure: knowing the issue exists is useful even if the MR lookup errors.

---

## Testing

- Unit test `GetWorkflowStatus` against a mock `SCMProvider`:
  - Branch with no MR → `SuggestedNext == "tix mr"`
  - Draft MR → `SuggestedNext == "tix setdesc"`
  - Non-draft MR without ready label → `SuggestedNext == "tix ready"`
  - Non-draft MR with ready label → `SuggestedNext == ""`
- Integration tests follow the existing pattern in `cmd/start_test.go` and `cmd/cleanup_test.go`.

---

## Files Changed

| File | Change |
|------|--------|
| `cmd/repo_selector.go` | **New** — `RepoInfo`, `selectRepository()` moved here |
| `cmd/setdesc.go` | Remove `RepoInfo` and `selectRepository()` (now in shared file) |
| `internal/services/status.go` | **New** — `WorkflowStatus`, `GetWorkflowStatus()` |
| `cmd/status.go` | **New** — Cobra command, formatting, exit code logic |

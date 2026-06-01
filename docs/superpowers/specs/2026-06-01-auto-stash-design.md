# Auto-Stash on Branch Creation

**Date:** 2026-06-01  
**Status:** Approved

## Problem

When running `tix create` or `tix start` with a dirty working directory (uncommitted changes or untracked files), the command fails immediately with an error asking the user to commit or stash first. This is a common interruption in a normal workflow where you have in-progress work but want to start a new branch.

## Goal

Automatically stash dirty changes before creating the branch, then pop them back after. The working directory should end up in the same state it started in. An opt-out flag lets users disable this when they want the old strict behavior.

## Scope

- Affects `tix create` and `tix start` only (the two commands that do branch creation without worktrees).
- Does **not** affect worktree mode — worktrees don't require a clean state and that check is already skipped.
- No config file setting; flag only.

## Git Layer (`internal/git/git.go`)

Add two methods to `git.Repository`:

### `Stash() error`
```go
func (r *Repository) Stash() error
```
Runs `git stash -u` (include untracked files). Returns an error if the command exits non-zero.

### `StashPop() error`
```go
func (r *Repository) StashPop() error
```
Runs `git stash pop`. Returns an error if the command exits non-zero.

Both methods follow the existing shell-out pattern (`exec.Command` with `cmd.Dir = r.path`).

## Command Layer

### Flag

Each command gets a `--no-auto-stash` boolean flag (default: `false`, meaning auto-stash is **on** by default):

- `create`: `--no-auto-stash` stored in `noAutoStash bool`
- `start`: `--no-auto-stash` stored in `startNoAutoStash bool`

### Control Flow (non-worktree path only)

Replace the existing dirty-check block in both commands with:

```
isClean, err := gitRepo.IsClean()
if err != nil { return err }

stashed := false
if !isClean {
    if noAutoStash {
        return fmt.Errorf("git repository has uncommitted changes - commit or stash them first")
    }
    if err := gitRepo.Stash(); err != nil {
        return fmt.Errorf("failed to stash changes: %w", err)
    }
    stashed = true
    defer func() {
        if err := gitRepo.StashPop(); err != nil {
            fmt.Fprintf(os.Stderr, "Warning: failed to restore stashed changes: %v\n", err)
            fmt.Fprintf(os.Stderr, "Your changes are still in the stash — run `git stash pop` manually.\n")
        }
    }()
}
```

Key behaviors:
- `defer` ensures pop runs regardless of what happens later (success, error, or panic).
- If `StashPop` fails, the user is warned with a message explaining the stash is still there and how to recover. The command does **not** return an error in this case — the branch was created successfully, and the stash is not lost.
- A short `fmt.Printf("Stashed changes, will restore after branch creation.\n")` message is printed when a stash occurs so the user knows what happened.

### Error Handling Matrix

| Scenario | Behavior |
|---|---|
| Repo clean | No stash, proceed normally |
| Repo dirty, auto-stash on | Stash, create branch, pop |
| Repo dirty, `--no-auto-stash` | Error: ask user to stash manually |
| Stash fails | Error before branch creation; nothing changes |
| Branch creation fails | defer pops stash; error returned |
| Stash pop fails | Warning printed; branch created; stash left intact |

## Files Changed

| File | Change |
|---|---|
| `internal/git/git.go` | Add `Stash()` and `StashPop()` methods |
| `cmd/create.go` | Add `noAutoStash` flag; replace dirty-check with stash logic |
| `cmd/start.go` | Add `startNoAutoStash` flag; replace dirty-check with stash logic |

## Out of Scope

- Config file setting for default behavior
- `tix ready`, `tix mr`, or other commands (don't create branches)
- Worktree mode (already bypasses the clean check)

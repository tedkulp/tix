# Worktree Redesign

**Date:** 2026-03-09

## Summary

Rework worktree handling so that worktrees are always an opt-in choice at
command invocation time (via `--worktree` flag) rather than a static per-repo
config toggle. Add a `tix cleanup` command to remove worktrees.

## Motivation

The current design has `worktree.enabled` as a per-repo boolean in config.
This means a repo either always uses worktrees or never does. The new design
lets the user decide per invocation, making worktrees a first-class option
without forcing it.

---

## Config Structure

A shared `WorktreeConfig` struct is used at both the global `Settings` level
and per `Repository` level. Per-repo values override global values. Hardcoded
defaults apply when neither is set.

```go
type WorktreeConfig struct {
    Path          string `yaml:"path" mapstructure:"path"`
    DefaultBranch string `yaml:"default_branch" mapstructure:"default_branch"`
}
```

**Resolution order for each field:**
1. Per-repo `worktree.path` / `worktree.default_branch`
2. Global `worktree.path` / `worktree.default_branch`
3. Hardcoded default: path = `<repo-dir>/.worktrees`, default_branch = `main`

**Example `.tix.yaml`:**
```yaml
worktree:
  path: ~/.worktrees
  default_branch: main

repositories:
  - name: myrepo
    directory: ~/src/myrepo
    worktree:
      path: ~/src/myrepo/.worktrees  # overrides global
```

**Removals:**
- `Worktree.Enabled` field is removed entirely
- All `if repo.Worktree.Enabled` checks are removed

---

## `tix create` and `tix start` Changes

### New flag

Both commands gain `--worktree` / `-w` (bool flag, default `false`).

### Worktree path

When `--worktree` is passed, the worktree base path is resolved using the
resolution order above. The final worktree directory is:
`<resolved-base-path>/<branch-name>`

### Git operation

Use `exec.Command("git", "worktree", "add", worktreeDir, "-b", branchName)`
run from the repo's root directory. This replaces the broken `go-git`-based
`AddWorktree` implementation.

### Clean check

The clean working tree check is **skipped** when `--worktree` is passed.
It is still enforced for the normal (non-worktree) branch flow.

### Output

```
Created worktree: /path/to/.worktrees/123-my-feature
```

### `createBranch` signature change

```go
func createBranch(gitRepo *git.Repository, repo *config.Repository, cfg *config.Settings,
    issueNumber int, issueTitle string, projectPrefix string, useWorktree bool) error
```

---

## `tix cleanup` Command

New file: `cmd/cleanup.go`

### Usage

```
tix cleanup [branch-name]
```

### Behavior

1. Detect current directory. If it is inside the resolved worktree base path,
   extract the branch name from the path segment immediately below the base.
2. Show an interactive text prompt pre-filled with the detected branch name
   (empty string if not detected).
3. Resolve the worktree base path using the same resolution order.
4. Run `git worktree remove <worktree-dir>` via `exec.Command` from the
   repo root.
5. Print:
   ```
   Removed worktree: /path/to/.worktrees/123-my-feature
   ```

### What is NOT done

- The branch is **not** deleted — left intact for future use or PR creation.
- No `cd` or shell navigation — user is responsible for navigating back.

---

## Git Layer Changes

`internal/git/worktree.go`:

- Replace `AddWorktree` with a real implementation using
  `exec.Command("git", "worktree", "add", path, "-b", branchName)`
- Add `RemoveWorktree(path string) error` using
  `exec.Command("git", "worktree", "remove", path)`
- Remove `ListWorktrees` and `GetWorktreePath` (unused after this change)

---

## Files Changed

| File | Change |
|------|--------|
| `internal/config/repository.go` | Replace `Worktree` struct with `WorktreeConfig`; add global `Worktree` to `Settings` |
| `internal/git/worktree.go` | Rewrite `AddWorktree` with exec; add `RemoveWorktree`; remove unused funcs |
| `cmd/create.go` | Add `--worktree` flag; skip clean check when worktree; update `createBranch` |
| `cmd/start.go` | Add `--worktree` flag; skip clean check when worktree; update branch creation |
| `cmd/cleanup.go` | New file implementing `tix cleanup` |

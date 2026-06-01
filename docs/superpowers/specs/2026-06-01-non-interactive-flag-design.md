# Non-Interactive Flag for `tix create` and `tix start`

**Date:** 2026-06-01  
**Status:** Approved for implementation

## Summary

Add a `--non-interactive` / `-n` flag to `tix create` and `tix start` that suppresses all interactive prompts and substitutes sensible defaults. The primary motivation is machine-driven use (LLM agents, CI scripts) where stdin is unavailable or interactive prompts are undesirable.

## Scope

Changes are limited to `cmd/create.go` and `cmd/start.go`. No changes to internal packages, config schema, or other commands.

## `tix create --non-interactive`

### Flag

```
--non-interactive, -n   Skip all interactive prompts and use defaults
```

### Prompt substitution

| Prompt | Non-interactive default |
|--------|------------------------|
| Repo selector | Auto-detected from cwd (longest path match); falls through to the existing single-repo shortcut if only one repo is configured |
| Title input | **No default** — caller must pass `-t`/`--title` |
| Labels input | `defaultLabels` from the matched repo's config (empty string if unset) |
| Milestone input | `utils.GenerateMilestone(time.Now())` — the same value the interactive prompt pre-fills |

### Error conditions

- `-n` without `-t`/`--title` → error: `--non-interactive requires -t/--title`
- `-n` with no cwd-matching repo and multiple repos configured → error: `--non-interactive requires an unambiguous repository; pass the repo name as an argument or cd into a configured directory`

### Unchanged behavior

- `--worktree` / `-w` still works alongside `-n`
- `--no-auto-stash` still works alongside `-n`
- Self-assign (`-a`, default `true`) is unaffected
- Cross-repo form `tix create [issue-repo] [code-repo]` still works; `-n` suppresses only the prompts within that flow (labels, milestone)

## `tix start --non-interactive`

### Flag

```
--non-interactive, -n   Skip all interactive prompts and use defaults
```

### Prompt substitution

| Prompt | Non-interactive default |
|--------|------------------------|
| Project selector | Auto-detected from cwd (longest path match); falls back to first repo if only one code repo configured |
| Issue number input | **No default** — caller must pass the issue number as a positional argument |

### Error conditions

- `-n` with no positional issue-number argument → error: `--non-interactive requires an issue number argument` (must be handled explicitly — `tix start` currently falls through to an interactive prompt when no args are given, so Cobra does not catch this automatically)
- `-n` with no cwd-matching repo and multiple repos configured → error: `--non-interactive requires an unambiguous repository; pass the project name as an argument or cd into a configured directory`

### Unchanged behavior

- `--worktree` / `-w` still works alongside `-n`
- `--no-auto-stash` still works alongside `-n`
- Two-argument form `tix start [project] [issue-number]` still works; `-n` suppresses only the interactive project selector when the project argument is omitted

## Implementation Notes

Both commands already have the "auto-detect from cwd" logic that finds the best-matching repo by longest path prefix. The `-n` flag simply makes that auto-detection mandatory rather than presenting it as the pre-selected default in a selector.

The `promptForLabels` and `promptForMilestone` functions in `cmd/create.go` are already structured so their logic can be bypassed inline — no refactoring of those helpers is needed.

## Testing

- Unit tests for the error conditions (missing `-t` with `-n`, ambiguous repo with `-n`) in `cmd/create_test.go` and `cmd/start_test.go`
- Existing tests should continue to pass without modification

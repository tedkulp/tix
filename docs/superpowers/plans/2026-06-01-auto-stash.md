# Auto-Stash on Branch Creation — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** When `tix create` or `tix start` detects a dirty working directory, automatically stash changes, create the branch, then pop the stash — restoring the working tree to its prior state.

**Architecture:** Add `Stash()` and `StashPop()` shell-out methods to `git.Repository`. Replace the hard-fail dirty-check in both commands with conditional stash logic guarded by a `--no-auto-stash` flag. A `defer` on the pop ensures the stash is always restored even if branch creation fails.

**Tech Stack:** Go, `os/exec`, Cobra (spf13/cobra), standard `go test`

---

## File Map

| File | Change |
|---|---|
| `internal/git/git.go` | Add `Stash()` and `StashPop()` methods |
| `internal/git/git_test.go` | New file — integration tests for the two new methods |
| `cmd/create.go` | Add `noAutoStash` var + flag; replace dirty-check block with stash logic |
| `cmd/create_test.go` | New file — flag registration test |
| `cmd/start.go` | Add `startNoAutoStash` var + flag; replace dirty-check block with stash logic |
| `cmd/start_test.go` | New file — flag registration test |

---

## Task 1: Add `Stash()` and `StashPop()` to the git layer

**Files:**
- Modify: `internal/git/git.go`
- Create: `internal/git/git_test.go`

- [ ] **Step 1: Write the failing tests**

Create `internal/git/git_test.go`:

```go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func newTestRepo(t *testing.T) *Repository {
	t.Helper()
	dir := t.TempDir()

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")

	readme := filepath.Join(dir, "readme.txt")
	if err := os.WriteFile(readme, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	run("add", "readme.txt")
	run("commit", "-m", "initial commit")

	repo, err := Open(dir)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	return repo
}

func TestStashAndPop(t *testing.T) {
	repo := newTestRepo(t)

	// Write an untracked file to make the repo dirty
	change := filepath.Join(repo.path, "change.txt")
	if err := os.WriteFile(change, []byte("dirty"), 0644); err != nil {
		t.Fatal(err)
	}

	clean, err := repo.IsClean()
	if err != nil {
		t.Fatalf("IsClean() before stash error: %v", err)
	}
	if clean {
		t.Fatal("expected dirty repo before stash")
	}

	if err := repo.Stash(); err != nil {
		t.Fatalf("Stash() error: %v", err)
	}

	clean, err = repo.IsClean()
	if err != nil {
		t.Fatalf("IsClean() after stash error: %v", err)
	}
	if !clean {
		t.Fatal("expected clean repo after stash")
	}

	if err := repo.StashPop(); err != nil {
		t.Fatalf("StashPop() error: %v", err)
	}

	clean, err = repo.IsClean()
	if err != nil {
		t.Fatalf("IsClean() after pop error: %v", err)
	}
	if clean {
		t.Fatal("expected dirty repo after stash pop (change.txt should be restored)")
	}

	if _, err := os.Stat(change); os.IsNotExist(err) {
		t.Fatal("expected change.txt to be restored after stash pop")
	}
}

func TestStashOnCleanRepo(t *testing.T) {
	repo := newTestRepo(t)

	// Stashing a clean repo is a no-op in git but should not error
	if err := repo.Stash(); err != nil {
		t.Fatalf("Stash() on clean repo error: %v", err)
	}
}

func TestStashPopWithNoStash(t *testing.T) {
	repo := newTestRepo(t)

	// Pop with nothing stashed should return an error
	if err := repo.StashPop(); err == nil {
		t.Fatal("expected StashPop() to return error when stash is empty")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/git/... -v -run TestStash
```

Expected: FAIL — `repo.Stash undefined` and `repo.StashPop undefined`

- [ ] **Step 3: Add `Stash()` and `StashPop()` to `internal/git/git.go`**

Add after the `DeleteBranch` function at the end of the file:

```go
// Stash saves all working directory changes (including untracked files) to the stash
func (r *Repository) Stash() error {
	cmd := exec.Command("git", "stash", "-u")
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stash changes: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	logger.Debug("Changes stashed", map[string]interface{}{
		"output": strings.TrimSpace(string(output)),
	})
	return nil
}

// StashPop restores the most recently stashed changes
func (r *Repository) StashPop() error {
	cmd := exec.Command("git", "stash", "pop")
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to pop stash: %w (output: %s)", err, strings.TrimSpace(string(output)))
	}
	logger.Debug("Stash popped", map[string]interface{}{
		"output": strings.TrimSpace(string(output)),
	})
	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/git/... -v -run TestStash
```

Expected: PASS for `TestStashAndPop` and `TestStashOnCleanRepo`. `TestStashPopWithNoStash` should also PASS (git exits non-zero when stash is empty).

- [ ] **Step 5: Run the full test suite to confirm no regressions**

```bash
go test ./...
```

Expected: all existing tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/git/git.go internal/git/git_test.go
git commit -m "feat: add Stash and StashPop methods to git.Repository"
```

---

## Task 2: Add auto-stash to `tix create`

**Files:**
- Modify: `cmd/create.go`
- Create: `cmd/create_test.go`

- [ ] **Step 1: Write the failing flag test**

Create `cmd/create_test.go`:

```go
package cmd

import "testing"

func TestCreateNoAutoStashFlag(t *testing.T) {
	flag := createCmd.Flags().Lookup("no-auto-stash")
	if flag == nil {
		t.Fatal("expected --no-auto-stash flag to be registered on createCmd")
	}

	if flag.DefValue != "false" {
		t.Errorf("expected default 'false', got %q", flag.DefValue)
	}

	if flag.Usage == "" {
		t.Error("expected --no-auto-stash flag to have a usage description")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./cmd/... -v -run TestCreateNoAutoStashFlag
```

Expected: FAIL — `flag == nil`

- [ ] **Step 3: Declare the variable and register the flag in `cmd/create.go`**

In the `var` block at the top of `cmd/create.go` (around line 19), add `noAutoStash`:

```go
var (
	title       string
	selfAssign  bool
	useWorktree bool
	noAutoStash bool
)
```

In the `init()` function at the bottom of `cmd/create.go`, add:

```go
createCmd.Flags().BoolVar(&noAutoStash, "no-auto-stash", false, "Disable automatic stashing of uncommitted changes before branch creation")
```

- [ ] **Step 4: Run the flag test to verify it passes**

```bash
go test ./cmd/... -v -run TestCreateNoAutoStashFlag
```

Expected: PASS

- [ ] **Step 5: Replace the dirty-check block in `cmd/create.go` with auto-stash logic**

Find this block (around lines 82–90):

```go
if !useWorktree {
    isClean, err := gitRepo.IsClean()
    if err != nil {
        return fmt.Errorf("failed to check repository status: %w", err)
    }
    if !isClean {
        return fmt.Errorf("git repository has uncommitted changes - commit or stash them first")
    }
}
```

Replace it with:

```go
if !useWorktree {
    isClean, err := gitRepo.IsClean()
    if err != nil {
        return fmt.Errorf("failed to check repository status: %w", err)
    }
    if !isClean {
        if noAutoStash {
            return fmt.Errorf("git repository has uncommitted changes - commit or stash them first")
        }
        if err := gitRepo.Stash(); err != nil {
            return fmt.Errorf("failed to stash changes: %w", err)
        }
        fmt.Println("Stashed changes, will restore after branch creation.")
        defer func() {
            if popErr := gitRepo.StashPop(); popErr != nil {
                fmt.Fprintf(os.Stderr, "Warning: failed to restore stashed changes: %v\n", popErr)
                fmt.Fprintf(os.Stderr, "Your changes are still in the stash — run `git stash pop` manually.\n")
            }
        }()
    }
}
```

- [ ] **Step 6: Run the full test suite**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 7: Build and do a quick smoke test**

```bash
just build
# In any git repo with an uncommitted change:
# bin/tix create --help   (verify --no-auto-stash appears in help)
```

Expected: `--no-auto-stash` visible in help output.

- [ ] **Step 8: Commit**

```bash
git add cmd/create.go cmd/create_test.go
git commit -m "feat: auto-stash dirty changes in tix create, add --no-auto-stash flag"
```

---

## Task 3: Add auto-stash to `tix start`

**Files:**
- Modify: `cmd/start.go`
- Create: `cmd/start_test.go`

- [ ] **Step 1: Write the failing flag test**

Create `cmd/start_test.go`:

```go
package cmd

import "testing"

func TestStartNoAutoStashFlag(t *testing.T) {
	flag := startCmd.Flags().Lookup("no-auto-stash")
	if flag == nil {
		t.Fatal("expected --no-auto-stash flag to be registered on startCmd")
	}

	if flag.DefValue != "false" {
		t.Errorf("expected default 'false', got %q", flag.DefValue)
	}

	if flag.Usage == "" {
		t.Error("expected --no-auto-stash flag to have a usage description")
	}
}
```

- [ ] **Step 2: Run the test to verify it fails**

```bash
go test ./cmd/... -v -run TestStartNoAutoStashFlag
```

Expected: FAIL — `flag == nil`

- [ ] **Step 3: Declare the variable and register the flag in `cmd/start.go`**

`start.go` already has `var startUseWorktree bool` at the top (line 19). Add `startNoAutoStash` next to it:

```go
var (
	startUseWorktree  bool
	startNoAutoStash  bool
)
```

In the `init()` function at the bottom of `cmd/start.go`, add:

```go
startCmd.Flags().BoolVar(&startNoAutoStash, "no-auto-stash", false, "Disable automatic stashing of uncommitted changes before branch creation")
```

- [ ] **Step 4: Run the flag test to verify it passes**

```bash
go test ./cmd/... -v -run TestStartNoAutoStashFlag
```

Expected: PASS

- [ ] **Step 5: Replace the dirty-check block in `cmd/start.go` with auto-stash logic**

Find this block (around lines 227–235):

```go
if !startUseWorktree {
    isClean, err := gitRepo.IsClean()
    if err != nil {
        return fmt.Errorf("failed to check repository status: %w", err)
    }
    if !isClean {
        return fmt.Errorf("repository is not clean - commit or stash changes first")
    }
}
```

Replace it with:

```go
if !startUseWorktree {
    isClean, err := gitRepo.IsClean()
    if err != nil {
        return fmt.Errorf("failed to check repository status: %w", err)
    }
    if !isClean {
        if startNoAutoStash {
            return fmt.Errorf("repository is not clean - commit or stash changes first")
        }
        if err := gitRepo.Stash(); err != nil {
            return fmt.Errorf("failed to stash changes: %w", err)
        }
        fmt.Println("Stashed changes, will restore after branch creation.")
        defer func() {
            if popErr := gitRepo.StashPop(); popErr != nil {
                fmt.Fprintf(os.Stderr, "Warning: failed to restore stashed changes: %v\n", popErr)
                fmt.Fprintf(os.Stderr, "Your changes are still in the stash — run `git stash pop` manually.\n")
            }
        }()
    }
}
```

- [ ] **Step 6: Run the full test suite**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 7: Build and verify help output**

```bash
just build
bin/tix start --help
```

Expected: `--no-auto-stash` visible in the help output for `start`.

- [ ] **Step 8: Commit**

```bash
git add cmd/start.go cmd/start_test.go
git commit -m "feat: auto-stash dirty changes in tix start, add --no-auto-stash flag"
```

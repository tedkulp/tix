package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	run("config", "commit.gpgsign", "false")

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

func TestCreateAndCheckoutBranch(t *testing.T) {
	repo := newTestRepo(t)
	head := revParse(t, repo.path, "HEAD")

	if err := repo.CreateAndCheckoutBranch("feature-branch"); err != nil {
		t.Fatalf("CreateAndCheckoutBranch() error: %v", err)
	}

	branch, err := GetBranchFromDir(repo.path)
	if err != nil {
		t.Fatalf("GetBranchFromDir() error: %v", err)
	}
	if branch != "feature-branch" {
		t.Fatalf("expected branch feature-branch, got %q", branch)
	}

	// The new branch must point at the commit HEAD was on, not a fresh root.
	if head != revParse(t, repo.path, "HEAD") {
		t.Fatalf("expected HEAD to stay at %s after branching", head)
	}
}

// revParse returns the commit hash the given revision resolves to in dir.
func revParse(t *testing.T, dir, rev string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", rev)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", rev, err)
	}
	return strings.TrimSpace(string(out))
}

func TestCreateAndCheckoutBranchExisting(t *testing.T) {
	repo := newTestRepo(t)

	if err := repo.CreateAndCheckoutBranch("dupe"); err != nil {
		t.Fatalf("CreateAndCheckoutBranch() error: %v", err)
	}
	if err := repo.CreateAndCheckoutBranch("dupe"); err == nil {
		t.Fatal("expected error creating a branch that already exists")
	}
}

func TestOpenNonRepository(t *testing.T) {
	if _, err := Open(t.TempDir()); err == nil {
		t.Fatal("expected Open() to fail on a directory that is not a git repository")
	}
}

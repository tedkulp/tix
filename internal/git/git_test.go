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

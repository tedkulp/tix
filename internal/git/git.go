// Package git provides utilities for interacting with Git repositories
// including operations for branches, commits, and worktrees.
package git

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/tedkulp/tix/internal/logger"
)

// Repository represents a Git repository
type Repository struct {
	path string
}

// Open opens a Git repository at the given path
func Open(path string) (*Repository, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = path
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to open repository: %s: %w", strings.TrimSpace(string(output)), err)
	}

	return &Repository{path: path}, nil
}

// IsClean checks if the working directory is clean
func (r *Repository) IsClean() (bool, error) {
	// Use git status --porcelain to check if repository is clean
	// If it returns no output, the repository is clean
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = r.path
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to check git status: %w", err)
	}

	// If output is empty, repository is clean
	isClean := len(output) == 0

	logger.Debug("Git status check", map[string]interface{}{
		"is_clean": isClean,
		"output":   string(output),
	})

	return isClean, nil
}

// CreateAndCheckoutBranch creates a new branch from the current HEAD and checks it out.
// Runs: git checkout -b <name>
func (r *Repository) CreateAndCheckoutBranch(name string) error {
	cmd := exec.Command("git", "checkout", "-b", name)
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create and checkout branch: %s: %w", strings.TrimSpace(string(output)), err)
	}
	logger.Debug("Branch created and checked out", map[string]interface{}{
		"branch": name,
		"output": strings.TrimSpace(string(output)),
	})
	return nil
}

// GetBranchFromDir returns the current branch name for the given directory.
// This works correctly inside git worktrees, because the command runs with
// cmd.Dir set to that worktree.
func GetBranchFromDir(dir string) (string, error) {
	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get current branch: %w", err)
	}

	branch := strings.TrimSpace(string(output))
	if branch == "" {
		return "", fmt.Errorf("HEAD is not a branch")
	}

	return branch, nil
}

// Push pushes the current branch to the remote repository
func (r *Repository) Push(remoteName string, branchName string) error {
	logger.Debug("Pushing branch", map[string]interface{}{
		"remote": remoteName,
		"branch": branchName,
	})

	// Use git command line with -u flag to set up tracking
	cmd := exec.Command("git", "push", "-u", remoteName, branchName)
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Push failed", err, map[string]interface{}{
			"output": string(output),
		})
		return fmt.Errorf("failed to push to remote: %w", err)
	}

	logger.Debug("Branch pushed with upstream tracking", map[string]interface{}{
		"remote": remoteName,
		"branch": branchName,
	})

	return nil
}

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

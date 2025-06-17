package git

import (
	"fmt"
	"os/exec"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/tedkulp/tix/internal/logger"
)

// Repository represents a Git repository
type Repository struct {
	*git.Repository
	path string
}

// Open opens a Git repository at the given path
func Open(path string) (*Repository, error) {
	repo, err := git.PlainOpen(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	return &Repository{
		Repository: repo,
		path:       path,
	}, nil
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

// CreateBranch creates a new branch from the current HEAD
func (r *Repository) CreateBranch(name string) error {
	head, err := r.Head()
	if err != nil {
		return fmt.Errorf("failed to get HEAD: %w", err)
	}

	ref := plumbing.NewBranchReferenceName(name)
	err = r.Storer.SetReference(plumbing.NewHashReference(ref, head.Hash()))
	if err != nil {
		return fmt.Errorf("failed to create branch: %w", err)
	}

	return nil
}

// CheckoutBranch checks out the specified branch
func (r *Repository) CheckoutBranch(name string) error {
	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	err = wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
	})
	if err != nil {
		return fmt.Errorf("failed to checkout branch: %w", err)
	}

	return nil
}

// GetCurrentBranch returns the name of the current branch
func (r *Repository) GetCurrentBranch() (string, error) {
	head, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("failed to get HEAD: %w", err)
	}

	// Check if head is a branch
	if head.Name().IsBranch() {
		return head.Name().Short(), nil
	}

	return "", fmt.Errorf("HEAD is not a branch")
}

// Push pushes the current branch to the remote repository
func (r *Repository) Push(remoteName string, branchName string) error {
	// Get remote details
	remote, err := r.Remote(remoteName)
	if err != nil {
		return fmt.Errorf("failed to get remote: %w", err)
	}

	// Get first URL from remote
	urls := remote.Config().URLs
	if len(urls) == 0 {
		return fmt.Errorf("no URLs found for remote %s", remoteName)
	}

	logger.Debug("Pushing branch", map[string]interface{}{
		"remote": remoteName,
		"branch": branchName,
		"url":    urls[0],
	})

	// Use git command line
	cmd := exec.Command("git", "push", remoteName, branchName)
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Push failed", err, map[string]interface{}{
			"output": string(output),
		})
		return fmt.Errorf("failed to push to remote: %w", err)
	}

	return nil
}

// DeleteBranch deletes a branch
func (r *Repository) DeleteBranch(name string) error {
	ref := plumbing.NewBranchReferenceName(name)
	err := r.Storer.RemoveReference(ref)
	if err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	return nil
}

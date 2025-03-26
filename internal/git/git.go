package git

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
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
	wt, err := r.Worktree()
	if err != nil {
		return false, fmt.Errorf("failed to get worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return false, fmt.Errorf("failed to get status: %w", err)
	}

	return status.IsClean(), nil
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

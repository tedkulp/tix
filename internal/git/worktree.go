package git

import (
	"fmt"
	"os"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// AddWorktree adds a new worktree to the repository
func (r *Repository) AddWorktree(name, path string) error {
	// Create the worktree directory
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("failed to create worktree directory: %w", err)
	}

	// Add the worktree
	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Create and checkout the new branch
	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.NewBranchReferenceName(name),
		Create: true,
	}); err != nil {
		return fmt.Errorf("failed to checkout new branch: %w", err)
	}

	return nil
}

// RemoveWorktree removes a worktree from the repository
func (r *Repository) RemoveWorktree(name string) error {
	wt, err := r.Worktree()
	if err != nil {
		return fmt.Errorf("failed to get worktree: %w", err)
	}

	// Switch back to main branch before removing
	if err := wt.Checkout(&git.CheckoutOptions{
		Branch: plumbing.Master,
	}); err != nil {
		return fmt.Errorf("failed to switch to main branch: %w", err)
	}

	// Delete the branch
	if err := r.DeleteBranch(name); err != nil {
		return fmt.Errorf("failed to delete branch: %w", err)
	}

	return nil
}

// ListWorktrees returns a list of worktrees in the repository
func (r *Repository) ListWorktrees() ([]string, error) {
	branches, err := r.Branches()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var names []string
	err = branches.ForEach(func(ref *plumbing.Reference) error {
		names = append(names, ref.Name().Short())
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to iterate branches: %w", err)
	}

	return names, nil
}

// GetWorktreePath returns the path of a worktree
func (r *Repository) GetWorktreePath(name string) (string, error) {
	// For go-git, the worktree path is the repository path
	// since it doesn't support multiple worktrees directly
	return r.path, nil
}

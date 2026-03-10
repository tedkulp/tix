package git

import (
	"fmt"
	"os/exec"
)

// AddWorktree creates a new git worktree at worktreePath with a new branch branchName,
// based on baseBranch (e.g. "main").
// Runs: git worktree add <worktreePath> -b <branchName> <baseBranch>
func (r *Repository) AddWorktree(worktreePath, branchName, baseBranch string) error {
	cmd := exec.Command("git", "worktree", "add", worktreePath, "-b", branchName, baseBranch)
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create worktree: %s: %w", string(output), err)
	}
	return nil
}

// RemoveWorktree removes the git worktree at worktreePath.
// Runs: git worktree remove <worktreePath>
func (r *Repository) RemoveWorktree(worktreePath string) error {
	cmd := exec.Command("git", "worktree", "remove", worktreePath)
	cmd.Dir = r.path
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to remove worktree: %s: %w", string(output), err)
	}
	return nil
}

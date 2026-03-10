package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Remove a git worktree",
	Long:  `Remove a git worktree directory. The branch is left intact.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
		}

		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
		}

		// Find the code repo whose worktree path contains cwd
		var codeRepo *config.Repository
		var detectedBranch string
		var worktreeBase string

		for i := range cfg.Repositories {
			repo := &cfg.Repositories[i]
			if !repo.IsCodeRepo() {
				continue
			}
			base := cfg.ResolveWorktreePath(repo)
			branch := detectWorktreeBranch(wd, base)
			if branch != "" {
				codeRepo = repo
				detectedBranch = branch
				worktreeBase = base
				break
			}
		}

		// If we couldn't detect from cwd, pick first code repo
		if codeRepo == nil {
			for i := range cfg.Repositories {
				if cfg.Repositories[i].IsCodeRepo() {
					codeRepo = &cfg.Repositories[i]
					worktreeBase = cfg.ResolveWorktreePath(codeRepo)
					break
				}
			}
		}

		if codeRepo == nil {
			return fmt.Errorf("no code repositories configured")
		}

		// Prompt with detected branch as default
		branchName, err := pterm.DefaultInteractiveTextInput.
			WithDefaultText("Worktree branch to remove").
			WithDefaultValue(detectedBranch).
			Show()
		if err != nil || strings.TrimSpace(branchName) == "" {
			return fmt.Errorf("cleanup cancelled")
		}
		branchName = strings.TrimSpace(branchName)

		worktreeDir := filepath.Join(worktreeBase, branchName)

		logger.Info("Removing worktree", map[string]interface{}{
			"branch":    branchName,
			"directory": worktreeDir,
		})

		gitRepo, err := git.Open(codeRepo.Directory)
		if err != nil {
			return fmt.Errorf("failed to open git repository: %w", err)
		}

		if err := gitRepo.RemoveWorktree(worktreeDir); err != nil {
			return fmt.Errorf("failed to remove worktree: %w", err)
		}

		fmt.Printf("Removed worktree: %s\n", worktreeDir)
		fmt.Printf("Project root: %s\n", codeRepo.Directory)

		return nil
	},
}

// detectWorktreeBranch returns the branch name if cwd is inside worktreePath,
// or empty string if not.
func detectWorktreeBranch(cwd, worktreePath string) string {
	// Normalize: ensure worktreePath ends with separator for prefix matching
	base := worktreePath
	if !strings.HasSuffix(base, string(filepath.Separator)) {
		base += string(filepath.Separator)
	}

	if !strings.HasPrefix(cwd, base) {
		return ""
	}

	rel := cwd[len(base):]
	if rel == "" {
		return ""
	}

	// First path segment is the branch name
	parts := strings.SplitN(rel, string(filepath.Separator), 2)
	if parts[0] == "" {
		return ""
	}
	return parts[0]
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
}

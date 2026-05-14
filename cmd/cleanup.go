package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
)

var cleanupForce bool

var cleanupCmd = &cobra.Command{
	Use:   "cleanup [branch]",
	Short: "Remove a git worktree",
	Long: `Remove a git worktree directory. The branch is left intact.

When --force is provided, skips the interactive prompt and removes
the worktree directly. If a branch name is given as an argument,
that branch's worktree is removed. Otherwise, the branch is auto-detected
from the current working directory.`,
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

		// If we couldn't detect from cwd, try to match by repo directory
		if codeRepo == nil {
			for i := range cfg.Repositories {
				repo := &cfg.Repositories[i]
				if !repo.IsCodeRepo() {
					continue
				}
				repoDir := repo.Directory
				if !strings.HasSuffix(repoDir, string(filepath.Separator)) {
					repoDir += string(filepath.Separator)
				}
				if wd == repo.Directory || strings.HasPrefix(wd, repoDir) {
					codeRepo = repo
					worktreeBase = cfg.ResolveWorktreePath(repo)
					break
				}
			}
		}

		// Last resort: pick first code repo
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

		// Determine the branch name
		var branchName string

		if cleanupForce {
			// Non-interactive: use arg or detected branch
			if len(args) > 0 {
				branchName = args[0]
			} else if detectedBranch != "" {
				branchName = detectedBranch
			} else {
				return fmt.Errorf("no branch specified and no worktree detected in current directory (use: tix cleanup --force <branch-name>)")
			}
		} else {
			// If not inside a worktree and no arg given, show list selector
			if detectedBranch == "" && len(args) == 0 {
				worktreeBranches, err := listWorktreeBranches(worktreeBase)
				if err != nil {
					return fmt.Errorf("failed to list worktrees: %w", err)
				}
				if len(worktreeBranches) == 0 {
					return fmt.Errorf("no worktrees found in %s", worktreeBase)
				}
				selected, err := pterm.DefaultInteractiveSelect.
					WithOptions(worktreeBranches).
					WithDefaultText("Select a worktree branch to remove").
					Show()
				if err != nil {
					return fmt.Errorf("cleanup cancelled")
				}
				detectedBranch = selected
			} else if len(args) > 0 {
				detectedBranch = args[0]
			}

			// Prompt with detected branch as default
			branchName, err = pterm.DefaultInteractiveTextInput.
				WithDefaultText("Worktree branch to remove").
				WithDefaultValue(detectedBranch).
				Show()
			if err != nil || strings.TrimSpace(branchName) == "" {
				return fmt.Errorf("cleanup cancelled")
			}
			branchName = strings.TrimSpace(branchName)
		}

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

// listWorktreeBranches returns the names of subdirectories (worktree branches)
// in the given worktree base path, sorted alphabetically. Hidden directories
// (starting with '.') are excluded.
func listWorktreeBranches(worktreeBase string) ([]string, error) {
	entries, err := os.ReadDir(worktreeBase)
	if err != nil {
		return nil, err
	}

	var branches []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			branches = append(branches, entry.Name())
		}
	}

	sort.Strings(branches)
	return branches, nil
}

func init() {
	rootCmd.AddCommand(cleanupCmd)
	cleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "Skip confirmation prompt and remove directly")
}

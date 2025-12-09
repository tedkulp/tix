package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/services"
	"github.com/tedkulp/tix/internal/utils"
)

var startCmd = &cobra.Command{
	Use:   "start [project] [issue-number]",
	Short: "Create a branch from an existing issue",
	Long: `Create a new branch in a code repository based on an existing issue.
The issue can be from the same repository or a different repository.

Usage:
  tix start                        # Interactive: prompt for project and issue number
  tix start 123                    # Create branch from issue #123 in current repo
  tix start project 123            # Create branch from issue #123 in 'project' repo

If the issue is from a different repo, the branch name will include the project prefix.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting start command")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
		}

		// Parse arguments
		var projectName string
		var issueNumber int

		if len(args) == 0 {
			// No arguments: prompt for both
			// Will be prompted below after determining default project
		} else if len(args) == 1 {
			// Single argument: assume it's the issue number
			issueNumber, err = strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("invalid issue number: %s", args[0])
			}
		} else if len(args) == 2 {
			// Two arguments: project name and issue number
			projectName = args[0]
			issueNumber, err = strconv.Atoi(args[1])
			if err != nil {
				return fmt.Errorf("invalid issue number: %s", args[1])
			}
		} else {
			return fmt.Errorf("too many arguments. Usage: tix start [project] [issue-number]")
		}

		// Get current directory to find code repo
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory")
		}

		// Find code repo (must have directory)
		var codeRepo *config.Repository
		var codeRepoName string
		var bestMatchLength int

		for i, repo := range cfg.Repositories {
			if !repo.IsCodeRepo() {
				continue
			}
			absRepoDir, err := filepath.Abs(repo.Directory)
			if err != nil {
				continue
			}
			if strings.HasPrefix(wd, absRepoDir) {
				if len(absRepoDir) > bestMatchLength {
					codeRepo = &cfg.Repositories[i]
					codeRepoName = cfg.GetRepoNames()[i]
					bestMatchLength = len(absRepoDir)
				}
			}
		}

		// If no matching code repo found, prompt for one
		if codeRepo == nil {
			codeRepoNames := []string{}
			for i, repo := range cfg.Repositories {
				if repo.IsCodeRepo() {
					codeRepoNames = append(codeRepoNames, cfg.GetRepoNames()[i])
				}
			}

			if len(codeRepoNames) == 0 {
				return fmt.Errorf("no code repositories configured (repos with 'directory' field)")
			}

			selectedName, err := pterm.DefaultInteractiveSelect.
				WithOptions(codeRepoNames).
				WithDefaultText("Select a code repository for the branch").
				Show()
			if err != nil {
				return fmt.Errorf("repository selection cancelled")
			}

			codeRepo = cfg.GetRepo(selectedName)
			codeRepoName = selectedName
		}

		logger.Info("Code repository selected", map[string]interface{}{
			"repo": codeRepoName,
		})

		// Prompt for project name if not provided
		if len(args) == 0 && projectName == "" {
			repoNames := cfg.GetRepoNames()
			if len(repoNames) == 0 {
				return fmt.Errorf("no repositories configured")
			}

			selectedName, err := pterm.DefaultInteractiveSelect.
				WithOptions(repoNames).
				WithDefaultText("Select a repository for the issue").
				WithDefaultOption(codeRepoName).
				Show()
			if err != nil {
				return fmt.Errorf("repository selection cancelled")
			}
			projectName = selectedName
		}

		// Prompt for issue number if not provided
		if len(args) == 0 && issueNumber == 0 {
			result, err := pterm.DefaultInteractiveTextInput.
				WithDefaultText("Enter issue number").
				Show()
			if err != nil {
				return fmt.Errorf("issue number input cancelled")
			}

			issueNumber, err = strconv.Atoi(strings.TrimSpace(result))
			if err != nil {
				return fmt.Errorf("invalid issue number: %s", result)
			}
		}

		// Determine issue repo
		var issueRepo *config.Repository
		var issueRepoName string

		if projectName == "" {
			// Use current code repo for issue
			issueRepo = codeRepo
			issueRepoName = codeRepoName
		} else {
			// Look up project by name
			issueRepo = cfg.GetRepo(projectName)
			if issueRepo == nil {
				return fmt.Errorf("repository '%s' not found in config", projectName)
			}
			issueRepoName = projectName
		}

		logger.Info("Issue repository selected", map[string]interface{}{
			"repo": issueRepoName,
		})

		// Validate providers match
		if (codeRepo.GithubRepo != "" && issueRepo.GitlabRepo != "") ||
			(codeRepo.GitlabRepo != "" && issueRepo.GithubRepo != "") {
			return fmt.Errorf("issue repo and code repo must use the same provider (both GitHub or both GitLab)")
		}

		// Create provider for issue repo
		var issueProvider services.SCMProvider
		if issueRepo.GithubRepo != "" {
			issueProvider, err = services.NewGitHubProvider(issueRepo.GithubRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitHub provider: %w", err)
			}
		} else {
			issueProvider, err = services.NewGitLabProvider(issueRepo.GitlabRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitLab provider: %w", err)
			}
		}

		// Fetch issue details
		issue, err := issueProvider.GetIssue(issueNumber)
		if err != nil {
			return fmt.Errorf("failed to get issue #%d from %s: %w", issueNumber, issueRepoName, err)
		}

		logger.Info("Issue fetched", map[string]interface{}{
			"number": issue.Number,
			"title":  issue.Title,
		})

		// Generate branch name
		var branchName string
		if projectName != "" && issueRepoName != codeRepoName {
			// Cross-repo: use project prefix
			branchName = fmt.Sprintf("%s-%d-%s", projectName, issueNumber, utils.TruncateAndDashCase(issue.Title, 50))
		} else {
			// Same repo: no prefix
			branchName = fmt.Sprintf("%d-%s", issueNumber, utils.TruncateAndDashCase(issue.Title, 50))
		}

		logger.Debug("Branch name created", map[string]interface{}{
			"branch": branchName,
		})

		// Open git repo and validate it's clean
		gitRepo, err := git.Open(codeRepo.Directory)
		if err != nil {
			return fmt.Errorf("failed to open git repository at %s: %w", codeRepo.Directory, err)
		}

		isClean, err := gitRepo.IsClean()
		if err != nil {
			return fmt.Errorf("failed to check repository status: %w", err)
		}
		if !isClean {
			return fmt.Errorf("repository is not clean - commit or stash changes first")
		}

		// Create and checkout branch
		if codeRepo.Worktree.Enabled {
			worktreeDir := filepath.Join(codeRepo.Directory, branchName)
			logger.Info("Creating worktree", map[string]interface{}{
				"branch":    branchName,
				"directory": worktreeDir,
			})

			if err := gitRepo.AddWorktree(branchName, worktreeDir); err != nil {
				return fmt.Errorf("failed to create worktree: %w", err)
			}

			fmt.Printf("Created worktree: %s in %s\n", branchName, worktreeDir)
		} else {
			logger.Info("Creating and checking out branch", map[string]interface{}{
				"branch": branchName,
			})

			if err := gitRepo.CreateBranch(branchName); err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}
			if err := gitRepo.CheckoutBranch(branchName); err != nil {
				return fmt.Errorf("failed to checkout branch: %w", err)
			}

			fmt.Printf("Created and checked out branch: %s\n", branchName)
		}

		// Show issue URL
		issueURL := fmt.Sprintf("%s/issues/%d", issueProvider.GetURL(), issueNumber)
		fmt.Printf("Issue: %s\n", issueURL)

		logger.Debug("Start command completed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(startCmd)
}

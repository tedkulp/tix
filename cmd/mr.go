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
	"github.com/tedkulp/tix/internal/services"
	"github.com/tedkulp/tix/internal/utils"
)

var mrCmd = &cobra.Command{
	Use:     "mr",
	Aliases: []string{"pr"},
	Short:   "Create a merge request",
	Long: `Create a merge request in GitHub or GitLab for the current branch.
It will extract the issue number from the branch name and create a merge request.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting mr command")

		// Get the draft flag value
		isDraft, _ := cmd.Flags().GetBool("draft")
		// Get the remote flag value
		remote, _ := cmd.Flags().GetString("remote")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
		}

		logger.Debug("Config loaded successfully", map[string]interface{}{
			"repos_count": len(cfg.GetRepoNames()),
		})

		// Try to find the repository based on current directory
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine current directory")
		}

		logger.Debug("Current directory", map[string]interface{}{
			"directory": wd,
		})

		// Find repo that matches the current directory (only code repos)
		var matchingRepo *config.Repository
		var repoName string
		var bestMatchLength = 0 // Track the best match length

		for i, repo := range cfg.Repositories {
			if !repo.IsCodeRepo() {
				continue
			}
			absRepoDir, err := filepath.Abs(repo.Directory)
			if err != nil {
				continue
			}

			// Check if current directory is within the repo directory
			if strings.HasPrefix(wd, absRepoDir) {
				// If we found a better match (longer path), use it
				if len(absRepoDir) > bestMatchLength {
					matchingRepo = &cfg.Repositories[i]
					repoName = cfg.GetRepoNames()[i]
					bestMatchLength = len(absRepoDir)
				}
			}
		}

		// If no matching repo found, show selector (only code repos)
		if matchingRepo == nil {
			codeRepoNames := []string{}
			for i, repo := range cfg.Repositories {
				if repo.IsCodeRepo() {
					codeRepoNames = append(codeRepoNames, cfg.GetRepoNames()[i])
				}
			}

			if len(codeRepoNames) == 0 {
				return fmt.Errorf("no code repositories configured - add repositories with 'directory' field to your config file")
			}

			// Use pterm's interactive select component
			selectedName, err := pterm.DefaultInteractiveSelect.
				WithOptions(codeRepoNames).
				WithDefaultText("Select a code repository").
				WithDefaultOption(repoName).
				Show()

			if err != nil {
				return fmt.Errorf("repository selection cancelled")
			}

			matchingRepo = cfg.GetRepo(selectedName)
			repoName = selectedName
		}

		logger.Info("Repository selected", map[string]interface{}{
			"repo": repoName,
		})

		// Open Git repository
		gitRepo, err := git.Open(matchingRepo.Directory)
		if err != nil {
			return fmt.Errorf("couldn't open git repository at %s", matchingRepo.Directory)
		}

		// Get current branch
		currentBranch, err := gitRepo.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to determine current git branch")
		}

		logger.Info("Current branch", map[string]interface{}{
			"branch": currentBranch,
		})

		// Extract issue info from branch name (may include project name for cross-repo)
		projectName, issueNumber, err := utils.ExtractIssueInfo(currentBranch)
		if err != nil {
			return fmt.Errorf("couldn't extract issue number from branch name '%s' - use a branch like '123-my-feature' or 'project-123-my-feature'", currentBranch)
		}

		logger.Info("Issue info extracted", map[string]interface{}{
			"project": projectName,
			"issue":   issueNumber,
		})

		// Get target branch
		targetBranch := "main"
		if matchingRepo.DefaultBranch != "" {
			targetBranch = matchingRepo.DefaultBranch
		}

		// Create SCM provider for MR repo (where the code is)
		var provider services.SCMProvider

		if matchingRepo.GitlabRepo != "" {
			provider, err = services.NewGitLabProvider(matchingRepo.GitlabRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitLab provider - check your GITLAB_TOKEN environment variable")
			}
		} else {
			provider, err = services.NewGitHubProvider(matchingRepo.GithubRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitHub provider - check your GITHUB_TOKEN environment variable")
			}
		}

		// Handle cross-repo scenario
		var issueProvider services.SCMProvider
		var crossRepoIssueRef string

		if projectName != "" && projectName != repoName {
			// Cross-repo: look up issue repo
			issueRepo := cfg.GetRepo(projectName)
			if issueRepo == nil {
				return fmt.Errorf("repository '%s' not found in config", projectName)
			}

			// Validate providers match
			if (matchingRepo.GithubRepo != "" && issueRepo.GitlabRepo != "") ||
				(matchingRepo.GitlabRepo != "" && issueRepo.GithubRepo != "") {
				return fmt.Errorf("issue repo '%s' and code repo '%s' must use the same provider", projectName, repoName)
			}

			// Create provider for issue repo
			if issueRepo.GitlabRepo != "" {
				issueProvider, err = services.NewGitLabProvider(issueRepo.GitlabRepo)
				if err != nil {
					return fmt.Errorf("failed to create GitLab provider for issue repo: %w", err)
				}
			} else {
				issueProvider, err = services.NewGitHubProvider(issueRepo.GithubRepo)
				if err != nil {
					return fmt.Errorf("failed to create GitHub provider for issue repo: %w", err)
				}
			}

			// Generate cross-repo reference
			crossRepoIssueRef = issueProvider.GetCrossRepoIssueRef(issueNumber)
		} else {
			// Same repo: use same provider
			issueProvider = nil
		}

		// Create merge request using common function
		request, err := services.CreateMergeRequest(
			services.CreateMergeRequestParams{
				Provider:           provider,
				GitRepo:            gitRepo,
				CurrentBranch:      currentBranch,
				Remote:             remote,
				TargetBranch:       targetBranch,
				IssueNumber:        issueNumber,
				IsDraft:            isDraft,
				RemoveSourceBranch: true, // always true for now
				Squash:             true, // always true for now
				IssueProvider:      issueProvider,
				CrossRepoIssueRef:  crossRepoIssueRef,
			},
		)

		if err != nil {
			// Check for the custom error messages we created for existing merge requests
			if strings.Contains(err.Error(), "already exists for this branch") &&
				(strings.Contains(err.Error(), "View existing merge request") ||
					strings.Contains(err.Error(), "View existing pull request") ||
					strings.Contains(err.Error(), "View your pull requests")) {
				// This is our custom formatted error - just print it
				fmt.Println(err.Error())
				return nil
			}

			// Common push errors
			if strings.Contains(err.Error(), "failed to push to") {
				return fmt.Errorf("failed to push branch '%s' to remote '%s' - check network or permissions", currentBranch, remote)
			}

			// Common issue errors
			if strings.Contains(err.Error(), "failed to get issue details") {
				return fmt.Errorf("issue #%d not found - check if it exists", issueNumber)
			}

			// Otherwise, it's another error
			return fmt.Errorf("failed to create request: %w", err)
		}

		fmt.Printf("Created request: %s\n", request.URL)

		logger.Debug("MR command completed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mrCmd)
	mrCmd.Flags().BoolP("draft", "d", false, "Create the merge request as a draft")
	mrCmd.Flags().StringP("remote", "r", "origin", "Git remote to push to")
}

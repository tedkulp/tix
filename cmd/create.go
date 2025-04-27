package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/services"
	"github.com/tedkulp/tix/internal/utils"
)

var (
	title      string
	selfAssign bool
)

// RepoSettings represents repository settings and configuration
type RepoSettings struct {
	Repo          *config.Repository
	Name          string
	Directory     string
	Labels        string
	Milestone     string
	IssueProvider services.IssueProvider
}

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new ticket and branch",
	Long: `Create a new ticket in GitHub or GitLab and create a corresponding branch.
If no title is provided, you will be prompted for one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting create command")

		// Setup repository and configuration
		repoSettings, err := setupRepository()
		if err != nil {
			// Provide cleaner error messages for common setup issues
			if strings.Contains(err.Error(), "failed to load config") {
				return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
			}
			if strings.Contains(err.Error(), "no repositories configured") {
				return fmt.Errorf("no repositories configured - add repositories to your config file")
			}
			return err
		}

		// Open Git repository
		gitRepo, err := openAndValidateRepo(repoSettings.Directory)
		if err != nil {
			// Handle common git repository errors
			if strings.Contains(err.Error(), "repository is not clean") {
				return fmt.Errorf("git repository has uncommitted changes - commit or stash them first")
			}
			if strings.Contains(err.Error(), "failed to open repository") {
				return fmt.Errorf("couldn't open git repository at %s", repoSettings.Directory)
			}
			return err
		}

		// Prompt for and validate title if not provided
		if title == "" {
			title, err = promptForTitle()
			if err != nil {
				return fmt.Errorf("issue creation cancelled")
			}
		}

		logger.Info("Issue title set", map[string]interface{}{
			"title": title,
		})

		// Get labels
		repoSettings.Labels, err = promptForLabels(repoSettings.Repo.DefaultLabels)
		if err != nil {
			return fmt.Errorf("issue creation cancelled")
		}

		// Get milestone if needed
		if repoSettings.Repo.GitlabRepo != "" {
			repoSettings.Milestone, err = promptForMilestone()
			if err != nil {
				return fmt.Errorf("issue creation cancelled")
			}
		}

		// Create issue using the provider
		issueResult, err := createIssue(repoSettings)
		if err != nil {
			if strings.Contains(err.Error(), "failed to create GitHub") ||
				strings.Contains(err.Error(), "failed to create GitLab") {
				return fmt.Errorf("failed to create issue - check your API token and permissions")
			}
			return err
		}

		// Create and checkout branch
		if err := createBranch(gitRepo, repoSettings.Repo, issueResult.Number, issueResult.Title); err != nil {
			if strings.Contains(err.Error(), "failed to create branch") {
				return fmt.Errorf("branch creation failed - the issue was created but the branch couldn't be created")
			}
			if strings.Contains(err.Error(), "failed to checkout branch") {
				return fmt.Errorf("branch creation succeeded but checkout failed")
			}
			if strings.Contains(err.Error(), "failed to create worktree") {
				return fmt.Errorf("worktree creation failed - check directory permissions")
			}
			return err
		}

		logger.Debug("Create command completed successfully")
		return nil
	},
}

// setupRepository handles repository selection and configuration
func setupRepository() (*RepoSettings, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logger.Debug("Config loaded successfully", map[string]interface{}{
		"repos_count": len(cfg.GetRepoNames()),
	})

	// Get current directory to find the best matching repository
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	logger.Debug("Current directory", map[string]interface{}{
		"directory": wd,
	})

	// Find repo that matches the current directory, or the best candidate
	var matchingRepo *config.Repository
	var repoName string
	var matchingRepoIdx int = -1 // Index of matching repo in the list
	var bestMatchLength int = 0  // Length of the best match so far

	repoNames := cfg.GetRepoNames()
	for i, repo := range cfg.Repositories {
		absRepoDir, err := filepath.Abs(repo.Directory)
		if err != nil {
			continue
		}

		// Check if current directory is within the repo directory
		if strings.HasPrefix(wd, absRepoDir) {
			// If we found an exact match, use it
			if len(absRepoDir) > bestMatchLength {
				matchingRepo = &cfg.Repositories[i]
				repoName = repoNames[i]
				matchingRepoIdx = i
				bestMatchLength = len(absRepoDir)
			}
		}
	}

	// If we found a match, we'll offer it as the default option
	if matchingRepo != nil {
		logger.Info("Found matching repository", map[string]interface{}{
			"repo":      repoName,
			"directory": matchingRepo.Directory,
		})
	}

	// Show repository selector with the matching repo as default if found
	var selectedRepo *config.Repository
	var selectedRepoName string

	if len(repoNames) > 1 { // Only show the selector if there's more than one repo
		prompt := promptui.Select{
			Label: "Select a repository",
			Items: repoNames,
		}

		// If we found a matching repo, start with that as the cursor position
		if matchingRepoIdx >= 0 {
			prompt.CursorPos = matchingRepoIdx
			prompt.Label = fmt.Sprintf("Select a repository (default: %s)", repoName)
		}

		selectedIdx, selectedName, err := prompt.Run()
		if err != nil {
			return nil, fmt.Errorf("failed to select repository: %w", err)
		}

		selectedRepo = &cfg.Repositories[selectedIdx]
		selectedRepoName = selectedName
	} else if len(repoNames) == 1 {
		// If only one repo exists, use it
		selectedRepo = &cfg.Repositories[0]
		selectedRepoName = repoNames[0]
		logger.Info("Only one repository available, using it", map[string]interface{}{
			"repo": selectedRepoName,
		})
	} else {
		return nil, fmt.Errorf("no repositories configured")
	}

	logger.Info("Repository selected", map[string]interface{}{
		"repo": selectedRepoName,
	})

	// Validate repository configuration
	if (selectedRepo.GithubRepo == "" && selectedRepo.GitlabRepo == "") ||
		(selectedRepo.GithubRepo != "" && selectedRepo.GitlabRepo != "") {
		logger.Error("Invalid repository configuration", nil, map[string]interface{}{
			"repo":        selectedRepoName,
			"github_repo": selectedRepo.GithubRepo,
			"gitlab_repo": selectedRepo.GitlabRepo,
		})
		return nil, fmt.Errorf("repository must have exactly one of github_repo or gitlab_repo... %+v", selectedRepo)
	}

	// Create appropriate issue provider
	var issueProvider services.IssueProvider
	if selectedRepo.GithubRepo != "" {
		provider, err := services.NewGitHubIssueProvider(selectedRepo.GithubRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub provider: %w", err)
		}
		issueProvider = provider
	} else {
		provider, err := services.NewGitLabIssueProvider(selectedRepo.GitlabRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitLab provider: %w", err)
		}
		issueProvider = provider
	}

	return &RepoSettings{
		Repo:          selectedRepo,
		Name:          selectedRepoName,
		Directory:     selectedRepo.Directory,
		IssueProvider: issueProvider,
	}, nil
}

// openAndValidateRepo opens the Git repository and validates it's clean
func openAndValidateRepo(directory string) (*git.Repository, error) {
	logger.Debug("Opening git repository", map[string]interface{}{
		"directory": directory,
	})

	// Open Git repository
	gitRepo, err := git.Open(directory)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Check if repository is clean
	isClean, err := gitRepo.IsClean()
	if err != nil {
		return nil, fmt.Errorf("failed to check repository status: %w", err)
	}
	if !isClean {
		return nil, fmt.Errorf("repository is not clean")
	}

	logger.Debug("Repository is clean")
	return gitRepo, nil
}

// promptForTitle prompts for and validates the issue title
func promptForTitle() (string, error) {
	prompt := promptui.Prompt{
		Label: "Title of issue",
		Validate: func(input string) error {
			if len(input) > 255 {
				return fmt.Errorf("title must be less than 255 characters")
			}
			return nil
		},
	}
	return prompt.Run()
}

// promptForLabels prompts for issue labels
func promptForLabels(defaultLabels string) (string, error) {
	labelPrompt := promptui.Prompt{
		Label:   "Labels (comma separated)",
		Default: defaultLabels,
	}
	labels, err := labelPrompt.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get labels: %w", err)
	}

	logger.Info("Labels set", map[string]interface{}{
		"labels": labels,
	})
	return labels, nil
}

// promptForMilestone prompts for milestone (GitLab specific)
func promptForMilestone() (string, error) {
	// Calculate default milestone based on current date
	defaultMilestone := utils.GenerateMilestone(time.Now())

	milestonePrompt := promptui.Prompt{
		Label:   "Milestone",
		Default: defaultMilestone,
	}
	milestone, err := milestonePrompt.Run()
	if err != nil {
		return "", fmt.Errorf("failed to get milestone: %w", err)
	}

	logger.Info("Milestone set", map[string]interface{}{
		"milestone": milestone,
	})
	return milestone, nil
}

// createIssue creates a new issue using the provider
func createIssue(settings *RepoSettings) (*services.IssueCreationResult, error) {
	logger.Info("Creating issue", map[string]interface{}{
		"repo":        settings.Name,
		"self_assign": selfAssign,
		"milestone":   settings.Milestone,
	})

	issueResult, err := settings.IssueProvider.CreateIssue(
		title,
		settings.Labels,
		selfAssign,
		settings.Milestone,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	logger.Info("Issue created", map[string]interface{}{
		"number": issueResult.Number,
		"title":  issueResult.Title,
		"url":    issueResult.URL,
	})

	return issueResult, nil
}

// createBranch creates and checks out a new branch
func createBranch(gitRepo *git.Repository, repo *config.Repository, issueNumber int, issueTitle string) error {
	// Create branch name
	branchName := fmt.Sprintf("%d-%s", issueNumber, utils.TruncateAndDashCase(issueTitle, 50))
	logger.Debug("Branch name created", map[string]interface{}{
		"branch": branchName,
	})

	// Create and checkout branch
	if repo.Worktree.Enabled {
		// Get the worktree directory
		worktreeDir := filepath.Join(repo.Directory, branchName)
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
			logger.Error("Failed to create branch", err)
			return fmt.Errorf("failed to create branch: %w", err)
		}
		if err := gitRepo.CheckoutBranch(branchName); err != nil {
			logger.Error("Failed to checkout branch", err)
			return fmt.Errorf("failed to checkout branch: %w", err)
		}

		fmt.Printf("Created and checked out branch: %s\n", branchName)
	}

	return nil
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&title, "title", "t", "", "Title of the issue")
	createCmd.Flags().BoolVarP(&selfAssign, "assign", "a", true, "Assign the issue to yourself")
}

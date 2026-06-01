package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/services"
	"github.com/tedkulp/tix/internal/utils"
)

var (
	title       string
	selfAssign  bool
	useWorktree bool
	noAutoStash bool
)

// RepoSettings represents repository settings and configuration
type RepoSettings struct {
	Repo         *config.Repository
	Name         string
	Directory    string
	Labels       string
	Milestone    string
	Provider     services.SCMProvider
	CodeRepo     *config.Repository // For cross-repo: code repo where branch is created
	CodeRepoName string             // Name of code repo
}

var createCmd = &cobra.Command{
	Use:   "create [issue-repo] [code-repo]",
	Short: "Create a new ticket and branch",
	Long: `Create a new ticket in GitHub or GitLab and create a corresponding branch.
If no title is provided, you will be prompted for one.

Usage:
  tix create                       # Interactive: select issue and code repos
  tix create issues                # Create issue in issues repo, branch in current/matching repo
  tix create issues code           # Create issue in issues repo, branch in code`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting create command")

		// Parse arguments for issue repo and code repo
		var issueRepoArg, codeRepoArg string
		if len(args) >= 1 {
			issueRepoArg = args[0]
		}
		if len(args) >= 2 {
			codeRepoArg = args[1]
		}
		if len(args) > 2 {
			return fmt.Errorf("too many arguments. Usage: tix create [issue-repo] [code-repo]")
		}

		cfg, err := config.Load()
		if err != nil {
			if strings.Contains(err.Error(), "failed to load config") || strings.Contains(err.Error(), "failed to read config") {
				return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
			}
			return err
		}

		// Setup repository and configuration
		repoSettings, err := setupRepository(cfg, issueRepoArg, codeRepoArg)
		if err != nil {
			return err
		}

		// Open Git repository BEFORE any user interaction
		gitRepo, err := git.Open(repoSettings.Directory)
		if err != nil {
			return fmt.Errorf("couldn't open git repository at %s", repoSettings.Directory)
		}

		if !useWorktree {
			isClean, err := gitRepo.IsClean()
			if err != nil {
				return fmt.Errorf("failed to check repository status: %w", err)
			}
			if !isClean {
				if noAutoStash {
					return fmt.Errorf("git repository has uncommitted changes - commit or stash them first")
				}
				if err := gitRepo.Stash(); err != nil {
					return fmt.Errorf("failed to stash changes: %w", err)
				}
				fmt.Println("Stashed changes, will restore after branch creation.")
				defer func() {
					if popErr := gitRepo.StashPop(); popErr != nil {
						fmt.Fprintf(os.Stderr, "Warning: failed to restore stashed changes: %v\n", popErr)
						fmt.Fprintf(os.Stderr, "Your changes are still in the stash — run `git stash pop` manually.\n")
					}
				}()
			}
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
		// Use project prefix if issue repo differs from code repo
		projectPrefix := ""
		if repoSettings.Name != repoSettings.CodeRepoName {
			projectPrefix = repoSettings.Name
		}
		if err := createBranch(gitRepo, repoSettings.CodeRepo, cfg, issueResult.Number, issueResult.Title, projectPrefix, useWorktree); err != nil {
			if strings.Contains(err.Error(), "failed to create branch") {
				return fmt.Errorf("branch creation failed - the issue was created but the branch couldn't be created")
			}
			if strings.Contains(err.Error(), "failed to checkout branch") {
				return fmt.Errorf("branch creation succeeded but checkout failed")
			}
			return err
		}

		logger.Debug("Create command completed successfully")
		return nil
	},
}

// setupRepository handles repository selection and configuration
func setupRepository(cfg *config.Settings, issueRepoArg, codeRepoArg string) (*RepoSettings, error) {
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
	// Only consider repos with directories (code repos)
	var matchingRepo *config.Repository
	var repoName string
	var bestMatchLength = 0 // Length of the best match so far

	repoNames := cfg.GetRepoNames()
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
			// If we found an exact match, use it
			if len(absRepoDir) > bestMatchLength {
				matchingRepo = &cfg.Repositories[i]
				repoName = repoNames[i]
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
	// Allow selecting any repo (including issue-only repos)
	var selectedRepo *config.Repository
	var selectedRepoName string

	if issueRepoArg != "" {
		// Issue repo specified as argument
		selectedRepo = cfg.GetRepo(issueRepoArg)
		if selectedRepo == nil {
			return nil, fmt.Errorf("repository '%s' not found in config", issueRepoArg)
		}
		selectedRepoName = issueRepoArg
		logger.Info("Using issue repository from argument", map[string]interface{}{
			"repo": selectedRepoName,
		})
	} else if len(repoNames) > 1 {
		// Multiple repos: show selector with matching repo as default
		selectedName, err := pterm.DefaultInteractiveSelect.
			WithOptions(repoNames).
			WithDefaultText("Select a repository for the issue").
			WithDefaultOption(repoName).
			Show()

		if err != nil {
			return nil, fmt.Errorf("failed to select repository: %w", err)
		}

		selectedRepo = cfg.GetRepo(selectedName)
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

	// Create appropriate provider
	var provider services.SCMProvider
	if selectedRepo.GithubRepo != "" {
		githubProvider, err := services.NewGitHubProvider(selectedRepo.GithubRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub provider: %w", err)
		}
		provider = githubProvider
	} else {
		gitlabProvider, err := services.NewGitLabProvider(selectedRepo.GitlabRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitLab provider: %w", err)
		}
		provider = gitlabProvider
	}

	// Determine code repo (where branch will be created)
	var codeRepo *config.Repository
	var codeRepoName string

	if codeRepoArg != "" {
		// Code repo specified as argument
		codeRepo = cfg.GetRepo(codeRepoArg)
		if codeRepo == nil {
			return nil, fmt.Errorf("repository '%s' not found in config", codeRepoArg)
		}
		if !codeRepo.IsCodeRepo() {
			return nil, fmt.Errorf("repository '%s' is not a code repository (missing 'directory' field)", codeRepoArg)
		}
		codeRepoName = codeRepoArg
		logger.Info("Using code repository from argument", map[string]interface{}{
			"repo": codeRepoName,
		})

		// Validate providers match
		if (selectedRepo.GithubRepo != "" && codeRepo.GitlabRepo != "") ||
			(selectedRepo.GitlabRepo != "" && codeRepo.GithubRepo != "") {
			return nil, fmt.Errorf("issue repo '%s' and code repo '%s' must use the same provider", selectedRepoName, codeRepoName)
		}
	} else if selectedRepo.IsCodeRepo() {
		// Selected repo has directory, use it
		codeRepo = selectedRepo
		codeRepoName = selectedRepoName
	} else {
		// Selected repo is issue-only, prompt for code repo
		codeRepoNames := []string{}
		for i, repo := range cfg.Repositories {
			if repo.IsCodeRepo() {
				codeRepoNames = append(codeRepoNames, cfg.GetRepoNames()[i])
			}
		}

		if len(codeRepoNames) == 0 {
			return nil, fmt.Errorf("no code repositories configured (repos with 'directory' field)")
		}

		selectedCodeName, err := pterm.DefaultInteractiveSelect.
			WithOptions(codeRepoNames).
			WithDefaultText("Select a code repository for the branch").
			WithDefaultOption(repoName).
			Show()
		if err != nil {
			return nil, fmt.Errorf("code repository selection cancelled")
		}

		codeRepo = cfg.GetRepo(selectedCodeName)
		codeRepoName = selectedCodeName

		// Validate providers match
		if (selectedRepo.GithubRepo != "" && codeRepo.GitlabRepo != "") ||
			(selectedRepo.GitlabRepo != "" && codeRepo.GithubRepo != "") {
			return nil, fmt.Errorf("issue repo '%s' and code repo '%s' must use the same provider", selectedRepoName, codeRepoName)
		}
	}

	return &RepoSettings{
		Repo:         selectedRepo,
		Name:         selectedRepoName,
		Directory:    codeRepo.Directory,
		Provider:     provider,
		CodeRepo:     codeRepo,
		CodeRepoName: codeRepoName,
	}, nil
}

// promptForTitle prompts the user for a title for the issue
func promptForTitle() (string, error) {
	result, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText("Enter issue title").
		Show()

	if err != nil {
		return "", err
	}

	if strings.TrimSpace(result) == "" {
		return "", fmt.Errorf("title cannot be empty")
	}

	return result, nil
}

// promptForLabels prompts the user for labels for the issue
func promptForLabels(defaultLabels string) (string, error) {
	result, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText("Enter labels (comma separated)").
		WithDefaultValue(defaultLabels).
		Show()

	if err != nil {
		return "", err
	}

	return result, nil
}

// promptForMilestone prompts the user for a milestone for the issue
func promptForMilestone() (string, error) {
	defaultMilestone := utils.GenerateMilestone(time.Now())

	result, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText("Enter milestone").
		WithDefaultValue(defaultMilestone).
		Show()

	if err != nil {
		return "", err
	}

	return result, nil
}

// createIssue creates a new issue using the provider
func createIssue(settings *RepoSettings) (*services.IssueResult, error) {
	logger.Info("Creating issue", map[string]interface{}{
		"repo":        settings.Name,
		"self_assign": selfAssign,
		"milestone":   settings.Milestone,
	})

	params := services.IssueParams{
		Title:          title,
		Labels:         settings.Labels,
		SelfAssign:     selfAssign,
		MilestoneTitle: settings.Milestone,
	}

	issueResult, err := settings.Provider.CreateIssue(params)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	logger.Info("Issue created", map[string]interface{}{
		"number": issueResult.Number,
		"title":  issueResult.Title,
	})

	// Get issue URL from the provider
	var issueURL string
	if _, err := settings.Provider.GetIssue(issueResult.Number); err == nil {
		issueURL = fmt.Sprintf("%s/issues/%d", settings.Provider.GetURL(), issueResult.Number)
		logger.Info("Issue URL", map[string]interface{}{
			"url": issueURL,
		})
	}

	// Show URL in terminal
	if issueURL != "" {
		fmt.Printf("Created issue: %s\n", issueURL)
	} else {
		fmt.Printf("Created issue #%d: %s\n", issueResult.Number, issueResult.Title)
	}

	return issueResult, nil
}

// createBranch creates and checks out a new branch
func createBranch(gitRepo *git.Repository, repo *config.Repository, cfg *config.Settings, issueNumber int, issueTitle string, projectPrefix string, useWorktree bool) error {
	// Create branch name
	var branchName string
	if projectPrefix != "" {
		branchName = fmt.Sprintf("%s-%d-%s", projectPrefix, issueNumber, utils.TruncateAndDashCase(issueTitle, 50))
	} else {
		branchName = fmt.Sprintf("%d-%s", issueNumber, utils.TruncateAndDashCase(issueTitle, 50))
	}
	logger.Debug("Branch name created", map[string]interface{}{
		"branch": branchName,
	})

	if useWorktree {
		worktreeBase := cfg.ResolveWorktreePath(repo)
		worktreeDir := filepath.Join(worktreeBase, branchName)
		logger.Info("Creating worktree", map[string]interface{}{
			"branch":    branchName,
			"directory": worktreeDir,
		})

		defaultBranch := cfg.ResolveDefaultBranch(repo)
		if err := gitRepo.AddWorktree(worktreeDir, branchName, defaultBranch); err != nil {
			return fmt.Errorf("failed to create worktree: %w", err)
		}

		fmt.Printf("Created worktree: %s\n", worktreeDir)
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
	createCmd.Flags().BoolVarP(&useWorktree, "worktree", "w", false, "Create a git worktree instead of checking out a branch")
	createCmd.Flags().BoolVar(&noAutoStash, "no-auto-stash", false, "Disable automatic stashing of uncommitted changes before branch creation")
}

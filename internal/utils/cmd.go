package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pterm/pterm"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/services"
)

// SharedRepoInfo holds information about a selected repository and related data
type SharedRepoInfo struct {
	Repo        *config.Repository
	Name        string
	IsGitLab    bool
	CurrentDir  string
	IssueNumber int
	Branch      string
}

// SelectSharedRepository determines which repository to work with using shared logic
func SelectSharedRepository() (*SharedRepoInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logger.Debug("Config loaded successfully", map[string]interface{}{
		"repos_count": len(cfg.GetRepoNames()),
	})

	// Get current directory
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
	var bestMatchLength int = 0

	repoNames := cfg.GetRepoNames()
	for i, repo := range cfg.Repositories {
		absRepoDir, err := filepath.Abs(repo.Directory)
		if err != nil {
			continue
		}

		// Check if current directory is within the repo directory
		if strings.HasPrefix(wd, absRepoDir) {
			// If we found a better match (longer path prefix), use it
			if len(absRepoDir) > bestMatchLength {
				matchingRepo = &cfg.Repositories[i]
				repoName = repo.Name
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

	if matchingRepo != nil {
		// Use the matching repository directly
		selectedRepo = matchingRepo
		selectedRepoName = repoName
	} else {
		// No match found, show selector
		if len(repoNames) == 0 {
			return nil, fmt.Errorf("no repositories configured")
		}

		// Use pterm's interactive select component
		selected, err := pterm.DefaultInteractiveSelect.
			WithOptions(repoNames).
			WithDefaultText("Select a repository").
			Show()

		if err != nil {
			return nil, fmt.Errorf("repository selection cancelled")
		}

		// Find the selected repository
		selectedRepo = cfg.GetRepo(selected)
		if selectedRepo == nil {
			return nil, fmt.Errorf("selected repository not found")
		}
		selectedRepoName = selected
	}

	logger.Info("Repository selected", map[string]interface{}{
		"repo": selectedRepoName,
	})

	// Open Git repository
	gitRepo, err := git.Open(selectedRepo.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to open git repository at %s", selectedRepo.Directory)
	}

	// Get current branch
	currentBranch, err := gitRepo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch")
	}

	logger.Info("Current branch", map[string]interface{}{
		"branch": currentBranch,
	})

	// Extract issue number from branch name
	issueNumber, err := ExtractIssueNumber(currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to extract issue number from branch '%s': %w", currentBranch, err)
	}

	logger.Info("Issue number extracted", map[string]interface{}{
		"issue": issueNumber,
	})

	// Determine if this is GitLab or GitHub
	isGitLab := selectedRepo.GitlabRepo != ""

	return &SharedRepoInfo{
		Repo:        selectedRepo,
		Name:        selectedRepoName,
		IsGitLab:    isGitLab,
		CurrentDir:  wd,
		IssueNumber: issueNumber,
		Branch:      currentBranch,
	}, nil
}

// CreateSCMProvider creates the appropriate SCM provider for the repository
func CreateSCMProvider(repoInfo *SharedRepoInfo) (services.SCMProvider, error) {
	var provider services.SCMProvider
	var err error

	if repoInfo.IsGitLab {
		provider, err = services.NewGitLabProvider(repoInfo.Repo.GitlabRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitLab provider - check your GITLAB_TOKEN environment variable: %w", err)
		}
	} else {
		provider, err = services.NewGitHubProvider(repoInfo.Repo.GithubRepo)
		if err != nil {
			return nil, fmt.Errorf("failed to create GitHub provider - check your GITHUB_TOKEN environment variable: %w", err)
		}
	}

	return provider, nil
}

// GetReadyLabel returns the appropriate ready label for the repository
func GetReadyLabel(cfg *config.Settings, repo *config.Repository, overrideLabel string) string {
	// Use override label if provided
	if overrideLabel != "" {
		return overrideLabel
	}

	// Use repository-specific ready label if set
	if repo.ReadyLabel != "" {
		return repo.ReadyLabel
	}

	// Use global default ready label if set
	if cfg.ReadyLabel != "" {
		return cfg.ReadyLabel
	}

	// Fallback to "ready"
	return "ready"
}

// LabelOperation represents the type of label operation
type LabelOperation int

const (
	AddLabel LabelOperation = iota
	RemoveLabel
)

// HandleLabelOperation handles adding or removing labels from an issue
func HandleLabelOperation(operation LabelOperation, overrideLabel string) error {
	logger.Debug("Starting label operation", map[string]interface{}{
		"operation": operation,
	})

	// Find and select repository
	repoInfo, err := SelectSharedRepository()
	if err != nil {
		// Provide cleaner error messages for common setup issues
		if strings.Contains(err.Error(), "failed to load config") {
			return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
		}
		if strings.Contains(err.Error(), "no repositories configured") {
			return fmt.Errorf("no repositories configured - add repositories to your config file")
		}
		if strings.Contains(err.Error(), "failed to extract issue number") {
			return fmt.Errorf("couldn't extract issue number from branch name - are you on a feature branch?")
		}
		return err
	}

	// Load config to get ready label configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Get the ready label to use
	labelToUse := GetReadyLabel(cfg, repoInfo.Repo, overrideLabel)

	var operationName string
	if operation == AddLabel {
		operationName = "Adding"
	} else {
		operationName = "Removing"
	}

	logger.Info(fmt.Sprintf("%s ready label", operationName), map[string]interface{}{
		"label": labelToUse,
		"repo":  repoInfo.Name,
		"issue": repoInfo.IssueNumber,
	})

	// Create SCM provider
	provider, err := CreateSCMProvider(repoInfo)
	if err != nil {
		return err
	}

	// Perform the label operation
	if operation == AddLabel {
		err = provider.AddLabelsToIssue(repoInfo.IssueNumber, []string{labelToUse})
	} else {
		err = provider.RemoveLabelsFromIssue(repoInfo.IssueNumber, []string{labelToUse})
	}

	if err != nil {
		if strings.Contains(err.Error(), "failed to get issue") {
			return fmt.Errorf("issue #%d not found - check if it exists", repoInfo.IssueNumber)
		}
		if strings.Contains(err.Error(), "failed to add labels") || strings.Contains(err.Error(), "failed to remove label") {
			return fmt.Errorf("failed to %s label '%s' %s issue #%d - check your API token permissions",
				strings.ToLower(operationName), labelToUse,
				map[LabelOperation]string{AddLabel: "to", RemoveLabel: "from"}[operation],
				repoInfo.IssueNumber)
		}
		return fmt.Errorf("failed to %s label %s issue: %w",
			strings.ToLower(operationName),
			map[LabelOperation]string{AddLabel: "to", RemoveLabel: "from"}[operation],
			err)
	}

	// Success message
	actionWord := map[LabelOperation]string{AddLabel: "Added", RemoveLabel: "Removed"}[operation]
	preposition := map[LabelOperation]string{AddLabel: "to", RemoveLabel: "from"}[operation]
	fmt.Printf("%s label '%s' %s issue #%d\n", actionWord, labelToUse, preposition, repoInfo.IssueNumber)

	logger.Debug("Label operation completed successfully")
	return nil
}

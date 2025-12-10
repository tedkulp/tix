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

	logger.Debug("Config loaded successfully", map[string]any{
		"repos_count": len(cfg.GetRepoNames()),
	})

	// Get current directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	logger.Debug("Current directory", map[string]any{
		"directory": wd,
	})

	// Find repo that matches the current directory, or the best candidate
	// Only consider code repos (repos with directory)
	var matchingRepo *config.Repository
	var repoName string
	var bestMatchLength = 0

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
			// If we found a better match (longer path prefix), use it
			if len(absRepoDir) > bestMatchLength {
				matchingRepo = &cfg.Repositories[i]
				repoName = cfg.GetRepoNames()[i]
				bestMatchLength = len(absRepoDir)
			}
		}
	}

	// If we found a match, we'll offer it as the default option
	if matchingRepo != nil {
		logger.Info("Found matching repository", map[string]any{
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

	logger.Info("Repository selected", map[string]any{
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

	logger.Info("Current branch", map[string]any{
		"branch": currentBranch,
	})

	// Extract issue info from branch name (may include project name for cross-repo)
	projectName, issueNumber, err := ExtractIssueInfo(currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to extract issue number from branch '%s': %w", currentBranch, err)
	}

	logger.Info("Issue info extracted", map[string]any{
		"project": projectName,
		"issue":   issueNumber,
	})

	// Determine issue repo (for cross-repo scenarios)
	issueRepo := selectedRepo
	issueRepoName := selectedRepoName

	if projectName != "" && projectName != selectedRepoName {
		// Cross-repo: look up issue repo
		issueRepo = cfg.GetRepo(projectName)
		if issueRepo == nil {
			return nil, fmt.Errorf("repository '%s' not found in config", projectName)
		}
		issueRepoName = projectName

		// Validate providers match
		if (selectedRepo.GithubRepo != "" && issueRepo.GitlabRepo != "") ||
			(selectedRepo.GitlabRepo != "" && issueRepo.GithubRepo != "") {
			return nil, fmt.Errorf("issue repo '%s' and code repo '%s' must use the same provider", projectName, selectedRepoName)
		}

		logger.Info("Cross-repo scenario detected", map[string]any{
			"code_repo":  selectedRepoName,
			"issue_repo": issueRepoName,
		})
	}

	// Determine if this is GitLab or GitHub (use issue repo for provider)
	isGitLab := issueRepo.GitlabRepo != ""

	return &SharedRepoInfo{
		Repo:        issueRepo,
		Name:        issueRepoName,
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

	// Return empty string if no ready label configured
	return ""
}

// GetReadyStatus returns the appropriate ready status for the repository
func GetReadyStatus(cfg *config.Settings, repo *config.Repository, overrideStatus string) string {
	// Use override status if provided
	if overrideStatus != "" {
		return overrideStatus
	}

	// Use repository-specific ready status if set
	if repo.ReadyStatus != "" {
		return repo.ReadyStatus
	}

	// Use global default ready status if set
	if cfg.ReadyStatus != "" {
		return cfg.ReadyStatus
	}

	// Return empty string if no status configured (will be ignored)
	return ""
}

// GetUnreadyLabel returns the appropriate unready label for the repository
func GetUnreadyLabel(cfg *config.Settings, repo *config.Repository, overrideLabel string) string {
	// Use override label if provided
	if overrideLabel != "" {
		return overrideLabel
	}

	// Use repository-specific unready label if set
	if repo.UnreadyLabel != "" {
		return repo.UnreadyLabel
	}

	// Use global default unready label if set
	if cfg.UnreadyLabel != "" {
		return cfg.UnreadyLabel
	}

	// Return empty string if no unready label configured (will just remove ready label)
	return ""
}

// GetUnreadyStatus returns the appropriate unready status for the repository
func GetUnreadyStatus(cfg *config.Settings, repo *config.Repository, overrideStatus string) string {
	// Use override status if provided
	if overrideStatus != "" {
		return overrideStatus
	}

	// Use repository-specific unready status if set
	if repo.UnreadyStatus != "" {
		return repo.UnreadyStatus
	}

	// Use global default unready status if set
	if cfg.UnreadyStatus != "" {
		return cfg.UnreadyStatus
	}

	// Return empty string if no status configured (will be ignored)
	return ""
}

// LabelOperation represents the type of label operation
type LabelOperation int

const (
	AddLabel LabelOperation = iota
	RemoveLabel
)

// HandleLabelOperation handles adding or removing labels from an issue and updating status
func HandleLabelOperation(operation LabelOperation, overrideLabel string) error {
	return HandleLabelAndStatusOperation(operation, overrideLabel, "")
}

// HandleLabelAndStatusOperation handles adding or removing labels from an issue and updating status
func HandleLabelAndStatusOperation(operation LabelOperation, overrideLabel string, overrideStatus string) error {
	return HandleLabelAndStatusOperationWithUnready(operation, overrideLabel, overrideStatus, "")
}

// HandleLabelAndStatusOperationWithUnready handles adding or removing labels from an issue and updating status, with unready label support
func HandleLabelAndStatusOperationWithUnready(operation LabelOperation, overrideLabel string, overrideStatus string, overrideUnreadyLabel string) error {
	logger.Debug("Starting label operation", map[string]any{
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

	// Load config to get label and status configuration
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Create SCM provider
	provider, err := CreateSCMProvider(repoInfo)
	if err != nil {
		return err
	}

	if operation == AddLabel {
		// Ready operation: Add ready label and update status
		return handleReadyOperation(cfg, repoInfo, provider, overrideLabel, overrideStatus)
	} else {
		// Unready operation: Remove ready label, optionally add unready label, and update status
		return handleUnreadyOperation(cfg, repoInfo, provider, overrideLabel, overrideStatus, overrideUnreadyLabel)
	}
}

// handleReadyOperation handles the ready operation (add ready label and status)
func handleReadyOperation(cfg *config.Settings, repoInfo *SharedRepoInfo, provider services.SCMProvider, overrideLabel string, overrideStatus string) error {
	// Get the ready label and status to use
	labelToUse := GetReadyLabel(cfg, repoInfo.Repo, overrideLabel)
	statusToUse := GetReadyStatus(cfg, repoInfo.Repo, overrideStatus)

	// If no label or status configured, this becomes a no-op
	if labelToUse == "" && statusToUse == "" {
		fmt.Printf("No ready label or status configured for issue #%d - no changes made\n", repoInfo.IssueNumber)
		return nil
	}

	logger.Info("Ready operation", map[string]any{
		"label":  labelToUse,
		"status": statusToUse,
		"repo":   repoInfo.Name,
		"issue":  repoInfo.IssueNumber,
	})

	// Add ready label if configured
	if labelToUse != "" {
		err := provider.AddLabelsToIssue(repoInfo.IssueNumber, []string{labelToUse})
		if err != nil {
			if strings.Contains(err.Error(), "failed to get issue") {
				return fmt.Errorf("issue #%d not found - check if it exists", repoInfo.IssueNumber)
			}
			if strings.Contains(err.Error(), "failed to add labels") {
				return fmt.Errorf("failed to add label '%s' to issue #%d - check your API token permissions", labelToUse, repoInfo.IssueNumber)
			}
			return fmt.Errorf("failed to add label to issue: %w", err)
		}
	}

	// Update status if configured
	if statusToUse != "" {
		logger.Debug("Updating issue status", map[string]any{
			"status": statusToUse,
			"issue":  repoInfo.IssueNumber,
		})

		err := provider.UpdateIssueStatus(repoInfo.IssueNumber, statusToUse)
		if err != nil {
			// Log the error but don't fail the entire operation
			logger.Warn("Failed to update issue status", map[string]any{
				"error":  err.Error(),
				"status": statusToUse,
				"issue":  repoInfo.IssueNumber,
			})
			fmt.Printf("Warning: Failed to update issue status to '%s': %v\n", statusToUse, err)
		}
	}

	// Success message
	if labelToUse != "" && statusToUse != "" {
		fmt.Printf("Added label '%s' and updated status to '%s' for issue #%d\n", labelToUse, statusToUse, repoInfo.IssueNumber)
	} else if labelToUse != "" {
		fmt.Printf("Added label '%s' to issue #%d\n", labelToUse, repoInfo.IssueNumber)
	} else if statusToUse != "" {
		fmt.Printf("Updated status to '%s' for issue #%d\n", statusToUse, repoInfo.IssueNumber)
	}

	return nil
}

// handleUnreadyOperation handles the unready operation (remove ready label, optionally add unready label, and update status)
func handleUnreadyOperation(cfg *config.Settings, repoInfo *SharedRepoInfo, provider services.SCMProvider, overrideLabel string, overrideStatus string, overrideUnreadyLabel string) error {
	// Get the ready label to remove (using the same logic as ready command)
	readyLabelToRemove := GetReadyLabel(cfg, repoInfo.Repo, overrideLabel)

	// Get the unready label to add (if configured)
	unreadyLabelToAdd := GetUnreadyLabel(cfg, repoInfo.Repo, overrideUnreadyLabel)

	// Get the unready status to set
	statusToUse := GetUnreadyStatus(cfg, repoInfo.Repo, overrideStatus)

	// If no ready label configured and no status to set, this becomes a no-op
	if readyLabelToRemove == "" && statusToUse == "" {
		fmt.Printf("No ready label or status configured for issue #%d - no changes made\n", repoInfo.IssueNumber)
		return nil
	}

	logger.Info("Unready operation", map[string]any{
		"ready_label":   readyLabelToRemove,
		"unready_label": unreadyLabelToAdd,
		"status":        statusToUse,
		"repo":          repoInfo.Name,
		"issue":         repoInfo.IssueNumber,
	})

	// Remove ready label if configured
	if readyLabelToRemove != "" {
		err := provider.RemoveLabelsFromIssue(repoInfo.IssueNumber, []string{readyLabelToRemove})
		if err != nil {
			if strings.Contains(err.Error(), "failed to get issue") {
				return fmt.Errorf("issue #%d not found - check if it exists", repoInfo.IssueNumber)
			}
			if strings.Contains(err.Error(), "failed to remove label") {
				return fmt.Errorf("failed to remove label '%s' from issue #%d - check your API token permissions", readyLabelToRemove, repoInfo.IssueNumber)
			}
			return fmt.Errorf("failed to remove label from issue: %w", err)
		}

		// Add unready label if configured
		if unreadyLabelToAdd != "" {
			err = provider.AddLabelsToIssue(repoInfo.IssueNumber, []string{unreadyLabelToAdd})
			if err != nil {
				// Log the error but don't fail the entire operation since we already removed the ready label
				logger.Warn("Failed to add unready label", map[string]any{
					"error": err.Error(),
					"label": unreadyLabelToAdd,
					"issue": repoInfo.IssueNumber,
				})
				fmt.Printf("Warning: Failed to add unready label '%s': %v\n", unreadyLabelToAdd, err)
			}
		}
	}

	// Update status if configured
	if statusToUse != "" {
		logger.Debug("Updating issue status", map[string]any{
			"status": statusToUse,
			"issue":  repoInfo.IssueNumber,
		})

		err := provider.UpdateIssueStatus(repoInfo.IssueNumber, statusToUse)
		if err != nil {
			// Log the error but don't fail the entire operation
			logger.Warn("Failed to update issue status", map[string]any{
				"error":  err.Error(),
				"status": statusToUse,
				"issue":  repoInfo.IssueNumber,
			})
			fmt.Printf("Warning: Failed to update issue status to '%s': %v\n", statusToUse, err)
		}
	}

	// Success message
	var message string
	if readyLabelToRemove != "" && unreadyLabelToAdd != "" && statusToUse != "" {
		message = fmt.Sprintf("Removed label '%s', added label '%s', and updated status to '%s' for issue #%d", readyLabelToRemove, unreadyLabelToAdd, statusToUse, repoInfo.IssueNumber)
	} else if readyLabelToRemove != "" && unreadyLabelToAdd != "" {
		message = fmt.Sprintf("Removed label '%s' and added label '%s' for issue #%d", readyLabelToRemove, unreadyLabelToAdd, repoInfo.IssueNumber)
	} else if readyLabelToRemove != "" && statusToUse != "" {
		message = fmt.Sprintf("Removed label '%s' and updated status to '%s' for issue #%d", readyLabelToRemove, statusToUse, repoInfo.IssueNumber)
	} else if readyLabelToRemove != "" {
		message = fmt.Sprintf("Removed label '%s' from issue #%d", readyLabelToRemove, repoInfo.IssueNumber)
	} else if statusToUse != "" {
		message = fmt.Sprintf("Updated status to '%s' for issue #%d", statusToUse, repoInfo.IssueNumber)
	}

	fmt.Println(message)
	logger.Debug("Unready operation completed successfully")
	return nil
}

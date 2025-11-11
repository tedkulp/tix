package cmd

import (
	"context"
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

// RepoInfo holds information about a selected repository and related data
type RepoInfo struct {
	Repo                *config.Repository
	Name                string
	IsGitLab            bool
	CurrentDir          string
	IssueNumber         int
	Branch              string
	DescriptionProvider services.MRDescriptionProvider
}

// Command flags
var (
	onlyIssue        bool
	onlyMergeRequest bool
	onlyPullRequest  bool // For GitHub users
)

var setdescCmd = &cobra.Command{
	Use:   "setdesc",
	Short: "Generate and update descriptions for merge requests and issues",
	Long: `Generate descriptions for merge requests and issues using OpenAI.
It will download the diff of the merge request and use AI to generate descriptions.

You can use the --only-issue (-i) flag to update only the issue description,
or the --only-mr (-m) flag to update only the merge/pull request description.
GitHub users can also use --only-pr (-p) as an alternative.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting setdesc command")

		// Validate flags - combine the MR and PR flags
		updateMROnly := onlyMergeRequest || onlyPullRequest

		if onlyIssue && updateMROnly {
			return fmt.Errorf("cannot use both --only-issue and --only-mr/--only-pr flags together")
		}

		// Initial setup and validation
		client, err := services.NewOpenAIClient()
		if err != nil {
			if strings.Contains(err.Error(), "OPENAI_API_KEY") {
				return fmt.Errorf("OpenAI API key not found - set the OPENAI_API_KEY environment variable")
			}
			return fmt.Errorf("failed to initialize OpenAI client: %v", err)
		}

		// Find and select repository
		repoInfo, err := selectRepository()
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

		// Get merge request information
		mrInfo, err := getMergeRequestInfo(repoInfo)
		if err != nil {
			if strings.Contains(err.Error(), "no open requests found") {
				return fmt.Errorf("no open merge/pull requests found - create one with 'tix mr' first")
			}
			if strings.Contains(err.Error(), "failed to get merge request diff") {
				return fmt.Errorf("couldn't download the merge request diff - check your API token and permissions")
			}
			return err
		}

		// Setup OpenAI resources
		oaiResources, err := services.SetupOpenAIResources(cmd.Context(), client, mrInfo.Diff)

		// Always defer cleanup, but only if resources were created
		defer func() {
			logger.Info("Cleaning up OpenAI resources", map[string]any{
				"resources": oaiResources,
			})
			if oaiResources != nil {
				services.CleanupOpenAIResources(cmd.Context(), oaiResources)
			}
		}()

		if err != nil {
			return fmt.Errorf("failed to setup OpenAI resources: %v", err)
		}

		// Generate and update descriptions based on flags
		if !onlyIssue {
			// Generate and update MR description
			if err := generateAndUpdateMRDescription(cmd.Context(), oaiResources, repoInfo, mrInfo); err != nil {
				if strings.Contains(err.Error(), "failed to generate") {
					return fmt.Errorf("failed to generate description with OpenAI - try again or check API usage limits")
				}
				if strings.Contains(err.Error(), "failed to update") {
					return fmt.Errorf("generated description but failed to update merge request - check your API token permissions")
				}
				return err
			}
		}

		if !updateMROnly {
			// Generate and update issue description
			if err := generateAndUpdateIssueDescription(cmd.Context(), oaiResources, repoInfo, mrInfo); err != nil {
				if strings.Contains(err.Error(), "failed to generate") {
					return fmt.Errorf("failed to generate issue description with OpenAI - try again or check API usage limits")
				}
				if strings.Contains(err.Error(), "failed to update") {
					return fmt.Errorf("generated description but failed to update issue - check your API token permissions")
				}
				return err
			}
		}

		logger.Info("Setdesc command completed successfully")
		return nil
	},
}

// selectRepository determines which repository to work with
func selectRepository() (*RepoInfo, error) {
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
		logger.Info("Using matching repository", map[string]any{
			"repo": selectedRepoName,
		})
	} else {
		// No matching repo found, show selector
		if len(repoNames) == 0 {
			return nil, fmt.Errorf("no repositories configured")
		}

		// Use pterm's interactive select component
		selectedName, err := pterm.DefaultInteractiveSelect.
			WithOptions(repoNames).
			WithDefaultText("Select a repository").
			WithDefaultOption(repoName).
			Show()
		if err != nil {
			return nil, fmt.Errorf("repository selection cancelled")
		}

		// Find the index of the selected repository
		var selectedIdx int
		for i, name := range repoNames {
			if name == selectedName {
				selectedIdx = i
				break
			}
		}

		selectedRepo = &cfg.Repositories[selectedIdx]
		selectedRepoName = selectedName
	}

	logger.Info("Repository selected", map[string]any{
		"repo": selectedRepoName,
	})

	// Validate repository configuration
	if (selectedRepo.GithubRepo == "" && selectedRepo.GitlabRepo == "") ||
		(selectedRepo.GithubRepo != "" && selectedRepo.GitlabRepo != "") {
		logger.Error("Invalid repository configuration", nil, map[string]any{
			"repo":        selectedRepoName,
			"github_repo": selectedRepo.GithubRepo,
			"gitlab_repo": selectedRepo.GitlabRepo,
		})
		return nil, fmt.Errorf("repository must have exactly one of github_repo or gitlab_repo")
	}

	// Determine if GitLab or GitHub repository
	isGitlab := selectedRepo.GitlabRepo != ""

	// Open Git repository
	gitRepo, err := git.Open(selectedRepo.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get current branch
	currentBranch, err := gitRepo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	logger.Info("Current branch", map[string]any{
		"branch": currentBranch,
	})

	// Extract issue number from branch name
	issueNumber, err := utils.ExtractIssueNumber(currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to extract issue number from branch name: %w", err)
	}

	logger.Info("Issue number extracted", map[string]any{
		"issue": issueNumber,
	})

	// Create a description provider based on the repository type
	var descriptionProvider services.MRDescriptionProvider
	if isGitlab {
		descriptionProvider, err = services.NewGitLabMRDescriptionProvider(selectedRepo.GitlabRepo)
	} else {
		descriptionProvider, err = services.NewGitHubMRDescriptionProvider(selectedRepo.GithubRepo)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create description provider: %w", err)
	}

	return &RepoInfo{
		Repo:                selectedRepo,
		Name:                selectedRepoName,
		IsGitLab:            isGitlab,
		CurrentDir:          wd,
		IssueNumber:         issueNumber,
		Branch:              currentBranch,
		DescriptionProvider: descriptionProvider,
	}, nil
}

// getMergeRequestInfo retrieves information about the merge/pull request
func getMergeRequestInfo(repoInfo *RepoInfo) (*services.MRInfo, error) {
	// Get basic MR info (open requests for the issue)
	mrInfo, err := services.GetMRInfo(repoInfo.DescriptionProvider, repoInfo.IssueNumber)
	if err != nil {
		return nil, err
	}

	// Get merge request titles to display to the user
	mrTitles := make([]string, len(mrInfo.OpenRequests))
	for i, mr := range mrInfo.OpenRequests {
		mrTitles[i] = fmt.Sprintf("%s (#%d)", mr.Title, mr.IID)
	}

	// If multiple merge requests are found, ask the user to select one
	var selectedMR services.MRDescriptionResult
	if len(mrInfo.OpenRequests) > 1 {
		// Use pterm's interactive select component
		selectedTitle, err := pterm.DefaultInteractiveSelect.
			WithOptions(mrTitles).
			WithDefaultText("Select a merge request").
			Show()
		if err != nil {
			return nil, fmt.Errorf("failed to select merge request: %w", err)
		}

		// Find the index of the selected merge request
		var idx int
		for i, title := range mrTitles {
			if title == selectedTitle {
				idx = i
				break
			}
		}

		selectedMR = mrInfo.OpenRequests[idx]
	} else {
		selectedMR = mrInfo.OpenRequests[0]
	}

	logger.Info("Merge request selected", map[string]any{
		"id":    selectedMR.IID,
		"title": selectedMR.Title,
	})

	// Get merge request diff
	diff, err := repoInfo.DescriptionProvider.GetRequestDiff(selectedMR.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request diff: %w", err)
	}

	// Get issue web URL
	issue, err := repoInfo.DescriptionProvider.GetIssueDetails(repoInfo.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue details: %w", err)
	}

	// Update MRInfo with additional details
	mrInfo.SelectedID = selectedMR.ID
	mrInfo.Diff = diff
	mrInfo.WebURL = selectedMR.WebURL
	mrInfo.IssueURL = issue.WebURL

	return mrInfo, nil
}

// generateAndUpdateMRDescription generates and updates the merge request description
func generateAndUpdateMRDescription(ctx context.Context, oaiResources *services.OpenAIResources, repoInfo *RepoInfo, mrInfo *services.MRInfo) error {
	fmt.Println("Generating merge request description...")

	// Generate the description
	mrDescription, err := services.GenerateMRDescription(ctx, oaiResources)
	if err != nil {
		return fmt.Errorf("failed to generate merge request description: %w", err)
	}

	// Show the description and prompt for confirmation
	fmt.Println("\n========== MERGE REQUEST DESCRIPTION ==========")
	fmt.Println(mrDescription)
	fmt.Println("=============================================")
	fmt.Println()

	// Get user confirmation for updating MR description
	result, err := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(true).
		WithDefaultText("Do you want to update the merge request description?").
		Show()
	if err != nil {
		return fmt.Errorf("cancelled updating merge request description")
	}

	if !result {
		return nil
	}

	// Update the description using the provider
	if err := repoInfo.DescriptionProvider.UpdateRequestDescription(mrInfo.SelectedID, mrDescription); err != nil {
		return fmt.Errorf("failed to update merge request description: %w", err)
	}

	fmt.Println("Merge request description updated successfully!")
	fmt.Printf("Merge request URL: %s\n\n", mrInfo.WebURL)

	return nil
}

// generateAndUpdateIssueDescription generates and updates the issue description
func generateAndUpdateIssueDescription(ctx context.Context, oaiResources *services.OpenAIResources, repoInfo *RepoInfo, mrInfo *services.MRInfo) error {
	fmt.Println("Generating issue description...")

	// Get original issue details
	originalIssue, err := repoInfo.DescriptionProvider.GetIssueDetails(repoInfo.IssueNumber)
	if err != nil {
		return fmt.Errorf("failed to get original issue details: %w", err)
	}

	// Generate the description
	newTitle, issueDescription, err := services.GenerateIssueDescription(ctx, oaiResources, originalIssue.Title)
	if err != nil {
		return fmt.Errorf("failed to generate issue description: %w", err)
	}

	// Handle title update separately if a new title was generated
	var shouldUpdateTitle bool
	if newTitle != "" {
		// Check if the generated title is the same as the original
		if newTitle == originalIssue.Title {
			fmt.Printf("Generated title matches existing title, continuing with: %s\n\n", newTitle)
		} else {
			fmt.Println()
			fmt.Println("========== ORIGINAL ISSUE TITLE ==========")
			fmt.Println(originalIssue.Title)
			fmt.Println("========== NEW ISSUE TITLE ==========")
			fmt.Println(newTitle)
			fmt.Println("========================================")
			fmt.Println()

			// Get user confirmation for title update
			titleResult, err := pterm.DefaultInteractiveConfirm.
				WithDefaultValue(true).
				WithDefaultText("Do you want to update the issue title?").
				Show()
			if err != nil {
				return fmt.Errorf("cancelled updating issue title")
			}

			shouldUpdateTitle = titleResult
			fmt.Println()
		}
	}

	// Show the description and prompt for confirmation
	fmt.Println("========== ISSUE DESCRIPTION ==========")
	fmt.Println(issueDescription)
	fmt.Println("========================================")
	fmt.Println()

	// Get user confirmation for issue description update
	descResult, err := pterm.DefaultInteractiveConfirm.
		WithDefaultValue(true).
		WithDefaultText("Do you want to update the issue description?").
		Show()
	if err != nil {
		return fmt.Errorf("cancelled updating issue description")
	}

	if !descResult {
		return nil
	}

	// Update issue description
	if err := repoInfo.DescriptionProvider.UpdateIssueDescription(repoInfo.IssueNumber, issueDescription); err != nil {
		return fmt.Errorf("failed to update issue description: %w", err)
	}

	// Update issue title if both title and description updates were confirmed
	if shouldUpdateTitle && newTitle != "" {
		if err := repoInfo.DescriptionProvider.UpdateIssueTitle(repoInfo.IssueNumber, newTitle); err != nil {
			return fmt.Errorf("failed to update issue title: %w", err)
		}
		fmt.Println("Issue title updated successfully!")
	}

	fmt.Println("Issue description updated successfully!")
	fmt.Printf("Issue URL: %s\n", mrInfo.IssueURL)

	return nil
}

func init() {
	rootCmd.AddCommand(setdescCmd)

	// Add flags for selective description generation
	setdescCmd.Flags().BoolVarP(&onlyIssue, "only-issue", "i", false, "Only update the issue description")

	// Add flags for merge/pull request updates with different names for different platforms
	setdescCmd.Flags().BoolVarP(&onlyMergeRequest, "only-mr", "m", false, "Only update the merge/pull request description")
	setdescCmd.Flags().BoolVarP(&onlyPullRequest, "only-pr", "p", false, "Only update the pull request description")
}

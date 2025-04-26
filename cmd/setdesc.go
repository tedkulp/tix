package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/services"
	"github.com/tedkulp/tix/internal/utils"
)

// RepoInfo holds information about a selected repository and related data
type RepoInfo struct {
	Repo        *config.Repository
	Name        string
	IsGitLab    bool
	CurrentDir  string
	IssueNumber int
	Branch      string
}

// MRInfo holds information about a merge/pull request
type MRInfo struct {
	ID       int
	Diff     string
	WebURL   string
	IssueURL string
}

var setdescCmd = &cobra.Command{
	Use:   "setdesc",
	Short: "Generate and update descriptions for merge requests and issues",
	Long: `Generate descriptions for merge requests and issues using OpenAI.
It will download the diff of the merge request and use AI to generate descriptions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting setdesc command")

		// Initial setup and validation
		client, err := services.NewOpenAIClient()
		if err != nil {
			return err
		}

		// Find and select repository
		repoInfo, err := selectRepository()
		if err != nil {
			return err
		}

		// Get merge request information
		mrInfo, err := getMergeRequestInfo(repoInfo)
		if err != nil {
			return err
		}

		// Setup OpenAI resources
		oaiResources, err := services.SetupOpenAIResources(cmd.Context(), client, mrInfo.Diff)
		if err != nil {
			defer services.CleanupOpenAIResources(cmd.Context(), oaiResources)
			return err
		}
		defer services.CleanupOpenAIResources(cmd.Context(), oaiResources)

		// Generate and update MR description
		if err := generateAndUpdateMRDescription(cmd.Context(), oaiResources, repoInfo, mrInfo); err != nil {
			return err
		}

		// Generate and update issue description
		if err := generateAndUpdateIssueDescription(cmd.Context(), oaiResources, repoInfo, mrInfo); err != nil {
			return err
		}

		logger.Debug("Setdesc command completed successfully")
		return nil
	},
}

// selectRepository determines which repository to work with
func selectRepository() (*RepoInfo, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	logger.Debug("Config loaded successfully", map[string]interface{}{
		"repos_count": len(cfg.GetRepoNames()),
	})

	// Try to find the repository based on current directory
	wd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	logger.Debug("Current directory", map[string]interface{}{
		"directory": wd,
	})

	// Find repo that matches the current directory
	var matchingRepo *config.Repository
	var repoName string
	for _, repo := range cfg.Repositories {
		absRepoDir, err := filepath.Abs(repo.Directory)
		if err != nil {
			continue
		}

		// Check if current directory is within the repo directory
		if strings.HasPrefix(wd, absRepoDir) {
			matchingRepo = &repo
			repoName = repo.Name
			break
		}
	}

	// If no matching repo found, show selector
	if matchingRepo == nil {
		repoNames := cfg.GetRepoNames()
		prompt := promptui.Select{
			Label: "Select a repository",
			Items: repoNames,
		}

		idx, name, err := prompt.Run()
		if err != nil {
			return nil, fmt.Errorf("failed to select repository: %w", err)
		}

		matchingRepo = &cfg.Repositories[idx]
		repoName = name
	}

	logger.Info("Repository selected", map[string]interface{}{
		"repo": repoName,
	})

	// Validate repository configuration
	if (matchingRepo.GithubRepo == "" && matchingRepo.GitlabRepo == "") ||
		(matchingRepo.GithubRepo != "" && matchingRepo.GitlabRepo != "") {
		logger.Error("Invalid repository configuration", nil, map[string]interface{}{
			"repo":        repoName,
			"github_repo": matchingRepo.GithubRepo,
			"gitlab_repo": matchingRepo.GitlabRepo,
		})
		return nil, fmt.Errorf("repository must have exactly one of github_repo or gitlab_repo")
	}

	// Determine if GitLab or GitHub repository
	isGitlab := matchingRepo.GitlabRepo != ""

	// Open Git repository
	gitRepo, err := git.Open(matchingRepo.Directory)
	if err != nil {
		return nil, fmt.Errorf("failed to open repository: %w", err)
	}

	// Get current branch
	currentBranch, err := gitRepo.GetCurrentBranch()
	if err != nil {
		return nil, fmt.Errorf("failed to get current branch: %w", err)
	}

	logger.Info("Current branch", map[string]interface{}{
		"branch": currentBranch,
	})

	// Extract issue number from branch name
	issueNumber, err := utils.ExtractIssueNumber(currentBranch)
	if err != nil {
		return nil, fmt.Errorf("failed to extract issue number from branch name: %w", err)
	}

	logger.Info("Issue number extracted", map[string]interface{}{
		"issue": issueNumber,
	})

	return &RepoInfo{
		Repo:        matchingRepo,
		Name:        repoName,
		IsGitLab:    isGitlab,
		CurrentDir:  wd,
		IssueNumber: issueNumber,
		Branch:      currentBranch,
	}, nil
}

// getMergeRequestInfo retrieves information about the merge/pull request
func getMergeRequestInfo(repoInfo *RepoInfo) (*MRInfo, error) {
	if repoInfo.IsGitLab {
		return getGitlabMergeRequestInfo(repoInfo)
	}
	return getGithubPullRequestInfo(repoInfo)
}

// getGitlabMergeRequestInfo retrieves GitLab merge request information
func getGitlabMergeRequestInfo(repoInfo *RepoInfo) (*MRInfo, error) {
	project, err := services.NewGitlabProject(repoInfo.Repo.GitlabRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	// Get open MRs for the issue
	openMRs, err := project.GetOpenMergeRequestsForIssue(repoInfo.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get open merge requests: %w", err)
	}

	if len(openMRs) == 0 {
		return nil, fmt.Errorf("no open merge requests found for issue #%d, run 'mr' command first", repoInfo.IssueNumber)
	}

	// If multiple MRs found, let user select
	var selectedMR *services.GitlabMergeRequest
	if len(openMRs) > 1 {
		mrTitles := make([]string, len(openMRs))
		for i, mr := range openMRs {
			mrTitles[i] = fmt.Sprintf("#%d: %s", mr.IID, mr.Title)
		}

		prompt := promptui.Select{
			Label: "Select a merge request",
			Items: mrTitles,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return nil, fmt.Errorf("failed to select merge request: %w", err)
		}

		selectedMR = openMRs[idx]
	} else {
		selectedMR = openMRs[0]
	}

	logger.Info("Merge request selected", map[string]interface{}{
		"id":    selectedMR.IID,
		"title": selectedMR.Title,
	})

	// Get merge request diff
	diff, err := project.GetMergeRequestDiff(selectedMR.IID)
	if err != nil {
		return nil, fmt.Errorf("failed to get merge request diff: %w", err)
	}

	// Get issue web URL
	issue, err := project.GetIssue(repoInfo.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue details: %w", err)
	}

	return &MRInfo{
		ID:       selectedMR.IID,
		Diff:     diff,
		WebURL:   selectedMR.WebURL,
		IssueURL: issue.WebURL,
	}, nil
}

// getGithubPullRequestInfo retrieves GitHub pull request information
func getGithubPullRequestInfo(repoInfo *RepoInfo) (*MRInfo, error) {
	project, err := services.NewGithubProject(repoInfo.Repo.GithubRepo)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	// Get open PRs for the issue
	openPRs, err := project.GetOpenPullRequestsForIssue(repoInfo.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get open pull requests: %w", err)
	}

	if len(openPRs) == 0 {
		return nil, fmt.Errorf("no open pull requests found for issue #%d, run 'mr' command first", repoInfo.IssueNumber)
	}

	// If multiple PRs found, let user select
	var selectedPR *services.GithubPullRequest
	if len(openPRs) > 1 {
		prTitles := make([]string, len(openPRs))
		for i, pr := range openPRs {
			prTitles[i] = fmt.Sprintf("#%d: %s", pr.Number, pr.Title)
		}

		prompt := promptui.Select{
			Label: "Select a pull request",
			Items: prTitles,
		}

		idx, _, err := prompt.Run()
		if err != nil {
			return nil, fmt.Errorf("failed to select pull request: %w", err)
		}

		selectedPR = openPRs[idx]
	} else {
		selectedPR = openPRs[0]
	}

	logger.Info("Pull request selected", map[string]interface{}{
		"number": selectedPR.Number,
		"title":  selectedPR.Title,
	})

	// Get pull request diff
	diff, err := project.GetPullRequestDiff(selectedPR.Number)
	if err != nil {
		return nil, fmt.Errorf("failed to get pull request diff: %w", err)
	}

	// Get issue web URL
	issue, err := project.GetIssue(repoInfo.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue details: %w", err)
	}

	return &MRInfo{
		ID:       selectedPR.Number,
		Diff:     diff,
		WebURL:   selectedPR.HTMLURL,
		IssueURL: issue.HTMLURL,
	}, nil
}

// generateAndUpdateMRDescription generates and updates the merge request description
func generateAndUpdateMRDescription(ctx context.Context, oaiResources *services.OpenAIResources, repoInfo *RepoInfo, mrInfo *MRInfo) error {
	fmt.Println("Generating merge request description...")

	// Generate the description
	mrDescription, err := services.GenerateMRDescription(ctx, oaiResources)
	if err != nil {
		return fmt.Errorf("failed to generate merge request description: %w", err)
	}

	// Show the description and prompt for confirmation
	fmt.Println("\n========== MERGE REQUEST DESCRIPTION ==========")
	fmt.Println(mrDescription)
	fmt.Println("=============================================\n")

	confirm := promptui.Prompt{
		Label:     "Update merge request description",
		IsConfirm: true,
	}

	_, err = confirm.Run()
	if err != nil {
		fmt.Println("Description update canceled.")
		return nil
	}

	// Update the MR/PR description based on the platform
	if repoInfo.IsGitLab {
		// GitLab
		project, err := services.NewGitlabProject(repoInfo.Repo.GitlabRepo)
		if err != nil {
			return fmt.Errorf("failed to create GitLab client: %w", err)
		}

		if err := project.UpdateMergeRequestDescription(mrInfo.ID, mrDescription); err != nil {
			return fmt.Errorf("failed to update merge request description: %w", err)
		}
	} else {
		// GitHub
		project, err := services.NewGithubProject(repoInfo.Repo.GithubRepo)
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		if err := project.UpdatePullRequestDescription(mrInfo.ID, mrDescription); err != nil {
			return fmt.Errorf("failed to update pull request description: %w", err)
		}
	}

	fmt.Println("Merge request description updated successfully!")
	fmt.Printf("Merge request URL: %s\n\n", mrInfo.WebURL)

	return nil
}

// generateAndUpdateIssueDescription generates and updates the issue description
func generateAndUpdateIssueDescription(ctx context.Context, oaiResources *services.OpenAIResources, repoInfo *RepoInfo, mrInfo *MRInfo) error {
	fmt.Println("Generating issue description...")

	// Generate the description
	issueDescription, err := services.GenerateIssueDescription(ctx, oaiResources)
	if err != nil {
		return fmt.Errorf("failed to generate issue description: %w", err)
	}

	// Show the description and prompt for confirmation
	fmt.Println("\n========== ISSUE DESCRIPTION ==========")
	fmt.Println(issueDescription)
	fmt.Println("========================================\n")

	confirm := promptui.Prompt{
		Label:     "Update issue description",
		IsConfirm: true,
	}

	_, err = confirm.Run()
	if err != nil {
		fmt.Println("Issue description update canceled.")
		return nil
	}

	// Update the issue description based on the platform
	if repoInfo.IsGitLab {
		// GitLab
		project, err := services.NewGitlabProject(repoInfo.Repo.GitlabRepo)
		if err != nil {
			return fmt.Errorf("failed to create GitLab client: %w", err)
		}

		if err := project.UpdateIssueDescription(repoInfo.IssueNumber, issueDescription); err != nil {
			return fmt.Errorf("failed to update issue description: %w", err)
		}
	} else {
		// GitHub
		project, err := services.NewGithubProject(repoInfo.Repo.GithubRepo)
		if err != nil {
			return fmt.Errorf("failed to create GitHub client: %w", err)
		}

		if err := project.UpdateIssueDescription(repoInfo.IssueNumber, issueDescription); err != nil {
			return fmt.Errorf("failed to update issue description: %w", err)
		}
	}

	fmt.Println("Issue description updated successfully!")
	fmt.Printf("Issue URL: %s\n", mrInfo.IssueURL)

	return nil
}

func init() {
	rootCmd.AddCommand(setdescCmd)
}

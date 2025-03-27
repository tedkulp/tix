package cmd

import (
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

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		logger.Debug("Config loaded successfully", map[string]interface{}{
			"repos_count": len(cfg.GetRepoNames()),
		})

		// Try to find the repository based on current directory
		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current directory: %w", err)
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
				return fmt.Errorf("failed to select repository: %w", err)
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
			return fmt.Errorf("repository must have exactly one of github_repo or gitlab_repo... %+v", matchingRepo)
		}

		logger.Debug("Opening git repository", map[string]interface{}{
			"directory": matchingRepo.Directory,
		})

		// Open Git repository
		gitRepo, err := git.Open(matchingRepo.Directory)
		if err != nil {
			return fmt.Errorf("failed to open repository: %w", err)
		}

		// Get current branch
		currentBranch, err := gitRepo.GetCurrentBranch()
		if err != nil {
			return fmt.Errorf("failed to get current branch: %w", err)
		}

		logger.Info("Current branch", map[string]interface{}{
			"branch": currentBranch,
		})

		// Extract issue number from branch name
		issueNumber, err := utils.ExtractIssueNumber(currentBranch)
		if err != nil {
			return fmt.Errorf("failed to extract issue number from branch name: %w", err)
		}

		logger.Info("Issue number extracted", map[string]interface{}{
			"issue": issueNumber,
		})

		if matchingRepo.GitlabRepo != "" {
			// Check if there's already an open MR for this issue
			project, err := services.NewGitlabProject(matchingRepo.GitlabRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitLab client: %w", err)
			}

			openMRs, err := project.GetOpenMergeRequestsForIssue(issueNumber)
			if err != nil {
				logger.Warn("Failed to check for existing merge requests", map[string]interface{}{
					"error": err.Error(),
				})
			} else if len(openMRs) > 0 {
				// Check if any of the MRs use the same branch
				for _, mr := range openMRs {
					// If there's already an MR for this branch, just print a message and exit
					if strings.Contains(mr.Title, currentBranch) {
						fmt.Printf("A merge request already exists for this branch: %s\n", mr.WebURL)
						return nil
					}
				}
			}

			// Push current branch to origin
			logger.Info("Pushing branch to origin", map[string]interface{}{
				"branch": currentBranch,
			})

			if err := gitRepo.Push("origin", currentBranch); err != nil {
				return fmt.Errorf("failed to push to origin: %w", err)
			}

			logger.Info("Creating GitLab merge request", map[string]interface{}{
				"branch": currentBranch,
				"issue":  issueNumber,
			})

			// Get the default branch from config
			targetBranch := "main"
			if matchingRepo.DefaultBranch != "" {
				targetBranch = matchingRepo.DefaultBranch
			}

			// Get issue details
			issue, err := project.GetIssue(issueNumber)
			if err != nil {
				return fmt.Errorf("failed to get issue details: %w", err)
			}

			// Create MR title with issue number and full title
			mrTitle := fmt.Sprintf("#%d - %s", issueNumber, issue.Title)

			// Create merge request
			mr, err := project.CreateMergeRequest(mrTitle, currentBranch, targetBranch, issueNumber, isDraft)
			if err != nil {
				return fmt.Errorf("failed to create merge request: %w", err)
			}

			logger.Info("Merge request created", map[string]interface{}{
				"title": mr.Title,
				"id":    mr.IID,
				"url":   mr.WebURL,
				"draft": mr.IsDraft,
			})

			// Open the MR in browser
			if err := utils.OpenURL(mr.WebURL); err != nil {
				logger.Warn("Failed to open browser", map[string]interface{}{
					"error": err.Error(),
				})
			}

			fmt.Printf("Created merge request: %s\n", mr.WebURL)
		} else {
			// GitHub repository
			project, err := services.NewGithubProject(matchingRepo.GithubRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			openPRs, err := project.GetOpenPullRequestsForIssue(issueNumber)
			if err != nil {
				logger.Warn("Failed to check for existing pull requests", map[string]interface{}{
					"error": err.Error(),
				})
			} else if len(openPRs) > 0 {
				// Check if any of the PRs use the same branch
				for _, pr := range openPRs {
					// If there's already a PR for this branch, just print a message and exit
					if strings.Contains(pr.Title, currentBranch) {
						fmt.Printf("A pull request already exists for this branch: %s\n", pr.HTMLURL)
						return nil
					}
				}
			}

			// Push current branch to origin
			logger.Info("Pushing branch to origin", map[string]interface{}{
				"branch": currentBranch,
			})

			if err := gitRepo.Push("origin", currentBranch); err != nil {
				return fmt.Errorf("failed to push to origin: %w", err)
			}

			logger.Info("Creating GitHub pull request", map[string]interface{}{
				"branch": currentBranch,
				"issue":  issueNumber,
			})

			// Get the default branch from config
			targetBranch := "main"
			if matchingRepo.DefaultBranch != "" {
				targetBranch = matchingRepo.DefaultBranch
			}

			// Get issue details
			issue, err := project.GetIssue(issueNumber)
			if err != nil {
				return fmt.Errorf("failed to get issue details: %w", err)
			}

			// Create PR title with issue number and full title
			prTitle := fmt.Sprintf("#%d - %s", issueNumber, issue.Title)

			// Create pull request
			pr, err := project.CreatePullRequest(prTitle, currentBranch, targetBranch, issueNumber, isDraft)
			if err != nil {
				return fmt.Errorf("failed to create pull request: %w", err)
			}

			logger.Info("Pull request created", map[string]interface{}{
				"title":  pr.Title,
				"number": pr.Number,
				"url":    pr.HTMLURL,
				"draft":  pr.IsDraft,
			})

			// Open the PR in browser
			if err := utils.OpenURL(pr.HTMLURL); err != nil {
				logger.Warn("Failed to open browser", map[string]interface{}{
					"error": err.Error(),
				})
			}

			fmt.Printf("Created pull request: %s\n", pr.HTMLURL)
		}

		logger.Debug("MR command completed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(mrCmd)
	mrCmd.Flags().BoolP("draft", "d", false, "Create the merge request as a draft")
}

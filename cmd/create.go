package cmd

import (
	"fmt"
	"path/filepath"
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

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new ticket and branch",
	Long: `Create a new ticket in GitHub or GitLab and create a corresponding branch.
If no title is provided, you will be prompted for one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting create command")

		cfg, err := config.Load()
		if err != nil {
			// logger.Error("Failed to load config", err)
			return fmt.Errorf("failed to load config: %w", err)
		}

		logger.Debug("Config loaded successfully", map[string]interface{}{
			"repos_count": len(cfg.GetRepoNames()),
		})

		// Select repository
		repoNames := cfg.GetRepoNames()
		prompt := promptui.Select{
			Label: "Select a repository",
			Items: repoNames,
		}

		_, repoName, err := prompt.Run()
		if err != nil {
			// logger.Error("Failed to select repository", err)
			return fmt.Errorf("failed to select repository: %w", err)
		}

		logger.Info("Repository selected", map[string]interface{}{
			"repo": repoName,
		})

		repo := cfg.GetRepo(repoName)
		if repo == nil {
			// logger.Error("Repository not found", nil, map[string]interface{}{
			// 	"repo": repoName,
			// })
			return fmt.Errorf("repository not found: %s", repoName)
		}

		// Validate repository configuration
		if (repo.GithubRepo == "" && repo.GitlabRepo == "") ||
			(repo.GithubRepo != "" && repo.GitlabRepo != "") {
			logger.Error("Invalid repository configuration", nil, map[string]interface{}{
				"repo":        repoName,
				"github_repo": repo.GithubRepo,
				"gitlab_repo": repo.GitlabRepo,
			})
			return fmt.Errorf("repository must have exactly one of github_repo or gitlab_repo... %+v", repo)
		}

		logger.Debug("Opening git repository", map[string]interface{}{
			"directory": repo.Directory,
		})

		// Open Git repository
		gitRepo, err := git.Open(repo.Directory)
		if err != nil {
			// logger.Error("Failed to open repository", err, map[string]interface{}{
			// 	"directory": repo.Directory,
			// })
			return fmt.Errorf("failed to open repository: %w", err)
		}

		// Check if repository is clean
		isClean, err := gitRepo.IsClean()
		if err != nil {
			// logger.Error("Failed to check repository status", err)
			return fmt.Errorf("failed to check repository status: %w", err)
		}
		if !isClean {
			// logger.Error("Repository is not clean", nil)
			return fmt.Errorf("repository is not clean")
		}

		logger.Debug("Repository is clean")

		// Get issue title
		if title == "" {
			prompt := promptui.Prompt{
				Label: "Title of issue",
				Validate: func(input string) error {
					if len(input) > 255 {
						return fmt.Errorf("title must be less than 255 characters")
					}
					return nil
				},
			}
			title, err = prompt.Run()
			if err != nil {
				// logger.Error("Failed to get title", err)
				return fmt.Errorf("failed to get title: %w", err)
			}
		}

		logger.Info("Issue title set", map[string]interface{}{
			"title": title,
		})

		// Get labels
		labelPrompt := promptui.Prompt{
			Label:   "Labels (comma separated)",
			Default: repo.DefaultLabels,
		}
		labels, err := labelPrompt.Run()
		if err != nil {
			// logger.Error("Failed to get labels", err)
			return fmt.Errorf("failed to get labels: %w", err)
		}

		logger.Info("Labels set", map[string]interface{}{
			"labels": labels,
		})

		// Get milestone (only for GitLab)
		var milestone string
		if repo.GitlabRepo != "" {
			// Calculate default milestone based on current date
			defaultMilestone := utils.GenerateMilestone(time.Now())

			milestonePrompt := promptui.Prompt{
				Label:   "Milestone",
				Default: defaultMilestone,
			}
			milestone, err = milestonePrompt.Run()
			if err != nil {
				logger.Error("Failed to get milestone", err)
				return fmt.Errorf("failed to get milestone: %w", err)
			}

			logger.Info("Milestone set", map[string]interface{}{
				"milestone": milestone,
			})
		}

		// Create issue
		var issueNumber int
		var issueTitle string

		if repo.GithubRepo != "" {
			logger.Info("Creating GitHub issue", map[string]interface{}{
				"repo":        repo.GithubRepo,
				"self_assign": selfAssign,
			})

			project, err := services.NewGithubProject(repo.GithubRepo)
			if err != nil {
				// logger.Error("Failed to create GitHub client", err)
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}
			issue, err := project.CreateIssue(title, labels, selfAssign)
			if err != nil {
				// logger.Error("Failed to create GitHub issue", err)
				return fmt.Errorf("failed to create GitHub issue: %w", err)
			}
			issueNumber = issue.Number
			issueTitle = issue.Title

			logger.Info("GitHub issue created", map[string]interface{}{
				"number": issueNumber,
				"title":  issueTitle,
			})
		} else {
			logger.Info("Creating GitLab issue", map[string]interface{}{
				"repo":        repo.GitlabRepo,
				"milestone":   milestone,
				"self_assign": selfAssign,
			})

			project, err := services.NewGitlabProject(repo.GitlabRepo)
			if err != nil {
				// logger.Error("Failed to create GitLab client", err)
				return fmt.Errorf("failed to create GitLab client: %w", err)
			}
			issue, err := project.CreateIssue(title, labels, selfAssign, milestone)
			if err != nil {
				// logger.Error("Failed to create GitLab issue", err)
				return fmt.Errorf("failed to create GitLab issue: %w", err)
			}
			issueNumber = issue.IID
			issueTitle = issue.Title

			logger.Info("GitLab issue created", map[string]interface{}{
				"number":    issueNumber,
				"title":     issueTitle,
				"milestone": milestone,
			})
		}

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
				// logger.Error("Failed to create worktree", err)
				return fmt.Errorf("failed to create worktree: %w", err)
			}

			// logger.Info("Worktree created successfully")
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

			// logger.Info("Branch created and checked out successfully")
			fmt.Printf("Created and checked out branch: %s\n", branchName)
		}

		logger.Debug("Create command completed successfully")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&title, "title", "t", "", "Title of the issue")
	createCmd.Flags().BoolVarP(&selfAssign, "assign", "a", true, "Assign the issue to yourself")
}

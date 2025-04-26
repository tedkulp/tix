package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/manifoldco/promptui"
	openai "github.com/sashabaranov/go-openai"
	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/services"
	"github.com/tedkulp/tix/internal/utils"
)

var setdescCmd = &cobra.Command{
	Use:   "setdesc",
	Short: "Generate and update descriptions for merge requests and issues",
	Long: `Generate descriptions for merge requests and issues using OpenAI.
It will download the diff of the merge request and use AI to generate descriptions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting setdesc command")

		// Check if OpenAI API key is set
		openaiAPIKey := os.Getenv("OPENAI_API_KEY")
		if openaiAPIKey == "" {
			return fmt.Errorf("OPENAI_API_KEY environment variable is required")
		}

		// Create OpenAI client
		openaiClient := openai.NewClient(openaiAPIKey)

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
			return fmt.Errorf("repository must have exactly one of github_repo or gitlab_repo")
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

		var mrDiff string
		var mrWebURL string
		var mrID int
		var issueWebURL string

		// Process based on platform (GitLab or GitHub)
		if matchingRepo.GitlabRepo != "" {
			// GitLab repository
			project, err := services.NewGitlabProject(matchingRepo.GitlabRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitLab client: %w", err)
			}

			// Get open MRs for the issue
			openMRs, err := project.GetOpenMergeRequestsForIssue(issueNumber)
			if err != nil {
				return fmt.Errorf("failed to get open merge requests: %w", err)
			}

			if len(openMRs) == 0 {
				return fmt.Errorf("no open merge requests found for issue #%d, run 'mr' command first", issueNumber)
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
					return fmt.Errorf("failed to select merge request: %w", err)
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
				return fmt.Errorf("failed to get merge request diff: %w", err)
			}

			mrDiff = diff
			mrWebURL = selectedMR.WebURL
			mrID = selectedMR.IID

			// Get issue web URL
			issue, err := project.GetIssue(issueNumber)
			if err != nil {
				return fmt.Errorf("failed to get issue details: %w", err)
			}
			issueWebURL = issue.WebURL

		} else {
			// GitHub repository
			project, err := services.NewGithubProject(matchingRepo.GithubRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			// Get open PRs for the issue
			openPRs, err := project.GetOpenPullRequestsForIssue(issueNumber)
			if err != nil {
				return fmt.Errorf("failed to get open pull requests: %w", err)
			}

			if len(openPRs) == 0 {
				return fmt.Errorf("no open pull requests found for issue #%d, run 'mr' command first", issueNumber)
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
					return fmt.Errorf("failed to select pull request: %w", err)
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
				return fmt.Errorf("failed to get pull request diff: %w", err)
			}

			mrDiff = diff
			mrWebURL = selectedPR.HTMLURL
			mrID = selectedPR.Number

			// Get issue web URL
			issue, err := project.GetIssue(issueNumber)
			if err != nil {
				return fmt.Errorf("failed to get issue details: %w", err)
			}
			issueWebURL = issue.HTMLURL
		}

		// Create a temporary file for the diff
		tempFile, err := os.CreateTemp("", "mr-diff-*.txt")
		if err != nil {
			return fmt.Errorf("failed to create temporary file: %w", err)
		}
		defer os.Remove(tempFile.Name())

		// Write diff to the temporary file
		if err := os.WriteFile(tempFile.Name(), []byte(mrDiff), 0644); err != nil {
			return fmt.Errorf("failed to write diff to temporary file: %w", err)
		}

		logger.Info("Diff saved to temporary file", map[string]interface{}{
			"file": tempFile.Name(),
			"size": len(mrDiff),
		})

		// Create an assistant
		assistantName := "MR Description Generator"
		instructions := "You are an expert at reviewing code changes and providing concise, informative descriptions."
		assistant, err := openaiClient.CreateAssistant(
			cmd.Context(),
			openai.AssistantRequest{
				Model: openai.GPT4o,
				Name:  &assistantName,
				Tools: []openai.AssistantTool{
					{
						Type: openai.AssistantToolTypeFileSearch,
					},
				},
				Instructions: &instructions,
			},
		)
		if err != nil {
			return fmt.Errorf("failed to create OpenAI assistant: %w", err)
		}

		logger.Info("Created OpenAI assistant", map[string]interface{}{
			"id": assistant.ID,
		})

		// Upload the file
		fileResp, err := openaiClient.CreateFile(cmd.Context(), openai.FileRequest{
			FilePath: tempFile.Name(),
			Purpose:  string(openai.PurposeAssistants),
		})
		if err != nil {
			return fmt.Errorf("failed to upload file to OpenAI: %w", err)
		}

		logger.Info("Uploaded file to OpenAI", map[string]interface{}{
			"file_id": fileResp.ID,
		})

		content := "Please describe the changes in the attached git diff in plain English."

		// Create a thread
		threadReq := openai.ThreadRequest{
			Messages: []openai.ThreadMessage{
				{
					Role:    openai.ThreadMessageRoleUser,
					Content: string(content),
					Attachments: []openai.ThreadAttachment{
						{
							FileID: fileResp.ID,
							Tools: []openai.ThreadAttachmentTool{
								{
									Type: string(openai.AssistantToolTypeFileSearch),
								},
							},
						},
					},
				},
			},
		}
		thread, err := openaiClient.CreateThread(cmd.Context(), threadReq)
		if err != nil {
			return fmt.Errorf("failed to create OpenAI thread: %w", err)
		}

		logger.Info("Created OpenAI thread", map[string]interface{}{
			"id": thread.ID,
		})

		// Create a run
		run, err := openaiClient.CreateRun(cmd.Context(), thread.ID, openai.RunRequest{
			AssistantID: assistant.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to create OpenAI run: %w", err)
		}

		logger.Info("Created OpenAI run", map[string]interface{}{
			"id": run.ID,
		})

		// Poll until the run is completed
		fmt.Println("Generating merge request description...")
		runResult, err := pollRun(cmd.Context(), openaiClient, thread.ID, run.ID)
		if err != nil {
			return fmt.Errorf("failed to wait for OpenAI run completion: %w", err)
		}
		run = *runResult

		// Get the messages
		limit := 10
		order := "desc"
		messages, err := openaiClient.ListMessage(cmd.Context(), thread.ID, &limit, &order, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to list OpenAI messages: %w", err)
		}

		// The last message should be the assistant's response
		var mrDescription string
		if len(messages.Messages) > 1 {
			// Check message content type and extract text
			for _, contentItem := range messages.Messages[0].Content {
				if contentItem.Type == "text" {
					mrDescription = contentItem.Text.Value
					break
				}
			}
		}

		if mrDescription == "" {
			return fmt.Errorf("failed to get a description from OpenAI")
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
		if matchingRepo.GitlabRepo != "" {
			// GitLab
			project, err := services.NewGitlabProject(matchingRepo.GitlabRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitLab client: %w", err)
			}

			if err := project.UpdateMergeRequestDescription(mrID, mrDescription); err != nil {
				return fmt.Errorf("failed to update merge request description: %w", err)
			}

		} else {
			// GitHub
			project, err := services.NewGithubProject(matchingRepo.GithubRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			if err := project.UpdatePullRequestDescription(mrID, mrDescription); err != nil {
				return fmt.Errorf("failed to update pull request description: %w", err)
			}
		}

		fmt.Println("Merge request description updated successfully!")
		fmt.Printf("Merge request URL: %s\n\n", mrWebURL)

		// Now generate the issue description
		fmt.Println("Generating issue description...")

		_, err = openaiClient.CreateMessage(cmd.Context(), thread.ID, openai.MessageRequest{
			Role:    string(openai.ThreadMessageRoleUser),
			Content: "Please write a description for the related issue as if the code hadn't been written yet. This should describe the problem, requirements, and goals that this code is solving. Keep it concise but comprehensive.",
			Attachments: []openai.ThreadAttachment{
				{
					FileID: fileResp.ID,
					Tools: []openai.ThreadAttachmentTool{
						{
							Type: string(openai.AssistantToolTypeFileSearch),
						},
					},
				},
			},
		})
		if err != nil {
			return fmt.Errorf("failed to create OpenAI message for issue description: %w", err)
		}

		// Create a run for the issue description
		run, err = openaiClient.CreateRun(cmd.Context(), thread.ID, openai.RunRequest{
			AssistantID: assistant.ID,
		})
		if err != nil {
			return fmt.Errorf("failed to create OpenAI run for issue description: %w", err)
		}

		// Poll until the run is completed
		runResult, err = pollRun(cmd.Context(), openaiClient, thread.ID, run.ID)
		if err != nil {
			return fmt.Errorf("failed to wait for OpenAI run completion: %w", err)
		}
		run = *runResult

		// Get the messages
		messages, err = openaiClient.ListMessage(cmd.Context(), thread.ID, &limit, &order, nil, nil, nil)
		if err != nil {
			return fmt.Errorf("failed to list OpenAI messages: %w", err)
		}

		// The first message should now be the assistant's response for the issue description
		var issueDescription string
		if len(messages.Messages) > 0 {
			// Check message content type and extract text
			for _, contentItem := range messages.Messages[0].Content {
				if contentItem.Type == "text" {
					issueDescription = contentItem.Text.Value
					break
				}
			}
		}

		if issueDescription == "" {
			return fmt.Errorf("failed to get an issue description from OpenAI")
		}

		// Show the description and prompt for confirmation
		fmt.Println("\n========== ISSUE DESCRIPTION ==========")
		fmt.Println(issueDescription)
		fmt.Println("========================================\n")

		confirm = promptui.Prompt{
			Label:     "Update issue description",
			IsConfirm: true,
		}

		_, err = confirm.Run()
		if err != nil {
			fmt.Println("Issue description update canceled.")
			return nil
		}

		// Update the issue description based on the platform
		if matchingRepo.GitlabRepo != "" {
			// GitLab
			project, err := services.NewGitlabProject(matchingRepo.GitlabRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitLab client: %w", err)
			}

			if err := project.UpdateIssueDescription(issueNumber, issueDescription); err != nil {
				return fmt.Errorf("failed to update issue description: %w", err)
			}

		} else {
			// GitHub
			project, err := services.NewGithubProject(matchingRepo.GithubRepo)
			if err != nil {
				return fmt.Errorf("failed to create GitHub client: %w", err)
			}

			if err := project.UpdateIssueDescription(issueNumber, issueDescription); err != nil {
				return fmt.Errorf("failed to update issue description: %w", err)
			}
		}

		fmt.Println("Issue description updated successfully!")
		fmt.Printf("Issue URL: %s\n", issueWebURL)

		// Clean up OpenAI resources
		_, err = openaiClient.DeleteAssistant(cmd.Context(), assistant.ID)
		if err != nil {
			logger.Warn("Failed to delete OpenAI assistant", map[string]interface{}{
				"error": err.Error(),
			})
		}

		err = openaiClient.DeleteFile(cmd.Context(), fileResp.ID)
		if err != nil {
			logger.Warn("Failed to delete OpenAI file", map[string]interface{}{
				"error": err.Error(),
			})
		}

		logger.Debug("Setdesc command completed successfully")
		return nil
	},
}

// pollRun polls the OpenAI API until a run is completed
func pollRun(ctx context.Context, client *openai.Client, threadID, runID string) (*openai.Run, error) {
	for {
		run, err := client.RetrieveRun(ctx, threadID, runID)
		if err != nil {
			return nil, err
		}

		if run.Status == openai.RunStatusCompleted {
			return &run, nil
		}

		if run.Status == openai.RunStatusFailed || run.Status == openai.RunStatusCancelled || run.Status == openai.RunStatusExpired {
			return nil, fmt.Errorf("run failed with status: %s", run.Status)
		}

		// Wait before polling again
		logger.Debug("Waiting for OpenAI run to complete", map[string]interface{}{
			"status": run.Status,
		})
		time.Sleep(1 * time.Second)
	}
}

func init() {
	rootCmd.AddCommand(setdescCmd)
}

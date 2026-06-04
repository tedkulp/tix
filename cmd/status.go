package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tedkulp/tix/internal/config"
	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/services"
	"github.com/tedkulp/tix/internal/utils"
)

// statusJSON is the JSON output shape for --json flag.
type statusJSON struct {
	Branch        string   `json:"branch"`
	IssueNumber   int      `json:"issue_number"`
	IssueTitle    string   `json:"issue_title"`
	IssueLabels   []string `json:"issue_labels"`
	Milestone     string   `json:"milestone"`
	MRURL         string   `json:"mr_url"`
	MRNumber      int      `json:"mr_number"`
	MRIsDraft     bool     `json:"mr_is_draft"`
	SuggestedNext string   `json:"suggested_next"`
	MRLookupError string   `json:"mr_lookup_error"`
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current ticket, issue, and MR status for this branch",
	Long: `Show the current status of the branch: which issue it is linked to,
whether a merge/pull request exists, and what the suggested next tix command is.

Exits with code 1 if the current branch is not a ticket branch.
Exits with code 1 if the configuration or API calls fail.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Debug("Starting status command")

		jsonOutput, _ := cmd.Flags().GetBool("json")

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("couldn't load configuration file. Run with --verbose for details")
		}

		wd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to determine current directory")
		}

		// Find code repo matching the current directory
		var matchingRepo *config.Repository
		var repoName string
		bestMatchLength := 0

		for i, repo := range cfg.Repositories {
			if !repo.IsCodeRepo() {
				continue
			}
			absRepoDir, err := filepath.Abs(repo.Directory)
			if err != nil {
				continue
			}
			if strings.HasPrefix(wd, absRepoDir) && len(absRepoDir) > bestMatchLength {
				matchingRepo = &cfg.Repositories[i]
				repoName = repo.Name
				bestMatchLength = len(absRepoDir)
			}
		}

		if matchingRepo == nil {
			return fmt.Errorf("no configured repository found for directory %s", wd)
		}

		logger.Debug("Code repo resolved", map[string]interface{}{"repo": repoName})

		currentBranch, err := git.GetBranchFromDir(wd)
		if err != nil {
			return fmt.Errorf("failed to determine current git branch")
		}

		projectName, issueNumber, err := utils.ExtractIssueInfo(currentBranch)
		if err != nil {
			if jsonOutput {
				out := statusJSON{Branch: currentBranch, IssueLabels: []string{}}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(out)
				os.Exit(1)
			}
			return fmt.Errorf("branch %q is not a ticket branch", currentBranch)
		}

		// Build code repo provider
		var codeProvider services.SCMProvider
		if matchingRepo.GitlabRepo != "" {
			codeProvider, err = services.NewGitLabProvider(matchingRepo.GitlabRepo)
		} else {
			codeProvider, err = services.NewGitHubProvider(matchingRepo.GithubRepo)
		}
		if err != nil {
			return fmt.Errorf("failed to create SCM provider: %w", err)
		}

		// Build issue provider (same as code provider unless cross-repo)
		issueProvider := codeProvider
		if projectName != "" && projectName != repoName {
			issueRepo := cfg.GetRepo(projectName)
			if issueRepo == nil {
				return fmt.Errorf("repository %q not found in config", projectName)
			}
			if issueRepo.GitlabRepo != "" {
				issueProvider, err = services.NewGitLabProvider(issueRepo.GitlabRepo)
			} else {
				issueProvider, err = services.NewGitHubProvider(issueRepo.GithubRepo)
			}
			if err != nil {
				return fmt.Errorf("failed to create issue provider: %w", err)
			}
		}

		readyLabel := utils.GetReadyLabel(cfg, matchingRepo, "")

		ws, err := services.GetWorkflowStatus(codeProvider, issueProvider, currentBranch, issueNumber, readyLabel)
		if err != nil {
			return fmt.Errorf("failed to get workflow status: %w", err)
		}

		if jsonOutput {
			labels := ws.IssueLabels
			if labels == nil {
				labels = []string{}
			}
			mrLookupError := ""
			if ws.MRLookupErr != nil {
				mrLookupError = ws.MRLookupErr.Error()
			}
			out := statusJSON{
				Branch:        ws.Branch,
				IssueNumber:   ws.IssueNumber,
				IssueTitle:    ws.IssueTitle,
				IssueLabels:   labels,
				Milestone:     ws.Milestone,
				MRURL:         ws.MRURL,
				MRNumber:      ws.MRNumber,
				MRIsDraft:     ws.MRIsDraft,
				SuggestedNext: ws.SuggestedNext,
				MRLookupError: mrLookupError,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(out)
		}

		// Human-readable output
		fmt.Printf("Branch:    %s\n", ws.Branch)
		fmt.Printf("Issue:     #%d %q\n", ws.IssueNumber, ws.IssueTitle)

		if len(ws.IssueLabels) > 0 {
			fmt.Printf("Labels:    %s\n", strings.Join(ws.IssueLabels, ", "))
		} else {
			fmt.Printf("Labels:    (none)\n")
		}

		if ws.Milestone != "" {
			fmt.Printf("Milestone: %s\n", ws.Milestone)
		} else {
			fmt.Printf("Milestone: (none)\n")
		}

		if ws.MRNumber != 0 {
			draftMarker := ""
			if ws.MRIsDraft {
				draftMarker = " (draft)"
			}
			fmt.Printf("MR:        #%d%s  %s\n", ws.MRNumber, draftMarker, ws.MRURL)
		} else {
			fmt.Printf("MR:        (none)\n")
		}

		if ws.MRLookupErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: MR lookup failed: %v\n", ws.MRLookupErr)
		}

		if ws.SuggestedNext != "" {
			fmt.Printf("Next:      %s\n", ws.SuggestedNext)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(statusCmd)
	statusCmd.Flags().BoolP("json", "j", false, "Output status as JSON")
}

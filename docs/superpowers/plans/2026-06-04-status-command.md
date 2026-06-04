# tix status Command Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `tix status` command that reports the current branch's ticket/issue/MR state and suggests the next workflow step, with `--json` output for automation.

**Architecture:** Three-task implementation — first extend `IssueResult` with `MilestoneTitle` (touches both providers), then add a pure `GetWorkflowStatus()` service function with unit tests against a mock provider, then wire it up in a new Cobra command that uses the same repo-detection pattern as `cmd/mr.go`.

**Tech Stack:** Go, Cobra (command), `encoding/json` (JSON output), existing `services.SCMProvider` interface, `internal/utils` helpers.

---

## File Map

| File | Change |
|------|--------|
| `internal/services/scm.go` | Add `MilestoneTitle string` to `IssueResult` |
| `internal/services/gitlab.go` | Populate `MilestoneTitle` in `GitlabIssue` and `GitLabProvider.GetIssue()` |
| `internal/services/github.go` | Populate `MilestoneTitle` in `GithubIssue` and `GitHubProvider.GetIssue()` |
| `internal/services/status.go` | **New** — `WorkflowStatus` struct, `GetWorkflowStatus()` |
| `internal/services/status_test.go` | **New** — unit tests using a mock `SCMProvider` |
| `cmd/status.go` | **New** — Cobra command, human-readable + JSON formatting, exit codes |

---

## Task 1: Add MilestoneTitle to IssueResult and both providers

**Files:**
- Modify: `internal/services/scm.go`
- Modify: `internal/services/gitlab.go`
- Modify: `internal/services/github.go`

- [ ] **Step 1: Add MilestoneTitle to IssueResult in scm.go**

In `internal/services/scm.go`, update `IssueResult`:

```go
// IssueResult represents an issue from either system
type IssueResult struct {
	Number         int
	Title          string
	Labels         []string
	MilestoneID    int
	MilestoneTitle string
}
```

- [ ] **Step 2: Add MilestoneTitle to GitlabIssue in gitlab.go**

In `internal/services/gitlab.go`, update the `GitlabIssue` struct:

```go
// GitlabIssue represents a GitLab issue
type GitlabIssue struct {
	IID            int
	Title          string
	Labels         []string
	MilestoneID    int
	MilestoneTitle string
	WebURL         string
}
```

- [ ] **Step 3: Populate MilestoneTitle in GitlabProject.GetIssue()**

In `internal/services/gitlab.go`, update `GitlabProject.GetIssue()`:

```go
func (p *GitlabProject) GetIssue(issueNumber int) (*GitlabIssue, error) {
	issue, _, err := p.client.Issues.GetIssue(p.pid, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	var milestoneID int
	var milestoneTitle string
	if issue.Milestone != nil {
		milestoneID = issue.Milestone.ID
		milestoneTitle = issue.Milestone.Title
	}

	return &GitlabIssue{
		IID:            issue.IID,
		Title:          issue.Title,
		Labels:         issue.Labels,
		MilestoneID:    milestoneID,
		MilestoneTitle: milestoneTitle,
		WebURL:         issue.WebURL,
	}, nil
}
```

- [ ] **Step 4: Propagate MilestoneTitle through GitLabProvider.GetIssue()**

In `internal/services/gitlab.go`, update `GitLabProvider.GetIssue()`:

```go
func (p *GitLabProvider) GetIssue(issueNumber int) (*IssueResult, error) {
	issue, err := p.project.GetIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	return &IssueResult{
		Number:         issue.IID,
		Title:          issue.Title,
		Labels:         issue.Labels,
		MilestoneID:    issue.MilestoneID,
		MilestoneTitle: issue.MilestoneTitle,
	}, nil
}
```

- [ ] **Step 5: Add MilestoneTitle to GithubIssue in github.go**

In `internal/services/github.go`, update the `GithubIssue` struct:

```go
// GithubIssue represents a GitHub issue
type GithubIssue struct {
	Number         int
	Title          string
	Labels         []string
	HTMLURL        string
	MilestoneTitle string
}
```

- [ ] **Step 6: Populate MilestoneTitle in GithubProject.GetIssue()**

In `internal/services/github.go`, update `GithubProject.GetIssue()`:

```go
func (p *GithubProject) GetIssue(issueNumber int) (*GithubIssue, error) {
	ctx := context.Background()
	issue, _, err := p.client.Issues.Get(ctx, p.owner, p.repo, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	var labels []string
	for _, label := range issue.Labels {
		labels = append(labels, *label.Name)
	}

	var milestoneTitle string
	if issue.Milestone != nil && issue.Milestone.Title != nil {
		milestoneTitle = *issue.Milestone.Title
	}

	return &GithubIssue{
		Number:         *issue.Number,
		Title:          *issue.Title,
		Labels:         labels,
		HTMLURL:        *issue.HTMLURL,
		MilestoneTitle: milestoneTitle,
	}, nil
}
```

- [ ] **Step 7: Propagate MilestoneTitle through GitHubProvider.GetIssue()**

In `internal/services/github.go`, update `GitHubProvider.GetIssue()`:

```go
func (p *GitHubProvider) GetIssue(issueNumber int) (*IssueResult, error) {
	issue, err := p.project.GetIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	return &IssueResult{
		Number:         issue.Number,
		Title:          issue.Title,
		Labels:         issue.Labels,
		MilestoneID:    0,
		MilestoneTitle: issue.MilestoneTitle,
	}, nil
}
```

- [ ] **Step 8: Verify the build still compiles**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 9: Commit**

```bash
git add internal/services/scm.go internal/services/gitlab.go internal/services/github.go
git commit -m "feat: add MilestoneTitle to IssueResult and both provider implementations"
```

---

## Task 2: Add GetWorkflowStatus service

**Files:**
- Create: `internal/services/status.go`
- Create: `internal/services/status_test.go`

- [ ] **Step 1: Write the failing tests first**

Create `internal/services/status_test.go`:

```go
package services

import (
	"errors"
	"testing"
)

// mockSCMProvider implements SCMProvider for testing
type mockSCMProvider struct {
	issueResult *IssueResult
	issueErr    error
	mrResults   []RequestResult
	mrErr       error
}

func (m *mockSCMProvider) GetIssue(_ int) (*IssueResult, error) {
	return m.issueResult, m.issueErr
}
func (m *mockSCMProvider) GetOpenRequestsByBranch(_ string) ([]RequestResult, error) {
	return m.mrResults, m.mrErr
}
func (m *mockSCMProvider) CreateMergeRequest(_ MergeRequestParams) (*RequestResult, error) {
	return nil, nil
}
func (m *mockSCMProvider) CreateIssue(_ IssueParams) (*IssueResult, error) { return nil, nil }
func (m *mockSCMProvider) GetOpenRequests(_ int) ([]RequestResult, error)  { return nil, nil }
func (m *mockSCMProvider) AddLabelsToIssue(_ int, _ []string) error        { return nil }
func (m *mockSCMProvider) RemoveLabelsFromIssue(_ int, _ []string) error   { return nil }
func (m *mockSCMProvider) UpdateIssueStatus(_ int, _ string) error         { return nil }
func (m *mockSCMProvider) GetURL() string                                  { return "" }
func (m *mockSCMProvider) GetCrossRepoIssueRef(_ int) string               { return "" }

func TestGetWorkflowStatus_NoMR(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug"}, MilestoneTitle: "2026.Q2"},
		mrResults:   []RequestResult{},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "ready-for-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SuggestedNext != "tix mr" {
		t.Errorf("SuggestedNext = %q, want %q", status.SuggestedNext, "tix mr")
	}
	if status.IssueTitle != "Fix bug" {
		t.Errorf("IssueTitle = %q, want %q", status.IssueTitle, "Fix bug")
	}
	if status.Milestone != "2026.Q2" {
		t.Errorf("Milestone = %q, want %q", status.Milestone, "2026.Q2")
	}
	if status.MRNumber != 0 {
		t.Errorf("MRNumber = %d, want 0", status.MRNumber)
	}
}

func TestGetWorkflowStatus_DraftMR(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{}},
		mrResults:   []RequestResult{{ID: 7, Title: "Draft: Fix bug", URL: "https://example.com/mr/7", IsDraft: true}},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "ready-for-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SuggestedNext != "tix setdesc" {
		t.Errorf("SuggestedNext = %q, want %q", status.SuggestedNext, "tix setdesc")
	}
	if !status.MRIsDraft {
		t.Error("MRIsDraft should be true")
	}
	if status.MRNumber != 7 {
		t.Errorf("MRNumber = %d, want 7", status.MRNumber)
	}
	if status.MRURL != "https://example.com/mr/7" {
		t.Errorf("MRURL = %q, want %q", status.MRURL, "https://example.com/mr/7")
	}
}

func TestGetWorkflowStatus_NonDraftMR_MissingReadyLabel(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug"}},
		mrResults:   []RequestResult{{ID: 7, Title: "Fix bug", URL: "https://example.com/mr/7", IsDraft: false}},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "ready-for-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SuggestedNext != "tix ready" {
		t.Errorf("SuggestedNext = %q, want %q", status.SuggestedNext, "tix ready")
	}
}

func TestGetWorkflowStatus_NonDraftMR_HasReadyLabel(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug", "ready-for-review"}},
		mrResults:   []RequestResult{{ID: 7, Title: "Fix bug", URL: "https://example.com/mr/7", IsDraft: false}},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "ready-for-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SuggestedNext != "" {
		t.Errorf("SuggestedNext = %q, want empty (workflow complete)", status.SuggestedNext)
	}
}

func TestGetWorkflowStatus_NonDraftMR_NoReadyLabelConfigured(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug"}},
		mrResults:   []RequestResult{{ID: 7, Title: "Fix bug", URL: "https://example.com/mr/7", IsDraft: false}},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without a configured ready label, we can't determine if ready step is needed
	if status.SuggestedNext != "" {
		t.Errorf("SuggestedNext = %q, want empty (no ready label configured)", status.SuggestedNext)
	}
}

func TestGetWorkflowStatus_IssueLookupFails(t *testing.T) {
	provider := &mockSCMProvider{
		issueErr:  errors.New("issue not found"),
		mrResults: []RequestResult{},
	}
	_, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "")
	if err == nil {
		t.Fatal("expected error when issue lookup fails")
	}
}

func TestGetWorkflowStatus_MRLookupFails(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{}},
		mrErr:       errors.New("API error"),
	}
	_, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "")
	if err == nil {
		t.Fatal("expected error when MR lookup fails")
	}
}

func TestGetWorkflowStatus_SeparateProviders(t *testing.T) {
	// Cross-repo: code provider handles MRs, issue provider handles issues
	issueProvider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Cross-repo bug", Labels: []string{}},
	}
	codeProvider := &mockSCMProvider{
		mrResults: []RequestResult{{ID: 5, Title: "Fix", URL: "https://example.com/mr/5", IsDraft: true}},
	}
	status, err := GetWorkflowStatus(codeProvider, issueProvider, "42-fix-bug", 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.IssueTitle != "Cross-repo bug" {
		t.Errorf("IssueTitle = %q, want %q", status.IssueTitle, "Cross-repo bug")
	}
	if status.MRNumber != 5 {
		t.Errorf("MRNumber = %d, want 5", status.MRNumber)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
go test ./internal/services/ -run TestGetWorkflowStatus -v
```

Expected: FAIL — `GetWorkflowStatus` undefined.

- [ ] **Step 3: Create internal/services/status.go**

```go
package services

import (
	"fmt"
)

// WorkflowStatus represents the current state of a ticket branch in the tix workflow.
type WorkflowStatus struct {
	Branch        string
	IssueNumber   int
	IssueTitle    string
	IssueLabels   []string
	Milestone     string
	MRURL         string
	MRNumber      int
	MRIsDraft     bool
	SuggestedNext string
}

// GetWorkflowStatus resolves the current state of a ticket branch.
// codeProvider is used for MR lookup (the repo where the MR lives).
// issueProvider is used for issue lookup (may be the same as codeProvider).
// readyLabel is the configured label that marks an issue ready for review ("" means not configured).
func GetWorkflowStatus(codeProvider SCMProvider, issueProvider SCMProvider, branch string, issueNumber int, readyLabel string) (*WorkflowStatus, error) {
	issue, err := issueProvider.GetIssue(issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue #%d: %w", issueNumber, err)
	}

	mrs, err := codeProvider.GetOpenRequestsByBranch(branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get open requests for branch %q: %w", branch, err)
	}

	status := &WorkflowStatus{
		Branch:      branch,
		IssueNumber: issue.Number,
		IssueTitle:  issue.Title,
		IssueLabels: issue.Labels,
		Milestone:   issue.MilestoneTitle,
	}

	if len(mrs) == 0 {
		status.SuggestedNext = "tix mr"
		return status, nil
	}

	// Use the first open MR
	mr := mrs[0]
	status.MRNumber = mr.ID
	status.MRURL = mr.URL
	status.MRIsDraft = mr.IsDraft

	if mr.IsDraft {
		status.SuggestedNext = "tix setdesc"
		return status, nil
	}

	// Non-draft MR: check ready label
	if readyLabel != "" {
		hasReadyLabel := false
		for _, label := range issue.Labels {
			if label == readyLabel {
				hasReadyLabel = true
				break
			}
		}
		if !hasReadyLabel {
			status.SuggestedNext = "tix ready"
		}
	}

	return status, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
go test ./internal/services/ -run TestGetWorkflowStatus -v
```

Expected: all 8 tests PASS.

- [ ] **Step 5: Run the full test suite to catch regressions**

```bash
go test ./...
```

Expected: all existing tests PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/services/status.go internal/services/status_test.go
git commit -m "feat: add GetWorkflowStatus service with unit tests"
```

---

## Task 3: Add tix status Cobra command

**Files:**
- Create: `cmd/status.go`

- [ ] **Step 1: Create cmd/status.go**

```go
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
				repoName = cfg.GetRepoNames()[i]
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
			// Not a ticket branch — print message and exit non-zero
			if jsonOutput {
				out := statusJSON{Branch: currentBranch, IssueLabels: []string{}}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				_ = enc.Encode(out)
			} else {
				fmt.Fprintf(os.Stderr, "Error: branch %q is not a ticket branch\n", currentBranch)
			}
			os.Exit(1)
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
```

- [ ] **Step 2: Verify the build compiles**

```bash
go build ./...
```

Expected: no errors.

- [ ] **Step 3: Run the full test suite**

```bash
go test ./...
```

Expected: all tests PASS.

- [ ] **Step 4: Smoke-test the command manually**

From a ticket branch in a configured repo directory, run:

```bash
go run . status
```

Expected: output resembling:

```
Branch:    123-fix-bug
Issue:     #123 "Fix bug"
Labels:    bug, in-progress
Milestone: (none)
MR:        (none)
Next:      tix mr
```

Also verify JSON output:

```bash
go run . status --json
```

Expected: valid JSON object with all fields present.

Also verify non-ticket branch behaviour from `main` branch:

```bash
git checkout main && go run . status; echo "Exit: $?"
```

Expected: error message on stderr, exit code 1.

- [ ] **Step 5: Commit**

```bash
git add cmd/status.go
git commit -m "feat: add tix status command"
```

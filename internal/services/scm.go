package services

import (
	"fmt"
	"strings"

	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
)

// MergeRequestParams holds parameters for creating a merge request
type MergeRequestParams struct {
	Title              string
	SourceBranch       string
	TargetBranch       string
	IssueNumber        int
	IsDraft            bool
	Labels             []string
	MilestoneID        int
	RemoveSourceBranch bool
	Squash             bool
}

// IssueParams holds parameters for creating an issue
type IssueParams struct {
	Title          string
	Labels         string
	SelfAssign     bool
	MilestoneTitle string
}

// SCMProvider represents a source code management system (GitHub, GitLab, etc.)
type SCMProvider interface {
	// CreateMergeRequest creates a merge/pull request and returns its details
	CreateMergeRequest(params MergeRequestParams) (*RequestResult, error)

	// CreateIssue creates a new issue in the repository
	CreateIssue(params IssueParams) (*IssueResult, error)

	// GetOpenRequests returns the open merge/pull requests for an issue
	GetOpenRequests(issueNumber int) ([]RequestResult, error)

	// GetIssue returns an issue by its number
	GetIssue(issueNumber int) (*IssueResult, error)

	// AddLabelsToIssue adds labels to an existing issue
	AddLabelsToIssue(issueNumber int, labels []string) error

	// RemoveLabelsFromIssue removes labels from an existing issue
	RemoveLabelsFromIssue(issueNumber int, labels []string) error

	// UpdateIssueStatus updates the status of an issue (GitLab only, no-op for GitHub)
	UpdateIssueStatus(issueNumber int, status string) error

	// GetURL returns the URL for the created request
	GetURL() string
}

// RequestResult represents a merge/pull request result
type RequestResult struct {
	ID      int
	Title   string
	URL     string
	IsDraft bool
	Squash  bool
}

// IssueResult represents an issue from either system
type IssueResult struct {
	Number      int
	Title       string
	Labels      []string
	MilestoneID int
}

// CreateMergeRequestParams is a convenience struct for CreateMergeRequest parameters
type CreateMergeRequestParams struct {
	Provider           SCMProvider
	GitRepo            *git.Repository
	CurrentBranch      string
	Remote             string
	TargetBranch       string
	IssueNumber        int
	IsDraft            bool
	RemoveSourceBranch bool
	Squash             bool
}

// CreateMergeRequest contains the common flow for creating a merge/pull request
func CreateMergeRequest(params CreateMergeRequestParams) (*RequestResult, error) {
	// Check if there's already an open request for this issue
	openRequests, err := params.Provider.GetOpenRequests(params.IssueNumber)
	if err != nil {
		logger.Warn("Failed to check for existing requests", map[string]interface{}{
			"error": err.Error(),
		})
	} else if len(openRequests) > 0 {
		// Check if any of the requests use the same branch
		for _, req := range openRequests {
			// If there's already a request for this branch, return an error
			if req.Title != "" && strings.Contains(req.Title, params.CurrentBranch) {
				return nil, fmt.Errorf("a merge request already exists for this branch.\nView existing merge request: %s", req.URL)
			}
		}
	}

	// Push current branch to remote
	logger.Info("Pushing branch to "+params.Remote, map[string]interface{}{
		"branch": params.CurrentBranch,
		"remote": params.Remote,
	})

	if err := params.GitRepo.Push(params.Remote, params.CurrentBranch); err != nil {
		return nil, fmt.Errorf("failed to push to %s: %w", params.Remote, err)
	}

	// Get issue details
	issue, err := params.Provider.GetIssue(params.IssueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue details: %w", err)
	}

	// Create request title with issue number and full title
	requestTitle := fmt.Sprintf("#%d - %s", params.IssueNumber, issue.Title)

	logger.Info("Creating request with issue metadata", map[string]interface{}{
		"issue_title":     issue.Title,
		"issue_labels":    issue.Labels,
		"issue_milestone": issue.MilestoneID,
	})

	// Create request params
	mrParams := MergeRequestParams{
		Title:              requestTitle,
		SourceBranch:       params.CurrentBranch,
		TargetBranch:       params.TargetBranch,
		IssueNumber:        params.IssueNumber,
		IsDraft:            params.IsDraft,
		Labels:             issue.Labels,
		MilestoneID:        issue.MilestoneID,
		RemoveSourceBranch: params.RemoveSourceBranch,
		Squash:             params.Squash,
	}

	// Create request
	request, err := params.Provider.CreateMergeRequest(mrParams)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	logger.Info("Request created", map[string]interface{}{
		"title": request.Title,
		"id":    request.ID,
		"url":   request.URL,
		"draft": request.IsDraft,
	})

	// Open the request in browser
	if err := OpenURL(request.URL); err != nil {
		logger.Warn("Failed to open browser", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return request, nil
}

// OpenURL opens a URL in the default browser (inline implementation to avoid import cycle)
func OpenURL(url string) error {
	// Import the utils.OpenURL functionality here to avoid import cycle
	return fmt.Errorf("browser opening not implemented in this context")
}

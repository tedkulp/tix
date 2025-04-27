package services

import (
	"fmt"

	"github.com/tedkulp/tix/internal/git"
	"github.com/tedkulp/tix/internal/logger"
	"github.com/tedkulp/tix/internal/utils"
)

// SCMProvider represents a source code management system (GitHub, GitLab, etc.)
type SCMProvider interface {
	// CreateMergeRequest creates a merge/pull request and returns its details
	CreateMergeRequest(title, sourceBranch, targetBranch string, issueNumber int, isDraft bool, labels []string, milestoneID int, removeSourceBranch bool) (*RequestResult, error)

	// GetOpenRequests returns the open merge/pull requests for an issue
	GetOpenRequests(issueNumber int) ([]RequestResult, error)

	// GetIssue returns an issue by its number
	GetIssue(issueNumber int) (*IssueResult, error)

	// GetURL returns the URL for the created request
	GetURL() string
}

// RequestResult represents a merge/pull request result
type RequestResult struct {
	ID      int
	Title   string
	URL     string
	IsDraft bool
}

// IssueResult represents an issue from either system
type IssueResult struct {
	Number      int
	Title       string
	Labels      []string
	MilestoneID int
}

// CreateMergeRequest contains the common flow for creating a merge/pull request
func CreateMergeRequest(provider SCMProvider, gitRepo *git.Repository, currentBranch, remote, targetBranch string, issueNumber int, isDraft bool, removeSourceBranch bool) (*RequestResult, error) {
	// Check if there's already an open request for this issue
	openRequests, err := provider.GetOpenRequests(issueNumber)
	if err != nil {
		logger.Warn("Failed to check for existing requests", map[string]interface{}{
			"error": err.Error(),
		})
	} else if len(openRequests) > 0 {
		// Check if any of the requests use the same branch
		for _, req := range openRequests {
			// If there's already a request for this branch, return an error
			if req.Title != "" && utils.Contains(req.Title, currentBranch) {
				return nil, fmt.Errorf("a merge request already exists for this branch.\nView existing merge request: %s", req.URL)
			}
		}
	}

	// Push current branch to remote
	logger.Info("Pushing branch to "+remote, map[string]interface{}{
		"branch": currentBranch,
		"remote": remote,
	})

	if err := gitRepo.Push(remote, currentBranch); err != nil {
		return nil, fmt.Errorf("failed to push to %s: %w", remote, err)
	}

	// Get issue details
	issue, err := provider.GetIssue(issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue details: %w", err)
	}

	// Create request title with issue number and full title
	requestTitle := fmt.Sprintf("#%d - %s", issueNumber, issue.Title)

	logger.Info("Creating request with issue metadata", map[string]interface{}{
		"issue_title":     issue.Title,
		"issue_labels":    issue.Labels,
		"issue_milestone": issue.MilestoneID,
	})

	// Create request
	request, err := provider.CreateMergeRequest(
		requestTitle,
		currentBranch,
		targetBranch,
		issueNumber,
		isDraft,
		issue.Labels,
		issue.MilestoneID,
		removeSourceBranch,
	)
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
	if err := utils.OpenURL(request.URL); err != nil {
		logger.Warn("Failed to open browser", map[string]interface{}{
			"error": err.Error(),
		})
	}

	return request, nil
}

package services

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/go-github/v62/github"
	"golang.org/x/oauth2"
)

// GithubProject represents a GitHub repository
type GithubProject struct {
	client *github.Client
	owner  string
	repo   string
}

// GithubIssue represents a GitHub issue
type GithubIssue struct {
	Number int
	Title  string
	Labels []string
}

// GithubPullRequest represents a GitHub pull request
type GithubPullRequest struct {
	Number  int
	Title   string
	HTMLURL string
	IsDraft bool
}

// NewGithubProject creates a new GitHub project client
func NewGithubProject(repoName string) (*GithubProject, error) {
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	parts := strings.Split(repoName, "/")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repository name format: %s", repoName)
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	client := github.NewClient(oauth2.NewClient(context.Background(), ts))

	return &GithubProject{
		client: client,
		owner:  parts[0],
		repo:   parts[1],
	}, nil
}

// CreateIssue creates a new issue in the repository
func (p *GithubProject) CreateIssue(title, labels string, selfAssign bool, milestoneTitle ...string) (*GithubIssue, error) {
	labelSlice := strings.Split(labels, ",")
	for i, label := range labelSlice {
		labelSlice[i] = strings.TrimSpace(label)
	}

	issue := &github.IssueRequest{
		Title:  &title,
		Labels: &labelSlice,
	}

	// Self-assign if requested
	if selfAssign {
		user, _, err := p.client.Users.Get(context.Background(), "")
		if err != nil {
			return nil, fmt.Errorf("failed to get current user: %w", err)
		}
		issue.Assignees = &[]string{*user.Login}
	}

	// Note: milestoneTitle parameter is ignored for GitHub issues since we're
	// only implementing GitLab milestone support per requirements

	result, _, err := p.client.Issues.Create(context.Background(), p.owner, p.repo, issue)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return &GithubIssue{
		Number: *result.Number,
		Title:  *result.Title,
	}, nil
}

// GetOpenPullRequestsForIssue returns all open pull requests related to an issue
func (p *GithubProject) GetOpenPullRequestsForIssue(issueNumber int) ([]*GithubPullRequest, error) {
	opts := &github.PullRequestListOptions{
		State: "open",
	}

	allPRs, _, err := p.client.PullRequests.List(context.Background(), p.owner, p.repo, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list pull requests: %w", err)
	}

	var matchingPRs []*GithubPullRequest
	for _, pr := range allPRs {
		// Check if the body contains a reference to the issue
		issueRef := fmt.Sprintf("#%d", issueNumber)
		issueRefAlt := fmt.Sprintf("Closes #%d", issueNumber)

		if pr.Body != nil && (strings.Contains(*pr.Body, issueRef) || strings.Contains(*pr.Body, issueRefAlt)) {
			matchingPRs = append(matchingPRs, &GithubPullRequest{
				Number:  *pr.Number,
				Title:   *pr.Title,
				HTMLURL: *pr.HTMLURL,
				IsDraft: *pr.Draft,
			})
		}
	}

	return matchingPRs, nil
}

// CreatePullRequest creates a new pull request in the repository
func (p *GithubProject) CreatePullRequest(title, sourceBranch, targetBranch string, issueNumber int, isDraft bool, issueLabels []string) (*GithubPullRequest, error) {
	body := fmt.Sprintf("Closes #%d", issueNumber)

	// Create PR options
	pr := &github.NewPullRequest{
		Title:               &title,
		Head:                &sourceBranch,
		Base:                &targetBranch,
		Body:                &body,
		MaintainerCanModify: github.Bool(true),
		Draft:               &isDraft,
	}

	result, _, err := p.client.PullRequests.Create(context.Background(), p.owner, p.repo, pr)
	if err != nil {
		return nil, fmt.Errorf("failed to create pull request: %w", err)
	}

	// Apply labels after PR creation
	if len(issueLabels) > 0 {
		_, _, err = p.client.Issues.AddLabelsToIssue(context.Background(), p.owner, p.repo, *result.Number, issueLabels)
		if err != nil {
			// Just log the error but don't fail
			fmt.Printf("Warning: Failed to apply labels to pull request: %v\n", err)
		}
	}

	return &GithubPullRequest{
		Number:  *result.Number,
		Title:   *result.Title,
		HTMLURL: *result.HTMLURL,
		IsDraft: *result.Draft,
	}, nil
}

// GetIssue returns an issue by its number
func (p *GithubProject) GetIssue(issueNumber int) (*GithubIssue, error) {
	issue, _, err := p.client.Issues.Get(context.Background(), p.owner, p.repo, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	// Extract label names
	labels := make([]string, 0, len(issue.Labels))
	for _, label := range issue.Labels {
		labels = append(labels, *label.Name)
	}

	return &GithubIssue{
		Number: issue.GetNumber(),
		Title:  issue.GetTitle(),
		Labels: labels,
	}, nil
}

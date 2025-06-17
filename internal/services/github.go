package services

import (
	"context"
	"fmt"
	"io"
	"net/http"
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
	Number  int
	Title   string
	Labels  []string
	HTMLURL string
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

// GetPullRequestDiff returns the diff of a pull request
func (p *GithubProject) GetPullRequestDiff(prNumber int) (string, error) {
	// Get the diff using the raw API
	token := os.Getenv("GITHUB_TOKEN")
	if token == "" {
		return "", fmt.Errorf("GITHUB_TOKEN environment variable is required")
	}

	// Create HTTP client with token
	client := &http.Client{}
	diffURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls/%d", p.owner, p.repo, prNumber)
	req, err := http.NewRequest("GET", diffURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Add("Accept", "application/vnd.github.v3.diff")
	req.Header.Add("Authorization", "token "+token)

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to get pull request diff: %w", err)
	}
	defer resp.Body.Close()

	// Read the diff content
	diffContent, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read diff content: %w", err)
	}

	return string(diffContent), nil
}

// UpdatePullRequestDescription updates the description of a pull request
func (p *GithubProject) UpdatePullRequestDescription(prNumber int, description string) error {
	ctx := context.Background()

	// Get current PR to preserve metadata
	pr, _, err := p.client.PullRequests.Get(ctx, p.owner, p.repo, prNumber)
	if err != nil {
		return fmt.Errorf("failed to get pull request: %w", err)
	}

	// Check if the description contains a reference to an issue and preserve it
	var updatedDescription string
	lines := strings.Split(pr.GetBody(), "\n")
	issueRef := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "Closes #") || strings.HasPrefix(line, "Fixes #") || strings.HasPrefix(line, "Resolves #") {
			issueRef = line
			break
		}
	}

	if issueRef != "" {
		updatedDescription = fmt.Sprintf("%s\n\n%s", issueRef, description)
	} else {
		updatedDescription = description
	}

	// Update the pull request
	updatePR := &github.PullRequest{
		Body: &updatedDescription,
	}

	_, _, err = p.client.PullRequests.Edit(ctx, p.owner, p.repo, prNumber, updatePR)
	if err != nil {
		return fmt.Errorf("failed to update pull request description: %w", err)
	}

	return nil
}

// UpdateIssueDescription updates the description of an issue
func (p *GithubProject) UpdateIssueDescription(issueNumber int, description string) error {
	ctx := context.Background()

	// Update the issue
	updateIssue := &github.IssueRequest{
		Body: &description,
	}

	_, _, err := p.client.Issues.Edit(ctx, p.owner, p.repo, issueNumber, updateIssue)
	if err != nil {
		return fmt.Errorf("failed to update issue description: %w", err)
	}

	return nil
}

// UpdateIssueTitle updates the title of an issue
func (p *GithubProject) UpdateIssueTitle(issueNumber int, title string) error {
	ctx := context.Background()

	// Update the issue
	updateIssue := &github.IssueRequest{
		Title: &title,
	}

	_, _, err := p.client.Issues.Edit(ctx, p.owner, p.repo, issueNumber, updateIssue)
	if err != nil {
		return fmt.Errorf("failed to update issue title: %w", err)
	}

	return nil
}

// GetIssue returns an issue by its number
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

	return &GithubIssue{
		Number:  *issue.Number,
		Title:   *issue.Title,
		Labels:  labels,
		HTMLURL: *issue.HTMLURL,
	}, nil
}

// GitHubProvider adapts GithubProject to the SCMProvider interface
type GitHubProvider struct {
	project *GithubProject
}

// NewGitHubProvider creates a new GitHub provider
func NewGitHubProvider(repo string) (*GitHubProvider, error) {
	project, err := NewGithubProject(repo)
	if err != nil {
		return nil, err
	}

	return &GitHubProvider{
		project: project,
	}, nil
}

// GetOpenRequests returns the open pull requests related to an issue
func (p *GitHubProvider) GetOpenRequests(issueNumber int) ([]RequestResult, error) {
	githubPRs, err := p.project.GetOpenPullRequestsForIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	results := make([]RequestResult, len(githubPRs))
	for i, pr := range githubPRs {
		results[i] = RequestResult{
			ID:      pr.Number,
			Title:   pr.Title,
			URL:     pr.HTMLURL,
			IsDraft: pr.IsDraft,
		}
	}

	return results, nil
}

// CreateMergeRequest implements the SCMProvider interface
func (p *GitHubProvider) CreateMergeRequest(params MergeRequestParams) (*RequestResult, error) {
	// GitHub API doesn't support removeSourceBranch option directly,
	// it would have to be done via repository settings or post-PR operation

	pr, err := p.project.CreatePullRequest(params.Title, params.SourceBranch, params.TargetBranch, params.IssueNumber, params.IsDraft, params.Labels)
	if err != nil {
		// Check for GitHub's "pull request already exists" error
		// GitHub error message contains something like "A pull request already exists for octocat:patch-1."
		if strings.Contains(err.Error(), "pull request already exists") {
			// We don't have the PR number in the error message, but we can try to find it from open PRs
			openPRs, prErr := p.project.GetOpenPullRequestsForIssue(params.IssueNumber)
			if prErr == nil && len(openPRs) > 0 {
				for _, existingPR := range openPRs {
					// Try to find a PR with the matching branch
					if strings.Contains(existingPR.Title, params.SourceBranch) {
						return nil, fmt.Errorf("a pull request already exists for this branch.\nView existing pull request: %s", existingPR.HTMLURL)
					}
				}
			}
			// If we couldn't find the specific PR, at least provide a generic error with repo URL
			repoURL := fmt.Sprintf("https://github.com/%s/%s/pulls", p.project.owner, p.project.repo)
			return nil, fmt.Errorf("a pull request already exists for this branch.\nView your pull requests: %s", repoURL)
		}
		return nil, err
	}

	return &RequestResult{
		ID:      pr.Number,
		Title:   pr.Title,
		URL:     pr.HTMLURL,
		IsDraft: pr.IsDraft,
		Squash:  params.Squash,
	}, nil
}

// GetIssue implements the SCMProvider interface
func (p *GitHubProvider) GetIssue(issueNumber int) (*IssueResult, error) {
	issue, err := p.project.GetIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	// GitHub issues don't have milestoneID in the same format
	// We'll leave it as 0 for now
	return &IssueResult{
		Number:      issue.Number,
		Title:       issue.Title,
		Labels:      issue.Labels,
		MilestoneID: 0, // GitHub uses a different milestone format
	}, nil
}

// GetURL returns the GitHub URL for the repo
func (p *GitHubProvider) GetURL() string {
	return fmt.Sprintf("https://github.com/%s/%s", p.project.owner, p.project.repo)
}

// CreateIssue implements the SCMProvider interface
func (p *GitHubProvider) CreateIssue(params IssueParams) (*IssueResult, error) {
	issue, err := p.project.CreateIssue(params.Title, params.Labels, params.SelfAssign, params.MilestoneTitle)
	if err != nil {
		return nil, err
	}

	return &IssueResult{
		Number: issue.Number,
		Title:  issue.Title,
		Labels: issue.Labels,
	}, nil
}

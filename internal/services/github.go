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
func (p *GithubProject) CreateIssue(title, labels string, milestoneTitle ...string) (*GithubIssue, error) {
	labelSlice := strings.Split(labels, ",")
	for i, label := range labelSlice {
		labelSlice[i] = strings.TrimSpace(label)
	}

	issue := &github.IssueRequest{
		Title:  &title,
		Labels: &labelSlice,
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

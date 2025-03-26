package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/xanzy/go-gitlab"
)

// GitlabProject represents a GitLab repository
type GitlabProject struct {
	client *gitlab.Client
	pid    string
}

// GitlabIssue represents a GitLab issue
type GitlabIssue struct {
	IID   int
	Title string
}

// NewGitlabProject creates a new GitLab project client
func NewGitlabProject(repoName string) (*GitlabProject, error) {
	token := os.Getenv("GITLAB_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("GITLAB_TOKEN environment variable is required")
	}

	client, err := gitlab.NewClient(token)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitLab client: %w", err)
	}

	return &GitlabProject{
		client: client,
		pid:    repoName,
	}, nil
}

// CreateIssue creates a new issue in the repository
func (p *GitlabProject) CreateIssue(title, labels string) (*GitlabIssue, error) {
	labelSlice := strings.Split(labels, ",")
	for i, label := range labelSlice {
		labelSlice[i] = strings.TrimSpace(label)
	}

	labelsOpt := gitlab.LabelOptions(labelSlice)
	opt := &gitlab.CreateIssueOptions{
		Title:  &title,
		Labels: &labelsOpt,
	}

	result, _, err := p.client.Issues.CreateIssue(p.pid, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create issue: %w", err)
	}

	return &GitlabIssue{
		IID:   result.IID,
		Title: result.Title,
	}, nil
}

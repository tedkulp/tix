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

// GitlabMergeRequest represents a GitLab merge request
type GitlabMergeRequest struct {
	IID     int
	Title   string
	WebURL  string
	IsDraft bool
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

// GetMilestoneID returns the ID of a milestone by title
func (p *GitlabProject) GetMilestoneID(title string) (int, error) {
	if title == "" {
		return 0, nil
	}

	// List project milestones to find the one with matching title
	milestones, _, err := p.client.Milestones.ListMilestones(p.pid, &gitlab.ListMilestonesOptions{
		Title: &title,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to list milestones: %w", err)
	}

	// Return the ID if found
	if len(milestones) > 0 {
		return milestones[0].ID, nil
	}

	// Otherwise, create the milestone
	milestone, _, err := p.client.Milestones.CreateMilestone(p.pid, &gitlab.CreateMilestoneOptions{
		Title: &title,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create milestone: %w", err)
	}

	return milestone.ID, nil
}

// CreateIssue creates a new issue in the repository
func (p *GitlabProject) CreateIssue(title, labels string, milestoneTitle ...string) (*GitlabIssue, error) {
	labelSlice := strings.Split(labels, ",")
	for i, label := range labelSlice {
		labelSlice[i] = strings.TrimSpace(label)
	}

	labelsOpt := gitlab.LabelOptions(labelSlice)
	opt := &gitlab.CreateIssueOptions{
		Title:  &title,
		Labels: &labelsOpt,
	}

	// Add milestone if provided
	if len(milestoneTitle) > 0 && milestoneTitle[0] != "" {
		milestoneID, err := p.GetMilestoneID(milestoneTitle[0])
		if err != nil {
			return nil, fmt.Errorf("failed to get milestone ID: %w", err)
		}

		if milestoneID > 0 {
			opt.MilestoneID = &milestoneID
		}
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

// GetOpenMergeRequestsForIssue returns all open merge requests related to an issue
func (p *GitlabProject) GetOpenMergeRequestsForIssue(issueID int) ([]*GitlabMergeRequest, error) {
	opts := &gitlab.ListProjectMergeRequestsOptions{
		State: gitlab.String("opened"),
	}

	allMRs, _, err := p.client.MergeRequests.ListProjectMergeRequests(p.pid, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to list merge requests: %w", err)
	}

	var matchingMRs []*GitlabMergeRequest
	for _, mr := range allMRs {
		// Check if the description contains a reference to the issue
		issueRef := fmt.Sprintf("#%d", issueID)
		issueRefAlt := fmt.Sprintf("Closes #%d", issueID)

		if strings.Contains(mr.Description, issueRef) || strings.Contains(mr.Description, issueRefAlt) {
			isDraft := mr.WorkInProgress || strings.HasPrefix(mr.Title, "Draft:") || strings.HasPrefix(mr.Title, "WIP:")
			matchingMRs = append(matchingMRs, &GitlabMergeRequest{
				IID:     mr.IID,
				Title:   mr.Title,
				WebURL:  mr.WebURL,
				IsDraft: isDraft,
			})
		}
	}

	return matchingMRs, nil
}

// CreateMergeRequest creates a new merge request in the repository
func (p *GitlabProject) CreateMergeRequest(title, sourceBranch, targetBranch string, issueIID int, isDraft bool) (*GitlabMergeRequest, error) {
	// Add "Draft:" prefix if it's a draft MR
	if isDraft {
		title = "Draft: " + title
	}

	description := fmt.Sprintf("Closes #%d", issueIID)

	// Create MR options
	opt := &gitlab.CreateMergeRequestOptions{
		Title:        &title,
		SourceBranch: &sourceBranch,
		TargetBranch: &targetBranch,
		Description:  &description,
	}

	result, _, err := p.client.MergeRequests.CreateMergeRequest(p.pid, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge request: %w", err)
	}

	return &GitlabMergeRequest{
		IID:     result.IID,
		Title:   result.Title,
		WebURL:  result.WebURL,
		IsDraft: isDraft,
	}, nil
}

// GetIssue returns an issue by its number
func (p *GitlabProject) GetIssue(issueNumber int) (*GitlabIssue, error) {
	issue, _, err := p.client.Issues.GetIssue(p.pid, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	return &GitlabIssue{
		IID:   issue.IID,
		Title: issue.Title,
	}, nil
}

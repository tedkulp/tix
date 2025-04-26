package services

import (
	"fmt"
	"os"
	"strings"

	"github.com/tedkulp/tix/internal/logger"
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// GitlabProject represents a GitLab repository
type GitlabProject struct {
	client *gitlab.Client
	pid    string
}

// GitlabIssue represents a GitLab issue
type GitlabIssue struct {
	IID         int
	Title       string
	Labels      []string
	MilestoneID int
	WebURL      string
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
		return 0, fmt.Errorf("failed to list project milestones: %w", err)
	}

	logger.Debug("Project milestones found", map[string]interface{}{
		"milestones": milestones,
	})

	// Return the ID if found at project level
	if len(milestones) > 0 {
		return milestones[0].ID, nil
	}

	// Get project details to find the group
	project, _, err := p.client.Projects.GetProject(p.pid, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get project details: %w", err)
	}

	// Check if project belongs to a group
	if project.Namespace != nil && project.Namespace.Kind == "group" {
		// Get the group ID
		groupID := project.Namespace.ID

		// Set include ancestors to true to check all parent groups
		includeAncestors := true

		// List group milestones to find the one with matching title
		groupMilestones, _, err := p.client.GroupMilestones.ListGroupMilestones(groupID, &gitlab.ListGroupMilestonesOptions{
			Title:            &title,
			IncludeAncestors: &includeAncestors,
		})
		if err != nil {
			return 0, fmt.Errorf("failed to list group milestones: %w", err)
		}

		logger.Debug("Group milestones found", map[string]interface{}{
			"group_id":   groupID,
			"milestones": groupMilestones,
		})

		// Return the ID if found at group level
		if len(groupMilestones) > 0 {
			return groupMilestones[0].ID, nil
		}
	}

	// Otherwise, create the milestone at project level
	milestone, _, err := p.client.Milestones.CreateMilestone(p.pid, &gitlab.CreateMilestoneOptions{
		Title: &title,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to create milestone: %w", err)
	}

	return milestone.ID, nil
}

// CreateIssue creates a new issue in the repository
func (p *GitlabProject) CreateIssue(title, labels string, selfAssign bool, milestoneTitle ...string) (*GitlabIssue, error) {
	labelSlice := strings.Split(labels, ",")
	for i, label := range labelSlice {
		labelSlice[i] = strings.TrimSpace(label)
	}

	labelsOpt := gitlab.LabelOptions(labelSlice)
	opt := &gitlab.CreateIssueOptions{
		Title:  &title,
		Labels: &labelsOpt,
	}

	// Self-assign if requested
	if selfAssign {
		user, _, err := p.client.Users.CurrentUser()
		if err != nil {
			return nil, fmt.Errorf("failed to get current user: %w", err)
		}
		opt.AssigneeIDs = &[]int{user.ID}
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
		State: gitlab.Ptr("opened"),
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
			// isDraft := mr.WorkInProgress || strings.HasPrefix(mr.Title, "Draft:") || strings.HasPrefix(mr.Title, "WIP:")
			isDraft := strings.HasPrefix(mr.Title, "Draft:") || strings.HasPrefix(mr.Title, "WIP:")
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

// GetIssue returns an issue by its number
func (p *GitlabProject) GetIssue(issueNumber int) (*GitlabIssue, error) {
	issue, _, err := p.client.Issues.GetIssue(p.pid, issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue: %w", err)
	}

	var milestoneID int
	if issue.Milestone != nil {
		milestoneID = issue.Milestone.ID
	}

	return &GitlabIssue{
		IID:         issue.IID,
		Title:       issue.Title,
		Labels:      issue.Labels,
		MilestoneID: milestoneID,
		WebURL:      issue.WebURL,
	}, nil
}

// CreateMergeRequest creates a new merge request in the repository
func (p *GitlabProject) CreateMergeRequest(title, sourceBranch, targetBranch string, issueIID int, isDraft bool, issueLabels []string, milestoneid int) (*GitlabMergeRequest, error) {
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

	// Add labels if provided
	if len(issueLabels) > 0 {
		labelsOpt := gitlab.LabelOptions(issueLabels)
		opt.Labels = &labelsOpt
	}

	// Add milestone if provided
	if milestoneid > 0 {
		opt.MilestoneID = &milestoneid
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

// GetMergeRequestDiff returns the diff of a merge request
func (p *GitlabProject) GetMergeRequestDiff(mrIID int) (string, error) {
	// Get all versions of the merge request diffs
	versions, _, err := p.client.MergeRequests.GetMergeRequestDiffVersions(p.pid, mrIID, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get merge request diff versions: %w", err)
	}

	if len(versions) == 0 {
		return "", fmt.Errorf("no diff versions found for merge request")
	}

	// Get the latest version
	latestVersion := versions[0].ID
	diff, _, err := p.client.MergeRequests.GetSingleMergeRequestDiffVersion(p.pid, mrIID, latestVersion, nil)
	if err != nil {
		return "", fmt.Errorf("failed to get merge request diff: %w", err)
	}

	var diffContent strings.Builder
	for _, change := range diff.Diffs {
		diffContent.WriteString(fmt.Sprintf("--- a/%s\n", change.OldPath))
		diffContent.WriteString(fmt.Sprintf("+++ b/%s\n", change.NewPath))
		diffContent.WriteString(change.Diff)
		diffContent.WriteString("\n")
	}

	return diffContent.String(), nil
}

// UpdateMergeRequestDescription updates the description of a merge request
func (p *GitlabProject) UpdateMergeRequestDescription(mrIID int, description string) error {
	// Keep the "Closes #X" reference if it exists
	existingMR, _, err := p.client.MergeRequests.GetMergeRequest(p.pid, mrIID, nil)
	if err != nil {
		return fmt.Errorf("failed to get merge request: %w", err)
	}

	// Check if the description contains a reference to an issue and preserve it
	var updatedDescription string
	lines := strings.Split(existingMR.Description, "\n")
	issueRef := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "Closes #") || strings.HasPrefix(line, "Related to #") {
			issueRef = line
			break
		}
	}

	if issueRef != "" {
		updatedDescription = fmt.Sprintf("%s\n\n%s", issueRef, description)
	} else {
		updatedDescription = description
	}

	// Update merge request
	_, _, err = p.client.MergeRequests.UpdateMergeRequest(p.pid, mrIID, &gitlab.UpdateMergeRequestOptions{
		Description: &updatedDescription,
	})
	if err != nil {
		return fmt.Errorf("failed to update merge request description: %w", err)
	}

	return nil
}

// UpdateIssueDescription updates the description of an issue
func (p *GitlabProject) UpdateIssueDescription(issueIID int, description string) error {
	// Update issue
	_, _, err := p.client.Issues.UpdateIssue(p.pid, issueIID, &gitlab.UpdateIssueOptions{
		Description: &description,
	})
	if err != nil {
		return fmt.Errorf("failed to update issue description: %w", err)
	}

	return nil
}

// UpdateIssueTitle updates the title of an issue
func (p *GitlabProject) UpdateIssueTitle(issueIID int, title string) error {
	// Update issue
	_, _, err := p.client.Issues.UpdateIssue(p.pid, issueIID, &gitlab.UpdateIssueOptions{
		Title: &title,
	})
	if err != nil {
		return fmt.Errorf("failed to update issue title: %w", err)
	}

	return nil
}

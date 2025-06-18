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

// CreateMergeRequestOptions contains the optional parameters for creating a merge request
type CreateMergeRequestOptions struct {
	IsDraft            bool
	IssueLabels        []string
	MilestoneID        int
	RemoveSourceBranch bool
	Squash             bool
}

// GitLabProvider adapts GitlabProject to the SCMProvider interface
type GitLabProvider struct {
	project *GitlabProject
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
func (p *GitlabProject) CreateMergeRequest(title, sourceBranch, targetBranch string, issueIID int, options CreateMergeRequestOptions) (*GitlabMergeRequest, error) {
	// Add "Draft:" prefix if it's a draft MR
	if options.IsDraft {
		title = "Draft: " + title
	}

	description := fmt.Sprintf("Closes #%d", issueIID)

	// Create MR options
	opt := &gitlab.CreateMergeRequestOptions{
		Title:              &title,
		SourceBranch:       &sourceBranch,
		TargetBranch:       &targetBranch,
		Description:        &description,
		RemoveSourceBranch: &options.RemoveSourceBranch,
		Squash:             &options.Squash,
	}

	// Add labels if provided
	if len(options.IssueLabels) > 0 {
		labelsOpt := gitlab.LabelOptions(options.IssueLabels)
		opt.Labels = &labelsOpt
	}

	// Add milestone if provided
	if options.MilestoneID > 0 {
		opt.MilestoneID = &options.MilestoneID
	}

	result, _, err := p.client.MergeRequests.CreateMergeRequest(p.pid, opt)
	if err != nil {
		return nil, fmt.Errorf("failed to create merge request: %w", err)
	}

	return &GitlabMergeRequest{
		IID:     result.IID,
		Title:   result.Title,
		WebURL:  result.WebURL,
		IsDraft: options.IsDraft,
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

// AddLabelsToIssue adds labels to an existing issue
func (p *GitlabProject) AddLabelsToIssue(issueIID int, labels []string) error {
	// Get current issue to preserve existing labels
	issue, _, err := p.client.Issues.GetIssue(p.pid, issueIID, nil)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	// Combine existing labels with new ones (avoid duplicates)
	labelMap := make(map[string]bool)
	for _, label := range issue.Labels {
		labelMap[label] = true
	}
	for _, label := range labels {
		labelMap[strings.TrimSpace(label)] = true
	}

	// Convert back to slice
	var allLabels []string
	for label := range labelMap {
		if label != "" {
			allLabels = append(allLabels, label)
		}
	}

	labelsOpt := gitlab.LabelOptions(allLabels)
	_, _, err = p.client.Issues.UpdateIssue(p.pid, issueIID, &gitlab.UpdateIssueOptions{
		Labels: &labelsOpt,
	})
	if err != nil {
		return fmt.Errorf("failed to add labels to issue: %w", err)
	}

	return nil
}

// RemoveLabelsFromIssue removes labels from an existing issue
func (p *GitlabProject) RemoveLabelsFromIssue(issueIID int, labels []string) error {
	// Get current issue to get existing labels
	issue, _, err := p.client.Issues.GetIssue(p.pid, issueIID, nil)
	if err != nil {
		return fmt.Errorf("failed to get issue: %w", err)
	}

	// Create a map of labels to remove for efficient lookup
	labelsToRemove := make(map[string]bool)
	for _, label := range labels {
		labelsToRemove[strings.TrimSpace(label)] = true
	}

	// Filter out the labels to remove
	var filteredLabels []string
	for _, label := range issue.Labels {
		if !labelsToRemove[label] {
			filteredLabels = append(filteredLabels, label)
		}
	}

	// Update the issue with filtered labels
	labelsOpt := gitlab.LabelOptions(filteredLabels)
	_, _, err = p.client.Issues.UpdateIssue(p.pid, issueIID, &gitlab.UpdateIssueOptions{
		Labels: &labelsOpt,
	})
	if err != nil {
		return fmt.Errorf("failed to remove labels from issue: %w", err)
	}

	return nil
}

// NewGitLabProvider creates a new GitLab provider
func NewGitLabProvider(repo string) (*GitLabProvider, error) {
	project, err := NewGitlabProject(repo)
	if err != nil {
		return nil, err
	}

	return &GitLabProvider{
		project: project,
	}, nil
}

// GetOpenRequests returns the open merge requests related to an issue
func (p *GitLabProvider) GetOpenRequests(issueNumber int) ([]RequestResult, error) {
	gitlabMRs, err := p.project.GetOpenMergeRequestsForIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	results := make([]RequestResult, len(gitlabMRs))
	for i, mr := range gitlabMRs {
		results[i] = RequestResult{
			ID:      mr.IID,
			Title:   mr.Title,
			URL:     mr.WebURL,
			IsDraft: mr.IsDraft,
		}
	}

	return results, nil
}

// CreateMergeRequest implements the SCMProvider interface
func (p *GitLabProvider) CreateMergeRequest(params MergeRequestParams) (*RequestResult, error) {
	options := CreateMergeRequestOptions{
		IsDraft:            params.IsDraft,
		IssueLabels:        params.Labels,
		MilestoneID:        params.MilestoneID,
		RemoveSourceBranch: params.RemoveSourceBranch,
		Squash:             params.Squash,
	}

	mr, err := p.project.CreateMergeRequest(params.Title, params.SourceBranch, params.TargetBranch, params.IssueNumber, options)
	if err != nil {
		// Check if this is a "merge request already exists" error
		if strings.Contains(err.Error(), "Another open merge request already exists for this source branch") {
			// Extract the MR number from the error message - it looks like "!3" at the end
			parts := strings.Split(err.Error(), ":")
			if len(parts) > 0 {
				lastPart := strings.TrimSpace(parts[len(parts)-1])
				mrNumber := strings.Trim(lastPart, "![]")
				mrURL := fmt.Sprintf("https://gitlab.com/%s/-/merge_requests/%s", p.project.pid, mrNumber)
				return nil, fmt.Errorf("a merge request already exists for this branch.\nView existing merge request: %s", mrURL)
			}
		}
		return nil, err
	}

	return &RequestResult{
		ID:      mr.IID,
		Title:   mr.Title,
		URL:     mr.WebURL,
		IsDraft: mr.IsDraft,
		Squash:  params.Squash,
	}, nil
}

// GetIssue implements the SCMProvider interface
func (p *GitLabProvider) GetIssue(issueNumber int) (*IssueResult, error) {
	issue, err := p.project.GetIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	return &IssueResult{
		Number:      issue.IID,
		Title:       issue.Title,
		Labels:      issue.Labels,
		MilestoneID: issue.MilestoneID,
	}, nil
}

// GetURL returns the GitLab URL for the repo
func (p *GitLabProvider) GetURL() string {
	return fmt.Sprintf("https://gitlab.com/%s", p.project.pid)
}

// CreateIssue implements the SCMProvider interface
func (p *GitLabProvider) CreateIssue(params IssueParams) (*IssueResult, error) {
	var milestoneTitle string
	if params.MilestoneTitle != "" {
		milestoneTitle = params.MilestoneTitle
	}

	issue, err := p.project.CreateIssue(params.Title, params.Labels, params.SelfAssign, milestoneTitle)
	if err != nil {
		return nil, err
	}

	return &IssueResult{
		Number: issue.IID,
		Title:  issue.Title,
		Labels: issue.Labels,
	}, nil
}

// AddLabelsToIssue implements the SCMProvider interface
func (p *GitLabProvider) AddLabelsToIssue(issueNumber int, labels []string) error {
	return p.project.AddLabelsToIssue(issueNumber, labels)
}

// RemoveLabelsFromIssue implements the SCMProvider interface
func (p *GitLabProvider) RemoveLabelsFromIssue(issueNumber int, labels []string) error {
	return p.project.RemoveLabelsFromIssue(issueNumber, labels)
}

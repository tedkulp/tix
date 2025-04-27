package services

import (
	"fmt"
)

// MRDescriptionProvider represents a provider for merge/pull request descriptions
type MRDescriptionProvider interface {
	// GetOpenRequestsForIssue returns the open merge/pull requests for an issue
	GetOpenRequestsForIssue(issueNumber int) ([]MRDescriptionResult, error)

	// GetRequestDiff gets the diff for a merge/pull request
	GetRequestDiff(requestID int) (string, error)

	// GetIssueDetails returns an issue by its number
	GetIssueDetails(issueNumber int) (*IssueDetailsResult, error)

	// UpdateRequestDescription updates the description of a merge/pull request
	UpdateRequestDescription(requestID int, description string) error

	// UpdateIssueDescription updates the description of an issue
	UpdateIssueDescription(issueNumber int, description string) error

	// UpdateIssueTitle updates the title of an issue
	UpdateIssueTitle(issueNumber int, title string) error
}

// MRDescriptionResult represents a merge/pull request result
type MRDescriptionResult struct {
	ID     int
	IID    int // For GitLab compatibility
	Title  string
	WebURL string
}

// IssueDetailsResult represents issue details
type IssueDetailsResult struct {
	Number int
	IID    int // For GitLab compatibility
	Title  string
	WebURL string
}

// GetMRInfo retrieves information about the merge/pull request
func GetMRInfo(provider MRDescriptionProvider, issueNumber int) (*MRInfo, error) {
	// Get open MRs for the issue
	openMRs, err := provider.GetOpenRequestsForIssue(issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get open requests: %w", err)
	}

	if len(openMRs) == 0 {
		return nil, fmt.Errorf("no open requests found for issue #%d, run 'mr' command first", issueNumber)
	}

	// Handle multiple MRs case and selection in the calling function
	// For now, just return the info needed for further processing
	return &MRInfo{
		OpenRequests: openMRs,
	}, nil
}

// MRInfo holds information about a merge/pull request
type MRInfo struct {
	OpenRequests []MRDescriptionResult
	SelectedID   int
	Diff         string
	WebURL       string
	IssueURL     string
}

// GitLabMRDescriptionProvider adapts GitlabProject to the MRDescriptionProvider interface
type GitLabMRDescriptionProvider struct {
	project *GitlabProject
}

// NewGitLabMRDescriptionProvider creates a new GitLab MR description provider
func NewGitLabMRDescriptionProvider(repo string) (*GitLabMRDescriptionProvider, error) {
	project, err := NewGitlabProject(repo)
	if err != nil {
		return nil, err
	}

	return &GitLabMRDescriptionProvider{
		project: project,
	}, nil
}

// GetOpenRequestsForIssue returns the open merge requests for an issue
func (p *GitLabMRDescriptionProvider) GetOpenRequestsForIssue(issueNumber int) ([]MRDescriptionResult, error) {
	mrs, err := p.project.GetOpenMergeRequestsForIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	results := make([]MRDescriptionResult, len(mrs))
	for i, mr := range mrs {
		results[i] = MRDescriptionResult{
			ID:     mr.IID,
			IID:    mr.IID,
			Title:  mr.Title,
			WebURL: mr.WebURL,
		}
	}

	return results, nil
}

// GetRequestDiff gets the diff for a merge request
func (p *GitLabMRDescriptionProvider) GetRequestDiff(requestID int) (string, error) {
	return p.project.GetMergeRequestDiff(requestID)
}

// GetIssueDetails returns an issue by its number
func (p *GitLabMRDescriptionProvider) GetIssueDetails(issueNumber int) (*IssueDetailsResult, error) {
	issue, err := p.project.GetIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	return &IssueDetailsResult{
		Number: issue.IID,
		IID:    issue.IID,
		Title:  issue.Title,
		WebURL: issue.WebURL,
	}, nil
}

// UpdateRequestDescription updates the description of a merge request
func (p *GitLabMRDescriptionProvider) UpdateRequestDescription(requestID int, description string) error {
	return p.project.UpdateMergeRequestDescription(requestID, description)
}

// UpdateIssueDescription updates the description of an issue
func (p *GitLabMRDescriptionProvider) UpdateIssueDescription(issueNumber int, description string) error {
	return p.project.UpdateIssueDescription(issueNumber, description)
}

// UpdateIssueTitle updates the title of an issue
func (p *GitLabMRDescriptionProvider) UpdateIssueTitle(issueNumber int, title string) error {
	return p.project.UpdateIssueTitle(issueNumber, title)
}

// GitHubMRDescriptionProvider adapts GithubProject to the MRDescriptionProvider interface
type GitHubMRDescriptionProvider struct {
	project *GithubProject
}

// NewGitHubMRDescriptionProvider creates a new GitHub MR description provider
func NewGitHubMRDescriptionProvider(repo string) (*GitHubMRDescriptionProvider, error) {
	project, err := NewGithubProject(repo)
	if err != nil {
		return nil, err
	}

	return &GitHubMRDescriptionProvider{
		project: project,
	}, nil
}

// GetOpenRequestsForIssue returns the open pull requests for an issue
func (p *GitHubMRDescriptionProvider) GetOpenRequestsForIssue(issueNumber int) ([]MRDescriptionResult, error) {
	prs, err := p.project.GetOpenPullRequestsForIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	results := make([]MRDescriptionResult, len(prs))
	for i, pr := range prs {
		results[i] = MRDescriptionResult{
			ID:     pr.Number,
			IID:    pr.Number, // Use Number for IID in GitHub
			Title:  pr.Title,
			WebURL: pr.HTMLURL,
		}
	}

	return results, nil
}

// GetRequestDiff gets the diff for a pull request
func (p *GitHubMRDescriptionProvider) GetRequestDiff(requestID int) (string, error) {
	return p.project.GetPullRequestDiff(requestID)
}

// GetIssueDetails returns an issue by its number
func (p *GitHubMRDescriptionProvider) GetIssueDetails(issueNumber int) (*IssueDetailsResult, error) {
	issue, err := p.project.GetIssue(issueNumber)
	if err != nil {
		return nil, err
	}

	return &IssueDetailsResult{
		Number: issue.Number,
		IID:    issue.Number, // Use Number for IID in GitHub
		Title:  issue.Title,
		WebURL: issue.HTMLURL,
	}, nil
}

// UpdateRequestDescription updates the description of a pull request
func (p *GitHubMRDescriptionProvider) UpdateRequestDescription(requestID int, description string) error {
	return p.project.UpdatePullRequestDescription(requestID, description)
}

// UpdateIssueDescription updates the description of an issue
func (p *GitHubMRDescriptionProvider) UpdateIssueDescription(issueNumber int, description string) error {
	return p.project.UpdateIssueDescription(issueNumber, description)
}

// UpdateIssueTitle updates the title of an issue
func (p *GitHubMRDescriptionProvider) UpdateIssueTitle(issueNumber int, title string) error {
	return p.project.UpdateIssueTitle(issueNumber, title)
}

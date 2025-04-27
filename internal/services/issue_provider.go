package services

// IssueProvider represents a provider for creating issues
type IssueProvider interface {
	// CreateIssue creates a new issue and returns its details
	CreateIssue(title, labels string, selfAssign bool, milestone string) (*IssueCreationResult, error)
}

// IssueCreationResult represents the result of creating an issue
type IssueCreationResult struct {
	Number int    // Issue number (GitHub) or IID (GitLab)
	Title  string // Issue title
	URL    string // Issue URL
}

// GitLabIssueProvider adapts GitlabProject to the IssueProvider interface
type GitLabIssueProvider struct {
	project *GitlabProject
}

// NewGitLabIssueProvider creates a new GitLab issue provider
func NewGitLabIssueProvider(repo string) (*GitLabIssueProvider, error) {
	project, err := NewGitlabProject(repo)
	if err != nil {
		return nil, err
	}

	return &GitLabIssueProvider{
		project: project,
	}, nil
}

// CreateIssue implements the IssueProvider interface for GitLab
func (p *GitLabIssueProvider) CreateIssue(title, labels string, selfAssign bool, milestone string) (*IssueCreationResult, error) {
	issue, err := p.project.CreateIssue(title, labels, selfAssign, milestone)
	if err != nil {
		return nil, err
	}

	return &IssueCreationResult{
		Number: issue.IID,
		Title:  issue.Title,
		URL:    issue.WebURL,
	}, nil
}

// GitHubIssueProvider adapts GithubProject to the IssueProvider interface
type GitHubIssueProvider struct {
	project *GithubProject
}

// NewGitHubIssueProvider creates a new GitHub issue provider
func NewGitHubIssueProvider(repo string) (*GitHubIssueProvider, error) {
	project, err := NewGithubProject(repo)
	if err != nil {
		return nil, err
	}

	return &GitHubIssueProvider{
		project: project,
	}, nil
}

// CreateIssue implements the IssueProvider interface for GitHub
// Note: GitHub doesn't use the milestone parameter
func (p *GitHubIssueProvider) CreateIssue(title, labels string, selfAssign bool, milestone string) (*IssueCreationResult, error) {
	issue, err := p.project.CreateIssue(title, labels, selfAssign)
	if err != nil {
		return nil, err
	}

	return &IssueCreationResult{
		Number: issue.Number,
		Title:  issue.Title,
		URL:    issue.HTMLURL,
	}, nil
}

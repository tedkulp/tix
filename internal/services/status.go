package services

import (
	"fmt"
)

// WorkflowStatus represents the current state of a ticket branch in the tix workflow.
type WorkflowStatus struct {
	Branch        string
	IssueNumber   int
	IssueTitle    string
	IssueLabels   []string
	Milestone     string
	MRURL         string
	MRNumber      int
	MRIsDraft     bool
	SuggestedNext string
}

// GetWorkflowStatus resolves the current state of a ticket branch.
// codeProvider is used for MR lookup (the repo where the MR lives).
// issueProvider is used for issue lookup (may be the same as codeProvider).
// readyLabel is the configured label that marks an issue ready for review ("" means not configured).
func GetWorkflowStatus(codeProvider SCMProvider, issueProvider SCMProvider, branch string, issueNumber int, readyLabel string) (*WorkflowStatus, error) {
	issue, err := issueProvider.GetIssue(issueNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to get issue #%d: %w", issueNumber, err)
	}

	mrs, err := codeProvider.GetOpenRequestsByBranch(branch)
	if err != nil {
		return nil, fmt.Errorf("failed to get open requests for branch %q: %w", branch, err)
	}

	status := &WorkflowStatus{
		Branch:      branch,
		IssueNumber: issue.Number,
		IssueTitle:  issue.Title,
		IssueLabels: issue.Labels,
		Milestone:   issue.MilestoneTitle,
	}

	if len(mrs) == 0 {
		status.SuggestedNext = "tix mr"
		return status, nil
	}

	// Use the first open MR
	mr := mrs[0]
	status.MRNumber = mr.ID
	status.MRURL = mr.URL
	status.MRIsDraft = mr.IsDraft

	if mr.IsDraft {
		status.SuggestedNext = "tix setdesc"
		return status, nil
	}

	// Non-draft MR: check ready label
	if readyLabel != "" {
		hasReadyLabel := false
		for _, label := range issue.Labels {
			if label == readyLabel {
				hasReadyLabel = true
				break
			}
		}
		if !hasReadyLabel {
			status.SuggestedNext = "tix ready"
		}
	}

	return status, nil
}

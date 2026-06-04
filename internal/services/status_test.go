package services

import (
	"errors"
	"testing"
)

// mockSCMProvider implements SCMProvider for testing
type mockSCMProvider struct {
	issueResult *IssueResult
	issueErr    error
	mrResults   []RequestResult
	mrErr       error
}

func (m *mockSCMProvider) GetIssue(_ int) (*IssueResult, error) {
	return m.issueResult, m.issueErr
}
func (m *mockSCMProvider) GetOpenRequestsByBranch(_ string) ([]RequestResult, error) {
	return m.mrResults, m.mrErr
}
func (m *mockSCMProvider) CreateMergeRequest(_ MergeRequestParams) (*RequestResult, error) {
	return nil, nil
}
func (m *mockSCMProvider) CreateIssue(_ IssueParams) (*IssueResult, error) { return nil, nil }
func (m *mockSCMProvider) GetOpenRequests(_ int) ([]RequestResult, error)  { return nil, nil }
func (m *mockSCMProvider) AddLabelsToIssue(_ int, _ []string) error        { return nil }
func (m *mockSCMProvider) RemoveLabelsFromIssue(_ int, _ []string) error   { return nil }
func (m *mockSCMProvider) UpdateIssueStatus(_ int, _ string) error         { return nil }
func (m *mockSCMProvider) GetURL() string                                  { return "" }
func (m *mockSCMProvider) GetCrossRepoIssueRef(_ int) string               { return "" }

func TestGetWorkflowStatus_NoMR(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug"}, MilestoneTitle: "2026.Q2"},
		mrResults:   []RequestResult{},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "ready-for-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SuggestedNext != "tix mr" {
		t.Errorf("SuggestedNext = %q, want %q", status.SuggestedNext, "tix mr")
	}
	if status.IssueTitle != "Fix bug" {
		t.Errorf("IssueTitle = %q, want %q", status.IssueTitle, "Fix bug")
	}
	if status.Milestone != "2026.Q2" {
		t.Errorf("Milestone = %q, want %q", status.Milestone, "2026.Q2")
	}
	if status.MRNumber != 0 {
		t.Errorf("MRNumber = %d, want 0", status.MRNumber)
	}
}

func TestGetWorkflowStatus_DraftMR(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{}},
		mrResults:   []RequestResult{{ID: 7, Title: "Draft: Fix bug", URL: "https://example.com/mr/7", IsDraft: true}},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "ready-for-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SuggestedNext != "tix setdesc" {
		t.Errorf("SuggestedNext = %q, want %q", status.SuggestedNext, "tix setdesc")
	}
	if !status.MRIsDraft {
		t.Error("MRIsDraft should be true")
	}
	if status.MRNumber != 7 {
		t.Errorf("MRNumber = %d, want 7", status.MRNumber)
	}
	if status.MRURL != "https://example.com/mr/7" {
		t.Errorf("MRURL = %q, want %q", status.MRURL, "https://example.com/mr/7")
	}
}

func TestGetWorkflowStatus_NonDraftMR_MissingReadyLabel(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug"}},
		mrResults:   []RequestResult{{ID: 7, Title: "Fix bug", URL: "https://example.com/mr/7", IsDraft: false}},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "ready-for-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SuggestedNext != "tix ready" {
		t.Errorf("SuggestedNext = %q, want %q", status.SuggestedNext, "tix ready")
	}
}

func TestGetWorkflowStatus_NonDraftMR_HasReadyLabel(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug", "ready-for-review"}},
		mrResults:   []RequestResult{{ID: 7, Title: "Fix bug", URL: "https://example.com/mr/7", IsDraft: false}},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "ready-for-review")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.SuggestedNext != "" {
		t.Errorf("SuggestedNext = %q, want empty (workflow complete)", status.SuggestedNext)
	}
}

func TestGetWorkflowStatus_NonDraftMR_NoReadyLabelConfigured(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug"}},
		mrResults:   []RequestResult{{ID: 7, Title: "Fix bug", URL: "https://example.com/mr/7", IsDraft: false}},
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Without a configured ready label, we can't determine if ready step is needed
	if status.SuggestedNext != "" {
		t.Errorf("SuggestedNext = %q, want empty (no ready label configured)", status.SuggestedNext)
	}
}

func TestGetWorkflowStatus_IssueLookupFails(t *testing.T) {
	provider := &mockSCMProvider{
		issueErr:  errors.New("issue not found"),
		mrResults: []RequestResult{},
	}
	_, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "")
	if err == nil {
		t.Fatal("expected error when issue lookup fails")
	}
}

func TestGetWorkflowStatus_MRLookupFails(t *testing.T) {
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{}},
		mrErr:       errors.New("API error"),
	}
	status, err := GetWorkflowStatus(provider, provider, "42-fix-bug", 42, "")
	if err != nil {
		t.Fatalf("unexpected hard error: %v", err)
	}
	if status.MRLookupErr == nil {
		t.Fatal("expected MRLookupErr to be set when MR lookup fails")
	}
	if status.IssueTitle != "Fix bug" {
		t.Errorf("IssueTitle = %q, want %q — issue info should be preserved on MR lookup failure", status.IssueTitle, "Fix bug")
	}
}

func TestGetWorkflowStatus_SeparateProviders(t *testing.T) {
	// Cross-repo: code provider handles MRs, issue provider handles issues
	issueProvider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Cross-repo bug", Labels: []string{}},
	}
	codeProvider := &mockSCMProvider{
		mrResults: []RequestResult{{ID: 5, Title: "Fix", URL: "https://example.com/mr/5", IsDraft: true}},
	}
	status, err := GetWorkflowStatus(codeProvider, issueProvider, "42-fix-bug", 42, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status.IssueTitle != "Cross-repo bug" {
		t.Errorf("IssueTitle = %q, want %q", status.IssueTitle, "Cross-repo bug")
	}
	if status.MRNumber != 5 {
		t.Errorf("MRNumber = %d, want 5", status.MRNumber)
	}
}

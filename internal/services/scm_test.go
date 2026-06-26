package services

import (
	"errors"
	"testing"
)

// fakePusher records Push calls and returns a configurable error.
type fakePusher struct {
	err    error
	called bool
	remote string
	branch string
}

func (f *fakePusher) Push(remoteName, branchName string) error {
	f.called = true
	f.remote = remoteName
	f.branch = branchName
	return f.err
}

func TestBuildRequestTitle(t *testing.T) {
	tests := []struct {
		name         string
		issueNumber  int
		crossRepoRef string
		issueTitle   string
		want         string
	}{
		{
			name:        "same repo uses simple hash form",
			issueNumber: 225,
			issueTitle:  "some random title",
			want:        "#225: some random title",
		},
		{
			name:         "cross repo uses full reference as prefix",
			issueNumber:  225,
			crossRepoRef: "allocate.co/engineering/devlops/issues#225",
			issueTitle:   "some random title",
			want:         "allocate.co/engineering/devlops/issues#225: some random title",
		},
		{
			name:        "empty title still produces a valid prefix",
			issueNumber: 7,
			issueTitle:  "",
			want:        "#7: ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRequestTitle(tt.issueNumber, tt.crossRepoRef, tt.issueTitle)
			if got != tt.want {
				t.Errorf("buildRequestTitle(%d, %q, %q) = %q, want %q",
					tt.issueNumber, tt.crossRepoRef, tt.issueTitle, got, tt.want)
			}
		})
	}
}

func TestCreateMergeRequest_SameRepo(t *testing.T) {
	pusher := &fakePusher{}
	provider := &mockSCMProvider{
		issueResult:    &IssueResult{Number: 42, Title: "Fix bug", Labels: []string{"bug"}, MilestoneID: 7},
		openResults:    []RequestResult{},
		createMRResult: &RequestResult{ID: 1, Title: "#42: Fix bug", URL: "https://example.com/mr/1"},
	}

	result, err := CreateMergeRequest(CreateMergeRequestParams{
		Provider:           provider,
		GitRepo:            pusher,
		CurrentBranch:      "42-fix-bug",
		Remote:             "origin",
		TargetBranch:       "main",
		IssueNumber:        42,
		IsDraft:            true,
		RemoveSourceBranch: true,
		Squash:             true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.ID != 1 {
		t.Fatalf("expected request result with ID 1, got %+v", result)
	}
	if !pusher.called || pusher.remote != "origin" || pusher.branch != "42-fix-bug" {
		t.Errorf("expected push to origin/42-fix-bug, got called=%v remote=%q branch=%q",
			pusher.called, pusher.remote, pusher.branch)
	}

	got := provider.createMRParams
	if got == nil {
		t.Fatal("CreateMergeRequest was not called on the provider")
	}
	if got.Title != "#42: Fix bug" {
		t.Errorf("Title = %q, want %q", got.Title, "#42: Fix bug")
	}
	if got.Description != "" {
		t.Errorf("Description = %q, want empty for same-repo", got.Description)
	}
	if got.SourceBranch != "42-fix-bug" || got.TargetBranch != "main" {
		t.Errorf("branches = %q -> %q, want 42-fix-bug -> main", got.SourceBranch, got.TargetBranch)
	}
	if got.MilestoneID != 7 || len(got.Labels) != 1 || got.Labels[0] != "bug" {
		t.Errorf("issue metadata not propagated: milestone=%d labels=%v", got.MilestoneID, got.Labels)
	}
	if !got.IsDraft || !got.Squash || !got.RemoveSourceBranch {
		t.Errorf("flags not propagated: draft=%v squash=%v removeSource=%v",
			got.IsDraft, got.Squash, got.RemoveSourceBranch)
	}
}

func TestCreateMergeRequest_CrossRepo(t *testing.T) {
	pusher := &fakePusher{}
	// codeProvider is where the MR is created; it has no issue and is queried by branch.
	codeProvider := &mockSCMProvider{
		mrResults:      []RequestResult{},
		createMRResult: &RequestResult{ID: 9, URL: "https://example.com/mr/9"},
	}
	// issueProvider is the other repo that owns the issue.
	issueProvider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 225, Title: "some random title", Labels: []string{"feature"}},
	}

	_, err := CreateMergeRequest(CreateMergeRequestParams{
		Provider:          codeProvider,
		IssueProvider:     issueProvider,
		GitRepo:           pusher,
		CurrentBranch:     "225-thing",
		Remote:            "origin",
		TargetBranch:      "main",
		IssueNumber:       225,
		CrossRepoIssueRef: "allocate.co/engineering/devlops/issues#225",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := codeProvider.createMRParams
	if got == nil {
		t.Fatal("CreateMergeRequest was not called on the code provider")
	}
	wantTitle := "allocate.co/engineering/devlops/issues#225: some random title"
	if got.Title != wantTitle {
		t.Errorf("Title = %q, want %q", got.Title, wantTitle)
	}
	wantDesc := "Closes allocate.co/engineering/devlops/issues#225"
	if got.Description != wantDesc {
		t.Errorf("Description = %q, want %q", got.Description, wantDesc)
	}
	// Issue metadata must come from the issue provider, not the code provider.
	if len(got.Labels) != 1 || got.Labels[0] != "feature" {
		t.Errorf("Labels = %v, want [feature] from issue provider", got.Labels)
	}
}

func TestCreateMergeRequest_ExistingMRForBranch(t *testing.T) {
	pusher := &fakePusher{}
	provider := &mockSCMProvider{
		openResults: []RequestResult{
			{Title: "#42: existing 42-fix-bug", URL: "https://example.com/mr/existing"},
		},
	}

	_, err := CreateMergeRequest(CreateMergeRequestParams{
		Provider:      provider,
		GitRepo:       pusher,
		CurrentBranch: "42-fix-bug",
		IssueNumber:   42,
	})
	if err == nil {
		t.Fatal("expected error when an MR already exists for the branch")
	}
	if pusher.called {
		t.Error("should not push when an MR already exists for the branch")
	}
	if provider.createMRParams != nil {
		t.Error("should not create an MR when one already exists for the branch")
	}
}

func TestCreateMergeRequest_PushFails(t *testing.T) {
	pusher := &fakePusher{err: errors.New("network down")}
	provider := &mockSCMProvider{
		issueResult: &IssueResult{Number: 42, Title: "Fix bug"},
		openResults: []RequestResult{},
	}

	_, err := CreateMergeRequest(CreateMergeRequestParams{
		Provider:      provider,
		GitRepo:       pusher,
		CurrentBranch: "42-fix-bug",
		Remote:        "origin",
		IssueNumber:   42,
	})
	if err == nil {
		t.Fatal("expected error when push fails")
	}
	if provider.createMRParams != nil {
		t.Error("should not create an MR when push fails")
	}
}

func TestCreateMergeRequest_GetIssueFails(t *testing.T) {
	pusher := &fakePusher{}
	provider := &mockSCMProvider{
		issueErr:    errors.New("issue not found"),
		openResults: []RequestResult{},
	}

	_, err := CreateMergeRequest(CreateMergeRequestParams{
		Provider:      provider,
		GitRepo:       pusher,
		CurrentBranch: "42-fix-bug",
		Remote:        "origin",
		IssueNumber:   42,
	})
	if err == nil {
		t.Fatal("expected error when fetching the issue fails")
	}
	if provider.createMRParams != nil {
		t.Error("should not create an MR when the issue lookup fails")
	}
}

func TestOpenURL_NotImplemented(t *testing.T) {
	// OpenURL is a placeholder in this package to avoid an import cycle; the
	// real implementation lives in utils. It should report that it is a no-op
	// rather than silently succeeding.
	if err := OpenURL("https://example.com"); err == nil {
		t.Error("expected OpenURL to return an error in this context")
	}
}

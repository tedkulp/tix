package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// TestGitlabProjectCreateIssueTrailingCommaOmitsBlankEntry reproduces issue
// #7 for the GitLab provider: gitlab.LabelOptions serializes a labels slice
// by joining with commas, so an unfiltered blank entry from a trailing
// comma (as produced by a blank/whitespace DefaultLabels value plus any
// trailing separator) leaked into the API payload as a trailing comma.
func TestGitlabProjectCreateIssueTrailingCommaOmitsBlankEntry(t *testing.T) {
	var gotBody struct {
		Labels string `json:"labels"`
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v4/projects/owner%2Frepo/issues", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id": 1, "iid": 1, "title": "t"}`)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	client, err := gitlab.NewClient("test-token", gitlab.WithBaseURL(server.URL))
	if err != nil {
		t.Fatalf("create gitlab client: %v", err)
	}

	p := &GitlabProject{client: client, pid: "owner/repo"}

	if _, err := p.CreateIssue("t", "bug,", false); err != nil {
		t.Fatalf("CreateIssue returned error: %v", err)
	}

	if gotBody.Labels != "bug" {
		t.Errorf("expected labels %q, got %q", "bug", gotBody.Labels)
	}
}

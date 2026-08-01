package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v62/github"
)

// TestGithubProjectGetIssueLabelWithoutName reproduces a panic reported in
// issue #5: the GitHub API omits a label's "name" field for some labels
// (e.g. minimal/legacy label representations), which decodes to a nil
// *string. Dereferencing that pointer unconditionally panics.
func TestGithubProjectGetIssueLabelWithoutName(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues/42", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{
			"number": 42,
			"title": "Some issue",
			"html_url": "https://github.com/owner/repo/issues/42",
			"labels": [
				{"color": "ffffff"},
				{"name": "bug", "color": "ff0000"}
			]
		}`)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	client := github.NewClient(nil)
	baseURL, err := url.Parse(server.URL + "/")
	if err != nil {
		t.Fatalf("parse test server URL: %v", err)
	}
	client.BaseURL = baseURL

	p := &GithubProject{client: client, owner: "owner", repo: "repo"}

	issue, err := p.GetIssue(42)
	if err != nil {
		t.Fatalf("GetIssue returned error: %v", err)
	}

	if len(issue.Labels) != 2 {
		t.Fatalf("expected 2 labels, got %d: %v", len(issue.Labels), issue.Labels)
	}
	if issue.Labels[0] != "" {
		t.Errorf("expected empty string for label without a name, got %q", issue.Labels[0])
	}
	if issue.Labels[1] != "bug" {
		t.Errorf("expected second label %q, got %q", "bug", issue.Labels[1])
	}
}

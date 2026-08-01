package services

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-github/v62/github"
)

// TestGithubProjectCreateIssueEmptyLabelsOmitsBlankEntry reproduces issue #7:
// non-interactive issue creation with no configured default labels sent
// []string{""} to the API instead of an empty labels list.
func TestGithubProjectCreateIssueEmptyLabelsOmitsBlankEntry(t *testing.T) {
	var gotBody struct {
		Labels []string `json:"labels"`
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/issues", func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"number": 1, "title": "t"}`)
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

	if _, err := p.CreateIssue("t", "", false); err != nil {
		t.Fatalf("CreateIssue returned error: %v", err)
	}

	if len(gotBody.Labels) != 0 {
		t.Errorf("expected no labels sent for empty input, got %#v", gotBody.Labels)
	}
}

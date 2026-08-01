package services

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

// redirectTransport rewrites every request to point at a local test server,
// so GetPullRequestDiff's hardcoded api.github.com URL can be exercised
// without touching the network or the production code.
type redirectTransport struct {
	targetHost string
	underlying http.RoundTripper
}

func (t *redirectTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req2 := req.Clone(req.Context())
	req2.URL.Scheme = "http"
	req2.URL.Host = t.targetHost
	req2.Host = t.targetHost
	return t.underlying.RoundTrip(req2)
}

// TestGetPullRequestDiffUsesBearerAuthHeader reproduces issue #6: the diff
// download used the deprecated "token" Authorization scheme instead of the
// "bearer" scheme already used by the GraphQL call in this same file.
func TestGetPullRequestDiffUsesBearerAuthHeader(t *testing.T) {
	const testToken = "test-token-123"

	var gotAuthHeader string
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/owner/repo/pulls/7", func(w http.ResponseWriter, r *http.Request) {
		gotAuthHeader = r.Header.Get("Authorization")
		fmt.Fprint(w, "diff --git a/foo b/foo")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	origToken := os.Getenv("GITHUB_TOKEN")
	os.Setenv("GITHUB_TOKEN", testToken)
	defer os.Setenv("GITHUB_TOKEN", origToken)

	origTransport := http.DefaultTransport
	http.DefaultTransport = &redirectTransport{
		targetHost: server.Listener.Addr().String(),
		underlying: origTransport,
	}
	defer func() { http.DefaultTransport = origTransport }()

	p := &GithubProject{owner: "owner", repo: "repo"}

	if _, err := p.GetPullRequestDiff(7); err != nil {
		t.Fatalf("GetPullRequestDiff returned error: %v", err)
	}

	want := "bearer " + testToken
	if gotAuthHeader != want {
		t.Errorf("Authorization header = %q, want %q", gotAuthHeader, want)
	}
}

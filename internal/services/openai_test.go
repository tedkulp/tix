package services

import (
	"strings"
	"testing"
)

func TestShouldUseRAG(t *testing.T) {
	yes, no := true, false
	small := "small diff"
	// EstimateTokenCount is len/4, so this clears the 50000 threshold.
	large := strings.Repeat("x", 200001)

	tests := []struct {
		name     string
		forceRAG *bool
		diff     string
		want     bool
	}{
		{"forced on beats a small diff", &yes, small, true},
		{"forced off beats a large diff", &no, large, false},
		{"auto: small diff goes direct", nil, small, false},
		{"auto: large diff goes RAG", nil, large, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldUseRAG(tt.forceRAG, tt.diff); got != tt.want {
				t.Errorf("shouldUseRAG() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractTitle(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantTitle string
		wantBody  string
	}{
		{
			name:      "heading becomes the title",
			in:        "## A better title\n\n### Summary\n\nBody text.",
			wantTitle: "A better title",
			wantBody:  "### Summary\n\nBody text.",
		},
		{
			name:      "no heading leaves the body untouched",
			in:        "### Summary\n\nBody text.",
			wantTitle: "",
			wantBody:  "### Summary\n\nBody text.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			title, body := extractTitle(tt.in)
			if title != tt.wantTitle {
				t.Errorf("title = %q, want %q", title, tt.wantTitle)
			}
			if body != tt.wantBody {
				t.Errorf("body = %q, want %q", body, tt.wantBody)
			}
		})
	}
}

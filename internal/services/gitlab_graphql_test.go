package services

import (
	"testing"
)

// TestExtractWorkItemUpdateErrorsReportsMutationLevelFailures reproduces
// issue #14: UpdateIssueStatus discarded the workItemUpdate GraphQL
// response entirely, so a mutation that fails at the business-logic level
// (e.g. an invalid status transition) — which GitLab reports via the
// mutation payload's own "errors" field rather than the top-level GraphQL
// "errors" array — went unnoticed and was reported as a success.
func TestExtractWorkItemUpdateErrorsReportsMutationLevelFailures(t *testing.T) {
	data := map[string]any{
		"workItemUpdate": map[string]any{
			"workItem": nil,
			"errors":   []any{"Status transition is not allowed"},
		},
	}

	errs := extractWorkItemUpdateErrors(data)
	if len(errs) != 1 || errs[0] != "Status transition is not allowed" {
		t.Fatalf("expected one mutation error, got %v", errs)
	}
}

func TestExtractWorkItemUpdateErrorsNoErrors(t *testing.T) {
	data := map[string]any{
		"workItemUpdate": map[string]any{
			"workItem": map[string]any{"id": "gid://gitlab/WorkItem/1"},
			"errors":   []any{},
		},
	}

	if errs := extractWorkItemUpdateErrors(data); len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestExtractWorkItemUpdateErrorsMissingPayload(t *testing.T) {
	if errs := extractWorkItemUpdateErrors(map[string]any{}); len(errs) != 0 {
		t.Errorf("expected no errors for missing payload, got %v", errs)
	}
}

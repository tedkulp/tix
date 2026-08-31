package services

import (
	"errors"
	"testing"
)

func TestGetMRInfo_ReturnsOpenRequests(t *testing.T) {
	provider := &mockSCMProvider{openResults: []RequestResult{{ID: 7, Title: "fix"}}}

	info, err := GetMRInfo(provider, 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.OpenRequests) != 1 || info.OpenRequests[0].ID != 7 {
		t.Fatalf("got %+v, want one request with ID 7", info.OpenRequests)
	}
}

func TestGetMRInfo_NoRequests(t *testing.T) {
	_, err := GetMRInfo(&mockSCMProvider{}, 42)
	if err == nil {
		t.Fatal("expected an error when no open requests exist")
	}
}

func TestGetMRInfo_ProviderError(t *testing.T) {
	_, err := GetMRInfo(&mockSCMProvider{openErr: errors.New("api down")}, 42)
	if err == nil {
		t.Fatal("expected the provider error to surface")
	}
}

func TestGetMRInfoByBranch_ReturnsOpenRequests(t *testing.T) {
	provider := &mockSCMProvider{mrResults: []RequestResult{{ID: 9, Title: "feat"}}}

	info, err := GetMRInfoByBranch(provider, "issue-42-feat")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(info.OpenRequests) != 1 || info.OpenRequests[0].ID != 9 {
		t.Fatalf("got %+v, want one request with ID 9", info.OpenRequests)
	}
}

func TestGetMRInfoByBranch_NoRequests(t *testing.T) {
	_, err := GetMRInfoByBranch(&mockSCMProvider{}, "issue-42-feat")
	if err == nil {
		t.Fatal("expected an error when no open requests exist")
	}
}

package utils

import (
	"testing"
	"time"

	"github.com/qovery/qovery-client-go"
)

func TestIsLatestCommitKeyword(t *testing.T) {
	tests := []struct {
		value    string
		expected bool
	}{
		{"latest", true},
		{"LATEST", true},
		{"Latest", true},
		{" latest ", true},
		{"\tlatest\n", true},
		{"", false},
		{"a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0", false},
		{"latest-branch", false},
		{"v1.0.0", false},
	}

	for _, test := range tests {
		if got := IsLatestCommitKeyword(test.value); got != test.expected {
			t.Errorf("IsLatestCommitKeyword(%q) = %v, want %v", test.value, got, test.expected)
		}
	}
}

func TestNewestCommit(t *testing.T) {
	oldest := commitAt("oldest", "2024-01-01T00:00:00Z")
	middle := commitAt("middle", "2024-06-01T00:00:00Z")
	newest := commitAt("newest", "2024-12-01T00:00:00Z")

	tests := []struct {
		name     string
		commits  []qovery.Commit
		expected string
	}{
		{"newest first", []qovery.Commit{newest, middle, oldest}, "newest"},
		{"oldest first", []qovery.Commit{oldest, middle, newest}, "newest"},
		{"unordered", []qovery.Commit{middle, newest, oldest}, "newest"},
		{"single commit", []qovery.Commit{middle}, "middle"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			commit, err := newestCommit(test.commits)
			if err != nil {
				t.Fatalf("newestCommit returned an error: %v", err)
			}
			if commit.GitCommitId != test.expected {
				t.Errorf("newestCommit = %q, want %q", commit.GitCommitId, test.expected)
			}
		})
	}
}

func TestNewestCommitEmptyList(t *testing.T) {
	if _, err := newestCommit(nil); err == nil {
		t.Error("newestCommit(nil) returned no error, want one")
	}

	if _, err := newestCommit([]qovery.Commit{}); err == nil {
		t.Error("newestCommit([]) returned no error, want one")
	}
}

func TestNewestCommitTieKeepsFirst(t *testing.T) {
	first := commitAt("first", "2024-06-01T00:00:00Z")
	second := commitAt("second", "2024-06-01T00:00:00Z")

	commit, err := newestCommit([]qovery.Commit{first, second})
	if err != nil {
		t.Fatalf("newestCommit returned an error: %v", err)
	}
	if commit.GitCommitId != "first" {
		t.Errorf("newestCommit = %q, want %q", commit.GitCommitId, "first")
	}
}

func TestFirstLine(t *testing.T) {
	tests := []struct {
		message  string
		expected string
	}{
		{"feat: add thing", "feat: add thing"},
		{"feat: add thing\n\nlong body\nmore body", "feat: add thing"},
		{"  feat: add thing  \nbody", "feat: add thing"},
		{"", ""},
	}

	for _, test := range tests {
		if got := firstLine(test.message); got != test.expected {
			t.Errorf("firstLine(%q) = %q, want %q", test.message, got, test.expected)
		}
	}
}

func commitAt(gitCommitId string, createdAt string) qovery.Commit {
	parsed, err := time.Parse(time.RFC3339, createdAt)
	if err != nil {
		panic(err)
	}

	return qovery.Commit{
		GitCommitId: gitCommitId,
		CreatedAt:   parsed,
		Message:     "commit " + gitCommitId,
	}
}

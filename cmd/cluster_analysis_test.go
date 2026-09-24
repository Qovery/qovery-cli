package cmd

import (
	"testing"

	"github.com/qovery/qovery-client-go"
)

func TestParseAnalysisOutput(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected qovery.ClusterAnalysisOutputFormat
		wantErr  bool
	}{
		{
			name:     "empty defaults to json",
			input:    "",
			expected: qovery.CLUSTERANALYSISOUTPUTFORMAT_JSON,
		},
		{
			name:     "json",
			input:    "json",
			expected: qovery.CLUSTERANALYSISOUTPUTFORMAT_JSON,
		},
		{
			name:     "table",
			input:    "table",
			expected: qovery.CLUSTERANALYSISOUTPUTFORMAT_TABLE,
		},
		{
			name:     "csv",
			input:    "csv",
			expected: qovery.CLUSTERANALYSISOUTPUTFORMAT_CSV,
		},
		{
			name:     "trims and ignores case",
			input:    " CSV ",
			expected: qovery.CLUSTERANALYSISOUTPUTFORMAT_CSV,
		},
		{
			name:    "invalid format",
			input:   "yaml",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAnalysisOutput(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if got != tt.expected {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}

func TestIsFinalAnalysisStatus(t *testing.T) {
	tests := []struct {
		status   qovery.ClusterAnalysisStatus
		expected bool
	}{
		{status: qovery.CLUSTERANALYSISSTATUS_PENDING, expected: false},
		{status: qovery.CLUSTERANALYSISSTATUS_RUNNING, expected: false},
		{status: qovery.CLUSTERANALYSISSTATUS_SUCCEEDED, expected: true},
		{status: qovery.CLUSTERANALYSISSTATUS_FAILED, expected: true},
		{status: qovery.CLUSTERANALYSISSTATUS_TERMINATED, expected: true},
	}

	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			got := isFinalAnalysisStatus(tt.status)
			if got != tt.expected {
				t.Fatalf("expected %t, got %t", tt.expected, got)
			}
		})
	}
}

package pkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeExecutionId(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
		valid bool
	}{
		{
			name:  "timestamp trailing a suffixed key is stripped",
			input: "0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211-1791383662",
			want:  "0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211",
			valid: true,
		},
		{
			name:  "a lone trailing timestamp is stripped down to the bare uuid",
			input: "ef83256b-5df5-40f0-87a2-a30b62998836-1791386238",
			want:  "ef83256b-5df5-40f0-87a2-a30b62998836",
			valid: true,
		},
		{
			name:  "a suffix shorter than a timestamp is part of the key",
			input: "0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211",
			want:  "0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211",
			valid: true,
		},
		{
			name:  "a bare uuid is kept",
			input: "0781cf11-05c2-4844-8f4e-d72f73a37bfa",
			want:  "0781cf11-05c2-4844-8f4e-d72f73a37bfa",
			valid: true,
		},
		{
			name:  "not a uuid",
			input: "not-an-execution-id",
			valid: false,
		},
		{
			name:  "uuid with a non numeric suffix",
			input: "0781cf11-05c2-4844-8f4e-d72f73a37bfa-abc",
			valid: false,
		},
		{
			name:  "uuid with an uppercase hex digit",
			input: "0781CF11-05c2-4844-8f4e-d72f73a37bfa-1211",
			valid: false,
		},
		{
			name:  "a nine digit suffix is not a timestamp and is kept",
			input: "0781cf11-05c2-4844-8f4e-d72f73a37bfa-123456789",
			want:  "0781cf11-05c2-4844-8f4e-d72f73a37bfa-123456789",
			valid: true,
		},
		{
			name:  "more suffixes than we know how to read",
			input: "0781cf11-05c2-4844-8f4e-d72f73a37bfa-1211-1791383662-42",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := normalizeExecutionId(tt.input)
			assert.Equal(t, tt.valid, ok)
			if tt.valid {
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

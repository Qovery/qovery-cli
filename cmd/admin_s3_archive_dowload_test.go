package cmd

import (
	"testing"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestDownloadS3ArchiveRejectsInvalidFlags(t *testing.T) {
	tests := []struct {
		name    string
		execId  string
		profile string
		request bool
		dir     string
		want    string
	}{
		{"missing exec id", "", pkg.DefaultArchiveProfile, true, "", "--exec-id is required"},
		{"empty profile", "exec-id", "", true, "", "--profile must not be empty"},
		{"directory with request only", "exec-id", pkg.DefaultArchiveProfile, true, "/tmp", "--directory cannot be used with --request-only"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &cobra.Command{}
			cmd.Flags().String("directory", ".", "")
			if tt.dir != "" {
				assert.NoError(t, cmd.Flags().Set("directory", tt.dir))
			}
			oldExecId, oldProfile, oldRequestOnly := execId, archiveProfile, requestOnly
			t.Cleanup(func() { execId, archiveProfile, requestOnly = oldExecId, oldProfile, oldRequestOnly })
			execId, archiveProfile, requestOnly = tt.execId, tt.profile, tt.request

			assert.EqualError(t, downloadS3Archive(cmd), tt.want)
		})
	}
}

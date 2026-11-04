package pkg

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
)

func TestDescribeArchiveError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "missing archive profile",
			err:  config.SharedConfigProfileNotExistError{Profile: "qovery-archive-reader"},
			want: "AWS profile 'qovery-archive-reader' not found",
		},
		{
			name: "missing source profile names the source profile",
			err: config.SharedConfigAssumeRoleError{
				Profile: "qovery-prdo",
				Err:     config.SharedConfigProfileNotExistError{Profile: "qovery-prdo"},
			},
			want: "AWS profile 'qovery-prdo' not found",
		},
		{
			name: "sso session never opened",
			err:  fmt.Errorf("get credentials: %w", errors.New("failed to read cached SSO token file, open /x.json: no such file or directory")),
			want: "No valid AWS SSO session",
		},
		{
			name: "sso session expired",
			err:  errors.New("cached SSO token is expired, or not present, and cannot be refreshed"),
			want: "No valid AWS SSO session",
		},
		{
			name: "sso refresh rejected",
			err:  &smithy.GenericAPIError{Code: "InvalidGrantException"},
			want: "No valid AWS SSO session",
		},
		{
			name: "access denied",
			err:  &smithy.GenericAPIError{Code: "AccessDenied"},
			want: "Access denied to archive key.tgz",
		},
		{
			name: "other error",
			err:  errors.New("boom"),
			want: "Could not download archive key.tgz with AWS profile 'qovery-archive-reader': boom",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Contains(t, describeArchiveError(tt.err, DefaultArchiveProfile, "key.tgz"), tt.want)
		})
	}
}

func TestWritePrivateFileRestrictsAnExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "archive.tgz")
	assert.NoError(t, os.WriteFile(path, []byte("old"), 0644))

	assert.NoError(t, writePrivateFile(path, []byte("new")))

	info, err := os.Stat(path)
	assert.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
	content, err := os.ReadFile(path)
	assert.NoError(t, err)
	assert.Equal(t, "new", string(content))
}

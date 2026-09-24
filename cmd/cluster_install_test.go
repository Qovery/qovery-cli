package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateClusterInstallBaseValuesFile(t *testing.T) {
	t.Run("accepts an existing file", func(t *testing.T) {
		baseValuesFile := filepath.Join(t.TempDir(), "values-scaleway.yaml")
		if err := os.WriteFile(baseValuesFile, []byte("services: {}\n"), 0o600); err != nil {
			t.Fatalf("create base values file: %v", err)
		}

		validatedPath, err := validateClusterInstallBaseValuesFile(baseValuesFile)
		if err != nil {
			t.Fatalf("validate base values file: %v", err)
		}
		if validatedPath != baseValuesFile {
			t.Fatalf("expected %q, got %q", baseValuesFile, validatedPath)
		}
	})

	t.Run("rejects a directory", func(t *testing.T) {
		_, err := validateClusterInstallBaseValuesFile(t.TempDir())
		if err == nil {
			t.Fatal("expected an error for a directory path")
		}
	})
}

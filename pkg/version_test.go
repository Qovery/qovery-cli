package pkg

import (
	"testing"

	semver "github.com/Masterminds/semver/v3"
)

func TestVersion(t *testing.T) {
	// Increment version here when bumping CLI
	c, err := semver.NewConstraint("0.101.0") // ci-version-check
	if err != nil {
		t.Errorf("Error parsing constraint: %s", err)
	}
	if isValidVersion, err := c.Validate(GetCurrentVersion()); !isValidVersion {
		t.Errorf("Version doesn't match expected one: %s", err)
	}
}

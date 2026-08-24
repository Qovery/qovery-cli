package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateDemoChartPath(t *testing.T) {
	chartPath := t.TempDir()
	for _, filename := range []string{"Chart.yaml", "values-demo-local.yaml"} {
		if err := os.WriteFile(filepath.Join(chartPath, filename), nil, 0o600); err != nil {
			t.Fatalf("create %s: %v", filename, err)
		}
	}

	validatedPath, err := validateDemoChartPath(chartPath)
	if err != nil {
		t.Fatalf("validate demo chart path: %v", err)
	}
	if validatedPath != chartPath {
		t.Fatalf("expected %q, got %q", chartPath, validatedPath)
	}
}

func TestDemoEngineImageOverride(t *testing.T) {
	repository, tag, err := demoEngineImageOverride("qovery-demo-engine:local")
	if err != nil {
		t.Fatalf("parse engine image: %v", err)
	}
	if repository != "docker.io/library/qovery-demo-engine" || tag != "local" {
		t.Fatalf("expected docker.io/library/qovery-demo-engine:local, got %s:%s", repository, tag)
	}
}

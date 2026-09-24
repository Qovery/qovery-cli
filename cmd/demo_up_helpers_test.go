package cmd

import "testing"

func TestSplitImageReference(t *testing.T) {
	t.Run("simple image", func(t *testing.T) {
		repository, tag, err := splitImageReference("qovery-demo-engine:local")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if repository != "qovery-demo-engine" {
			t.Fatalf("expected repository qovery-demo-engine, got %s", repository)
		}

		if tag != "local" {
			t.Fatalf("expected tag local, got %s", tag)
		}
	})

	t.Run("registry with port", func(t *testing.T) {
		repository, tag, err := splitImageReference("localhost:5001/qovery-demo-engine:dev")
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		if repository != "localhost:5001/qovery-demo-engine" {
			t.Fatalf("expected repository localhost:5001/qovery-demo-engine, got %s", repository)
		}

		if tag != "dev" {
			t.Fatalf("expected tag dev, got %s", tag)
		}
	})

	t.Run("missing tag", func(t *testing.T) {
		_, _, err := splitImageReference("qovery-demo-engine")
		if err == nil {
			t.Fatal("expected an error for image without tag")
		}
	})
}

func TestNormalizeImageRepository(t *testing.T) {
	t.Run("short docker hub image", func(t *testing.T) {
		repository := normalizeImageRepository("qovery-demo-engine")
		if repository != "docker.io/library/qovery-demo-engine" {
			t.Fatalf("expected docker.io/library/qovery-demo-engine, got %s", repository)
		}
	})

	t.Run("docker hub namespace image", func(t *testing.T) {
		repository := normalizeImageRepository("qovery/demo-engine")
		if repository != "docker.io/qovery/demo-engine" {
			t.Fatalf("expected docker.io/qovery/demo-engine, got %s", repository)
		}
	})

	t.Run("explicit registry image", func(t *testing.T) {
		repository := normalizeImageRepository("ghcr.io/qovery/demo-engine")
		if repository != "ghcr.io/qovery/demo-engine" {
			t.Fatalf("expected ghcr.io/qovery/demo-engine, got %s", repository)
		}
	})
}

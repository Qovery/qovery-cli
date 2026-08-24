package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func expandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return homeDir, nil
	}

	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~"+string(filepath.Separator)) {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(homeDir, strings.TrimPrefix(strings.TrimPrefix(path, "~/"), "~"+string(filepath.Separator))), nil
	}

	return path, nil
}

func splitImageReference(image string) (string, string, error) {
	if image != strings.TrimSpace(image) {
		return "", "", fmt.Errorf("engine image cannot contain surrounding whitespace")
	}
	trimmedImage := strings.TrimSpace(image)
	if trimmedImage == "" {
		return "", "", fmt.Errorf("engine image cannot be empty")
	}

	lastSlash := strings.LastIndex(trimmedImage, "/")
	if strings.Contains(trimmedImage, "@") {
		return "", "", fmt.Errorf("engine image must include an explicit tag, not a digest: %s", trimmedImage)
	}
	lastColon := strings.LastIndex(trimmedImage, ":")
	if lastColon <= lastSlash {
		return "", "", fmt.Errorf("engine image must include an explicit tag: got %s", trimmedImage)
	}

	repository := trimmedImage[:lastColon]
	tag := trimmedImage[lastColon+1:]
	if repository == "" || tag == "" {
		return "", "", fmt.Errorf("invalid engine image reference: %s", trimmedImage)
	}

	return repository, tag, nil
}

func normalizeImageRepository(repository string) string {
	if repository == "" {
		return repository
	}

	parts := strings.Split(repository, "/")
	firstPart := parts[0]
	if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") || firstPart == "localhost" {
		return repository
	}

	if len(parts) == 1 {
		return "docker.io/library/" + repository
	}

	return "docker.io/" + repository
}

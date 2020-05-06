package io

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

func CurrentDockerfileContent() string {
	path, _ := os.Getwd()
	return CurrentDockerfileContentFromPath(path)
}

func CurrentDockerfileContentFromPath(path string) string {
	absolutePath := filepath.Join(path, "Dockerfile")
	if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
		if path == "" {
			return ""
		}

		return CurrentDockerfileContentFromPath(GetAbsoluteParentPath(path))
	}

	contentBytes, _ := ioutil.ReadFile(absolutePath)
	return string(contentBytes)
}

func ExposePortsFromCurrentDockerfile() []string {
	dockerfileContent := strings.Split(CurrentDockerfileContent(), "\n")

	var ports []string

	for _, v := range dockerfileContent {
		if strings.HasPrefix(v, "EXPOSE") {
			ports = append(ports, strings.Split(v, " ")[1])
		}
	}

	return ports
}

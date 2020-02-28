package util

import (
	"io/ioutil"
	"strings"
)

func CurrentDockerfileContent() string {
	contentBytes, _ := ioutil.ReadFile("Dockerfile")
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

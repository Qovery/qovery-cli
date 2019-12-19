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
	s := strings.Split(CurrentDockerfileContent(), "\n")

	var ports []string

	for _, v := range s {
		if strings.Contains(strings.ToLower(v), "expose") {
			ports = append(ports, strings.Split(v, " ")[1])
		}
	}

	return ports
}

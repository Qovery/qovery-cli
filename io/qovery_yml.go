package io

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

type QoveryYML struct {
	Application QoveryYMLApplication `yaml:"application,omitempty"`
	Databases   []QoveryYMLDatabase  `yaml:"databases,omitempty"`
	Brokers     []QoveryYMLBroker    `yaml:"brokers,omitempty"`
	// Storage   []QoveryYMLStorage  `yaml:"storage"`
	Routers []QoveryYMLRouter `yaml:"routers,omitempty"`
}

type QoveryYMLApplication struct {
	Name               string `yaml:"name,omitempty"`
	Project            string `yaml:"project,omitempty"`
	PubliclyAccessible bool   `yaml:"publicly_accessible,omitempty"`
	Dockerfile         string `yaml:"dockerfile,omitempty"`
	Organization       string `yaml:"organization,omitempty"`
}

func (q *QoveryYMLApplication) GetSanitizeName() string {
	return strings.ToLower(q.Name)
}

func (q *QoveryYMLApplication) DockerfilePath() string {
	dockerfilePath := q.Dockerfile
	if dockerfilePath == "" {
		dockerfilePath = "Dockerfile"
	}

	return dockerfilePath
}

type QoveryYMLDatabase struct {
	Type    string `yaml:"type,omitempty"`
	Version string `yaml:"version,omitempty"`
	Name    string `yaml:"name,omitempty"`
}

type QoveryYMLRouter struct {
	Name   string           `yaml:"name,omitempty"`
	DNS    string           `yaml:"dns,omitempty"`
	Routes []QoveryYMLRoute `yaml:"routes,omitempty"`
}

type QoveryYMLRoute struct {
	ApplicationName string   `yaml:"application_name,omitempty"`
	Paths           []string `yaml:"paths,omitempty"`
}

type QoveryYMLBroker struct {
	Type    string `yaml:"type,omitempty"`
	Version string `yaml:"version,omitempty"`
	Name    string `yaml:"name,omitempty"`
}

type QoveryYMLStorage struct {
	Type    string `yaml:"type,omitempty"`
	Version string `yaml:"version,omitempty"`
	Name    string `yaml:"name,omitempty"`
}

func CurrentQoveryYML(envNameForConfigValidation string, checkconfig bool) (QoveryYML, error) {
	path, _ := os.Getwd()
	return CurrentQoveryYMLFromPath(path, envNameForConfigValidation, checkconfig)
}

func CurrentQoveryYMLFromPath(path string, envNameForConfigValidation string, checkConfig bool) (QoveryYML, error) {
	q := QoveryYML{}

	absolutePath := filepath.Join(path, ".qovery.yml")
	if _, err := os.Stat(absolutePath); os.IsNotExist(err) {
		if path == "" {
			return q, err
		}

		return CurrentQoveryYMLFromPath(GetAbsoluteParentPath(path), envNameForConfigValidation, checkConfig)
	}

	f, err := ioutil.ReadFile(absolutePath)

	if err != nil {
		return q, err
	}

	_ = yaml.Unmarshal(f, &q)

	if checkConfig {
		configIsValid := validateConfig(string(f), CurrentDockerfileContent(), envNameForConfigValidation)
		if !configIsValid {
			os.Exit(1)
		}
	}

	return q, nil
}

func validateConfig(qoveryYMLContent string, dockerfileContent string, envName string) bool {
	response := DoCheckConfiguration(ConfigurationCheckRequest{
		QoveryYMLContent:  qoveryYMLContent,
		DockerfileContent: dockerfileContent,
		EnvironmentName:   envName,
	})

	if response.Valid {
		return true
	}

	for _, err := range response.Errors {
		PrintError(err.Reason)
		PrintSolution(err.Hint)
		println()
	}

	fmt.Printf("Total errors found: %d", len(response.Errors))

	return false
}

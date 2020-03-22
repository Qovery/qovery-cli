package util

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
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
	CloudRegion        string `yaml:"cloud_region,omitempty"`
	PubliclyAccessible bool   `yaml:"publicly_accessible,omitempty"`
	Dockerfile         string `yaml:"dockerfile,omitempty"`
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

func CurrentQoveryYML() (QoveryYML, error) {
	q := QoveryYML{}

	if _, err := os.Stat(".qovery.yml"); os.IsNotExist(err) {
		return q, err
	}

	f, err := ioutil.ReadFile(".qovery.yml")

	if err != nil {
		return q, err
	}

	_ = yaml.Unmarshal(f, &q)

	configIsValid := validateConfig(q)
	if configIsValid == false {
		os.Exit(1)
	}

	return q, nil
}

func validateConfig(qoveryYML QoveryYML) bool {
	counter := 0

	if CurrentBranchName() == "" {
		PrintError("Unable to find the current branch name")
		PrintSolution("Please 'git checkout' to a valid branch name")
		counter++
	}

	if qoveryYML.Application.PubliclyAccessible == true {
		if len(ExposePortsFromCurrentDockerfile()) == 0 {
			PrintError("You requested your application to be publicly accessible, but no exposed ports are defined")
			PrintSolution("Update your Dockerfile and add an 'EXPOSE' line with your application port " +
				"(https://docs.docker.com/engine/reference/builder/#expose)")
			counter++
		}
	}

	if qoveryYML.Application.Project == "" {
		PrintError("No project name defined")
		PrintSolution("Add in your .qovery.yml file, the 'project' name inside 'application' section")
		counter++
	}
	if qoveryYML.Application.Name == "" {
		PrintError("No application name defined")
		PrintSolution("Add in your .qovery.yml file, the 'name' name inside 'application' section")
		counter++
	}

	if counter > 0 {
		fmt.Printf("\nTotal errors found: %d", counter)
		return false
	}
	return true
}

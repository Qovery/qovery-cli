package util

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type QoveryYML struct {
	Qovery      QoveryYMLQovery      `yaml:"qovery,omitempty"`
	Application QoveryYMLApplication `yaml:"application,omitempty"`
	Databases   []QoveryYMLDatabase  `yaml:"databases,omitempty"`
	Brokers     []QoveryYMLBroker    `yaml:"brokers,omitempty"`
	// Storage   []QoveryYMLStorage  `yaml:"storage"`
	Routers []QoveryYMLRouter `yaml:"routers,omitempty"`
}

type QoveryYMLQovery struct {
	Key string `yaml:"key,omitempty"`
}

type QoveryYMLApplication struct {
	Name               string `yaml:"name,omitempty"`
	Project            string `yaml:"project,omitempty"`
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

func CurrentQoveryYML() QoveryYML {
	q := QoveryYML{}

	if _, err := os.Stat(".qovery.yml"); os.IsNotExist(err) {
		return q
	}

	f, err := ioutil.ReadFile(".qovery.yml")

	if err != nil {
		return q
	}

	_ = yaml.Unmarshal(f, &q)

	return q
}

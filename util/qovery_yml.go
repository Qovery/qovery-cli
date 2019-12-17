package util

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type QoveryYML struct {
	Qovery      QoveryYMLQovery      `yaml:"qovery,omitempty"`
	Application QoveryYMLApplication `yaml:"application,omitempty"`
	Network     QoveryYMLNetwork     `yaml:"network,omitempty"`
	Databases   []QoveryYMLDatabase  `yaml:"databases,omitempty"`
	Brokers     []QoveryYMLBroker    `yaml:"brokers,omitempty"`
	// Storage   []QoveryYMLStorage  `yaml:"storage"`
}

type QoveryYMLQovery struct {
	Key string `yaml:"key,omitempty"`
}

type QoveryYMLApplication struct {
	Name               string `yaml:"name,omitempty"`
	Project            string `yaml:"project,omitempty"`
	PubliclyAccessible bool   `yaml:"publicly_accessible,omitempty"`
}

type QoveryYMLDatabase struct {
	Type    string `yaml:"type,omitempty"`
	Version string `yaml:"version,omitempty"`
	Name    string `yaml:"name,omitempty"`
}

type QoveryYMLNetwork struct {
	DNS string `yaml:"dns,omitempty"`
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

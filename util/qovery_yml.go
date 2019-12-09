package util

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

type QoveryYML struct {
	Application QoveryYMLApplication `yaml:"application"`
	Databases   []QoveryYMLDatabase  `yaml:"databases"`
}

type QoveryYMLApplication struct {
	Name               string `yaml:"name"`
	Project            string `yaml:"project"`
	PubliclyAccessible bool   `yaml:"publicly_accessible"`
}

type QoveryYMLDatabase struct {
	Name string `yaml:"name"`
	Type string `yaml:"type"`
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

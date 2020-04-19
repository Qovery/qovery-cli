package util

import (
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"strings"
)

type TemplateSummary struct {
	Name        string
	Description string
}

func (t *TemplateSummary) ToString() string {
	return t.Name + " > " + t.Description
}

type Template struct {
	Name              string
	QoveryYML         QoveryYML
	DockerfileContent string
	Commands          []string
}

const rootTemplateURL = "https://raw.githubusercontent.com/Qovery/qovery-templates/master/"

func GetTemplate(templateName string) Template {
	qoveryYMLContent := getQoveryYMLContent(templateName)
	dockerfileContent := getDockerfileContent(templateName)
	commands := getCommandsConfigTemplate(templateName)

	qoveryYML := QoveryYML{}
	_ = yaml.Unmarshal(qoveryYMLContent, &qoveryYML)

	return Template{
		Name:              templateName,
		QoveryYML:         qoveryYML,
		DockerfileContent: dockerfileContent,
		Commands:          commands,
	}
}

func getQoveryYMLContent(templateName string) []byte {
	return getTemplateContent(templateName, ".qovery.yml")
}

func getDockerfileContent(templateName string) string {
	return string(getTemplateContent(templateName, "Dockerfile"))
}

func getCommandsConfigTemplate(templateName string) []string {
	var commands []string

	for _, s := range strings.Split(string(getTemplateContent(templateName, "commands")), "\n") {
		t := strings.TrimSpace(s)
		if t != "" {
			commands = append(commands, t)
		}
	}

	return commands
}

/**
Get the list of all templates by name
*/
func ListAvailableTemplates() []TemplateSummary {
	resp, err := http.Get(rootTemplateURL + "/templates")

	if err != nil {
		return []TemplateSummary{}
	}

	defer resp.Body.Close()

	templateBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil || len(templateBytes) == 0 {
		return []TemplateSummary{}
	}

	var results []TemplateSummary
	for _, line := range strings.Split(string(templateBytes), "\n") {
		t := strings.TrimSpace(line)
		if len(t) > 0 {
			s := strings.Split(t, "|")
			results = append(results, TemplateSummary{Name: s[0], Description: s[1]})
		}
	}

	return results
}

func getTemplateContent(projectName string, fileName string) []byte {
	resp, err := http.Get(rootTemplateURL + projectName + "/" + fileName)

	if err != nil {
		return []byte{}
	}

	defer resp.Body.Close()

	templateBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return []byte{}
	}

	return templateBytes
}

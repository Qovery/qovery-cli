package cmd

import (
	"fmt"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"qovery.go/api"
	"qovery.go/util"
	"strings"
)

var templateCmd = &cobra.Command{
	Use:   "template <name>",
	Short: "Bootstraps a template for given language or framework",
	Long: `TEMPLATE allows you to bootstrap required files for deploying given framework on Qovery. For example:

qovery template hasura
	
will create a basic .qovery.yml config and a valid Dockerfile`,

	Run: func(cmd *cobra.Command, args []string) {
		const configFileName = ".qovery.yml"
		const dockerfileName = "Dockerfile"

		if len(args) != 1 {
			_ = cmd.Help()
			return
		}

		assureNotExists(configFileName)
		assureNotExists(dockerfileName)

		var projectName = util.AskForInput(false, "Enter project name")
		var applicationName = util.AskForInput(false, "Enter application name")
		var templateName = strings.ToLower(args[0])

		dockerfileTemplate := api.GetDockerfileTemplate(templateName)
		qoveryConfigTemplate := api.GetQoveryConfigTemplate(templateName)

		dockerfile := prepareDockerfile(dockerfileTemplate)
		qoveryConfig := prepareQoveryConfig(qoveryConfigTemplate, projectName, applicationName)

		_ = ioutil.WriteFile(dockerfileName, []byte(dockerfile), 0777)
		_ = ioutil.WriteFile(configFileName, []byte(qoveryConfig), 0777)

		fmt.Println("Created " + configFileName + " and " + dockerfileName + " for " + templateName)
	},
}

func assureNotExists(fileName string) {
	if _, err := os.Stat(fileName); err == nil {
		fmt.Println("You already have a " + fileName + " file")
		fmt.Println("To bootstrap project configuration, delete " + fileName)
		os.Exit(0)
	}
}

func prepareQoveryConfig(template api.Template, projectName string, appName string) string {
	return replaceAppName(replaceProjectName(template.Content, projectName), appName)
}

func prepareDockerfile(template api.Template) string {
	return template.Content
}

func replaceProjectName(templateContent string, projectName string) string {
	return strings.ReplaceAll(templateContent, "${PROJECT_NAME}", projectName)
}

func replaceAppName(templateContent string, appName string) string {
	return strings.ReplaceAll(templateContent, "${APP_NAME}", appName)
}

func init() {
	RootCmd.AddCommand(templateCmd)
}

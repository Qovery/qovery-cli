package cmd

import (
	"fmt"
	"github.com/AlecAivazis/survey/v2"
	"github.com/joho/godotenv"
	"github.com/manifoldco/promptui"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"strings"
)

var envImportCmd = &cobra.Command{
	Use:   "import",
	Short: "Import environment variables/secrets for a Qovery app",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		dotEnvFilePath := ""
		if len(args) == 0 {
			file, err := scanAndSelectDotEnvFile()
			if err != nil {
				utils.PrintlnError(err)
				return
			}

			dotEnvFilePath = file
		} else if len(args) == 1 {
			dotEnvFilePath = args[0]
		} else {
			utils.PrintlnError(fmt.Errorf("more than one arg is not allowed"))
			return
		}

		if dotEnvFilePath == "" {
			utils.PrintlnError(fmt.Errorf("no dot env file specified"))
			return
		}

		envs, err := godotenv.Read(dotEnvFilePath)
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		application, _, err := utils.CurrentApplication()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}

		utils.PrintlnInfo(fmt.Sprintf("dot env file to import: '%s'", dotEnvFilePath))

		prompt := &survey.Select{
			Message: "Do you want to import Environment Variables or Secrets?",
			Options: []string{"Environment Variables", "Secrets"},
		}

		var envVarOrSecret string
		err = survey.AskOne(prompt, &envVarOrSecret)
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		isSecrets := false
		if envVarOrSecret == "Secrets" {
			isSecrets = true
		}

		envsToImport := getEnvsToImport(envs)
		if envsToImport == nil {
			utils.PrintlnError(fmt.Errorf("no environment variables to import"))
			return
		}

		for k, v := range envsToImport {
			err = nil
			if isSecrets {
				err = utils.AddSecret(application, k, v)
			} else {
				err = utils.AddEnvironmentVariable(application, k, v)
			}

			if err != nil {
				utils.PrintlnError(err)
			}
		}
	},
}

func scanAndSelectDotEnvFile() (string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	var files []string
	err = filepath.Walk(currentDir, func(path string, info os.FileInfo, err error) error {
		if strings.Contains(path, ".env") {
			// only include files containing ".env"
			files = append(files, path)
		}
		return nil
	})

	if err != nil {
		panic(err)
	}

	if len(files) == 0 {
		return "", fmt.Errorf("no .env file found in '%s'", currentDir)
	}

	prompt := promptui.Select{
		Label: "Select your .env file",
		Items: files,
	}

	_, result, err := prompt.Run()

	if err != nil {
		return "", err
	}

	return result, nil
}

func getEnvsToImport(envs map[string]string) map[string]string {
	var envKeys []string

	for k, v := range envs {
		envKeys = append(envKeys, fmt.Sprintf("%s=%s", k, v))
	}

	prompt := &survey.MultiSelect{
		Message: "What environment variables do you want to import?",
		Options: envKeys,
	}

	err := survey.AskOne(prompt, &envKeys)
	if err != nil {
		return nil
	}

	results := make(map[string]string)
	for _, key := range envKeys {
		sKey := strings.Split(key, "=")
		results[sKey[0]] = envs[sKey[0]]
	}

	return results
}

func init() {
	envCmd.AddCommand(envImportCmd)
}

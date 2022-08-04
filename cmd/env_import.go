package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/joho/godotenv"
	"github.com/manifoldco/promptui"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
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

		service, err := utils.CurrentService()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}

		if service.Type != utils.ApplicationType {
			utils.PrintlnError(fmt.Errorf("cannot import variables for service different than Application"))
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
		if len(envsToImport) == 0 {
			utils.PrintlnError(fmt.Errorf("no environment variables to import"))
			return
		}

		prompt = &survey.Select{
			Message: fmt.Sprintf("Do you want to overwrite existing %s?", envVarOrSecret),
			Options: []string{"No", "Yes"},
		}

		var overrideEnvVarOrSecretString string
		err = survey.AskOne(prompt, &overrideEnvVarOrSecretString)
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		overrideEnvVarOrSecret := false
		if overrideEnvVarOrSecretString == "Yes" {
			overrideEnvVarOrSecret = true
		}

		var errors []string

		for k, v := range envsToImport {
			var err error
			if isSecrets {
				if overrideEnvVarOrSecret {
					_ = utils.DeleteSecret(service.ID, k)
				}

				err = utils.AddSecret(service.ID, k, v)
			} else {
				if overrideEnvVarOrSecret {
					_ = utils.DeleteEnvironmentVariable(service.ID, k)
				}

				err = utils.AddEnvironmentVariable(service.ID, k, v)
			}

			if err != nil {
				errors = append(errors, fmt.Sprintf("%s (%s)", k, err))
			}
		}

		if len(errors) == 0 {
			utils.PrintlnInfo(fmt.Sprintf("✅ %s successfully imported!", envVarOrSecret))
		} else {
			utils.PrintlnError(fmt.Errorf("those %s have failed to be imported: %s", envVarOrSecret, strings.Join(errors, ", ")))
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

	var selectedEnvKeys []string
	err := survey.AskOne(prompt, &selectedEnvKeys)
	if err != nil {
		return nil
	}

	results := make(map[string]string)
	for _, key := range selectedEnvKeys {
		sKey := strings.Split(key, "=")
		results[sKey[0]] = envs[sKey[0]]
	}

	return results
}

func init() {
	envCmd.AddCommand(envImportCmd)
}

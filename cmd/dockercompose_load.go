package cmd

import (
	"fmt"
	"github.com/qovery/qovery-client-go"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"

	"github.com/compose-spec/compose-go/loader"
	"github.com/compose-spec/compose-go/types"
)

var dockerComposeLoadCmd = &cobra.Command{
	Use:   "load",
	Short: "load a docker compose file and create resources on Qovery",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, _, err = getContextResourcesId(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		// load docker compose file
		data, err := os.ReadFile(dockerComposeFile)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		project, err := loader.ParseYAML(data)
		// Load the project
		loadedProject, err := loader.Load(types.ConfigDetails{
			ConfigFiles: []types.ConfigFile{
				{
					Config: project,
				},
			},
		})

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		for _, svc := range loadedProject.ServiceNames() {
			service, err := loadedProject.GetService(svc)
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
			}

			for _, port := range service.Ports {
				fmt.Printf("%d\n", port.Target)
			}
		}

		// convert docker compose file to qovery resources
		// create qovery resources
	},
}

func createApplicationFromServiceConfig(environmentId string, service *types.ServiceConfig) (error, *qovery.Application) {
	return nil, nil
}

func createDatabaseFromServiceConfig(environmentId string, service *types.ServiceConfig) (error, *qovery.Database) {

	return nil, nil
}

func init() {
	dockerComposeCmd.AddCommand(dockerComposeLoadCmd)
	dockerComposeLoadCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	dockerComposeLoadCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	dockerComposeLoadCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	dockerComposeLoadCmd.Flags().StringVarP(&dockerComposeFile, "file", "f", "", "Docker Compose File")

	_ = dockerComposeLoadCmd.MarkFlagRequired("file")
}

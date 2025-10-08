package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var containerEnvListCmd = &cobra.Command{
	Use:   "list",
	Short: "List container environment variables",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)

		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		containers, _, err := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		container := utils.FindByContainerName(containers.GetResults(), containerName)

		if container == nil {
			utils.PrintlnError(fmt.Errorf("container %s not found", containerName))
			utils.PrintlnInfo("You can list all containers with: qovery container list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVars, err := utils.ListServiceVariables(
			client,
			container.Id,
			utils.ContainerType,
		)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		envVarLines := utils.NewEnvVarLines()
		var variables []utils.EnvVarLineOutput

		for _, envVar := range envVars {
			s := utils.FromEnvironmentVariableToEnvVarLineOutput(envVar)
			variables = append(variables, s)
			envVarLines.Add(s)
		}

		if jsonFlag {
			utils.Println(utils.GetEnvVarJsonOutput(variables, utils.SortKeys))
			return
		}

		err = utils.PrintTable(envVarLines.Header(utils.PrettyPrint), envVarLines.Lines(utils.ShowValues, utils.PrettyPrint, utils.SortKeys))

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

func init() {
	containerEnvCmd.AddCommand(containerEnvListCmd)
	containerEnvListCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	containerEnvListCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	containerEnvListCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	containerEnvListCmd.Flags().StringVarP(&containerName, "container", "n", "", "Container Name")
	containerEnvListCmd.Flags().BoolVarP(&utils.ShowValues, "show-values", "", false, "Show env var values")
	containerEnvListCmd.Flags().BoolVarP(&utils.PrettyPrint, "pretty-print", "", false, "Pretty print output")
	containerEnvListCmd.Flags().BoolVarP(&utils.SortKeys, "sort", "", false, "Sort environment variables by key")
	containerEnvListCmd.Flags().BoolVarP(&jsonFlag, "json", "", false, "JSON output")

	_ = containerEnvListCmd.MarkFlagRequired("container")
}

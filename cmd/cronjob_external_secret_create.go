package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var cronjobExternalSecretCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create cronjob external secret",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, projectId, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjobs, _, err := client.JobsAPI.ListJobs(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		cronjob := utils.FindByJobName(cronjobs.GetResults(), cronjobName)

		if cronjob == nil || cronjob.CronJobResponse == nil {
			utils.PrintlnError(fmt.Errorf("cronjob %s not found", cronjobName))
			utils.PrintlnInfo("You can list all cronjobs with: qovery cronjob list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		secretManagerAccessId, err := getSecretManagerAccessIdByName(client, organizationId, envId, utils.SecretManagerAccessName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		err = utils.CreateServiceExternalSecret(client, projectId, envId, cronjob.CronJobResponse.Id, utils.JobScope, utils.Key, utils.Reference, secretManagerAccessId, utils.MountPath)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("External secret %s has been created", pterm.FgBlue.Sprintf("%s", utils.Key)))
	},
}

func init() {
	cronjobExternalSecretCmd.AddCommand(cronjobExternalSecretCreateCmd)
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&cronjobName, "cronjob", "n", "", "Cronjob Name")
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&utils.Key, "key", "k", "", "External secret key")
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&utils.Reference, "reference", "r", "", "Reference to the secret in the secrets provider")
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&utils.SecretManagerAccessName, "secret-manager-access-name", "", "", "Secret manager access name")
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&utils.JobScope, "scope", "", "JOB", "Scope of this external secret <PROJECT|ENVIRONMENT|JOB>")
	cronjobExternalSecretCreateCmd.Flags().StringVarP(&utils.MountPath, "mount-path", "", "", "Path where the secret will be mounted as a file")

	_ = cronjobExternalSecretCreateCmd.MarkFlagRequired("key")
	_ = cronjobExternalSecretCreateCmd.MarkFlagRequired("reference")
	_ = cronjobExternalSecretCreateCmd.MarkFlagRequired("secret-manager-access-name")
	_ = cronjobExternalSecretCreateCmd.MarkFlagRequired("cronjob")
}

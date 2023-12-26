package cmd

import (
	"os"
	"os/exec"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var k9sCmd = &cobra.Command{
	Use:   "k9s",
	Short: "Launch k9s with a cluster ID",
	Run: func(cmd *cobra.Command, args []string) {
		launchK9s(args)
	},
}

func init() {
	adminCmd.AddCommand(k9sCmd)
}

func launchK9s(args []string) {
	checkEnv()

	if len(args) == 0 {
		log.Error("You must enter a cluster ID.")
		return
	}

	clusterId := args[0]
	vars := pkg.GetVarsByClusterId(clusterId)
	if len(vars) == 0 {
		return
	}

	for _, variable := range vars {
		os.Setenv(variable.Key, variable.Value)

		// Generate temporary file + ENV for GCP auth
		// https://serverfault.com/questions/848580/how-to-use-google-application-credentials-with-gcloud-on-a-server
		if variable.Key == "GOOGLE_CREDENTIALS" {
			googleCredentialsFile, err := os.CreateTemp("", "sample")
			if err != nil {
				log.Error("Can't create google credentials file : " + err.Error())
			}
			defer os.Remove(googleCredentialsFile.Name())

			_, err = googleCredentialsFile.WriteString(variable.Value)
			if err != nil {
				log.Error("Can't create google credentials file : " + err.Error())
			}

			os.Setenv("CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", googleCredentialsFile.Name())
		}
	}
	utils.GenerateExportEnvVarsScript(vars, args[0])

	log.Info("Launching k9s.")
	cmd := exec.Command("k9s")
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		log.Error("Can't launch k9s : " + err.Error())
	}

	utils.DeleteFolder(os.Getenv("KUBECONFIG")[0 : len(os.Getenv("KUBECONFIG"))-len("kubeconfig")])
}

func checkEnv() {
	if _, ok := os.LookupEnv("VAULT_ADDR"); !ok {
		log.Error("You must set vault address env variable (VAULT_ADDR).")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	if _, ok := os.LookupEnv("VAULT_TOKEN"); !ok {
		log.Error("You must set vault token env variable (VAULT_TOKEN).")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

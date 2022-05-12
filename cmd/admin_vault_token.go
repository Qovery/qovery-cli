package cmd

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"os"
	"os/exec"
	"time"
)

var vaultTokenCmd = &cobra.Command{
	Use:   "vault_token",
	Short: "Get Vault Token",
	Run: func(cmd *cobra.Command, args []string) {
		getAndShowVaultToken(args)
	},
}

func init() {
	adminCmd.AddCommand(vaultTokenCmd)
}

func getAndShowVaultToken(args []string) {
	tokenFilePath, vaultToken := getVaultToken(args)
	log.Info(fmt.Sprintf("Your Vault Token (%s):\n%s", tokenFilePath, vaultToken))
}

func getTokenFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error("Can't get home directory")
		os.Exit(1)
	}
	return fmt.Sprintf("%s/.vault-token", homeDir)
}

func getVaultToken(args []string) (string, string) {
	var tokenFileModificationTime time.Time
	tokenValiditySec := 43200
	renewBeforeSec := 7200
	maxTokenValidity := tokenValiditySec - renewBeforeSec
	tokenFilePath := getTokenFilePath()

	vaultPath, ghToken := checkVaultEnv()

	// check if token file exists
	fileStat, err := os.Stat(tokenFilePath)
	if err != nil {
		tokenFileModificationTime = time.Now().Add(-24 * time.Hour)
	} else {
		tokenFileModificationTime = fileStat.ModTime()
	}

	// get and store new token
	if tokenFileModificationTime.Before(time.Now().Add(time.Duration(-maxTokenValidity) * time.Second)) {
		log.Info("Getting vault token")

		cmd := exec.Command(vaultPath, "login", "-token-only", "-method=github", fmt.Sprintf("token=%s", ghToken))
		secret, err := cmd.CombinedOutput()
		if err != nil {
			log.Error("error with Vault: " + err.Error())
			os.Exit(1)
		}

		err = os.WriteFile(tokenFilePath, []byte(secret), 0600)
		if err != nil {
			log.Error(fmt.Sprintf("error while writing token to vault token file (%s)", tokenFilePath))
			log.Error(err)
			os.Exit(1)
		}
	}

	vaultToken, err := os.ReadFile(tokenFilePath)
	if err != nil {
		log.Error(fmt.Sprintf("can't read file %s", tokenFilePath))
		os.Exit(1)
	}

	return tokenFilePath, string(vaultToken)
}

func checkVaultEnv() (string, string) {
	if _, ok := os.LookupEnv("VAULT_ADDR"); !ok {
		log.Error("You must set vault address env variable (VAULT_ADDR).")
		os.Exit(1)
	}

	ghToken, err := os.LookupEnv("VAULT_GH_TOKEN")
	if !err {
		log.Error("You must set your personal token env variable (VAULT_GH_TOKEN).")
		os.Exit(1)
	}

	vaultPath, e := exec.LookPath("vault")
	if e != nil {
		log.Error("vault binary is not found in your path")
		os.Exit(1)
	}

	return vaultPath, ghToken
}

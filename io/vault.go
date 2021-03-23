package io

import (
	"github.com/hashicorp/vault/api"
	log "github.com/sirupsen/logrus"
	"os"
	"qovery-cli/utils"
	"strings"
	b64 "encoding/base64"
)

func connectToVault() *api.Client {
	var token = os.Getenv("VAULT_TOKEN")
	var vaultAddr = os.Getenv("VAULT_ADDR")

	config := &api.Config{
		Address: vaultAddr,
	}
	client, err := api.NewClient(config)

	if err != nil {
		log.Error("Can't create Vault client : " + err.Error())
		return nil
	}

	client.SetToken(token)

	return client
}

func getClusterPath(client *api.Client, clusterID string) string {
	result, err := client.Logical().List("official-clusters-access/metadata")
	if err != nil {
		log.Error(err)
	}

	for _, secret := range (result.Data["keys"]).([]interface {}) {
		if strings.Contains(secret.(string), clusterID) {
			return secret.(string)
		}
	}

	return ""
}

func GetVarsByClusterId(clusterID string) []utils.Var {
	client := connectToVault()
	path := getClusterPath(client, clusterID)

	result, err := client.Logical().Read("official-clusters-access/data/" + path)
	if err != nil {
		log.Error(err)
	}

	var vaultVars []utils.Var
	for key, value := range (result.Data["data"]).(map[string]interface {}) {
		switch key {
			case "AWS_ACCESS_KEY_ID":
				vaultVars = append(vaultVars, utils.Var{Key: key, Value: value.(string)})
			case "AWS_DEFAULT_REGION":
				vaultVars = append(vaultVars, utils.Var{Key: key, Value: value.(string)})
			case "AWS_SECRET_ACCESS_KEY":
				vaultVars = append(vaultVars, utils.Var{Key: key, Value: value.(string)})
			case "KUBECONFIG_b64":
				decodedValue, encErr := b64.StdEncoding.DecodeString(value.(string))
				if encErr != nil {
					log.Error("Can't decode KUBECONFIG")
					return []utils.Var{}
				}
				filePath := utils.WriteInFile(clusterID, "kubeconfig", decodedValue)
				vaultVars = append(vaultVars, utils.Var{Key: "KUBECONFIG", Value: filePath})
		}
	}

	return vaultVars
}

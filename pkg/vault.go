package pkg

import (
	b64 "encoding/base64"
	"github.com/hashicorp/vault/api"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"os"
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

func GetVarsByClusterId(clusterID string) []utils.Var {
	client := connectToVault()

	result, err := client.Logical().Read("/official-clusters-access/data/" + clusterID)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	if result == nil {
		log.Error("Cluster information are not found")
		os.Exit(1)
	}

	var vaultVars []utils.Var
	for key, value := range (result.Data["data"]).(map[string]interface{}) {
		switch key {
		case "AWS_ACCESS_KEY_ID":
			vaultVars = append(vaultVars, utils.Var{Key: key, Value: value.(string)})
		case "AWS_DEFAULT_REGION":
			vaultVars = append(vaultVars, utils.Var{Key: key, Value: value.(string)})
		case "AWS_SECRET_ACCESS_KEY":
			vaultVars = append(vaultVars, utils.Var{Key: key, Value: value.(string)})
		case "kubeconfig_b64", "KUBECONFIG_b64":
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

package pkg

import (
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"os"

	"github.com/hashicorp/vault/api"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
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

func GetVarsByClusterId(clusterID string) ([]utils.Var, error) {
	client := connectToVault()

	result, err := client.Logical().Read("/official-clusters-access/data/" + clusterID)
	if err != nil {
		log.Error(err)
		return nil, err
	}
	if result == nil {
		log.Error("Cluster information are not found")
		return nil, errors.New("cluster information are not found")
	}

	var vaultVars []utils.Var
	for key, value := range (result.Data["data"]).(map[string]interface{}) {
		switch key {
		case "AWS_ACCESS_KEY_ID", "aws_access_key":
			vaultVars = append(vaultVars, utils.Var{Key: "AWS_ACCESS_KEY_ID", Value: value.(string)})
		case "AWS_DEFAULT_REGION", "aws_default_region":
			vaultVars = append(vaultVars, utils.Var{Key: "AWS_DEFAULT_REGION", Value: value.(string)})
		case "AWS_SECRET_ACCESS_KEY", "aws_secret_access_key":
			vaultVars = append(vaultVars, utils.Var{Key: "AWS_SECRET_ACCESS_KEY", Value: value.(string)})
		case "GOOGLE_CREDENTIALS", "google_credentials":
			jsonStr, err := json.Marshal(value)
			if err != nil {
				log.Error("Can't convert to json GOOGLE_CREDENTIALS")
				return []utils.Var{}, nil
			}
			vaultVars = append(vaultVars, utils.Var{Key: "GOOGLE_CREDENTIALS", Value: string(jsonStr)})
		case "kubeconfig_b64", "KUBECONFIG_b64":
			decodedValue, encErr := b64.StdEncoding.DecodeString(value.(string))
			if encErr != nil {
				log.Error("Can't decode KUBECONFIG")
				return []utils.Var{}, nil
			}
			filePath := utils.WriteInFile(clusterID, "kubeconfig", decodedValue)
			vaultVars = append(vaultVars, utils.Var{Key: "KUBECONFIG", Value: filePath})
		}
	}

	return vaultVars, nil
}

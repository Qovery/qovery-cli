package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
)

func qoveryDirectoryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.FromSlash(fmt.Sprintf("%s/.qovery", home))
}

func GetAuthorizationToken() string {
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/access_token")
	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("fail to read file '%s': %s", filePath, err.Error())
	}
	return string(fileBytes)
}

func SetAuthorizationToken(token string) {
	_ = os.MkdirAll(qoveryDirectoryPath(), 0755)
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/access_token")
	_ = ioutil.WriteFile(filePath, []byte(token), 0755)
}

func GetAccountId() string {
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/account")
	fileBytes, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Fatalf("fail to read file '%s': %s", filePath, err.Error())
	}
	return string(fileBytes)
}

func SetAccountId(id string) {
	_ = os.MkdirAll(qoveryDirectoryPath(), 0755)
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/account")
	_ = ioutil.WriteFile(filePath, []byte(id), 0755)
}

func SaveLocalConfiguration(root string, data map[string]interface{}) {
	j, _ := json.Marshal(data)

	if root == "" {
		_ = os.MkdirAll(".qovery", 0755)
		_ = ioutil.WriteFile(filepath.FromSlash(".qovery/local_configuration.json"), j, 0755)
		return
	}

	_ = os.MkdirAll(filepath.FromSlash(fmt.Sprintf("%s/.qovery", root)), 0755)
	_ = ioutil.WriteFile(filepath.FromSlash(fmt.Sprintf("%s/.qovery/local_configuration.json", root)), j, 0755)
}

func DeleteLocalConfiguration(root string) {
	if root == "" {
		_ = os.Remove(filepath.FromSlash(".qovery/local_configuration.json"))
		return
	}

	_ = os.Remove(filepath.FromSlash(fmt.Sprintf("%s/.qovery/local_configuration.json", root)))
}

package api

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func qoveryDirectoryPath() string {
	home, _ := os.UserHomeDir()
	return filepath.FromSlash(fmt.Sprintf("%s/.qovery", home))
}

func GetAuthorizationToken() string {
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/access_token")
	fileBytes, _ := ioutil.ReadFile(filePath)
	return string(fileBytes)
}

func SetAuthorizationToken(token string) {
	_ = os.MkdirAll(qoveryDirectoryPath(), 0755)
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/access_token")
	_ = ioutil.WriteFile(filePath, []byte(token), 0755)
}

func GetAccountId() string {
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/account")
	fileBytes, _ := ioutil.ReadFile(filePath)
	return string(fileBytes)
}

func SetAccountId(id string) {
	_ = os.MkdirAll(qoveryDirectoryPath(), 0755)
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/account")
	_ = ioutil.WriteFile(filePath, []byte(id), 0755)
}

func SaveLocalConfiguration(data map[string]interface{}) {
	j, _ := json.Marshal(data)
	_ = os.MkdirAll(".qovery", 0755)
	_ = ioutil.WriteFile(filepath.FromSlash(".qovery/local_configuration.json"), j, 0755)
}

func DeleteLocalConfiguration() {
	_ = os.Remove(filepath.FromSlash(".qovery/local_configuration.json"))
}

package api

import (
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

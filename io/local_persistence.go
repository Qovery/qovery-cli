package io

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"time"
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

func GetAuthorizationTokenExpiration() (time.Time, error) {
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/expired_at")
	fileBytes, _ := ioutil.ReadFile(filePath)
	expiredAt, err := strconv.ParseInt(string(fileBytes), 10, 64)

	if err != nil {
		return time.Time{}, err
	}

	return time.Unix(expiredAt, 0), nil
}

func SetAuthorizationTokenExpiration(exiredAt time.Time) {
	_ = os.MkdirAll(qoveryDirectoryPath(), 0755)
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/expired_at")
	_ = ioutil.WriteFile(filePath, []byte(fmt.Sprintf("%d", exiredAt.Unix())), 0755)
}

func GetRefreshToken() string {
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/refresh_token")
	fileBytes, _ := ioutil.ReadFile(filePath)
	return string(fileBytes)
}

func SetRefreshToken(token string) {
	_ = os.MkdirAll(qoveryDirectoryPath(), 0755)
	filePath := filepath.FromSlash(qoveryDirectoryPath() + "/refresh_token")
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

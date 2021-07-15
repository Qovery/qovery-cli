package utils

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"time"
)

const ContextFileName = "context"

type QoveryContext struct {
	AccessToken           AccessToken  `json:"access_token"`
	AccessTokenExpiration time.Time    `json:"access_token_expiration"`
	RefreshToken          RefreshToken `json:"refresh_token"`
	OrganizationId        Id           `json:"organization_id"`
	OrganizationName      Name         `json:"organization_name"`
	ProjectId             Id           `json:"project_id"`
	ProjectName           Name         `json:"project_name"`
	EnvironmentId         Id           `json:"environment_id"`
	EnvironmentName       Name         `json:"environment_name"`
	ApplicationId         Id           `json:"application_id"`
	ApplicationName       Name         `json:"application_name"`
}
type Name string
type AccessToken string
type RefreshToken string
type Id string

func CurrentContext() (QoveryContext, error) {
	context := QoveryContext{}

	path, err := QoveryContextPath()
	if err != nil {
		return context, err
	}

	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return context, err
	}

	err = json.Unmarshal(bytes, &context)
	if err != nil {
		return context, err
	}

	return context, err
}

func StoreContext(context QoveryContext) error {
	bytes, err := json.Marshal(context)
	if err != nil {
		return err
	}

	path, err := QoveryContextPath()
	if err != nil {
		return err
	}

	return ioutil.WriteFile(path, bytes, os.ModePerm)
}

func CurrentOrganization() (Id, Name, error) {
	context, err := CurrentContext()
	if err != nil {
		return "", "", err
	}

	id := context.OrganizationId
	if id == "" {
		return "", "", errors.New("Current organization has not been selected. Please, use 'qovery context' to set up Qovery context. ")
	}
	name := context.OrganizationName
	if name == "" {
		return "", "", errors.New("Current organization has not been selected. Please, use 'qovery context' to set up Qovery context. ")
	}

	return id, name, nil
}

func SetOrganization(name Name, id Id) error {
	context, err := CurrentContext()
	if err != nil {
		return err
	}

	context.OrganizationName = name
	context.OrganizationId = id

	return StoreContext(context)
}

func CurrentProject() (Id, Name, error) {
	context, err := CurrentContext()
	if err != nil {
		return "", "", err
	}

	id := context.ProjectId
	if id == "" {
		return "", "", errors.New("Current project has not been selected. Please, use 'qovery context' to set up Qovery context. ")
	}
	name := context.ProjectName
	if name == "" {
		return "", "", errors.New("Current project has not been selected. Please, use 'qovery context' to set up Qovery context. ")
	}

	return id, name, nil
}

func SetProject(name Name, id Id) error {
	context, err := CurrentContext()
	if err != nil {
		return err
	}

	context.ProjectName = name
	context.ProjectId = id

	return StoreContext(context)
}

func CurrentEnvironment() (Id, Name, error) {
	context, err := CurrentContext()
	if err != nil {
		return "", "", err
	}

	id := context.EnvironmentId
	if id == "" {
		return "", "", errors.New("Current environment has not been selected. Please, use 'qovery context' to set up Qovery context. ")
	}
	name := context.EnvironmentName
	if name == "" {
		return "", "", errors.New("Current environment has not been selected. Please, use 'qovery context' to set up Qovery context. ")
	}

	return id, name, nil
}

func SetEnvironment(name Name, id Id) error {
	context, err := CurrentContext()
	if err != nil {
		return err
	}

	context.EnvironmentName = name
	context.EnvironmentId = id

	return StoreContext(context)
}

func CurrentApplication() (Id, Name, error) {
	context, err := CurrentContext()
	if err != nil {
		return "", "", err
	}

	id := context.ApplicationId
	if id == "" {
		return "", "", errors.New("Current application has not been selected. Please, use 'qovery context' to set up Qovery context. ")
	}
	name := context.ApplicationName
	if name == "" {
		return "", "", errors.New("Current application has not been selected. Please, use 'qovery context' to set up Qovery context. ")
	}

	return id, name, nil
}

func SetApplication(name Name, id Id) error {
	context, err := CurrentContext()
	if err != nil {
		return err
	}

	context.ApplicationName = name
	context.ApplicationId = id

	return StoreContext(context)
}

func GetAccessToken() (AccessToken, error) {
	context, err := CurrentContext()
	if err != nil {
		return AccessToken(""), err
	}

	token := context.AccessToken
	if token == "" {
		return "", errors.New("Access token has not been found. Please, sign in using 'qovery auth' command. ")
	}

	expired := context.AccessTokenExpiration.Before(time.Now())
	if expired {
		RefreshExpiredTokenSilently()
		refreshed, err := GetAccessToken()
		if err != nil {
			return AccessToken(""), err
		}
		token = refreshed
	}

	return token, nil
}

func GetAccessTokenExpiration() (time.Time, error) {
	context, err := CurrentContext()
	t := time.Time{}
	if err != nil {
		return t, err
	}

	expiration := context.AccessTokenExpiration
	if expiration == t {
		return t, errors.New("Access token has not been found. Please, sign in using 'qovery auth' command. ")
	}

	return expiration, nil
}

func SetAccessToken(token AccessToken, expiration time.Time) error {
	context, err := CurrentContext()
	if err != nil {
		return err
	}

	context.AccessToken = token
	context.AccessTokenExpiration = expiration

	return StoreContext(context)
}

func GetRefreshToken() (RefreshToken, error) {
	context, err := CurrentContext()
	if err != nil {
		return RefreshToken(""), err
	}

	token := context.RefreshToken
	if token == "" {
		return "", errors.New("Refresh token has not been found. Please, sign in using 'qovery auth' command. ")
	}

	return token, nil
}

func SetRefreshToken(token RefreshToken) error {
	context, err := CurrentContext()
	if err != nil {
		return err
	}

	context.RefreshToken = token

	return StoreContext(context)
}

func InitializeQoveryContext() error {
	if !QoveryDirExists() {
		path, err := QoveryDirPath()
		if err != nil {
			return err
		}

		err = os.Mkdir(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	path, err := QoveryContextPath()
	if err != nil {
		return err
	}

	_, err = os.Create(path)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(path, []byte("{}"), os.ModePerm)
	if err != nil {
		return err
	}

	return nil
}

func QoveryContextPath() (string, error) {
	path, err := QoveryDirPath()
	if err != nil {
		return "", err
	}
	return path + string(os.PathSeparator) + ContextFileName + ".json", nil
}

func QoveryDirPath() (string, error) {
	dir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return dir + string(os.PathSeparator) + ".qovery", nil
}

func QoveryDirExists() bool {
	path, err := QoveryDirPath()
	if err != nil {
		PrintlnError(err)
		os.Exit(0)
	}
	return pathExists(path)
}

func QoveryContextExists() bool {
	path, err := QoveryContextPath()
	if err != nil {
		PrintlnError(err)
		os.Exit(0)
	}
	return pathExists(path)
}

func pathExists(path string) bool {
	if _, err := os.Stat(path); err == nil {
		return true
	} else if os.IsNotExist(err) {
		return false
	} else {
		return false
	}
}

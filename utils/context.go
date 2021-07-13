package utils

import (
	"encoding/json"
	"errors"
	"fmt"
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
		return "", "", errors.New("organization_id not selected")
	}
	name := context.OrganizationName
	if name == "" {
		return "", "", errors.New("organization_name not selected")
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
		return "", "", errors.New("project_id not selected")
	}
	name := context.ProjectName
	if name == "" {
		return "", "", errors.New("project_name not selected")
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
		return "", "", errors.New("environment_id not selected")
	}
	name := context.EnvironmentName
	if name == "" {
		return "", "", errors.New("environment_name not selected")
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
		return "", "", errors.New("application_id not selected")
	}
	name := context.ApplicationName
	if name == "" {
		return "", "", errors.New("application_name not selected")
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
		return "", errors.New("access_token not present")
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
		return t, errors.New("access_token_expiration not present")
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
		return "", errors.New("refresh_token not present")
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
		fmt.Println(err)
		os.Exit(1)
	}
	return pathExists(path)
}

func QoveryContextExists() bool {
	path, err := QoveryContextPath()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
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

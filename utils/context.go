package utils

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
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
	ServiceId             Id           `json:"service_id"`
	ServiceName           Name         `json:"service_name"`
	ServiceType           ServiceType  `json:"service_type"`
	User                  Name         `json:"user"`
}
type Name string
type AccessTokenType string
type AccessToken string
type RefreshToken string
type Id string

func isMinimalContextValid(context QoveryContext) bool {
	// this is the minimal context that we need to have to be able to use the CLI
	return context.AccessToken != "" &&
		context.AccessTokenExpiration.After(time.Now()) &&
		context.RefreshToken != "" &&
		context.OrganizationId != ""
}

func GetOrSetCurrentContext() (QoveryContext, error) {
	context, _ := GetCurrentContext()
	if isMinimalContextValid(context) {
		return context, nil
	}

	err := SetContext(false)

	if err != nil {
		return context, err
	}

	return GetCurrentContext()
}

func GetCurrentContext() (QoveryContext, error) {
	context := QoveryContext{}

	path, err := QoveryContextPath()
	if err != nil {
		return context, err
	}

	bytes, err := os.ReadFile(path)
	if err != nil {
		return context, err
	}

	err = json.Unmarshal(bytes, &context)
	if err != nil {
		return context, err
	}

	return context, err
}

func SetContext(printFinalContext bool) error {
	_ = PrintContext()
	_ = ResetApplicationContext()

	org, err := SelectAndSetOrganization()
	if err != nil {
		return err
	}

	project, err := SelectAndSetProject(org.ID)
	if err != nil {
		return err
	}

	env, err := SelectAndSetEnvironment(project.ID)
	if err != nil {
		return err
	}

	_, err = SelectAndSetService(env.ID)
	if err != nil {
		return err
	}
	_, _ = CurrentService(false)

	if printFinalContext {
		println()
		err = PrintContext()
		if err != nil {
			PrintlnError(err)
		}
		println()
	}

	return nil
}

func (c QoveryContext) ToPosthogProperties() map[string]interface{} {
	return map[string]interface{}{
		"organization": c.OrganizationName,
		"project":      c.ProjectName,
		"environment":  c.EnvironmentName,
		"service":      c.ServiceName,
		"type":         c.ServiceType,
	}
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

	return os.WriteFile(path, bytes, os.ModePerm)
}

func CurrentOrganization(promptContext bool) (Id, Name, error) {
	context, err := GetCurrentContext()

	if (context.OrganizationId == "" || err != nil) && promptContext {
		context, err = GetOrSetCurrentContext()
	}

	if err != nil {
		return "", "", err
	}

	id := context.OrganizationId
	if id == "" {
		return "", "", errors.New("Current organization has not been selected. Please, use 'qovery context set' to set up Qovery context. ")
	}
	name := context.OrganizationName
	if name == "" {
		return "", "", errors.New("Current organization has not been selected. Please, use 'qovery context set' to set up Qovery context. ")
	}

	return id, name, nil
}

func SetOrganization(org *Organization) error {
	context, err := GetCurrentContext()
	if err != nil {
		return err
	}

	context.OrganizationName = org.Name
	context.OrganizationId = org.ID

	return StoreContext(context)
}

func CurrentProject(promptContext bool) (Id, Name, error) {
	context, err := GetCurrentContext()

	if (context.ProjectId == "" || err != nil) && promptContext {
		context, err = GetOrSetCurrentContext()
	}

	if err != nil {
		return "", "", err
	}

	id := context.ProjectId
	if id == "" {
		return "", "", errors.New("Current project has not been selected. Please, use 'qovery context set' to set up Qovery context. ")
	}
	name := context.ProjectName
	if name == "" {
		return "", "", errors.New("Current project has not been selected. Please, use 'qovery context set' to set up Qovery context. ")
	}

	return id, name, nil
}

func SetProject(project *Project) error {
	context, err := GetCurrentContext()
	if err != nil {
		return err
	}

	context.ProjectName = project.Name
	context.ProjectId = project.ID

	return StoreContext(context)
}

func CurrentEnvironment(promptContext bool) (Id, Name, error) {
	context, err := GetCurrentContext()

	if (context.EnvironmentId == "" || err != nil) && promptContext {
		context, err = GetOrSetCurrentContext()
	}

	if err != nil {
		return "", "", err
	}

	id := context.EnvironmentId
	if id == "" {
		return "", "", errors.New("Current environment has not been selected. Please, use 'qovery context set' to set up Qovery context. ")
	}
	name := context.EnvironmentName
	if name == "" {
		return "", "", errors.New("Current environment has not been selected. Please, use 'qovery context set' to set up Qovery context. ")
	}

	return id, name, nil
}

func SetEnvironment(env *Environment) error {
	context, err := GetCurrentContext()
	if err != nil {
		return err
	}

	context.EnvironmentName = env.Name
	context.EnvironmentId = env.ID

	return StoreContext(context)
}

func CurrentService(promptContext bool) (*Service, error) {
	context, err := GetCurrentContext()

	if (context.ServiceId == "" || err != nil) && promptContext {
		context, err = GetOrSetCurrentContext()
	}

	if err != nil {
		return nil, err
	}

	id := context.ServiceId
	if id == "" {
		return nil, errors.New("Current service has not been selected. Please, use 'qovery context set' to set up Qovery context. ")
	}

	name := context.ServiceName
	if name == "" {
		return nil, errors.New("Current service has not been selected. Please, use 'qovery context set' to set up Qovery context. ")
	}

	return &Service{ID: id, Name: name, Type: context.ServiceType}, nil
}

func SetService(service *Service) error {
	context, err := GetCurrentContext()
	if err != nil {
		return err
	}

	context.ServiceName = service.Name
	context.ServiceId = service.ID
	context.ServiceType = service.Type

	return StoreContext(context)
}

func GetAuthorizationHeaderValue(tokenType AccessTokenType, token AccessToken) string {
	return string(tokenType) + " " + strings.TrimSpace(string(token))
}

func GetAccessToken() (AccessTokenType, AccessToken, error) {
	tokenType := os.Getenv("QOVERY_CLI_ACCESS_TOKEN_TYPE")
	token := os.Getenv("QOVERY_CLI_ACCESS_TOKEN")

	if tokenType == "" {
		tokenType = os.Getenv("Q_CLI_ACCESS_TOKEN_TYPE")
	}

	if token == "" {
		token = os.Getenv("Q_CLI_ACCESS_TOKEN")
	}

	if tokenType == "" {
		tokenType = "Bearer"
	}

	if token != "" {
		return AccessTokenType("Token"), AccessToken(token), nil
	}

	context, err := GetCurrentContext()
	if err != nil {
		return "", "", err
	}

	token = string(context.AccessToken)
	if token == "" {
		return "", "", errors.New("Access token has not been found. Sign in using 'qovery auth' or 'qovery auth --headless' command. ")
	}

	expired := context.AccessTokenExpiration.Before(time.Now())
	if expired {
		RefreshExpiredTokenSilently()
		_, refreshed, err := GetAccessToken()
		if err != nil {
			return "", "", err
		}
		token = string(refreshed)
	}

	return AccessTokenType(tokenType), AccessToken(token), nil
}

func GetAccessTokenExpiration() (time.Time, error) {
	context, err := GetCurrentContext()
	t := time.Time{}
	if err != nil {
		return t, err
	}

	expiration := context.AccessTokenExpiration
	if expiration == t {
		return t, errors.New("Access token has not been found. Sign in using 'qovery auth' or 'qovery auth --headless' command. ")
	}

	return expiration, nil
}

func SetAccessToken(token AccessToken, expiration time.Time) error {
	context, err := GetCurrentContext()
	if err != nil {
		return err
	}

	context.AccessToken = token
	context.AccessTokenExpiration = expiration

	claims := jwt.MapClaims{}
	_, _ = jwt.ParseWithClaims(string(token), claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(""), nil
	})

	sub, ok := claims["sub"]
	if ok {
		subStr := sub.(string)
		context.User = Name(subStr)
	}

	return StoreContext(context)
}

func GetRefreshToken() (RefreshToken, error) {
	context, err := GetCurrentContext()
	if err != nil {
		return RefreshToken(""), err
	}

	token := context.RefreshToken
	if token == "" {
		return "", errors.New("Refresh token has not been found. Sign in using 'qovery auth' or 'qovery auth --headless' command. ")
	}

	return token, nil
}

func SetRefreshToken(token RefreshToken) error {
	context, err := GetCurrentContext()
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

	err = os.WriteFile(path, []byte("{}"), os.ModePerm)
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

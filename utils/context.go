package utils

import (
	context2 "context"
	"encoding/json"
	"errors"
	"github.com/qovery/qovery-client-go"
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

func GetOrSetCurrentContext(setProject bool, setEnvironment bool, setService bool) (QoveryContext, error) {
	context, _ := GetCurrentContext()
	if isMinimalContextValid(context) &&
		((setProject && context.ProjectId != "") || !setProject) &&
		((setEnvironment && context.EnvironmentId != "") || !setEnvironment) &&
		((setService && context.ServiceId != "") || !setService) {
		return context, nil
	}

	err := SetContext(setProject, setEnvironment, setService, false)

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

func SetContext(setProject bool, setEnvironment bool, setService bool, printFinalContext bool) error {
	_ = PrintContext()
	_ = ResetApplicationContext()

	org, err := SelectAndSetOrganization()
	if err != nil {
		return err
	}

	if !setProject {
		return nil
	}

	project, err := SelectAndSetProject(org.ID)
	if err != nil {
		return err
	}

	if !setEnvironment {
		return nil
	}

	env, err := SelectAndSetEnvironment(project.ID)
	if err != nil {
		return err
	}

	if !setService {
		return nil
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

	if (err != nil || context.OrganizationId == "") && promptContext {
		context, err = GetOrSetCurrentContext(false, false, false)
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

	if (err != nil || context.ProjectId == "") && promptContext {
		context, err = GetOrSetCurrentContext(true, false, false)
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

	if (err != nil || context.EnvironmentId == "") && promptContext {
		context, err = GetOrSetCurrentContext(true, true, false)
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

	if (err != nil || context.ServiceId == "") && promptContext {
		context, err = GetOrSetCurrentContext(true, true, true)
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

func checkOrgaValid(orgaList *qovery.OrganizationResponseList) error {
	if len(orgaList.GetResults()) == 0 {
		return errors.New("you don't have any organization. Please create an account on https://start.qovery.com . ")
	} else {
		return nil
	}
}

func GetAccessToken() (AccessTokenType, AccessToken, error) {
	apiToken := os.Getenv("QOVERY_CLI_ACCESS_TOKEN")
	if apiToken == "" {
		apiToken = os.Getenv("Q_CLI_ACCESS_TOKEN")
	}
	if apiToken != "" {
		return "Token", AccessToken(apiToken), nil
	}

	// User does not use a Token, but a Jwt/Bearer token retrieve it from the context and check it has not expired
	context, err := GetCurrentContext()
	if err != nil {
		return "", "", err
	}

	token := context.AccessToken
	if token == "" {
		return "", "", errors.New("Access token has not been found. Sign in using 'qovery auth' or 'qovery auth --headless' command. ")
	}

	// check the token is valid by trying to list the organizations
	if orgaList, _, err := GetQoveryClient("Bearer", token).OrganizationMainCallsAPI.ListOrganization(context2.Background()).Execute(); err == nil {
		// everything is fine, return the token
		if err = checkOrgaValid(orgaList); err != nil {
			return "", "", err
		}
		return "Bearer", token, nil
	}

	// Means the token is expired or invalid. Try to refresh it
	if token, err = RefreshAccessToken(context.RefreshToken); err != nil {
		return "", "", err
	}

	if orgaList, _, err := GetQoveryClient("Bearer", token).OrganizationMainCallsAPI.ListOrganization(context2.Background()).Execute(); err == nil {
		// everything is fine, return the token
		if err = checkOrgaValid(orgaList); err != nil {
			return "", "", err
		}

		return "Bearer", token, nil
	}

	return "", "", errors.New("Access token is invalid or expired. Sign in using 'qovery auth' or 'qovery auth --headless' command. ")
}

func SetAccessToken(token AccessToken, expiration time.Time, refreshToken RefreshToken) error {
	context, err := GetCurrentContext()
	if err != nil {
		return err
	}

	context.AccessToken = token
	context.AccessTokenExpiration = expiration
	context.RefreshToken = refreshToken

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

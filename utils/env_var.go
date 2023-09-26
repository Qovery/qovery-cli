package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"os"
	"strings"
	"time"
)

var ShowValues bool
var PrettyPrint bool
var IsSecret bool
var ApplicationScope string
var JobScope string
var ContainerScope string
var Alias string
var Key string
var Value string

type EnvVarLines struct {
	lines map[string][]EnvVarLineOutput
}

func NewEnvVarLines() EnvVarLines {
	return EnvVarLines{
		lines: make(map[string][]EnvVarLineOutput),
	}
}
func (e EnvVarLines) Add(env EnvVarLineOutput) {
	var parentKey *string

	if env.AliasParentKey != nil {
		parentKey = env.AliasParentKey
	} else if env.OverrideParentKey != nil {
		parentKey = env.OverrideParentKey
	}

	if parentKey != nil {
		e.lines[*parentKey] = append(e.lines[*parentKey], env)
		return
	}

	e.lines[env.Key] = []EnvVarLineOutput{env}
}

func (e EnvVarLines) Header(prettyPrint bool) []string {
	if prettyPrint {
		return []string{"Key", "Type", "Value", "Updated at", "Service", "Scope"}
	}

	return []string{"Key", "Type", "Parent Key", "Value", "Updated at", "Service", "Scope"}
}

func (e EnvVarLines) Lines(showValues bool, prettyPrint bool) [][]string {
	var lines [][]string

	for _, envVars := range e.lines {
		for idx, envVar := range envVars {
			x := envVar.Data(showValues)
			if idx == 0 || !prettyPrint {
				if prettyPrint {
					lines = append(lines, []string{x[0], x[1], x[3], x[4], x[5], x[6]})
				} else {
					lines = append(lines, x)
				}
			} else {
				x[0] = "└── " + x[0]
				// remove Parent Key value
				lines = append(lines, []string{x[0], x[1], x[3], x[4], x[5], x[6]})
			}
		}
	}

	return lines
}

type EnvVarLineOutput struct {
	Id                string
	Key               string
	Value             *string
	CreatedAt         time.Time
	UpdatedAt         *time.Time
	Service           *string
	Scope             string
	IsSecret          bool
	AliasParentKey    *string
	OverrideParentKey *string
}

func (e EnvVarLineOutput) Data(showValues bool) []string {
	service := "N/A"
	if e.Service != nil {
		service = *e.Service
	}

	value := "********"
	if showValues && e.Value != nil && !e.IsSecret {
		value = *e.Value
	}

	keyType := "Variable"
	if e.IsSecret {
		keyType = "Secret"
	}

	parentKey := "N/A"
	if e.AliasParentKey != nil {
		parentKey = *e.AliasParentKey
		keyType = keyType + " Alias"
	}

	if e.OverrideParentKey != nil {
		parentKey = *e.OverrideParentKey
		keyType = keyType + " Override"
	}

	return []string{e.Key, keyType, parentKey, value, e.UpdatedAt.Format(time.RFC822), service, e.Scope}
}

func FromEnvironmentVariableToEnvVarLineOutput(envVar qovery.EnvironmentVariable) EnvVarLineOutput {
	var aliasParentKey *string
	if envVar.AliasedVariable != nil {
		aliasParentKey = &envVar.AliasedVariable.Key
	}

	var overrideParentKey *string
	if envVar.OverriddenVariable != nil {
		overrideParentKey = &envVar.OverriddenVariable.Key
	}

	return EnvVarLineOutput{
		Id:                envVar.Id,
		Key:               envVar.Key,
		Value:             envVar.Value,
		CreatedAt:         envVar.CreatedAt,
		UpdatedAt:         envVar.UpdatedAt,
		Service:           envVar.ServiceName,
		Scope:             string(envVar.Scope),
		IsSecret:          false,
		AliasParentKey:    aliasParentKey,
		OverrideParentKey: overrideParentKey,
	}
}

func FromSecretToEnvVarLineOutput(secret qovery.Secret) EnvVarLineOutput {
	var aliasParentKey *string
	if secret.AliasedSecret != nil {
		aliasParentKey = &secret.AliasedSecret.Key
	}

	var overrideParentKey *string
	if secret.OverriddenSecret != nil {
		overrideParentKey = &secret.OverriddenSecret.Key
	}

	return EnvVarLineOutput{
		Id:                secret.Id,
		Key:               secret.Key,
		Value:             nil,
		CreatedAt:         secret.CreatedAt,
		UpdatedAt:         secret.UpdatedAt,
		Service:           secret.ServiceName,
		Scope:             string(secret.Scope),
		IsSecret:          true,
		AliasParentKey:    aliasParentKey,
		OverrideParentKey: overrideParentKey,
	}
}

func CreateEnvironmentVariable(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	key string,
	value string,
	scope string,
) error {
	req := qovery.EnvironmentVariableRequest{
		Key:       key,
		Value:     &value,
		MountPath: qovery.NullableString{},
	}

	switch strings.ToUpper(scope) {
	case "PROJECT":
		_, _, err := client.ProjectEnvironmentVariableApi.CreateProjectEnvironmentVariable(
			context.Background(),
			projectId,
		).EnvironmentVariableRequest(req).Execute()

		return err
	case "ENVIRONMENT":
		_, _, err := client.EnvironmentVariableApi.CreateEnvironmentEnvironmentVariable(
			context.Background(),
			environmentId,
		).EnvironmentVariableRequest(req).Execute()

		return err
	case "APPLICATION":
		_, _, err := client.ApplicationEnvironmentVariableApi.CreateApplicationEnvironmentVariable(
			context.Background(),
			serviceId,
		).EnvironmentVariableRequest(req).Execute()

		return err
	case "JOB":
		_, _, err := client.JobEnvironmentVariableApi.CreateJobEnvironmentVariable(
			context.Background(),
			serviceId,
		).EnvironmentVariableRequest(req).Execute()

		return err
	case "CONTAINER":
		_, _, err := client.ContainerEnvironmentVariableApi.CreateContainerEnvironmentVariable(
			context.Background(),
			serviceId,
		).EnvironmentVariableRequest(req).Execute()

		return err
	}

	return errors.New("invalid scope")
}

func CreateSecret(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	key string,
	value string,
	scope string,
) error {
	req := qovery.SecretRequest{
		Key:       key,
		Value:     &value,
		MountPath: qovery.NullableString{},
	}

	switch strings.ToUpper(scope) {
	case "PROJECT":
		_, _, err := client.ProjectSecretApi.CreateProjectSecret(
			context.Background(),
			projectId,
		).SecretRequest(req).Execute()

		return err
	case "ENVIRONMENT":
		_, _, err := client.EnvironmentSecretApi.CreateEnvironmentSecret(
			context.Background(),
			environmentId,
		).SecretRequest(req).Execute()

		return err
	case "APPLICATION":
		_, _, err := client.ApplicationSecretApi.CreateApplicationSecret(
			context.Background(),
			serviceId,
		).SecretRequest(req).Execute()

		return err
	case "JOB":
		_, _, err := client.JobSecretApi.CreateJobSecret(
			context.Background(),
			serviceId,
		).SecretRequest(req).Execute()

		return err
	case "CONTAINER":
		_, _, err := client.ContainerSecretApi.CreateContainerSecret(
			context.Background(),
			serviceId,
		).SecretRequest(req).Execute()

		return err
	}

	return errors.New("invalid scope")
}

func FindEnvironmentVariableByKey(key string, envVars []qovery.EnvironmentVariable) *qovery.EnvironmentVariable {
	for _, envVar := range envVars {
		if envVar.Key == key {
			return &envVar
		}
	}

	return nil
}

func FindSecretByKey(key string, secrets []qovery.Secret) *qovery.Secret {
	for _, secret := range secrets {
		if secret.Key == key {
			return &secret
		}
	}

	return nil
}

func ListEnvironmentVariables(
	client *qovery.APIClient,
	serviceId string,
	serviceType ServiceType,
) ([]qovery.EnvironmentVariable, error) {
	var res *qovery.EnvironmentVariableResponseList

	switch serviceType {
	case ApplicationType:
		r, _, err := client.ApplicationEnvironmentVariableApi.ListApplicationEnvironmentVariable(context.Background(), serviceId).Execute()
		if err != nil {
			return nil, err
		}

		res = r
	case ContainerType:
		r, _, err := client.ContainerEnvironmentVariableApi.ListContainerEnvironmentVariable(context.Background(), serviceId).Execute()
		if err != nil {
			return nil, err
		}

		res = r
	case JobType:
		r, _, err := client.JobEnvironmentVariableApi.ListJobEnvironmentVariable(context.Background(), serviceId).Execute()
		if err != nil {
			return nil, err
		}

		res = r
	}

	if res == nil {
		return nil, errors.New("invalid service type")
	}

	return res.Results, nil
}

func ListSecrets(
	client *qovery.APIClient,
	serviceId string,
	serviceType ServiceType,
) ([]qovery.Secret, error) {
	var res *qovery.SecretResponseList

	switch serviceType {
	case ApplicationType:
		r, _, err := client.ApplicationSecretApi.ListApplicationSecrets(context.Background(), serviceId).Execute()
		if err != nil {
			return nil, err
		}

		res = r
	case ContainerType:
		r, _, err := client.ContainerSecretApi.ListContainerSecrets(context.Background(), serviceId).Execute()
		if err != nil {
			return nil, err
		}

		res = r
	case JobType:
		r, _, err := client.JobSecretApi.ListJobSecrets(context.Background(), serviceId).Execute()
		if err != nil {
			return nil, err
		}

		res = r
	}

	if res == nil {
		return nil, errors.New("invalid service type")
	}

	return res.Results, nil
}

func DeleteEnvironmentVariableByKey(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	serviceType ServiceType,
	key string,
) error {
	envVars, err := ListEnvironmentVariables(client, serviceId, serviceType)
	if err != nil {
		return err
	}

	envVar := FindEnvironmentVariableByKey(key, envVars)

	if envVar == nil {
		return fmt.Errorf("environment variable %s not found", pterm.FgRed.Sprintf(key))
	}

	switch string(envVar.Scope) {
	case "PROJECT":
		_, err := client.ProjectEnvironmentVariableApi.DeleteProjectEnvironmentVariable(
			context.Background(),
			projectId,
			envVar.Id,
		).Execute()

		return err
	case "ENVIRONMENT":
		_, err := client.EnvironmentVariableApi.DeleteEnvironmentEnvironmentVariable(
			context.Background(),
			environmentId,
			envVar.Id,
		).Execute()

		return err
	case "APPLICATION":
		_, err := client.ApplicationEnvironmentVariableApi.DeleteApplicationEnvironmentVariable(
			context.Background(),
			serviceId,
			envVar.Id,
		).Execute()

		return err
	case "JOB":
		_, err := client.JobEnvironmentVariableApi.DeleteJobEnvironmentVariable(
			context.Background(),
			serviceId,
			envVar.Id,
		).Execute()

		return err
	case "CONTAINER":
		_, err := client.ContainerEnvironmentVariableApi.DeleteContainerEnvironmentVariable(
			context.Background(),
			serviceId,
			envVar.Id,
		).Execute()

		return err
	}

	return errors.New("invalid scope")
}

func DeleteSecretByKey(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	serviceType ServiceType,
	key string,
) error {
	secrets, err := ListSecrets(client, serviceId, serviceType)
	if err != nil {
		return err
	}

	secret := FindSecretByKey(key, secrets)

	if secret == nil {
		return fmt.Errorf("secret %s not found", pterm.FgRed.Sprintf(key))
	}

	switch string(secret.Scope) {
	case "PROJECT":
		_, err := client.ProjectSecretApi.DeleteProjectSecret(
			context.Background(),
			projectId,
			secret.Id,
		).Execute()

		return err
	case "ENVIRONMENT":
		_, err := client.EnvironmentVariableApi.DeleteEnvironmentEnvironmentVariable(
			context.Background(),
			environmentId,
			secret.Id,
		).Execute()

		return err
	case "APPLICATION":
		_, err := client.ApplicationSecretApi.DeleteApplicationSecret(
			context.Background(),
			serviceId,
			secret.Id,
		).Execute()

		return err
	case "JOB":
		_, err := client.JobSecretApi.DeleteJobSecret(
			context.Background(),
			serviceId,
			secret.Id,
		).Execute()

		return err
	case "CONTAINER":
		_, err := client.ContainerSecretApi.DeleteContainerSecret(
			context.Background(),
			serviceId,
			secret.Id,
		).Execute()

		return err
	}

	return errors.New("invalid scope")
}

func DeleteByKey(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	serviceType ServiceType,
	key string,
) error {
	err := DeleteEnvironmentVariableByKey(client, projectId, environmentId, serviceId, serviceType, key)
	if err == nil {
		return nil
	}

	err = DeleteSecretByKey(client, projectId, environmentId, serviceId, serviceType, key)
	if err == nil {
		return nil
	}

	return fmt.Errorf("environment variable or secret %s not found", pterm.FgRed.Sprintf(key))
}

func CreateEnvironmentVariableAlias(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	parentEnvironmentVariableId string,
	alias string,
	scope string,
) error {
	key := *qovery.NewKey(alias)

	switch strings.ToUpper(scope) {
	case "PROJECT":
		_, _, err := client.ProjectEnvironmentVariableApi.CreateProjectEnvironmentVariableAlias(
			context.Background(),
			projectId,
			parentEnvironmentVariableId,
		).Key(key).Execute()

		return err
	case "ENVIRONMENT":
		_, _, err := client.EnvironmentVariableApi.CreateEnvironmentEnvironmentVariableAlias(
			context.Background(),
			environmentId,
			parentEnvironmentVariableId,
		).Key(key).Execute()

		return err
	case "APPLICATION":
		_, _, err := client.ApplicationEnvironmentVariableApi.CreateApplicationEnvironmentVariableAlias(
			context.Background(),
			serviceId,
			parentEnvironmentVariableId,
		).Key(key).Execute()

		return err
	case "JOB":
		_, _, err := client.JobEnvironmentVariableApi.CreateJobEnvironmentVariableAlias(
			context.Background(),
			serviceId,
			parentEnvironmentVariableId,
		).Key(key).Execute()

		return err
	case "CONTAINER":
		_, _, err := client.ContainerEnvironmentVariableApi.CreateContainerEnvironmentVariableAlias(
			context.Background(),
			serviceId,
			parentEnvironmentVariableId,
		).Key(key).Execute()

		return err
	}

	return errors.New("invalid scope")
}

func CreateSecretAlias(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	parentSecretId string,
	alias string,
	scope string,
) error {
	key := *qovery.NewKey(alias)

	switch strings.ToUpper(scope) {
	case "PROJECT":
		_, _, err := client.ProjectSecretApi.CreateProjectSecretAlias(
			context.Background(),
			projectId,
			parentSecretId,
		).Key(key).Execute()

		return err
	case "ENVIRONMENT":
		_, _, err := client.EnvironmentSecretApi.CreateEnvironmentSecretAlias(
			context.Background(),
			environmentId,
			parentSecretId,
		).Key(key).Execute()

		return err
	case "APPLICATION":
		_, _, err := client.ApplicationSecretApi.CreateApplicationSecretAlias(
			context.Background(),
			serviceId,
			parentSecretId,
		).Key(key).Execute()

		return err
	case "JOB":
		_, _, err := client.JobSecretApi.CreateJobSecretAlias(
			context.Background(),
			serviceId,
			parentSecretId,
		).Key(key).Execute()

		return err
	case "CONTAINER":
		_, _, err := client.ContainerSecretApi.CreateContainerSecretAlias(
			context.Background(),
			serviceId,
			parentSecretId,
		).Key(key).Execute()

		return err
	}

	return errors.New("invalid scope")
}

func CreateAlias(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	serviceType ServiceType,
	key string,
	alias string,
	scope string,
) error {
	envVars, err := ListEnvironmentVariables(client, serviceId, serviceType)
	if err != nil {
		return err
	}

	envVar := FindEnvironmentVariableByKey(key, envVars)

	if envVar != nil {
		// create alias for environment variable
		return CreateEnvironmentVariableAlias(client, projectId, environmentId, serviceId, envVar.Id, alias, scope)
	}

	secrets, err := ListSecrets(client, serviceId, serviceType)
	if err != nil {
		return err
	}

	secret := FindSecretByKey(key, secrets)
	if secret != nil {
		// create alias for secret
		return CreateSecretAlias(client, projectId, environmentId, serviceId, secret.Id, alias, scope)
	}

	return fmt.Errorf("Environment variable or secret %s not found", pterm.FgRed.Sprintf(key))
}

func CreateEnvironmentVariableOverride(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	parentEnvironmentVariableId string,
	value *string,
	scope string,
) error {
	v := *qovery.NewValue()
	if value != nil {
		v.SetValue(*value)
	}

	switch strings.ToUpper(scope) {
	case "PROJECT":
		_, _, err := client.ProjectEnvironmentVariableApi.CreateProjectEnvironmentVariableOverride(
			context.Background(),
			projectId,
			parentEnvironmentVariableId,
		).Value(v).Execute()

		return err
	case "ENVIRONMENT":
		_, _, err := client.EnvironmentVariableApi.CreateEnvironmentEnvironmentVariableOverride(
			context.Background(),
			environmentId,
			parentEnvironmentVariableId,
		).Value(v).Execute()

		return err
	case "APPLICATION":
		_, _, err := client.ApplicationEnvironmentVariableApi.CreateApplicationEnvironmentVariableOverride(
			context.Background(),
			serviceId,
			parentEnvironmentVariableId,
		).Value(v).Execute()

		return err
	case "JOB":
		_, _, err := client.JobEnvironmentVariableApi.CreateJobEnvironmentVariableOverride(
			context.Background(),
			serviceId,
			parentEnvironmentVariableId,
		).Value(v).Execute()

		return err
	case "CONTAINER":
		_, _, err := client.ContainerEnvironmentVariableApi.CreateContainerEnvironmentVariableOverride(
			context.Background(),
			serviceId,
			parentEnvironmentVariableId,
		).Value(v).Execute()

		return err
	}

	return errors.New("invalid scope")
}

func CreateSecretOverride(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	parentSecretId string,
	value *string,
	scope string,
) error {
	v := *qovery.NewValue()
	if value != nil {
		v.SetValue(*value)
	}

	switch strings.ToUpper(scope) {
	case "PROJECT":
		_, _, err := client.ProjectSecretApi.CreateProjectSecretOverride(
			context.Background(),
			projectId,
			parentSecretId,
		).Value(v).Execute()

		return err
	case "ENVIRONMENT":
		_, _, err := client.EnvironmentSecretApi.CreateEnvironmentSecretOverride(
			context.Background(),
			environmentId,
			parentSecretId,
		).Value(v).Execute()

		return err
	case "APPLICATION":
		_, _, err := client.ApplicationSecretApi.CreateApplicationSecretOverride(
			context.Background(),
			serviceId,
			parentSecretId,
		).Value(v).Execute()

		return err
	case "JOB":
		_, _, err := client.JobSecretApi.CreateJobSecretOverride(
			context.Background(),
			serviceId,
			parentSecretId,
		).Value(v).Execute()

		return err
	case "CONTAINER":
		_, _, err := client.ContainerSecretApi.CreateContainerSecretOverride(
			context.Background(),
			serviceId,
			parentSecretId,
		).Value(v).Execute()

		return err
	}

	return errors.New("invalid scope")
}

func CreateOverride(
	client *qovery.APIClient,
	projectId string,
	environmentId string,
	serviceId string,
	serviceType ServiceType,
	key string,
	value *string,
	scope string,
) error {
	envVars, err := ListEnvironmentVariables(client, serviceId, serviceType)
	if err != nil {
		return err
	}

	envVar := FindEnvironmentVariableByKey(key, envVars)

	if envVar != nil {
		return CreateEnvironmentVariableOverride(client, projectId, environmentId, serviceId, envVar.Id, value, scope)
	}

	secrets, err := ListSecrets(client, serviceId, serviceType)
	if err != nil {
		return err
	}

	secret := FindSecretByKey(key, secrets)
	if secret != nil {
		return CreateSecretOverride(client, projectId, environmentId, serviceId, secret.Id, value, scope)
	}

	return fmt.Errorf("Environment variable or secret %s not found", pterm.FgRed.Sprintf(key))
}

func insertAtIndex(src string, insert string, index int) string {
	// Convert to rune slice if you expect to be working with Unicode
	srcRunes := []rune(src)

	// Handle index out of range cases
	if index < 0 || index > len(srcRunes) {
		return src
	}

	// Create a new rune slice that consists of the original string
	// with the new string inserted at the index
	newRunes := make([]rune, len(srcRunes)+len([]rune(insert)))
	copy(newRunes, srcRunes[:index])
	copy(newRunes[index:], []rune(insert))
	copy(newRunes[index+len([]rune(insert)):], srcRunes[index:])

	// Convert the rune slice back to a string and return it
	return string(newRunes)
}

func getInterpolatedValue(value *string, variables []EnvVarLineOutput) *string {
	if value == nil {
		return nil
	}

	if !strings.Contains(*value, "{{") {
		return value
	}

	runes := []rune(*value)

	startIndex := -1
	endIndex := -1

	// let's found the startIndex and endIndex with "hello_${world}" -> startIndex = 6, endIndex = 11
	foundFirstFirstDelimiter := false
	foundFirstLastDelimiter := false
	for idx, char := range runes {
		if char == '{' && !foundFirstFirstDelimiter {
			foundFirstFirstDelimiter = true
		} else if char == '{' {
			startIndex = idx - 1 // 2 chars -> {{
		} else if startIndex > -1 && char == '}' && !foundFirstLastDelimiter {
			foundFirstLastDelimiter = true
		} else if startIndex > -1 && char == '}' {
			endIndex = idx
			break // we can stop here and interpolate the value
		}
	}

	if startIndex == -1 || endIndex == -1 {
		return value
	}

	// extract key from {{key}}
	keyToInterpolate := string(runes[startIndex+2 : endIndex-1])

	// remove ${{key}} from value
	valueWithoutInterpolation := string(runes[:startIndex]) + string(runes[endIndex+1:])

	finalValue := *value

FirstLoop:
	for _, v := range variables {
		if v.Key == keyToInterpolate {
			if v.AliasParentKey != nil {
				// where v is an Alias, we should interpolate the value of the parent key
				for _, x := range variables {
					if v.AliasParentKey != nil && *v.AliasParentKey == x.Key {
						finalValue = insertAtIndex(valueWithoutInterpolation, *x.Value, startIndex)
						continue FirstLoop
					}
				}
			}

			// work only if the key is a secret or an environment variable
			finalValue = insertAtIndex(valueWithoutInterpolation, *v.Value, startIndex)
			break
		}
	}

	if strings.Contains(finalValue, "{{") && finalValue != *value {
		return getInterpolatedValue(&finalValue, variables)
	}

	return &finalValue
}

func GetEnvVarJsonOutput(variables []EnvVarLineOutput) string {
	var results []interface{}

	for _, v := range variables {
		// TODO improve this

		results = append(results, map[string]interface{}{
			"id":                    v.Id,
			"created_at":            ToIso8601(&v.CreatedAt),
			"updated_at":            ToIso8601(v.UpdatedAt),
			"key":                   v.Key,
			"value":                 v.Value,
			"interpolated_value":    getInterpolatedValue(v.Value, variables),
			"service_name":          v.Service,
			"scope":                 v.Scope,
			"alias_parent_key":      v.AliasParentKey,
			"override_parent_value": v.OverrideParentKey,
		})
	}

	j, err := json.Marshal(results)

	if err != nil {
		PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return string(j)
}

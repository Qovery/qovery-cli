package utils

import (
	"context"
	"errors"
	"github.com/qovery/qovery-client-go"
	"strings"
	"time"
)

var ShowValues bool
var PrettyPrint bool
var IsSecret bool
var Scope string
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
	Key               string
	Value             *string
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
		Key:               envVar.Key,
		Value:             &envVar.Value,
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
		Key:               secret.Key,
		Value:             nil,
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
		Value:     value,
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
		Value:     value,
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

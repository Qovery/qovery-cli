package utils

import (
	"github.com/qovery/qovery-client-go"
	"time"
)

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

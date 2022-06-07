package utils

type Var struct {
	Key   string
	Value string
}

func GenerateExportEnvVarsScript(vars []Var, clusterId string) {
	content := []byte("#!/bin/bash \n")
	for _, variable := range vars {
		line := []byte("echo 'export " + variable.Key + "=" + variable.Value + "'\n")
		content = append(content, line...)
	}

	WriteInFile(clusterId, "script", content)
}

package io

import (
	"github.com/joho/godotenv"
	"strings"
)

func getDotEnvs() map[string]string {
	envs, err := godotenv.Read()
	if err != nil {
		return map[string]string{}
	}

	return envs
}

func GetDotEnvs(envs map[string]string) map[string]string {
	return GetDotEnvsWithAliasSubstitution(getDotEnvs(), envs, 0)
}

func GetDotEnvsWithAliasSubstitution(dotEnvs map[string]string, envs map[string]string, recursionCount int) map[string]string {
	results := map[string]string{}

	for k, v := range dotEnvs {
		if strings.HasPrefix(v, "$") {
			runes := []rune(v)
			k1 := string(runes[1 : len(v)-1])

			for eKey, eValue := range envs {
				if eKey == k1 {
					// substitute
					results[k1] = eValue
				}
			}

			for k, v := range results {
				if k == k1 {
					// substitute
					results[k1] = v
				}
			}

			// prevent the case where there is no valid substitution - eg: the $TEST does not exist
			if recursionCount < 10 {
				return GetDotEnvsWithAliasSubstitution(results, envs, recursionCount+1)
			}
		} else {
			results[k] = v
		}
	}

	return results
}

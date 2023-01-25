package utils

import (
	"errors"
	"fmt"
	"os"

	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
)

func Validate(actionType string) bool {
	return getInput(actionType) == "yes"
}

func getInput(actionType string) string {
	validate := func(input string) error {
		if len(input) <= 0 {
			return errors.New("no input")
		}
		return nil
	}

	templates := &promptui.PromptTemplates{
		Prompt:  "{{ . }}",
		Valid:   "{{ . }}",
		Invalid: "{{ . }}",
		Success: "{{ . }}",
	}

	prompt := promptui.Prompt{
		Label:     fmt.Sprintf("Please type \"yes\" to validate %s: ", actionType),
		Templates: templates,
		Validate:  validate,
	}

	result, err := prompt.Run()
	if err != nil {
		log.Errorf("Prompt failed %v", err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return result
}

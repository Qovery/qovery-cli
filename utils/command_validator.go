package utils

import (
	"errors"
	"fmt"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"os"
)

func Validate(actionType string) bool {
	yes := getInput(actionType)
	if yes != "yes" {
		return false
	}

	return true
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
	}

	return result
}

package util

import (
	"fmt"
	"os"
)

func AskForStringConfirmation(noPrompt bool, message string, requiredAnswer string) bool {
	var response string

	if noPrompt {
		return true
	}

	if message == "" {
		fmt.Printf("Internal CLI error, required answer can't be empty")
		os.Exit(1)
	}

	fmt.Print("\n➤ " + message + ": ")

	_, err := fmt.Scanln(&response)

	if err != nil {
		return AskForStringConfirmation(noPrompt, message, requiredAnswer)
	}

	if response == requiredAnswer {
		return true
	} else {
		fmt.Printf("\nPlease exactly enter '%s' to confirm or Ctrl+c to cancel\n", requiredAnswer)
		return AskForStringConfirmation(noPrompt, message, requiredAnswer)
	}
}

// posString returns the first index of element in slice.
// If slice does not contain element, returns -1.
func posString(slice []string, element string) int {
	for index, elem := range slice {
		if elem == element {
			return index
		}
	}
	return -1
}

// containsString returns true iff slice contains element
func containsString(slice []string, element string) bool {
	return !(posString(slice, element) == -1)
}

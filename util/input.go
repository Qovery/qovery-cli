package util

import (
	"fmt"
)

func AskForInput(optional bool, message string) string {
	var response string

	fmt.Print("➤ " + message + ": ")
	_, err := fmt.Scanln(&response)

	if err != nil {
		fmt.Println(err)
	}

	strResponse := fmt.Sprint(response)

	if !optional && strResponse == "" {
		fmt.Println("Please fill this field")
		return AskForInput(optional, message)
	}

	return strResponse
}

func AskForConfirmation(noPrompt bool, msg string, defaultValue string) bool {
	var response string

	if noPrompt {
		return true
	}

	if msg == "" {
		if defaultValue == "" {
			fmt.Print("➤ Are you sure? (y/n): ")
		} else {
			fmt.Print("➤ Are you sure? (y/n) [default=" + defaultValue + "]: ")
		}
	} else {
		if defaultValue == "" {
			fmt.Print("➤ " + msg + " (y/n): ")
		} else {
			fmt.Print("➤ " + msg + " (y/n) [default=" + defaultValue + "]: ")
		}
	}

	_, err := fmt.Scanln(&response)

	if response == "" {
		response = defaultValue
	}

	if err != nil && defaultValue == "" {
		fmt.Println("Please type yes or no and then press enter:")
		return AskForConfirmation(noPrompt, msg, defaultValue)
	}

	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	notOkayResponses := []string{"n", "N", "no", "No", "NO", ""}

	if containsString(okayResponses, response) {
		return true
	} else if containsString(notOkayResponses, response) {
		return false
	} else {
		fmt.Println("Please type yes or no and then press enter:")
		return AskForConfirmation(noPrompt, msg, defaultValue)
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

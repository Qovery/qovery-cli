package util

import (
	"fmt"
	"strconv"
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

func AskForSelect(choices []string, message string, defaultChoice string) string {
	var response string

	fmt.Println("➤ " + message + ": ")

	if defaultChoice == "" {
		fmt.Println("0. none")
	}

	for k, v := range choices {
		fmt.Printf("%d. %s\n", k+1, v)
	}

	fmt.Print("➤ Your choice: ")
	_, err := fmt.Scanln(&response)

	if err != nil {
		fmt.Println(err)
	}

	strChoice := fmt.Sprint(response)

	if defaultChoice != "" && strChoice == "" {
		fmt.Printf("Please choose an option between 1 and %d\n", len(choices))
		return AskForSelect(choices, message, defaultChoice)
	}

	choice, err := strconv.Atoi(strChoice)
	if err != nil && (choice < 0 || choice > len(choices)) {
		fmt.Printf("Please choose an option between 1 and %d\n", len(choices))
		return AskForSelect(choices, message, defaultChoice)
	}

	if choice == 0 {
		return ""
	}

	return choices[choice-1]
}

func AskForConfirmation(noPrompt bool, message string, defaultValue string) bool {
	var response string

	if noPrompt {
		return true
	}

	if message == "" {
		if defaultValue == "" {
			fmt.Print("➤ Are you sure? (y/n): ")
		} else {
			fmt.Print("➤ Are you sure? (y/n) [default=" + defaultValue + "]: ")
		}
	} else {
		if defaultValue == "" {
			fmt.Print("➤ " + message + " (y/n): ")
		} else {
			fmt.Print("➤ " + message + " (y/n) [default=" + defaultValue + "]: ")
		}
	}

	_, err := fmt.Scanln(&response)

	if response == "" {
		response = defaultValue
	}

	if err != nil && defaultValue == "" {
		fmt.Println("Please type yes or no and then press enter:")
		return AskForConfirmation(noPrompt, message, defaultValue)
	}

	okayResponses := []string{"y", "Y", "yes", "Yes", "YES"}
	notOkayResponses := []string{"n", "N", "no", "No", "NO", ""}

	if containsString(okayResponses, response) {
		return true
	} else if containsString(notOkayResponses, response) {
		return false
	} else {
		fmt.Println("Please type yes or no and then press enter:")
		return AskForConfirmation(noPrompt, message, defaultValue)
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

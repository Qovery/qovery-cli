package utils

import (
	"fmt"
	"github.com/fatih/color"
	"os"
)

func PrintlnError(err error) {
	fmt.Printf("%s %v\n", color.RedString("Error:"), err)
}

func PrintlnInfo(info string) {
	fmt.Printf("%v: %v\n", color.CyanString("Qovery"), info)
}

func PrintlnContext() {
	PrintlnInfo("Current context:")
	_, oName, err := CurrentOrganization()
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
	}
	fmt.Printf("%v: %v\n", color.CyanString("Organization"), oName)
	_, pName, err := CurrentProject()
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
	}
	fmt.Printf("%v: %v\n", color.CyanString("Project"), pName)
	_, eName, err := CurrentEnvironment()
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
	}
	fmt.Printf("%v: %v\n", color.CyanString("Environment"), eName)
	_, aName, err := CurrentApplication()
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
	}
	fmt.Printf("%v: %v\n", color.CyanString("Application"), aName)
}

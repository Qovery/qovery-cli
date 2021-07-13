package utils

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/pterm/pterm"
	"os"
)

func PrintlnError(err error) {
	fmt.Printf("%s %v\n", color.RedString("Error:"), err)
}

func PrintlnInfo(info string) {
	fmt.Printf("%v: %v\n", color.CyanString("Qovery"), info)
}

func PrintlnContext() {
	_, oName, err := CurrentOrganization()
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
	}
	_, pName, err := CurrentProject()
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
	}
	_, eName, err := CurrentEnvironment()
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
	}
	_, aName, err := CurrentApplication()
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
	}
	err = pterm.DefaultTable.WithData(pterm.TableData{
		{"Organization", string(oName)},
		{"Project", string(pName)},
		{"Environment", string(eName)},
		{"Application", string(aName)},
	}).Render()
}

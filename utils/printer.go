package utils

import (
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/getsentry/sentry-go"
	"github.com/pterm/pterm"
)

func PrintlnErrorMessage(err string) {
	PrintlnError(errors.New(err))
}

func PrintlnError(err error) {
	sentry.CaptureException(err)
	fmt.Printf("%s: %v\n", color.RedString("Error"), err)
}

func PrintlnInfo(info string) {
	fmt.Printf("%v: %v\n", color.CyanString("Qovery"), info)
}

func PrintlnContext() error {
	_, oName, err := CurrentOrganization()
	if err != nil {
		return err
	}
	_, pName, err := CurrentProject()
	if err != nil {
		return err
	}
	_, eName, err := CurrentEnvironment()
	if err != nil {
		return err
	}
	_, aName, err := CurrentApplication()
	if err != nil {
		return err
	}
	err = pterm.DefaultTable.WithData(pterm.TableData{
		{"Organization", string(oName)},
		{"Project", string(pName)},
		{"Environment", string(eName)},
		{"Application", string(aName)},
	}).Render()

	return nil
}

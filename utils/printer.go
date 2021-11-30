package utils

import (
	"fmt"
	"github.com/fatih/color"
	"github.com/getsentry/sentry-go"
	"github.com/pterm/pterm"
	log "github.com/sirupsen/logrus"
	"time"
)

func PrintlnError(err error) {
	localHub := sentry.CurrentHub().Clone()
	localHub.Scope().SetTransaction(err.Error())
	localHub.CaptureException(err)
	fmt.Printf("%s: %v\n", color.RedString("Error"), err)
	defer localHub.Flush(5 * time.Second)
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
	_ = pterm.DefaultTable.WithData(pterm.TableData{
		{"Organization", string(oName)},
		{"Project", string(pName)},
		{"Environment", string(eName)},
		{"Application", string(aName)},
	}).Render()

	return nil
}

func DryRunPrint(dryRunDisbled bool) {
	green := color.New(color.FgGreen).SprintFunc()

	message := green("enabled")

	if dryRunDisbled {
		red := color.New(color.FgRed).SprintFunc()
		message = red("disabled")
	}

	log.Infof("Dry run: %s", message)
}

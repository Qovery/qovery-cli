package utils

import (
	"fmt"
	"github.com/fatih/color"
	"os"
	//	"github.com/getsentry/sentry-go"
	"github.com/pterm/pterm"
	log "github.com/sirupsen/logrus"
	//	"time"
)

func PrintlnError(err error) {
	//localHub := sentry.CurrentHub().Clone()
	//localHub.Scope().SetTransaction(err.Error())
	//localHub.CaptureException(err)
	fmt.Printf("%s: %v\n", color.RedString("Error"), err)
	//defer localHub.Flush(5 * time.Second)
}

func PrintlnInfo(info string) {
	fmt.Printf("%v: %v\n", color.CyanString("Info"), info)
}

func Println(text string) {
	fmt.Printf("%v\n", text)
}

// PrintlnErrorToStderr and PrintlnInfoToStderr mirror PrintlnError / PrintlnInfo but write
// to stderr. Use them from any command whose stdout carries data the caller pipes -- JSON,
// log lines, a list meant for a shell loop -- where a message printed to stdout would end up
// mixed into the payload and break the pipe.
func PrintlnErrorToStderr(err error) {
	_, _ = fmt.Fprintf(os.Stderr, "%s: %v\n", color.RedString("Error"), err)
}

func PrintlnInfoToStderr(info string) {
	_, _ = fmt.Fprintf(os.Stderr, "%v: %v\n", color.CyanString("Info"), info)
}

func PrintContext() error {
	_, oName, err := CurrentOrganization(false)
	if err != nil {
		return err
	}
	_, pName, err := CurrentProject(false)
	if err != nil {
		return err
	}
	_, eName, err := CurrentEnvironment(false)
	if err != nil {
		return err
	}
	srv, err := CurrentService(false)
	if err != nil {
		return err
	}
	_ = pterm.DefaultTable.WithData(pterm.TableData{
		{"Organization", string(oName)},
		{"Project", string(pName)},
		{"Environment", string(eName)},
		{"Service", string(srv.Name)},
		{"Type", string(srv.Type)},
	}).Render()

	return nil
}

func DryRunPrint(dryRunDisabled bool) {
	green := color.New(color.FgGreen).SprintFunc()

	message := green("enabled")

	if dryRunDisabled {
		red := color.New(color.FgRed).SprintFunc()
		message = red("disabled")
	}

	log.Infof("Dry run: %s", message)
}

func PrintTable(headers []string, data [][]string) error {
	table := pterm.TableData{
		headers,
	}

	for _, row := range data {
		table = append(table, row)
	}

	return pterm.DefaultTable.WithHasHeader().WithData(table).Render()
}

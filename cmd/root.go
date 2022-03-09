package cmd

import (
	"github.com/getsentry/sentry-go"
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
	"time"
)

var rootCmd = &cobra.Command{
	Use:   "qovery",
	Short: "A Command-line Interface of the Qovery platform",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	if !utils.QoveryContextExists() {
		err := utils.InitializeQoveryContext()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
	}
	initSentry()
}

func initSentry() {
	pkg.GetCurrentVersion()
	err := sentry.Init(sentry.ClientOptions{
		Dsn:         "https://199e1e8385d94377a98676dadcd77e2d@o471935.ingest.sentry.io/5866472",
		Environment: "prod",
		Release:     pkg.GetCurrentVersion(),
		// Enable printing of SDK debug messages.
		// Useful when getting started or trying to figure something out.
		Debug: false,
		BeforeSend: func(event *sentry.Event, hint *sentry.EventHint) *sentry.Event {
			if event.Exception == nil {
				return event
			}
			if len(event.Exception) > 0 && (event.Exception[0].Stacktrace == nil || event.Exception[0].Stacktrace.Frames == nil) {
				return event
			}
			if len(event.Exception[0].Stacktrace.Frames) > 0 {
				frames := event.Exception[0].Stacktrace.Frames
				event.Exception[0].Stacktrace.Frames = frames[:len(frames)-1]
				frames = event.Exception[0].Stacktrace.Frames
				path := frames[len(frames)-1].AbsPath
				event.Transaction = path
			}
			return event
		},
	})
	if err != nil {
		utils.PrintlnError(err)
	}
	// Flush buffered events before the program terminates.
	// Set the timeout to the maximum duration the program can afford to wait.
	defer sentry.Recover()
	defer sentry.Flush(5 * time.Second)
}

//go:build windows

package cmd

import "os"

func interactiveCommandSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

//go:build !unix

package cmd

import (
	"fmt"
	"os"
)

func lockDemoTokenDirectory(dir string) (*os.File, error) {
	return nil, fmt.Errorf("qovery demo requires a Unix shell; on Windows, use WSL")
}

//go:build unix

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func lockDemoTokenDirectory(dir string) (*os.File, error) {
	file, err := os.OpenFile(filepath.Join(dir, ".qovery-token.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("open demo token lock: %w", err)
	}
	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("lock demo token directory (another demo command may be running): %w", err)
	}
	return file, nil
}

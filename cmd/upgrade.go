// +build !windows

package cmd

import (
	"fmt"
	"github.com/kardianos/osext"
	"github.com/mholt/archiver/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
	"net/http"
	"os"
	"os/exec"
	"qovery.go/io"
	"runtime"
)

import iio "io"

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade Qovery CLI to latest version",
	Long: `UPGRADE performs an upgrade of the binary. For example:

	qovery upgrade`,
	Run: func(cmd *cobra.Command, args []string) {
		currentBinaryFilename, _ := osext.Executable()
		filename := "qovery"
		archivePath := "/tmp/"
		archiveName := filename + ".tgz"
		archivePathName := archivePath + archiveName
		uncompressPath := "/tmp/" + filename + "/"
		uncompressQoveryBinaryPath := uncompressPath + filename
		cleanList := []string{uncompressPath, archivePathName}

		available, message, desiredVersion := io.CheckAvailableNewVersion()
		if !available {
			fmt.Print(message)
			os.Exit(0)
		}

		url := fmt.Sprintf("https://github.com/Qovery/qovery-cli/releases/download/v%s/qovery-cli_%s_%s_%s.tar.gz",
			desiredVersion, desiredVersion, runtime.GOOS, runtime.GOARCH)

		binaryWriteAccess := unix.Access(currentBinaryFilename, unix.W_OK)
		if binaryWriteAccess != nil {
			fmt.Printf("Upgrade cancelled: no write permission on the Qovery CLI binary file: %s", currentBinaryFilename)
			cleanArchives(cleanList)
			os.Exit(1)
		}

		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("Error while downloading the latest version: %s", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		out, err := os.Create(archivePathName)
		if err != nil {
			fmt.Printf("Error while overriding Qovery CLI binary file: %s", err)
			os.Exit(1)
		}
		defer out.Close()

		_, err = iio.Copy(out, resp.Body)
		if err != nil {
			fmt.Printf("Error while adding content to Qovery CLI binary file: %s", err)
			os.Exit(1)
		}

		if _, err := os.Stat(uncompressPath); !os.IsNotExist(err) {
			os.RemoveAll(uncompressPath)
		}

		err = archiver.Unarchive(archivePathName, uncompressPath)
		if err != nil {
			fmt.Printf("Error while uncompressing the archive: %s", err)
			os.Exit(1)
		}

		// Fork to avoid override issue on a a running program
		fmt.Printf("\nUpgrading Qovery CLI to version %s\n", desiredVersion)
		command := exec.Command("/bin/sh", "-c", "sleep 1 ; mv "+uncompressQoveryBinaryPath+" "+
			currentBinaryFilename)
		command.Start()
	},
}

func init() {
	RootCmd.AddCommand(upgradeCmd)
}

func cleanArchives(listToRemove []string) {
	for _, value := range listToRemove {
		err := os.RemoveAll(value)
		if err != nil {
			fmt.Printf("Error while removing the element: %s", err)
			os.Exit(1)
		}
	}
}

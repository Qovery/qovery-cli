//go:build !windows
// +build !windows

package cmd

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"

	"github.com/kardianos/osext"
	"github.com/mholt/archives"
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

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
		uncompressPath := "/tmp/"
		uncompressQoveryBinaryPath := uncompressPath + filename
		cleanList := []string{archivePathName, uncompressQoveryBinaryPath}

		available, message, desiredVersion := pkg.CheckAvailableNewVersion()
		if !available {
			fmt.Print(message)
			os.Exit(0)
		}

		urlFilename := fmt.Sprintf("qovery-cli_%s_%s_%s.tar.gz", desiredVersion, runtime.GOOS, runtime.GOARCH)
		url := fmt.Sprintf("https://github.com/Qovery/qovery-cli/releases/download/v%s/%s", desiredVersion, urlFilename)

		binaryWriteAccess := unix.Access(currentBinaryFilename, unix.W_OK)
		if binaryWriteAccess != nil {
			utils.PrintlnError(fmt.Errorf("upgrade cancelled: no write permission on the Qovery CLI binary file: %s", currentBinaryFilename))
			cleanArchives(cleanList)
			os.Exit(0)
		}

		resp, err := http.Get(url)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("error while downloading the latest version: %s", err))
			os.Exit(0)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				utils.PrintlnError(fmt.Errorf("error while closing response body: %s", err))
			}
		}()

		out, err := os.Create(archivePathName)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("error while overriding Qovery CLI binary file: %s", err))
			os.Exit(0)
		}
		defer func() {
			if err := out.Close(); err != nil {
				utils.PrintlnError(fmt.Errorf("error while closing output file: %s", err))
			}
		}()

		if _, err := os.Stat(uncompressPath); !os.IsNotExist(err) {
			if err := os.RemoveAll(uncompressPath); err != nil {
				utils.PrintlnError(fmt.Errorf("error while removing uncompressed path: %s", err))
				os.Exit(0)
			}
		}

		// Decompress the tar.gz and extract the cli
		format, stream, err := archives.Identify(context.Background(), urlFilename, resp.Body)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("cannot identify archive format: %s", err))
			os.Exit(0)
		}

		if ex, ok := format.(archives.Extractor); ok {

			// function that will be called for every file inside the archive.
			// archives.FileInfo is going to contain the file info inside the archive
			err = ex.Extract(context.Background(), stream, func(ctx context.Context, f archives.FileInfo) error {
				if f.NameInArchive != "qovery" {
					return nil
				}

				// Extract the cli from the archive on disk
				cliFileInsideArchive, _ := f.Open()
				defer func() {
					if err := cliFileInsideArchive.Close(); err != nil {
						utils.PrintlnError(fmt.Errorf("error while closing archive file: %s", err))
					}
				}()

				cliFileOnFS, _ := os.Create(uncompressQoveryBinaryPath)
				defer func() {
					if err := cliFileOnFS.Close(); err != nil {
						utils.PrintlnError(fmt.Errorf("error while closing filesystem file: %s", err))
					}
				}()

				_, err = io.Copy(cliFileOnFS, cliFileInsideArchive)
				if err != nil {
					utils.PrintlnError(fmt.Errorf("error while uncompressing the cli on disk: %s", err))
					os.Exit(0)
				}

				_ = cliFileOnFS.Chmod(0555)
				return nil
			})
		}

		if err != nil {
			utils.PrintlnError(fmt.Errorf("error while uncompressing the archive: %s", err))
			os.Exit(0)
		}

		// Fork to avoid override issue on a running program
		utils.PrintlnInfo(fmt.Sprintf("\nUpgrading Qovery CLI to version %s\n", desiredVersion))
		command := exec.Command("/bin/sh", "-c", "mv "+uncompressQoveryBinaryPath+" "+currentBinaryFilename)
		err = command.Start()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}

func cleanArchives(listToRemove []string) {
	for _, value := range listToRemove {
		err := os.RemoveAll(value)
		if err != nil {
			utils.PrintlnError(fmt.Errorf("error while removing the element: %s", err))
			os.Exit(0)
		}
	}
}

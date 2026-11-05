package cmd

import (
	"errors"
	"os"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	archiveProfile       string
	requestOnly          bool
	downloadS3ArchiveCmd = &cobra.Command{
		Use:   "download-s3-archive",
		Short: "Download S3 archive by execution id, with an AWS profile that assumes the archive-reader role",
		Run: func(cmd *cobra.Command, args []string) {
			if err := downloadS3Archive(cmd); err != nil {
				utils.PrintlnErrorToStderr(err)
				os.Exit(1)
			}
		},
	}
)

func init() {
	downloadS3ArchiveCmd.Flags().StringVarP(&execId, "exec-id", "e", "", "Execution id (required)")
	downloadS3ArchiveCmd.Flags().StringVarP(&directory, "directory", "d", ".", "Directory where the archive will be downloaded")
	downloadS3ArchiveCmd.Flags().BoolVar(&requestOnly, "request-only", false, "Print only the engine request of the archive on stdout instead of writing the archive")
	downloadS3ArchiveCmd.Flags().StringVarP(&archiveProfile, "profile", "p", pkg.DefaultArchiveProfile, "AWS profile that assumes the archive-reader role")
	adminCmd.AddCommand(downloadS3ArchiveCmd)
}

func downloadS3Archive(cmd *cobra.Command) error {
	switch {
	case execId == "":
		return errors.New("--exec-id is required")
	case archiveProfile == "":
		return errors.New("--profile must not be empty")
	case requestOnly && cmd.Flags().Changed("directory"):
		return errors.New("--directory cannot be used with --request-only")
	case requestOnly:
		return pkg.PrintS3ArchiveEngineRequest(execId, archiveProfile)
	default:
		return pkg.DownloadS3Archive(execId, directory, archiveProfile)
	}
}

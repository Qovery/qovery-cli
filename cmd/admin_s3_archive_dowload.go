package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	archiveProfile       string
	downloadS3ArchiveCmd = &cobra.Command{
		Use:   "download-s3-archive",
		Short: "Download S3 archive by execution id, with an AWS profile that assumes the archive-reader role",
		Run: func(cmd *cobra.Command, args []string) {
			downloadS3Archive()
		},
	}
)

func init() {
	downloadS3ArchiveCmd.Flags().StringVarP(&execId, "exec-id", "e", "", "Execution id")
	downloadS3ArchiveCmd.Flags().StringVarP(&directory, "directory", "d", ".", "Directory where the archive will be downloaded")
	downloadS3ArchiveCmd.Flags().StringVarP(&archiveProfile, "profile", "p", pkg.DefaultArchiveProfile, "AWS profile that assumes the archive-reader role")
	orgaErr = downloadS3ArchiveCmd.MarkFlagRequired("exec-id")
	adminCmd.AddCommand(downloadS3ArchiveCmd)
}

func downloadS3Archive() {
	if orgaErr != nil {
		log.Error("Invalid organization Id")
	} else if archiveProfile == "" {
		log.Error("--profile must not be empty")
	} else {
		pkg.DownloadS3Archive(execId, directory, archiveProfile)
	}
}

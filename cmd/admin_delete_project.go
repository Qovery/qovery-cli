package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

var (
	adminDeleteProjectCmd = &cobra.Command{
		Use:   "force-delete-project",
		Short: "Force delete project by id (only Qovery DB side, without calling the engine)",
		Run: func(cmd *cobra.Command, args []string) {
			deleteProjectById()
		},
	}
)

func init() {
	adminDeleteProjectCmd.Flags().StringVarP(&projectId, "project", "p", "", "Project's id")
	adminDeleteProjectCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	orgaErr = adminDeleteProjectCmd.MarkFlagRequired("project")
	adminCmd.AddCommand(adminDeleteProjectCmd)
}

func deleteProjectById() {
	if orgaErr != nil {
		log.Error("Invalid project Id")
	} else {
		pkg.DeleteProjectById(projectId, dryRun)
	}
}

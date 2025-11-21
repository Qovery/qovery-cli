package cmd

import (
	"strings"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/spf13/cobra"
)

var (
	organizationIds      []string
	allowFailedClusters  bool

	adminDeleteOrgaCmd = &cobra.Command{
		Use:   "delete",
		Short: "Delete one or more organizations by their IDs",
		Long: `Delete one or more organizations by providing their IDs.

Examples:
  # Delete a single organization
  qovery admin organization delete --organization-id org-123

  # Delete multiple organizations (comma-separated)
  qovery admin organization delete --organization-id "org-123,org-456,org-789"

  # Delete multiple organizations (repeated flag)
  qovery admin organization delete --organization-id org-123 --organization-id org-456

  # Mix both formats
  qovery admin organization delete -o "org-123,org-456" -o org-789

  # Allow deletion of organizations with failed clusters
  qovery admin organization delete -o org-123 --allow-failed-clusters

  # Disable dry-run to actually delete
  qovery admin organization delete -o org-123 --disable-dry-run`,
		Run: func(cmd *cobra.Command, args []string) {
			deleteOrganizations()
		},
	}
)

func init() {
	adminDeleteOrgaCmd.Flags().StringSliceVarP(&organizationIds, "organization-id", "o", []string{}, "Organization ID(s) to delete (comma-separated or repeated flag)")
	adminDeleteOrgaCmd.Flags().BoolVarP(&allowFailedClusters, "allow-failed-clusters", "f", false, "Allow deletion of organizations with failed or non-deployed clusters")
	adminDeleteOrgaCmd.Flags().BoolVarP(&dryRun, "disable-dry-run", "y", false, "Disable dry run mode")
	_ = adminDeleteOrgaCmd.MarkFlagRequired("organization-id")
	adminCmd.AddCommand(adminDeleteOrgaCmd)
}

func deleteOrganizations() {
	// Parse comma-separated values in case user provides "id1,id2,id3"
	var parsedIds []string
	for _, id := range organizationIds {
		// Split by comma and trim spaces
		parts := strings.Split(id, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				parsedIds = append(parsedIds, trimmed)
			}
		}
	}

	pkg.DeleteOrganizations(parsedIds, allowFailedClusters, dryRun)
}

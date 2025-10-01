package cmd

import (
	"github.com/qovery/qovery-cli/pkg/auditlog"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var (
	auditLogDowndloadCmd = &cobra.Command{
		Use:   "download",
		Short: "Download audit logs",
		Long: `> Description
-------------
This command provides an easy way to download audit logs.
Date parameters must follow the ISO-8601 format, i.e:
* 2025-10-02T01:04:45+12:00 is valid
* 2025-10-02T01:04:45Z is valid
* 2025-10-02 01:04:45Z is invalid (missing T separator)

> Examples 
----------
* Search from a specific date to now:
qovery audit-log download --from-date 2025-09-01T01:04:45+02:00

* Search between a range of dates
qovery audit-log download --from-date 2025-09-01T01:04:45Z --to-date 2025-09-02T02:00:00Z
`,
		Run: func(cmd *cobra.Command, args []string) {
			downloadAuditLogs()
		},
	}

	fromDate string
	toDate   string
)

func init() {
	auditLogDowndloadCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	auditLogDowndloadCmd.Flags().StringVarP(&fromDate, "from-date", "f", "", "Start date for the search following ISO-8601 format")
	auditLogDowndloadCmd.Flags().StringVarP(&toDate, "to-date", "t", "", "End date for the search following ISO-8601 format (defaulted to 'now')")

	_ = auditLogDowndloadCmd.MarkFlagRequired("from-date")

	auditLogCmd.AddCommand(auditLogDowndloadCmd)
}

func downloadAuditLogs() {
	// Get organization ID
	client := utils.GetQoveryClientPanicInCaseOfError()
	organizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
	checkError(err)

	// Get access token
	tokenType, token, err := utils.GetAccessToken()
	checkError(err)

	// Create audit log service
	auditLogService := auditlog.NewService()

	// Download audit logs
	options := auditlog.DownloadOptions{
		OrganizationID: organizationId,
		FromDate:       fromDate,
		ToDate:         toDate,
		TokenType:      string(tokenType),
		Token:          string(token),
	}

	err = auditLogService.DownloadAuditLogs(options)
	checkError(err)
}

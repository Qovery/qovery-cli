package cmd

import (
	"context"
	"encoding/json"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
)

var operatorStatusOrganization string
var operatorStatusCluster string
var operatorStatusJSON bool

var clusterOperatorStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the Qovery Operator connection and version status",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		commandContext, err := newOperatorCommandContext(operatorStatusOrganization, operatorStatusCluster)
		if err != nil {
			utils.PrintlnError(err)
			return
		}
		status, _, err := commandContext.api.ClusterOperatorAPI.
			GetClusterOperatorStatus(context.Background(), commandContext.organizationID, commandContext.clusterID).
			Execute()
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		if operatorStatusJSON {
			output, err := json.MarshalIndent(status, "", "  ")
			if err != nil {
				utils.PrintlnError(err)
				return
			}
			utils.Println(string(output))
			return
		}

		connected := "no"
		if status.OperatorConnected {
			connected = "yes"
		}
		lastHeartbeat := "never"
		if heartbeat := status.LastHeartbeat.Get(); heartbeat != nil {
			lastHeartbeat = heartbeat.Format("2006-01-02T15:04:05Z07:00")
		}
		data := [][]string{{
			string(status.Status),
			connected,
			lastHeartbeat,
			displayVersion(status.OperatorVersion),
			displayVersion(status.DesiredImageVersion),
			displayVersion(status.ReportedChartVersion),
			displayVersion(status.DesiredChartVersion),
		}}
		if err := utils.PrintTable(
			[]string{"Status", "Connected", "Last heartbeat", "Image", "Target image", "Chart", "Target chart"},
			data,
		); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
	},
}

func init() {
	clusterOperatorCmd.AddCommand(clusterOperatorStatusCmd)
	clusterOperatorStatusCmd.Flags().StringVar(&operatorStatusOrganization, "organization", "", "Organization name")
	clusterOperatorStatusCmd.Flags().StringVar(&operatorStatusCluster, "cluster", "", "Cluster name")
	clusterOperatorStatusCmd.Flags().BoolVar(&operatorStatusJSON, "json", false, "JSON output")
	_ = clusterOperatorStatusCmd.MarkFlagRequired("cluster")
}

package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"os"
)

var clusterDebugPodCmd = &cobra.Command{
	Use:   "debug-pod",
	Short: "Launch a debug pod and attach to it",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		if organizationId != "" {
			organizationId, err = usercontext.GetOrganizationContextResourceId(client, organizationName)
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
			}
		}

		flavor := "REGULAR_PRIVILEGE"
		if fullPriviledge {
			flavor = "FULL_PRIVILEGE"
		}
		request := DebugPodRequest{
			utils.Id(organizationId),
			utils.Id(clusterId),
			0,
			0,
			flavor,
			nodeSelector,
		}

		pkg.ExecShell(&request, "/shell/debug")
	},
}

type DebugPodRequest struct {
	OrganizationID utils.Id `url:"organization"`
	ClusterID      utils.Id `url:"cluster"`
	TtyWidth       uint16   `url:"tty_width"`
	TtyHeight      uint16   `url:"tty_height"`
	Flavor         string   `url:"flavor"`
	NodeSelector   string   `url:"node_selector,omitempty"`
}

func (s *DebugPodRequest) SetTtySize(width uint16, height uint16) {
	s.TtyWidth = width
	s.TtyHeight = height
}

var fullPriviledge bool
var nodeSelector string

func init() {
	clusterCmd.AddCommand(clusterDebugPodCmd)
	clusterDebugPodCmd.Flags().StringVarP(&organizationId, "organization-id", "o", "", "Organization ID")
	clusterDebugPodCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster ID")
	clusterDebugPodCmd.Flags().StringVarP(&nodeSelector, "node-selector", "n", "", "Specify a node selector for the debug pod to be started on")
	clusterDebugPodCmd.Flags().BoolVarP(&fullPriviledge, "full-privilege", "p", false, "Start a full privileged debug pod which has access to host machine. ")
	_ = clusterDebugPodCmd.MarkFlagRequired("cluster-id")
}

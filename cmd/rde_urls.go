package cmd

import (
	"fmt"
	"os"

	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeUrlsCmd = &cobra.Command{
	Use:   "urls",
	Short: "List workspace URLs for running RDEs",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		var children []rdeChildInfo

		if rdeBlueprintProjectName != "" {
			bp, err := rdeFindBlueprintByProjectName(client, orgId, rdeBlueprintProjectName)
			if err != nil {
				utils.PrintlnError(err)
				os.Exit(1)
				panic("unreachable")
			}
			children, err = rdeListChildren(client, orgId, bp.ProjectId)
			checkError(err)
		} else {
			children, err = rdeListAllChildren(client, orgId)
			checkError(err)
		}

		if len(children) == 0 {
			utils.Println("No RDE instances found.")
			return
		}

		var data [][]string
		for _, child := range children {
			if child.EnvId == "" {
				continue
			}
			status, err := rdeGetEnvStatus(client, child.EnvId)
			if err != nil {
				continue
			}
			if status == qovery.STATEENUM_DEPLOYED || status == qovery.STATEENUM_RESTARTED {
				url := rdeGetWorkspaceUrl(client, child.EnvId)
				if url == "" {
					url = "-"
				}
				data = append(data, []string{child.ProjectName, url})
			}
		}

		if len(data) == 0 {
			utils.Println("No running RDEs with workspace URLs found.")
			return
		}

		err = utils.PrintTable([]string{"Name", "Workspace URL"}, data)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable")
		}

		utils.Println(fmt.Sprintf("\n%d running RDE(s) with workspace URLs.", len(data)))
	},
}

func init() {
	rdeCmd.AddCommand(rdeUrlsCmd)
	rdeUrlsCmd.Flags().StringVarP(&rdeBlueprintProjectName, "blueprint", "b", "", "Filter by Blueprint Project Name")
	rdeUrlsCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
}

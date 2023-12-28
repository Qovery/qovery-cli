package cmd

import (
	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/spf13/cobra"
	"strconv"
	"strings"
)

var serviceListPods = &cobra.Command{
	Use:   "list-pods",
	Short: "List the pods of a service with their pods",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		var portForwardRequest *pkg.PortForwardRequest
		var err error
		if len(args) > 0 {
			portForwardRequest, err = portForwardRequestWithApplicationUrl(args)
		} else {
			portForwardRequest, err = portForwardRequestWithoutArg()
		}
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		pods, err := pkg.ExecListPods(portForwardRequest)
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		var data [][]string
		for _, pod := range pods.Pods {
			ports := make([]string, len(pod.Ports))
			for i, x := range pod.Ports {
				ports[i] = strconv.FormatUint(uint64(x), 10)
			}
			data = append(data, []string{pod.Name, strings.Join(ports, ", ")})
		}
		_ = utils.PrintTable([]string{"Pod Name", "Ports"}, data)
	},
}

func init() {
	var serviceListPodsCmd = serviceListPods
	rootCmd.AddCommand(serviceListPodsCmd)
}

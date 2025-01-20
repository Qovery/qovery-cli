package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/appscode/go-querystring/query"
	"github.com/gorilla/websocket"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
)

var clusterListNodesCmd = &cobra.Command{
	Use:   "list-nodes",
	Short: "List cluster nodes",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		organizationId, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		request := ListNodesRequest{
			utils.Id(organizationId),
			utils.Id(clusterId),
		}

		nodes, err := ExecListNodes(&request)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var data [][]string
		for _, node := range nodes.Nodes {
			data = append(data, []string{node.Name})
		}

		err = utils.PrintTable([]string{"Name"}, data)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}
	},
}

type ListNodesRequest struct {
	OrganizationID utils.Id `url:"organization"`
	ClusterID      utils.Id `url:"cluster"`
}
type NodeResponse struct {
	Name string
}
type ListNodeResponse struct {
	Nodes []NodeResponse
}

func ExecListNodes(req *ListNodesRequest) (*ListNodeResponse, error) {
	command, err := query.Values(req)
	if err != nil {
		return nil, err
	}

	wsURL, err := url.Parse(fmt.Sprintf("%s/cluster/nodes", utils.WebsocketUrl()))
	if err != nil {
		return nil, err
	}
	pattern := regexp.MustCompile("%5B([0-9]+)%5D=")
	wsURL.RawQuery = pattern.ReplaceAllString(command.Encode(), "[${1}]=")

	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	headers := http.Header{"Authorization": {utils.GetAuthorizationHeaderValue(tokenType, token)}}
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL.String(), headers)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = wsConn.Close()
	}()

	msgType, payload, err := wsConn.ReadMessage()
	if err != nil {
		return nil, err
	}

	switch msgType {
	case websocket.TextMessage:
		var data ListNodeResponse
		err = json.Unmarshal(payload, &data)
		if err != nil {
			return nil, err
		}
		return &data, nil
	default:
		return nil, errors.New("received invalid message while listing pods: " + string(rune(msgType)) + " " + string(payload))
	}
}

func init() {
	clusterCmd.AddCommand(clusterListNodesCmd)
	clusterListNodesCmd.Flags().StringVarP(&clusterId, "cluster-id", "c", "", "Cluster ID")
}

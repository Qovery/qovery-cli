package pkg

import (
	"fmt"

	"github.com/qovery/qovery-cli/utils"
)

type ShellRequest struct {
	ApplicationID  utils.Id
	EnvironmentID  utils.Id
	ProjectID      utils.Id
	OrganizationID utils.Id
}

func ExecShell(req *ShellRequest) {
	wsURL := fmt.Sprintf("wss://ws.qovery.com/shell/exec?organization=%s&environment=%s&project=%s&organization=%s&cluster=%s",
		req.ApplicationID,
		req.EnvironmentID,
		req.ProjectID,
		req.OrganizationID,
		"clusterId",
	)

	fmt.Println(wsURL)
}

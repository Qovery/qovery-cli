package pkg

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/qovery/qovery-cli/utils"
)

func DeleteProjectById(projectId string, dryRunDisabled bool) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("delete") {
		res := delete(utils.AdminUrl+"/project/"+projectId, http.MethodDelete, dryRunDisabled)

		if !dryRunDisabled {
			fmt.Println("Project with id " + projectId + " deletable.")
		} else if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not delete project with id %s : %s. %s", projectId, res.Status, string(result))
		} else {
			fmt.Println("Project with id " + projectId + " deleted.")
		}
	}
}

package pkg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/qovery/qovery-cli/utils"
)

func DeleteClusterById(clusterId string, dryRunDisabled bool) {
	utils.CheckAdminUrl()

	utils.DryRunPrint(dryRunDisabled)
	if utils.Validate("delete") {
		res := delete(utils.AdminUrl+"/cluster/"+clusterId, http.MethodDelete, dryRunDisabled)

		if !dryRunDisabled {
			fmt.Println("Cluster with id " + clusterId + " deletable.")
		} else if !strings.Contains(res.Status, "200") {
			result, _ := ioutil.ReadAll(res.Body)
			log.Errorf("Could not delete cluster with id %s : %s. %s", clusterId, res.Status, string(result))
		} else {
			fmt.Println("Cluster with id " + clusterId + " deleted.")
		}
	}
}

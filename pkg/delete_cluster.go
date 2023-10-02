package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
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
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not delete cluster with id %s : %s. %s", clusterId, res.Status, string(result))
		} else {
			fmt.Println("Cluster with id " + clusterId + " deleted.")
		}
	}
}
func DeleteClusterUnDeployedInError() {
	utils.CheckAdminUrl()

	if utils.Validate("delete") {
		res := delete(utils.AdminUrl+"/cluster/deleteNotDeployedInErrorClusters", http.MethodPost, true)

		if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not delete all clusters undeployed and in error : %s. %s", res.Status, string(result))
		} else {
			result, _ := io.ReadAll(res.Body)
			fmt.Println("Clusters deleted: " + string(result))
		}
	}
}

func DeleteOldClustersWithInvalidCredentials(ageInDay int, dryRunDisabled bool) {
	utils.CheckAdminUrl()

	if utils.Validate("delete") {

		params := map[string]interface{}{
			"last_update_in_days": ageInDay,
			"dry_run":            !dryRunDisabled,
		}

		requestBody, err := json.Marshal(params)
		if err != nil {
			log.Errorf("Could not create body for the request")
			return
		}

		res := deleteWithBody(utils.AdminUrl+"/cluster/deleteOldClustersWithInvalidCredentials", http.MethodPost, true, bytes.NewBuffer(requestBody))

		if !strings.Contains(res.Status, "200") {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not delete all clusters with invalid credentials : %s. %s", res.Status, string(result))
		} else {
			result, _ := io.ReadAll(res.Body)
			if dryRunDisabled {
				fmt.Println("Clusters deleted: " + string(result))
			} else {
				fmt.Println("Clusters that will be deleted: " + string(result))
			}
		}
	}
}



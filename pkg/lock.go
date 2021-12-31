package pkg

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
)

func LockById(clusterId string) {
	utils.CheckAdminUrl()

	if utils.Validate("lock") {
		res := updateLockById(clusterId, http.MethodPost)

		if res.StatusCode != http.StatusOK {
			result, _ := ioutil.ReadAll(res.Body)
			log.Errorf("Could not unlock cluster : %s. %s", res.Status, string(result))
		} else {
			fmt.Println("Cluster locked.")
		}
	}
}

func UnockById(clusterId string) {
	utils.CheckAdminUrl()

	if utils.Validate("unlock") {
		res := updateLockById(clusterId, http.MethodDelete)

		if res.StatusCode != http.StatusOK {
			result, _ := ioutil.ReadAll(res.Body)
			log.Errorf("Could not unlock cluster : %s. %s", res.Status, string(result))
		} else {
			fmt.Println("Cluster unlocked.")
		}
	}
}

func updateLockById(clusterId string, method string) *http.Response {
	authToken, tokenErr := utils.GetAccessToken()
	if tokenErr != nil {
		utils.PrintlnError(tokenErr)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/cluster/lock/%s", os.Getenv("ADMIN_URL"), clusterId)
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(authToken)))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	return res
}

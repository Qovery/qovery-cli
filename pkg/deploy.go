package pkg

import (
	"bytes"
	"fmt"
	"github.com/fatih/color"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
)

func DeployById(clusterId string, dryRunDisabled bool){
	checkAdminUrl()

	dryRunPrint(dryRunDisabled)
	if utils.Validate("deployment") {
		res := deploy(os.Getenv("ADMIN_URL") + "/admin/cluster/deploy/" + clusterId, http.MethodPost, dryRunDisabled )

		if !strings.Contains(res.Status, "200") {
			result, _ := ioutil.ReadAll(res.Body)
			log.Errorf("Could not deploy cluster : %s. %s", res.Status, string(result) )
		} else {
			fmt.Println("Cluster " + clusterId + " deploying.")
		}
	}
}

func DeployAll(dryRunDisabled bool) {
	checkAdminUrl()

	dryRunPrint(dryRunDisabled)
	if utils.Validate("deployment") {
		res := deploy(os.Getenv("ADMIN_URL") + "/admin/cluster/deploy", http.MethodPost, dryRunDisabled )

		if !strings.Contains(res.Status, "200") {
			result, _ := ioutil.ReadAll(res.Body)
			log.Errorf("Could not deploy clusters : %s. %s", res.Status, string(result) )
		} else {
			fmt.Println("Clusters deploying.")
		}
	}
}

func deploy(url string, method string, dryRunDisabled bool) *http.Response {
	authToken, tokenErr := utils.GetAccessToken()
	if tokenErr != nil {
		utils.PrintlnError(tokenErr)
		os.Exit(0)
	}

	var body *bytes.Buffer

	if !dryRunDisabled {
		body = bytes.NewBuffer([]byte( `{"metadata": {"dry_run_deploy": true}}`))
	}

	req, err  := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", "Bearer " + strings.TrimSpace(string(authToken)))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	return res
}

func checkAdminUrl() {
	if _, ok := os.LookupEnv("ADMIN_URL"); !ok {
		log.Error("You must set the Qovery admin root url (ADMIN_URL).")
		os.Exit(1)
	}
}

func dryRunPrint(dryRunDisbled bool) {
	green := color.New(color.FgGreen).SprintFunc()

	message := green("enabled")

	if dryRunDisbled {
		red := color.New(color.FgRed).SprintFunc()
		message = red("disabled")
	}

	log.Infof("Dry run: %s", message)
}

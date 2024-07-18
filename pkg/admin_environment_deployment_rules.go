package pkg

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"

	"github.com/qovery/qovery-cli/utils"
)

func PublishEnvironmentDeploymentRules() error {
	utils.CheckAdminUrl()

	utils.Println("Publishing environment deployment rules to scheduler...")
	err := callPublishEnvironmentDeploymentRulesApi()
	if err != nil {
		return err
	}
	utils.Println("Environment deployment rules successfully published to scheduler.")
	return nil
}

func callPublishEnvironmentDeploymentRulesApi() error {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/environmentDeploymentRules/pushToScheduler", utils.AdminUrl)
	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if res.StatusCode != 200 {
		log.Fatal(fmt.Sprintf("Failed to publish environment deployment rules to scheduler. Status code: %s", res.Status))
	}
	if err != nil {
		log.Fatal(err)
	}
	return err
}

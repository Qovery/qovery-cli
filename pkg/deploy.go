package pkg

import (
	"bytes"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func execAdminRequest(url string, method string, dryRunDisabled bool, queryParams map[string]string) *http.Response {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	body := bytes.NewBuffer([]byte(`{ "metadata": { "dry_run_deploy": true } }`))

	if dryRunDisabled {
		body = bytes.NewBuffer([]byte(`{ "metadata": { "dry_run_deploy": false } }`))
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		log.Fatal(err)
	}

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")
	query := req.URL.Query()
	for key, value := range queryParams {
		query.Add(key, value)
	}
	req.URL.RawQuery = query.Encode()

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	return res
}

func ForceFailedDeploymentsToInternalErrorStatus(safeguardDuration time.Duration) {
	if !utils.Validate("force deployment status") {
		return
	}
	nbMinutes := int(safeguardDuration.Minutes())
	if nbMinutes < 5 {
		log.Errorf("Could not force the deployments if safeguard is lower than 5minutes. Got %d", nbMinutes)
	}

	durationIso8601 := fmt.Sprintf("PT%dM", nbMinutes)
	queryParams := map[string]string{"safeguardDuration": durationIso8601}
	res := execAdminRequest(utils.AdminUrl+"/deployment/forceFailedDeploymentsToInternalErrorStatus", http.MethodPost, true, queryParams)
	if !strings.Contains(res.Status, "200") {
		result, _ := io.ReadAll(res.Body)
		log.Errorf("Could not force the deployments status : %s. %s", res.Status, string(result))
	} else {
		fmt.Println("INTERNAL_ERROR status forced")
	}
}

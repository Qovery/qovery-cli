package pkg

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/qovery/qovery-cli/utils"
)

func NotifyUsersClusterFailure(clusterId *string) error {
	var body string
	if clusterId != nil {
		body = fmt.Sprintf(`{"cluster_ids": ["%s"]}`, *clusterId)
	} else {
		body = `{"all_failing_clusters": true}`
	}

	notifiedClustersResponse, err := postWithBody(utils.AdminUrl+"/cluster/notifyFailedClustersAdmins", body)
	if err != nil {
		return err
	}
	result, _ := io.ReadAll(notifiedClustersResponse.Body)
	if !strings.Contains(notifiedClustersResponse.Status, "200") {
		return fmt.Errorf("could not notify (error %s: %s)", notifiedClustersResponse.Status, string(result))
	}

	utils.Println(fmt.Sprintf("Notification sent for admins of these clusters %s", string(result)))
	if err != nil {
		return err
	}
	return nil
}

func postWithBody(url string, bodyAsString string) (*http.Response, error) {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	body := bytes.NewBuffer([]byte(bodyAsString))

	req, err := http.NewRequest(http.MethodPost, url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	return http.DefaultClient.Do(req)
}

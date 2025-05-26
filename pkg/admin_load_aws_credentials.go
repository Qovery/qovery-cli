package pkg

import (
	"fmt"
	"io"
	"net/http"

	"github.com/qovery/qovery-cli/utils"
)

func LoadAwsCredentials(roleArn string) error {
	awsCredentials, err := fetchAwsCredentials(roleArn)
	println(awsCredentials)
	if err != nil {
		return err
	}
	return nil
}

func fetchAwsCredentials(roleArn string) (string, error) {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest(http.MethodPost, utils.GetAdminUrl()+"/aws/credentials/assume-role?role_arn="+roleArn, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if res.StatusCode != 200 {
		return "", fmt.Errorf("cannot fetch aws credentials (status_code=%d)", res.StatusCode)
	}

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	return string(bodyBytes), nil
}

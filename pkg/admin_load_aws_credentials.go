package pkg

import (
	"fmt"
	"github.com/go-jose/go-jose/v4/json"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/qovery/qovery-cli/utils"
)

type AwsStsCredentials struct {
	AccessKeyId     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token"`
}

func LoadAwsCredentials(roleArn string) error {
	awsStsCredentialsBody, err := fetchAwsCredentials(roleArn)
	utils.CheckError(err)

	awsStsCredentials := AwsStsCredentials{}
	err = json.Unmarshal(awsStsCredentialsBody, &awsStsCredentials)
	utils.CheckError(err)

	// Set the environment variables for child processes
	os.Setenv("AWS_ACCESS_KEY_ID", awsStsCredentials.AccessKeyId)
	os.Setenv("AWS_SECRET_ACCESS_KEY", awsStsCredentials.SecretAccessKey)
	os.Setenv("AWS_SESSION_TOKEN", awsStsCredentials.SessionToken)
	utils.PrintlnInfo("AWS credentials loaded successfully in current environment for child process. (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY, AWS_SESSION_TOKEN)")

	// Get the user's default shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash" // Default to bash if SHELL is not set
	}
	// Launch the shell
	utils.PrintlnInfo("Launching new shell with AWS credentials...")
	cmd := exec.Command(shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("error launching shell: %v", err)
	}
	return nil
}

func fetchAwsCredentials(roleArn string) ([]byte, error) {
	tokenType, token, err := utils.GetAccessToken()
	utils.CheckError(err)

	req, err := http.NewRequest(http.MethodPost, utils.GetAdminUrl()+"/aws/credentials/assume-role?role_arn="+roleArn, nil)
	utils.CheckError(err)
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	utils.CheckError(err)
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cannot fetch aws credentials (status_code=%d) %s", res.StatusCode, body)
	}
	utils.CheckError(err)
	return body, nil
}

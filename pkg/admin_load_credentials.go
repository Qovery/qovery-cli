package pkg

import (
	"bytes"
	"fmt"
	"github.com/go-jose/go-jose/v4/json"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"os/exec"

	"github.com/qovery/qovery-cli/utils"
)

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

	return StartChildShell()
}

func LoadCredentials(clusterId string, doNotConnectToBastion bool) error {
	if !doNotConnectToBastion {
		SetBastionConnection()
	}
	clusterCredentials := getClusterCredentials(clusterId)
	if len(clusterCredentials) == 0 {
		return fmt.Errorf("no credentials found for cluster ID %s", clusterId)
	}
	// Set the environment variables for child processes
	for _, cred := range clusterCredentials {
		os.Setenv(cred.Key, cred.Value)
		utils.PrintlnInfo(fmt.Sprintf("Set environment variable %s for child process", cred.Key))
	}
	kubeconfig := GetKubeconfigByClusterId(clusterId)
	filePath := utils.WriteInFile(clusterId, "kubeconfig", []byte(kubeconfig))
	os.Setenv("KUBECONFIG", filePath)
	return StartChildShell()
}

func StartChildShell() error {
	// Get the user's default shell
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash" // Default to bash if SHELL is not set
	}
	// Launch the shell
	utils.PrintlnInfo("Launching new shell with credentials...")
	cmd := exec.Command(shell)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("error launching shell: %v", err)
	}
	return nil
}

type AwsStsCredentials struct {
	AccessKeyId     string `json:"access_key_id"`
	SecretAccessKey string `json:"secret_access_key"`
	SessionToken    string `json:"session_token"`
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

func getClusterCredentials(clusterId string) []utils.Var {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/cluster/%s/credential", utils.GetAdminUrl(), clusterId)
	req, err := http.NewRequest(http.MethodGet, url, bytes.NewBuffer([]byte("{}")))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != http.StatusOK {
		err := fmt.Errorf("error retrieving cluster credentials: %s %s", res.Status, body)
		utils.PrintlnError(err)
		log.Fatal(err)
	}

	payload := map[string]string{}
	err = json.Unmarshal(body, &payload)
	if err != nil {
		log.Fatal(err)
	}

	var clusterCreds []utils.Var
	for key, value := range payload {
		switch key {
		case "access_key_id":
			clusterCreds = append(clusterCreds, utils.Var{Key: "AWS_ACCESS_KEY_ID", Value: value})
		case "secret_access_key":
			clusterCreds = append(clusterCreds, utils.Var{Key: "AWS_SECRET_ACCESS_KEY", Value: value})
		case "aws_session_token":
			clusterCreds = append(clusterCreds, utils.Var{Key: "AWS_SESSION_TOKEN", Value: value})
		case "region":
			clusterCreds = append(clusterCreds, utils.Var{Key: "AWS_DEFAULT_REGION", Value: value})
		case "scaleway_access_key":
			clusterCreds = append(clusterCreds, utils.Var{Key: "SCW_ACCESS_KEY", Value: value})
		case "scaleway_secret_key":
			clusterCreds = append(clusterCreds, utils.Var{Key: "SCW_SECRET_KEY", Value: value})
		case "scaleway_project_id":
			clusterCreds = append(clusterCreds, utils.Var{Key: "SCW_PROJECT_ID", Value: value})
		case "scaleway_organization_id":
			clusterCreds = append(clusterCreds, utils.Var{Key: "SCW_ORGANIZATION_ID", Value: value})
		case "json_credentials":
			filepath := utils.WriteInFile(clusterId, "google_creds.json", []byte(value))

			clusterCreds = append(clusterCreds, utils.Var{Key: "CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", Value: filepath})
			clusterCreds = append(clusterCreds, utils.Var{Key: "GOOGLE_CREDENTIALS", Value: value})
		}
	}
	return clusterCreds
}

package pkg

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/go-jose/go-jose/v4/json"
	log "github.com/sirupsen/logrus"

	"github.com/qovery/qovery-cli/utils"
)

func LoadAwsCredentials(roleArn string) error {
	awsStsCredentialsBody, err := fetchAwsCredentials(roleArn)
	utils.CheckError(err)

	awsStsCredentials := AwsStsCredentials{}
	err = json.Unmarshal(awsStsCredentialsBody, &awsStsCredentials)
	utils.CheckError(err)

	// Set the environment variables for child processes
	if err := os.Setenv("AWS_ACCESS_KEY_ID", awsStsCredentials.AccessKeyId); err != nil {
		return fmt.Errorf("failed to set AWS_ACCESS_KEY_ID: %w", err)
	}
	if err := os.Setenv("AWS_SECRET_ACCESS_KEY", awsStsCredentials.SecretAccessKey); err != nil {
		return fmt.Errorf("failed to set AWS_SECRET_ACCESS_KEY: %w", err)
	}
	if err := os.Setenv("AWS_SESSION_TOKEN", awsStsCredentials.SessionToken); err != nil {
		return fmt.Errorf("failed to set AWS_SESSION_TOKEN: %w", err)
	}
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
		if err := os.Setenv(cred.Key, cred.Value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", cred.Key, err)
		}
		utils.PrintlnInfo(fmt.Sprintf("Set environment variable %s for child process", cred.Key))
	}
	kubeconfig := GetKubeconfigByClusterId(clusterId, false)
	filePath := utils.WriteInFile(clusterId, "kubeconfig", []byte(kubeconfig))
	if err := os.Setenv("KUBECONFIG", filePath); err != nil {
		return fmt.Errorf("failed to set KUBECONFIG: %w", err)
	}
	if kubeconfigRequiresQoveryCommand(kubeconfig) {
		if _, err := exec.LookPath("qovery"); err != nil {
			utils.PrintlnInfo(fmt.Sprintf("KUBECONFIG uses qovery as an exec credential command, but qovery was not found in PATH: %v", err))
		}
	}
	return StartChildShell()
}

func kubeconfigRequiresQoveryCommand(kubeconfig string) bool {
	return strings.Contains(kubeconfig, "command: qovery")
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

	return clusterCredentialsFromPayload(clusterId, payload)
}

func clusterCredentialsFromPayload(clusterId string, payload map[string]string) []utils.Var {
	var clusterCreds []utils.Var
	isGcpPayload := isGcpCredentialsPayload(payload)
	for key, value := range payload {
		switch key {
		case "access_key_id":
			clusterCreds = append(clusterCreds, utils.Var{Key: "AWS_ACCESS_KEY_ID", Value: value})
		case "secret_access_key":
			clusterCreds = append(clusterCreds, utils.Var{Key: "AWS_SECRET_ACCESS_KEY", Value: value})
		case "aws_session_token":
			clusterCreds = append(clusterCreds, utils.Var{Key: "AWS_SESSION_TOKEN", Value: value})
		case "region":
			if isGcpPayload {
				if _, hasGcpRegion := payload["gcp_region"]; !hasGcpRegion {
					clusterCreds = appendGcpRegionVars(clusterCreds, value)
				}
			} else {
				clusterCreds = append(clusterCreds, utils.Var{Key: "AWS_DEFAULT_REGION", Value: value})
			}
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
		case "gcp_access_token":
			filepath := utils.WriteInFile(clusterId, "google_access_token", []byte(value))

			clusterCreds = append(clusterCreds, utils.Var{Key: "GOOGLE_OAUTH_ACCESS_TOKEN", Value: value})
			clusterCreds = append(clusterCreds, utils.Var{Key: "CLOUDSDK_AUTH_ACCESS_TOKEN_FILE", Value: filepath})
		case "gcp_project_id":
			clusterCreds = appendGcpProjectVars(clusterCreds, value)
		case "gcp_region":
			clusterCreds = appendGcpRegionVars(clusterCreds, value)
		case "gcp_access_token_expiration":
			clusterCreds = append(clusterCreds, utils.Var{Key: "GOOGLE_OAUTH_ACCESS_TOKEN_EXPIRATION", Value: value})
		case "gcp_credentials_type":
			clusterCreds = append(clusterCreds, utils.Var{Key: "GCP_CREDENTIALS_TYPE", Value: value})
		}
	}
	return clusterCreds
}

func appendGcpProjectVars(clusterCreds []utils.Var, projectId string) []utils.Var {
	clusterCreds = append(clusterCreds, utils.Var{Key: "GOOGLE_PROJECT", Value: projectId})
	clusterCreds = append(clusterCreds, utils.Var{Key: "GOOGLE_CLOUD_PROJECT", Value: projectId})
	clusterCreds = append(clusterCreds, utils.Var{Key: "CLOUDSDK_CORE_PROJECT", Value: projectId})
	return clusterCreds
}

func appendGcpRegionVars(clusterCreds []utils.Var, region string) []utils.Var {
	clusterCreds = append(clusterCreds, utils.Var{Key: "GOOGLE_REGION", Value: region})
	clusterCreds = append(clusterCreds, utils.Var{Key: "CLOUDSDK_COMPUTE_REGION", Value: region})
	return clusterCreds
}

func isGcpCredentialsPayload(payload map[string]string) bool {
	gcpKeys := []string{
		"json_credentials",
		"gcp_access_token",
		"gcp_project_id",
		"gcp_region",
		"gcp_access_token_expiration",
		"gcp_credentials_type",
	}
	for _, key := range gcpKeys {
		if _, ok := payload[key]; ok {
			return true
		}
	}
	return false
}

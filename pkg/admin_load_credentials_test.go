package pkg

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/qovery/qovery-cli/utils"
)

func TestClusterCredentialsFromPayload(t *testing.T) {
	t.Run("Should map legacy GCP json credentials", func(t *testing.T) {
		// given
		clusterId := "test-legacy-gcp"
		payload := map[string]string{
			"json_credentials": "base64-json",
		}

		// when
		credentials := clusterCredentialsFromPayload(clusterId, payload)

		// then
		assert.Contains(t, credentials, utils.Var{Key: "GOOGLE_CREDENTIALS", Value: "base64-json"})
		assert.Contains(t, credentials, utils.Var{Key: "CLOUDSDK_AUTH_CREDENTIAL_FILE_OVERRIDE", Value: "/tmp/qovery_test-legacy-gcp/google_creds.json"})
	})

	t.Run("Should map GCP workload identity federation access token credentials", func(t *testing.T) {
		// given
		payload := map[string]string{
			"gcp_access_token":            "access-token",
			"gcp_project_id":              "project-id",
			"gcp_region":                  "europe-west1",
			"gcp_access_token_expiration": "2026-06-08T17:00:00Z",
			"gcp_credentials_type":        "WORKLOAD_IDENTITY_FEDERATION",
		}

		// when
		credentials := clusterCredentialsFromPayload("test-wif-gcp", payload)

		// then
		assert.Contains(t, credentials, utils.Var{Key: "GOOGLE_OAUTH_ACCESS_TOKEN", Value: "access-token"})
		assert.Contains(t, credentials, utils.Var{Key: "CLOUDSDK_AUTH_ACCESS_TOKEN_FILE", Value: "/tmp/qovery_test-wif-gcp/google_access_token"})
		assert.Contains(t, credentials, utils.Var{Key: "GOOGLE_PROJECT", Value: "project-id"})
		assert.Contains(t, credentials, utils.Var{Key: "GOOGLE_CLOUD_PROJECT", Value: "project-id"})
		assert.Contains(t, credentials, utils.Var{Key: "CLOUDSDK_CORE_PROJECT", Value: "project-id"})
		assert.Contains(t, credentials, utils.Var{Key: "GOOGLE_REGION", Value: "europe-west1"})
		assert.Contains(t, credentials, utils.Var{Key: "CLOUDSDK_COMPUTE_REGION", Value: "europe-west1"})
		assert.Contains(t, credentials, utils.Var{Key: "GOOGLE_OAUTH_ACCESS_TOKEN_EXPIRATION", Value: "2026-06-08T17:00:00Z"})
		assert.Contains(t, credentials, utils.Var{Key: "GCP_CREDENTIALS_TYPE", Value: "WORKLOAD_IDENTITY_FEDERATION"})
	})

	t.Run("Should not map generic region to AWS_DEFAULT_REGION for GCP credentials", func(t *testing.T) {
		// given
		payload := map[string]string{
			"gcp_access_token":     "access-token",
			"gcp_project_id":       "project-id",
			"gcp_credentials_type": "WORKLOAD_IDENTITY_FEDERATION",
			"region":               "europe-west9",
		}

		// when
		credentials := clusterCredentialsFromPayload("test-wif-gcp-region", payload)

		// then
		assert.Contains(t, credentials, utils.Var{Key: "GOOGLE_REGION", Value: "europe-west9"})
		assert.Contains(t, credentials, utils.Var{Key: "CLOUDSDK_COMPUTE_REGION", Value: "europe-west9"})
		assert.NotContains(t, credentials, utils.Var{Key: "AWS_DEFAULT_REGION", Value: "europe-west9"})
	})

	t.Run("Should keep mapping generic region to AWS_DEFAULT_REGION for AWS credentials", func(t *testing.T) {
		// given
		payload := map[string]string{
			"access_key_id":     "access-key",
			"secret_access_key": "secret-key",
			"region":            "eu-west-3",
		}

		// when
		credentials := clusterCredentialsFromPayload("test-aws", payload)

		// then
		assert.Contains(t, credentials, utils.Var{Key: "AWS_DEFAULT_REGION", Value: "eu-west-3"})
	})
}

func TestAssumeRoleUrl(t *testing.T) {
	roleArn := "arn:aws:iam::123456789012:role/qovery-user-role"
	organizationId := "11111111-1111-1111-1111-111111111111"

	t.Run("Should send the role ARN and the organization id", func(t *testing.T) {
		// when
		assumeRole, err := url.Parse(assumeRoleUrl("https://admin.qovery.com/admin", roleArn, organizationId, ""))

		// then
		assert.NoError(t, err)
		assert.Equal(t, "/admin/aws/credentials/assume-role", assumeRole.Path)
		assert.Equal(t, url.Values{"role_arn": {roleArn}, "organization_id": {organizationId}}, assumeRole.Query())
	})

	t.Run("Should also send the cluster id when given", func(t *testing.T) {
		// given
		clusterId := "22222222-2222-2222-2222-222222222222"

		// when
		assumeRole, err := url.Parse(assumeRoleUrl("https://admin.qovery.com/admin", roleArn, organizationId, clusterId))

		// then
		assert.NoError(t, err)
		assert.Equal(t, url.Values{"role_arn": {roleArn}, "organization_id": {organizationId}, "cluster_id": {clusterId}}, assumeRole.Query())
	})
}

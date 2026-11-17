package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testProfileOrganizationID = "11111111-1111-1111-1111-111111111111"
	testProfileClusterID      = "22222222-2222-2222-2222-222222222222"
	testProfileBaseURL        = "https://api.test"
)

func testProfileCluster(t *testing.T, observabilityProfile string, kedaEnabled bool) map[string]any {
	t.Helper()
	cluster := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(`{
		"id": "`+testProfileClusterID+`",
		"name": "dev-staging",
		"disk_size": 40,
		"metrics_parameters": {
			"enabled": true,
			"configuration": {
				"kind": "MANAGED_BY_QOVERY",
				"resource_profile": "`+observabilityProfile+`",
				"high_availability": true,
				"alerting": {"enabled": true}
			}
		},
		"keda": {"enabled": false, "resource_profile": "NORMAL", "availability_profile": "NORMAL"}
	}`), &cluster))
	cluster["keda"].(map[string]any)["enabled"] = kedaEnabled
	return cluster
}

func testProfileAPI() *clusterProfileAPI {
	return &clusterProfileAPI{baseURL: testProfileBaseURL, authorization: "Bearer fake", httpClient: &http.Client{}}
}

func profilePointer[T any](value T) *T {
	return &value
}

func TestParseProfile(t *testing.T) {
	profile, err := parseProfile("keda-resource-profile", "low", clusterResourceProfiles)
	require.NoError(t, err)
	assert.Equal(t, clusterResourceProfileLow, *profile)

	_, err = parseProfile("keda-availability-profile", "LOW", kedaAvailabilityProfiles)
	assert.EqualError(t, err, `invalid --keda-availability-profile "LOW": must be one of NORMAL, HIGH`)
}

func TestReadClusterProfiles(t *testing.T) {
	profiles := readClusterProfiles(testProfileCluster(t, "LOW", true))

	assert.Equal(t, [][]string{
		{"observability.enabled", "true"},
		{"observability.resource_profile", "LOW"},
		{"observability.high_availability", "true"},
		{"keda.enabled", "true"},
		{"keda.resource_profile", "NORMAL"},
		{"keda.availability_profile", "NORMAL"},
	}, profiles.rows())
}

func TestReadClusterProfilesWithoutAddons(t *testing.T) {
	profiles := readClusterProfiles(map[string]any{"metrics_parameters": map[string]any{"enabled": false}})

	assert.False(t, profiles.observabilityEnabled)
	assert.False(t, profiles.kedaEnabled)
	assert.Nil(t, profiles.observabilityHighAvailability)
}

func TestApplyClusterProfileChangesOnlyTouchesRequestedFields(t *testing.T) {
	cluster := testProfileCluster(t, "LOW", true)

	err := applyClusterProfileChanges(cluster, clusterProfileChanges{
		observabilityResourceProfile:  profilePointer(clusterResourceProfileNormal),
		observabilityHighAvailability: profilePointer(false),
		kedaAvailabilityProfile:       profilePointer(kedaAvailabilityProfileHigh),
	})

	require.NoError(t, err)
	expected := testProfileCluster(t, "NORMAL", true)
	expected["metrics_parameters"].(map[string]any)["configuration"].(map[string]any)["high_availability"] = false
	expected["keda"].(map[string]any)["availability_profile"] = "HIGH"
	assert.Equal(t, expected, cluster)
}

func TestApplyClusterProfileChangesRefusesDisabledAddons(t *testing.T) {
	cluster := testProfileCluster(t, "LOW", false)
	err := applyClusterProfileChanges(cluster, clusterProfileChanges{kedaResourceProfile: profilePointer(clusterResourceProfileHigh)})
	assert.ErrorContains(t, err, "KEDA is not enabled")
	assert.Equal(t, testProfileCluster(t, "LOW", false), cluster)

	cluster = map[string]any{"metrics_parameters": map[string]any{"enabled": false}}
	err = applyClusterProfileChanges(cluster, clusterProfileChanges{observabilityResourceProfile: profilePointer(clusterResourceProfileHigh)})
	assert.ErrorContains(t, err, "observability is not enabled")
}

func TestClusterProfileAPIFindsCluster(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET", testProfileBaseURL+"/organization/"+testProfileOrganizationID+"/cluster",
		func(req *http.Request) (*http.Response, error) {
			assert.Equal(t, "Bearer fake", req.Header.Get("Authorization"))
			return httpmock.NewJsonResponse(200, map[string]any{"results": []any{
				map[string]any{"id": "other"},
				testProfileCluster(t, "LOW", false),
			}})
		})

	cluster, err := testProfileAPI().cluster(testProfileOrganizationID, testProfileClusterID)
	require.NoError(t, err)
	assert.Equal(t, "dev-staging", cluster["name"])
	assert.Equal(t, json.Number("40"), cluster["disk_size"])

	_, err = testProfileAPI().cluster(testProfileOrganizationID, "missing")
	assert.ErrorContains(t, err, "cluster missing not found")
}

func TestClusterProfileAPIReportsHTTPErrors(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	httpmock.RegisterResponder("GET", testProfileBaseURL+"/organization/"+testProfileOrganizationID+"/cluster",
		httpmock.NewStringResponder(403, `{"message":"forbidden"}`))

	_, err := testProfileAPI().cluster(testProfileOrganizationID, testProfileClusterID)
	assert.ErrorContains(t, err, "403")
	assert.ErrorContains(t, err, "forbidden")
}

func TestUpdateClusterProfilesSendsTheWholeClusterAndChecksTheResult(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	clusterURL := testProfileBaseURL + "/organization/" + testProfileOrganizationID + "/cluster"

	stored := testProfileCluster(t, "LOW", false)
	var sent map[string]any
	httpmock.RegisterResponder("PUT", clusterURL+"/"+testProfileClusterID,
		func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			require.NoError(t, json.Unmarshal(body, &sent))
			assert.Equal(t, "application/json", req.Header.Get("Content-Type"))
			stored = sent
			return httpmock.NewJsonResponse(200, sent)
		})
	httpmock.RegisterResponder("GET", clusterURL,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]any{"results": []any{stored}})
		})

	err := updateClusterProfiles(testProfileAPI(), testProfileOrganizationID, testProfileClusterID,
		readClusterProfiles(stored),
		clusterProfileChanges{observabilityResourceProfile: profilePointer(clusterResourceProfileNormal)})

	require.NoError(t, err)
	assert.Equal(t, testProfileCluster(t, "NORMAL", false), sent)
}

func TestUpdateClusterProfilesStartsFromTheCurrentCluster(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	clusterURL := testProfileBaseURL + "/organization/" + testProfileOrganizationID + "/cluster"

	stored := testProfileCluster(t, "LOW", false)
	stored["name"] = "renamed-while-the-prompt-was-open"
	var sent map[string]any
	httpmock.RegisterResponder("PUT", clusterURL+"/"+testProfileClusterID,
		func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			require.NoError(t, json.Unmarshal(body, &sent))
			stored = sent
			return httpmock.NewJsonResponse(200, sent)
		})
	httpmock.RegisterResponder("GET", clusterURL,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]any{"results": []any{stored}})
		})

	err := updateClusterProfiles(testProfileAPI(), testProfileOrganizationID, testProfileClusterID,
		readClusterProfiles(stored),
		clusterProfileChanges{observabilityResourceProfile: profilePointer(clusterResourceProfileNormal)})

	require.NoError(t, err)
	assert.Equal(t, "renamed-while-the-prompt-was-open", sent["name"])
}

func TestUpdateClusterProfilesIgnoresProfilesItDidNotChange(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	clusterURL := testProfileBaseURL + "/organization/" + testProfileOrganizationID + "/cluster"

	stored := testProfileCluster(t, "LOW", false)
	stored["keda"] = nil
	var sent map[string]any
	httpmock.RegisterResponder("PUT", clusterURL+"/"+testProfileClusterID,
		func(req *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(req.Body)
			require.NoError(t, json.Unmarshal(body, &sent))
			stored = testProfileCluster(t, "NORMAL", false)
			return httpmock.NewJsonResponse(200, stored)
		})
	httpmock.RegisterResponder("GET", clusterURL,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]any{"results": []any{stored}})
		})

	err := updateClusterProfiles(testProfileAPI(), testProfileOrganizationID, testProfileClusterID,
		readClusterProfiles(stored),
		clusterProfileChanges{observabilityResourceProfile: profilePointer(clusterResourceProfileNormal)})

	assert.NoError(t, err)
	assert.Contains(t, sent, "keda")
	assert.Nil(t, sent["keda"])
}

func TestUpdateClusterProfilesRefusesProfilesChangedSinceTheConfirmation(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	clusterURL := testProfileBaseURL + "/organization/" + testProfileOrganizationID + "/cluster"

	httpmock.RegisterResponder("PUT", clusterURL+"/"+testProfileClusterID, httpmock.NewStringResponder(200, `{}`))
	httpmock.RegisterResponder("GET", clusterURL,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]any{"results": []any{testProfileCluster(t, "HIGH", false)}})
		})

	err := updateClusterProfiles(testProfileAPI(), testProfileOrganizationID, testProfileClusterID,
		readClusterProfiles(testProfileCluster(t, "LOW", false)),
		clusterProfileChanges{observabilityResourceProfile: profilePointer(clusterResourceProfileNormal)})

	assert.ErrorContains(t, err, "changed since the confirmation")
	assert.Zero(t, httpmock.GetCallCountInfo()["PUT "+clusterURL+"/"+testProfileClusterID])
}

func TestUpdateClusterProfilesFailsWhenTheAPIDoesNotSave(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()
	clusterURL := testProfileBaseURL + "/organization/" + testProfileOrganizationID + "/cluster"

	httpmock.RegisterResponder("PUT", clusterURL+"/"+testProfileClusterID, httpmock.NewStringResponder(200, `{}`))
	httpmock.RegisterResponder("GET", clusterURL,
		func(req *http.Request) (*http.Response, error) {
			return httpmock.NewJsonResponse(200, map[string]any{"results": []any{testProfileCluster(t, "LOW", false)}})
		})

	err := updateClusterProfiles(testProfileAPI(), testProfileOrganizationID, testProfileClusterID,
		readClusterProfiles(testProfileCluster(t, "LOW", false)),
		clusterProfileChanges{observabilityResourceProfile: profilePointer(clusterResourceProfileNormal)})

	assert.EqualError(t, err, "the API accepted the update but did not save the new profiles")
}

func TestClusterProfileChangeRows(t *testing.T) {
	before := readClusterProfiles(testProfileCluster(t, "LOW", false))
	after := readClusterProfiles(testProfileCluster(t, "NORMAL", false))

	rows := clusterProfileChangeRows(before, after)

	assert.Equal(t, []string{"observability.resource_profile", "LOW", "NORMAL"}, rows[1])
	assert.Equal(t, []string{"keda.enabled", "false", "false"}, rows[3])
}

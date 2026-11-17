package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

type clusterResourceProfile string

const (
	clusterResourceProfileLow    clusterResourceProfile = "LOW"
	clusterResourceProfileNormal clusterResourceProfile = "NORMAL"
	clusterResourceProfileHigh   clusterResourceProfile = "HIGH"
)

type kedaAvailabilityProfile string

const (
	kedaAvailabilityProfileNormal kedaAvailabilityProfile = "NORMAL"
	kedaAvailabilityProfileHigh   kedaAvailabilityProfile = "HIGH"
)

var (
	clusterResourceProfiles  = []clusterResourceProfile{clusterResourceProfileLow, clusterResourceProfileNormal, clusterResourceProfileHigh}
	kedaAvailabilityProfiles = []kedaAvailabilityProfile{kedaAvailabilityProfileNormal, kedaAvailabilityProfileHigh}
)

func parseProfile[T ~string](flag string, value string, allowed []T) (*T, error) {
	profile := T(strings.ToUpper(value))
	if !slices.Contains(allowed, profile) {
		return nil, fmt.Errorf("invalid --%s %q: must be one of %s", flag, value, joinProfiles(allowed))
	}
	return &profile, nil
}

func joinProfiles[T ~string](profiles []T) string {
	values := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		values = append(values, string(profile))
	}
	return strings.Join(values, ", ")
}

type clusterProfileChanges struct {
	observabilityResourceProfile  *clusterResourceProfile
	observabilityHighAvailability *bool
	kedaResourceProfile           *clusterResourceProfile
	kedaAvailabilityProfile       *kedaAvailabilityProfile
}

func (c clusterProfileChanges) isEmpty() bool {
	return c == clusterProfileChanges{}
}

type clusterProfiles struct {
	observabilityEnabled          bool
	observabilityResourceProfile  string
	observabilityHighAvailability *bool
	kedaEnabled                   bool
	kedaResourceProfile           string
	kedaAvailabilityProfile       string
}

func (p clusterProfiles) rows() [][]string {
	highAvailability := ""
	if p.observabilityHighAvailability != nil {
		highAvailability = strconv.FormatBool(*p.observabilityHighAvailability)
	}
	return [][]string{
		{"observability.enabled", strconv.FormatBool(p.observabilityEnabled)},
		{"observability.resource_profile", p.observabilityResourceProfile},
		{"observability.high_availability", highAvailability},
		{"keda.enabled", strconv.FormatBool(p.kedaEnabled)},
		{"keda.resource_profile", p.kedaResourceProfile},
		{"keda.availability_profile", p.kedaAvailabilityProfile},
	}
}

func jsonObject(parent map[string]any, key string) map[string]any {
	object, _ := parent[key].(map[string]any)
	return object
}

func readClusterProfiles(cluster map[string]any) clusterProfiles {
	var profiles clusterProfiles

	metrics := jsonObject(cluster, "metrics_parameters")
	profiles.observabilityEnabled, _ = metrics["enabled"].(bool)
	configuration := jsonObject(metrics, "configuration")
	profiles.observabilityResourceProfile, _ = configuration["resource_profile"].(string)
	if highAvailability, ok := configuration["high_availability"].(bool); ok {
		profiles.observabilityHighAvailability = &highAvailability
	}

	keda := jsonObject(cluster, "keda")
	profiles.kedaEnabled, _ = keda["enabled"].(bool)
	profiles.kedaResourceProfile, _ = keda["resource_profile"].(string)
	profiles.kedaAvailabilityProfile, _ = keda["availability_profile"].(string)

	return profiles
}

func applyClusterProfileChanges(cluster map[string]any, changes clusterProfileChanges) error {
	current := readClusterProfiles(cluster)

	if changes.observabilityResourceProfile != nil || changes.observabilityHighAvailability != nil {
		configuration := jsonObject(jsonObject(cluster, "metrics_parameters"), "configuration")
		if !current.observabilityEnabled || configuration == nil {
			return errors.New("observability is not enabled on this cluster: its profile cannot be changed")
		}
		if changes.observabilityResourceProfile != nil {
			configuration["resource_profile"] = string(*changes.observabilityResourceProfile)
		}
		if changes.observabilityHighAvailability != nil {
			configuration["high_availability"] = *changes.observabilityHighAvailability
		}
	}

	if changes.kedaResourceProfile != nil || changes.kedaAvailabilityProfile != nil {
		if !current.kedaEnabled {
			return errors.New("KEDA is not enabled on this cluster: its profile cannot be changed")
		}
		keda := jsonObject(cluster, "keda")
		if changes.kedaResourceProfile != nil {
			keda["resource_profile"] = string(*changes.kedaResourceProfile)
		}
		if changes.kedaAvailabilityProfile != nil {
			keda["availability_profile"] = string(*changes.kedaAvailabilityProfile)
		}
	}

	return nil
}

type clusterProfileAPI struct {
	baseURL       string
	authorization string
	httpClient    *http.Client
}

func newClusterProfileAPI() (*clusterProfileAPI, error) {
	tokenType, token, err := utils.GetAccessToken(false)
	if err != nil {
		return nil, err
	}
	return &clusterProfileAPI{
		baseURL:       utils.GetAPIBaseURL(),
		authorization: utils.GetAuthorizationHeaderValue(tokenType, token),
		httpClient:    &http.Client{Timeout: 60 * time.Second},
	}, nil
}

func (a *clusterProfileAPI) do(method string, path string, body any) ([]byte, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, a.baseURL+path, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", a.authorization)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(content)))
	}
	return content, nil
}

func (a *clusterProfileAPI) cluster(organizationID string, clusterID string) (map[string]any, error) {
	content, err := a.do(http.MethodGet, "/organization/"+organizationID+"/cluster", nil)
	if err != nil {
		return nil, err
	}
	var list struct {
		Results []map[string]any `json:"results"`
	}
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.UseNumber()
	if err := decoder.Decode(&list); err != nil {
		return nil, err
	}
	for _, cluster := range list.Results {
		if cluster["id"] == clusterID {
			return cluster, nil
		}
	}
	return nil, fmt.Errorf("cluster %s not found in organization %s", clusterID, organizationID)
}

func (a *clusterProfileAPI) editCluster(organizationID string, clusterID string, cluster map[string]any) error {
	_, err := a.do(http.MethodPut, "/organization/"+organizationID+"/cluster/"+clusterID, cluster)
	return err
}

func (a *clusterProfileAPI) deployCluster(organizationID string, clusterID string) error {
	_, err := a.do(http.MethodPost, "/organization/"+organizationID+"/cluster/"+clusterID+"/deploy", nil)
	return err
}

func (c clusterProfileChanges) savedIn(profiles clusterProfiles) bool {
	if c.observabilityResourceProfile != nil && string(*c.observabilityResourceProfile) != profiles.observabilityResourceProfile {
		return false
	}
	if c.observabilityHighAvailability != nil &&
		(profiles.observabilityHighAvailability == nil || *c.observabilityHighAvailability != *profiles.observabilityHighAvailability) {
		return false
	}
	if c.kedaResourceProfile != nil && string(*c.kedaResourceProfile) != profiles.kedaResourceProfile {
		return false
	}
	if c.kedaAvailabilityProfile != nil && string(*c.kedaAvailabilityProfile) != profiles.kedaAvailabilityProfile {
		return false
	}
	return true
}

func updateClusterProfiles(api *clusterProfileAPI, organizationID string, clusterID string, changes clusterProfileChanges) error {
	cluster, err := api.cluster(organizationID, clusterID)
	if err != nil {
		return err
	}
	if err := applyClusterProfileChanges(cluster, changes); err != nil {
		return err
	}
	if err := api.editCluster(organizationID, clusterID, cluster); err != nil {
		return err
	}
	saved, err := api.cluster(organizationID, clusterID)
	if err != nil {
		return err
	}
	if !changes.savedIn(readClusterProfiles(saved)) {
		return errors.New("the API accepted the update but did not save the new profiles")
	}
	return nil
}

func clusterProfileChangeRows(before clusterProfiles, after clusterProfiles) [][]string {
	rows := before.rows()
	for i, row := range after.rows() {
		rows[i] = append(rows[i], row[1])
	}
	return rows
}

var (
	adminClusterProfileOrganizationID             string
	adminClusterProfileClusterID                  string
	adminClusterProfileObservabilityResource      string
	adminClusterProfileObservabilityHighAvailable bool
	adminClusterProfileKedaResource               string
	adminClusterProfileKedaAvailability           string
	adminClusterProfileDeploy                     bool
	adminClusterProfileNoConfirm                  bool
)

const (
	adminClusterProfileObservabilityResourceFlag   = "observability-resource-profile"
	adminClusterProfileObservabilityHighAvailFlag  = "observability-high-availability"
	adminClusterProfileKedaResourceFlag            = "keda-resource-profile"
	adminClusterProfileKedaAvailabilityProfileFlag = "keda-availability-profile"
)

var adminClusterProfileCmd = &cobra.Command{
	Use:   "profile",
	Short: "Read or change the observability and KEDA profiles of a cluster",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		_ = cmd.Help()
		os.Exit(0)
	},
}

var adminClusterProfileGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the observability and KEDA profiles of a cluster",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		api, err := newClusterProfileAPI()
		exitOnClusterProfileError(err)
		cluster, err := api.cluster(adminClusterProfileOrganizationID, adminClusterProfileClusterID)
		exitOnClusterProfileError(err)
		exitOnClusterProfileError(utils.PrintTable([]string{"Setting", "Value"}, readClusterProfiles(cluster).rows()))
	},
}

var adminClusterProfileSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Change the observability and KEDA profiles of a cluster",
	Long: `Change the observability and KEDA profiles of a cluster.

Only the profiles given as flags change. The profile of an add-on that is not enabled cannot be changed.
The new profiles apply at the next cluster deployment: use --deploy to trigger it.

Examples:
qovery admin cluster profile set --organization-id <org-id> --cluster-id <cluster-id> --observability-resource-profile NORMAL
qovery admin cluster profile set --organization-id <org-id> --cluster-id <cluster-id> --observability-resource-profile NORMAL --observability-high-availability=false --deploy
qovery admin cluster profile set --organization-id <org-id> --cluster-id <cluster-id> --keda-resource-profile HIGH --keda-availability-profile HIGH`,
	Args: cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		changes, err := clusterProfileChangesFromFlags(cmd)
		exitOnClusterProfileError(err)

		api, err := newClusterProfileAPI()
		exitOnClusterProfileError(err)
		cluster, err := api.cluster(adminClusterProfileOrganizationID, adminClusterProfileClusterID)
		exitOnClusterProfileError(err)

		before := readClusterProfiles(cluster)
		exitOnClusterProfileError(applyClusterProfileChanges(cluster, changes))
		exitOnClusterProfileError(utils.PrintTable(
			[]string{"Setting", "Current", "New"},
			clusterProfileChangeRows(before, readClusterProfiles(cluster)),
		))

		if !adminClusterProfileNoConfirm && !utils.Validate("profile update") {
			utils.PrintlnInfo("profile update cancelled")
			return
		}

		exitOnClusterProfileError(updateClusterProfiles(api, adminClusterProfileOrganizationID, adminClusterProfileClusterID, changes))
		utils.PrintlnInfo("profiles updated")

		if !adminClusterProfileDeploy {
			utils.PrintlnInfo("the new profiles apply at the next cluster deployment")
			return
		}
		exitOnClusterProfileError(api.deployCluster(adminClusterProfileOrganizationID, adminClusterProfileClusterID))
		utils.PrintlnInfo("cluster deployment triggered")
	},
}

func clusterProfileChangesFromFlags(cmd *cobra.Command) (clusterProfileChanges, error) {
	var changes clusterProfileChanges
	var err error
	flags := cmd.Flags()

	if flags.Changed(adminClusterProfileObservabilityResourceFlag) {
		changes.observabilityResourceProfile, err = parseProfile(adminClusterProfileObservabilityResourceFlag, adminClusterProfileObservabilityResource, clusterResourceProfiles)
		if err != nil {
			return changes, err
		}
	}
	if flags.Changed(adminClusterProfileObservabilityHighAvailFlag) {
		highAvailability := adminClusterProfileObservabilityHighAvailable
		changes.observabilityHighAvailability = &highAvailability
	}
	if flags.Changed(adminClusterProfileKedaResourceFlag) {
		changes.kedaResourceProfile, err = parseProfile(adminClusterProfileKedaResourceFlag, adminClusterProfileKedaResource, clusterResourceProfiles)
		if err != nil {
			return changes, err
		}
	}
	if flags.Changed(adminClusterProfileKedaAvailabilityProfileFlag) {
		changes.kedaAvailabilityProfile, err = parseProfile(adminClusterProfileKedaAvailabilityProfileFlag, adminClusterProfileKedaAvailability, kedaAvailabilityProfiles)
		if err != nil {
			return changes, err
		}
	}
	if changes.isEmpty() {
		return changes, errors.New("nothing to change: set at least one profile flag")
	}
	return changes, nil
}

func exitOnClusterProfileError(err error) {
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
}

func init() {
	for _, cmd := range []*cobra.Command{adminClusterProfileGetCmd, adminClusterProfileSetCmd} {
		cmd.Flags().StringVar(&adminClusterProfileOrganizationID, "organization-id", "", "The organization of the cluster")
		cmd.Flags().StringVar(&adminClusterProfileClusterID, "cluster-id", "", "The cluster to target")
		_ = cmd.MarkFlagRequired("organization-id")
		_ = cmd.MarkFlagRequired("cluster-id")
		adminClusterProfileCmd.AddCommand(cmd)
	}

	flags := adminClusterProfileSetCmd.Flags()
	flags.StringVar(&adminClusterProfileObservabilityResource, adminClusterProfileObservabilityResourceFlag, "", "Observability resource profile: "+joinProfiles(clusterResourceProfiles))
	flags.BoolVar(&adminClusterProfileObservabilityHighAvailable, adminClusterProfileObservabilityHighAvailFlag, false, "Run the observability stack in high availability")
	flags.StringVar(&adminClusterProfileKedaResource, adminClusterProfileKedaResourceFlag, "", "KEDA resource profile: "+joinProfiles(clusterResourceProfiles))
	flags.StringVar(&adminClusterProfileKedaAvailability, adminClusterProfileKedaAvailabilityProfileFlag, "", "KEDA availability profile: "+joinProfiles(kedaAvailabilityProfiles))
	flags.BoolVar(&adminClusterProfileDeploy, "deploy", false, "Deploy the cluster after the update")
	flags.BoolVarP(&adminClusterProfileNoConfirm, "no-confirm", "c", false, "Do not ask for confirmation")

	adminClusterCmd.AddCommand(adminClusterProfileCmd)
}

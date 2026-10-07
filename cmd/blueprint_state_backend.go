package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

const (
	stateBackendQovery = "qovery"
	stateBackendS3     = "s3"
)

var blueprintName string
var blueprintId string
var stateBackend string
var stateBackendDeploy bool
var stateBackendYes bool

type stateBackendResponse struct {
	Backend               string `json:"backend"`
	StateMigrationPending bool   `json:"state_migration_pending"`
}

type stateBackendErrorResponse struct {
	Detail string `json:"detail"`
}

var blueprintStateBackendCmd = &cobra.Command{
	Use:   "state-backend",
	Short: "Manage the Terraform state backend of a blueprint",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		_ = cmd.Help()
	},
}

var blueprintStateBackendGetCmd = &cobra.Command{
	Use:   "get",
	Short: "Show the Terraform state backend of a blueprint",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		id := resolveBlueprintIdPanicInCaseOfError()
		backend, err := callStateBackendAPI(http.MethodGet, id, "")
		utils.CheckError(err)

		utils.Println(fmt.Sprintf("Backend: %s", pterm.FgBlue.Sprint(backend.Backend)))
		utils.Println(fmt.Sprintf("State migration pending: %t", backend.StateMigrationPending))
	},
}

var blueprintStateBackendSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Switch the Terraform state backend of a blueprint",
	Long: `Switch the Terraform state backend of a blueprint between Qovery (Kubernetes secret)
and a Qovery-managed S3 bucket of the cluster (AWS clusters only).

The next deployment of the blueprint migrates the Terraform state from the
current backend to the new one. Use --deploy to trigger that deployment now.`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if err := validateStateBackend(stateBackend); err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}

		id := resolveBlueprintIdPanicInCaseOfError()
		current, err := callStateBackendAPI(http.MethodGet, id, "")
		utils.CheckError(err)

		if current.Backend == stateBackend {
			utils.Println(stateBackendUnchangedMessage(*current))
		} else {
			if !stateBackendYes {
				utils.PrintlnInfo(stateBackendSwitchWarning(*current, stateBackend))
				if !utils.Validate("the state backend switch") {
					os.Exit(0)
				}
			}

			current, err = callStateBackendAPI(http.MethodPut, id, stateBackend)
			utils.CheckError(err)
			utils.Println(fmt.Sprintf("Blueprint state backend set to %s (migration pending: %t)", pterm.FgBlue.Sprint(current.Backend), current.StateMigrationPending))
		}

		if !stateBackendDeploy {
			if current.StateMigrationPending {
				utils.Println("Deploy the blueprint to migrate the state, or rerun with --deploy")
			}
			return
		}

		client := utils.GetQoveryClientPanicInCaseOfError()
		ack, _, err := client.BlueprintMainCallsAPI.DeployBlueprint(context.Background(), id).Execute()
		if err != nil {
			utils.PrintlnError(stateBackendDeployFailure(*current, err))
			os.Exit(1)
		}
		utils.Println(fmt.Sprintf("Blueprint deployment %s has been queued..", pterm.FgBlue.Sprint(ack.DeploymentId)))
	},
}

func validateStateBackend(backend string) error {
	switch backend {
	case stateBackendQovery, stateBackendS3:
		return nil
	case "":
		return fmt.Errorf("--backend is required (%s|%s)", stateBackendS3, stateBackendQovery)
	default:
		return fmt.Errorf("invalid --backend %q: must be %s or %s", backend, stateBackendS3, stateBackendQovery)
	}
}

func otherStateBackend(backend string) string {
	if backend == stateBackendS3 {
		return stateBackendQovery
	}
	return stateBackendS3
}

// While a migration is pending, the state still lives in the backend opposite the configured one.
func stateBackendLocation(current stateBackendResponse) string {
	if current.StateMigrationPending {
		return otherStateBackend(current.Backend)
	}
	return current.Backend
}

func stateBackendSwitchWarning(current stateBackendResponse, target string) string {
	location := stateBackendLocation(current)
	if target == location {
		return fmt.Sprintf("This switch cancels the pending migration; the Terraform state stays in %s.", location)
	}
	return fmt.Sprintf("The next deployment migrates the Terraform state from %s to %s.", location, target)
}

func stateBackendUnchangedMessage(current stateBackendResponse) string {
	if current.StateMigrationPending {
		return fmt.Sprintf("Blueprint already uses the %s state backend; the state migration from %s is pending", current.Backend, stateBackendLocation(current))
	}
	return fmt.Sprintf("Blueprint already uses the %s state backend", current.Backend)
}

func stateBackendDeployFailure(current stateBackendResponse, err error) error {
	if current.StateMigrationPending {
		return fmt.Errorf("state backend is switched to %s (migration pending) but the deployment could not be triggered: %w; deploy the blueprint to migrate the state", current.Backend, err)
	}
	return fmt.Errorf("state backend is %s but the deployment could not be triggered: %w", current.Backend, err)
}

func buildStateBackendRequest(method string, blueprintId string, backend string) (string, io.Reader, error) {
	path := fmt.Sprintf("blueprint/%s/stateBackend", url.PathEscape(blueprintId))
	if method != http.MethodPut {
		return path, nil, nil
	}

	payload, err := json.Marshal(map[string]string{"backend": backend})
	if err != nil {
		return "", nil, err
	}
	return path, bytes.NewReader(payload), nil
}

func parseStateBackendResponse(resp *http.Response) (*stateBackendResponse, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var apiErr stateBackendErrorResponse
		if json.Unmarshal(body, &apiErr) == nil && apiErr.Detail != "" {
			return nil, fmt.Errorf("%s", apiErr.Detail)
		}
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, string(body))
	}

	var result stateBackendResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func callStateBackendAPI(method string, blueprintId string, backend string) (*stateBackendResponse, error) {
	path, body, err := buildStateBackendRequest(method, blueprintId, backend)
	if err != nil {
		return nil, err
	}

	req, err := utils.NewAPIRequest(method, path, body, false)
	if err != nil {
		return nil, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := utils.DoAPIRequest(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	return parseStateBackendResponse(resp)
}

func resolveBlueprintIdPanicInCaseOfError() string {
	if blueprintId != "" {
		return blueprintId
	}
	if blueprintName == "" {
		utils.PrintlnError(fmt.Errorf("--blueprint (-n) or --id is required"))
		os.Exit(1)
	}

	client := utils.GetQoveryClientPanicInCaseOfError()
	envId := getEnvironmentIdFromContextPanicInCaseOfError(client)
	id, err := findBlueprintIdByName(client, envId, blueprintName)
	utils.CheckError(err)
	return id
}

type blueprintCandidate struct {
	blueprintId string
	serviceName string
}

// Blueprints are not listable per environment: each one owns a Terraform service carrying its blueprint_id.
func findBlueprintIdByName(client *qovery.APIClient, envId string, name string) (string, error) {
	terraforms, _, err := client.TerraformsAPI.ListTerraforms(context.Background(), envId).Execute()
	if err != nil {
		return "", err
	}

	var candidates []blueprintCandidate
	for _, terraform := range terraforms.GetResults() {
		if id := terraform.GetBlueprintId(); id != "" {
			candidates = append(candidates, blueprintCandidate{blueprintId: id, serviceName: terraform.Name})
		}
	}

	blueprintNameOf := func(id string) (string, bool, error) {
		blueprint, res, err := client.BlueprintMainCallsAPI.GetBlueprint(context.Background(), id).Execute()
		if res != nil && (res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusForbidden) {
			return "", false, nil
		}
		if err != nil {
			return "", false, err
		}
		return blueprint.Name, true, nil
	}

	return matchBlueprintId(candidates, name, blueprintNameOf)
}

// Service names match first, so blueprints are only fetched when no service name matches.
func matchBlueprintId(candidates []blueprintCandidate, name string, blueprintNameOf func(id string) (string, bool, error)) (string, error) {
	var matches []string
	for _, candidate := range candidates {
		if candidate.serviceName == name {
			matches = append(matches, candidate.blueprintId)
		}
	}

	if len(matches) == 0 {
		for _, candidate := range candidates {
			candidateName, found, err := blueprintNameOf(candidate.blueprintId)
			if err != nil {
				return "", err
			}
			if found && candidateName == name {
				matches = append(matches, candidate.blueprintId)
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", fmt.Errorf("blueprint %s not found in the environment", name)
	case 1:
		return matches[0], nil
	default:
		return "", fmt.Errorf("several blueprints match %s, use --id instead", name)
	}
}

func addBlueprintTargetFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	cmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	cmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	cmd.Flags().StringVarP(&blueprintName, "blueprint", "n", "", "Blueprint Name")
	cmd.Flags().StringVarP(&blueprintId, "id", "", "", "Blueprint ID (skips name resolution)")
	cmd.MarkFlagsMutuallyExclusive("blueprint", "id")
}

func init() {
	blueprintCmd.AddCommand(blueprintStateBackendCmd)
	blueprintStateBackendCmd.AddCommand(blueprintStateBackendGetCmd)
	blueprintStateBackendCmd.AddCommand(blueprintStateBackendSetCmd)

	addBlueprintTargetFlags(blueprintStateBackendGetCmd)
	addBlueprintTargetFlags(blueprintStateBackendSetCmd)
	blueprintStateBackendSetCmd.Flags().StringVarP(&stateBackend, "backend", "", "", "Terraform state backend <s3|qovery>")
	blueprintStateBackendSetCmd.Flags().BoolVarP(&stateBackendDeploy, "deploy", "", false, "Deploy the blueprint right away to migrate the state")
	blueprintStateBackendSetCmd.Flags().BoolVarP(&stateBackendYes, "yes", "y", false, "Skip the confirmation prompt")
}

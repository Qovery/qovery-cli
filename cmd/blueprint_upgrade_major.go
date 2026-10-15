package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var blueprintUpgradeMajor string
var blueprintUpgradeSkipConfirmation bool

var blueprintUpgradeMajorCmd = &cobra.Command{
	Use:   "upgrade-major",
	Short: "Point a blueprint at the latest tag of a newer major version",
	Long: `Point a blueprint service at the latest tag of a newer major version (e.g. PostgreSQL 14 to 18).

Without --major, lists the newer majors and what moving to each one does:
  RELABEL      the service already runs that major (e.g. upgraded on AWS): only Qovery's records change
  UPGRADE      the service runs an older major: the next deploy upgrades it in place (downtime for databases)
  UNSUPPORTED  the move is refused, with the reason`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		service, err := findBlueprintService(client, envId, blueprintServiceName)
		checkError(err)

		availability, _, err := client.BlueprintMainCallsAPI.CheckBlueprintUpdate(context.Background(), service.blueprintId).Execute()
		checkError(apiError(err))
		currentTag := availability.GetCurrentTag()
		majors := availability.GetNewMajorVersions()

		if blueprintUpgradeMajor == "" {
			printNewMajors(currentTag, majors)
			return
		}

		target, err := pickMajor(majors, blueprintUpgradeMajor)
		checkError(err)
		if target.GetMode() == qovery.BLUEPRINTMAJORSWITCHMODE_UNSUPPORTED {
			utils.PrintlnError(fmt.Errorf("cannot move %s to major %s: %s", blueprintServiceName, target.GetServiceVersion(), target.GetUnsupportedReason()))
			os.Exit(1)
		}

		utils.Println(confirmationText(target.GetMode(), majorOfTag(currentTag), target.GetServiceVersion()))
		if !blueprintUpgradeSkipConfirmation && !confirmed("Do you want to proceed? [Yes/No]") {
			utils.Println("Upgrade aborted")
			os.Exit(1)
		}

		blueprint, _, err := client.BlueprintMainCallsAPI.GetBlueprint(context.Background(), service.blueprintId).Execute()
		checkError(apiError(err))

		request := buildUpdateRequest(blueprint.GetName(), target.GetLatestTag(), service.iconUri)
		_, _, err = client.BlueprintMainCallsAPI.UpdateBlueprint(context.Background(), service.blueprintId).BlueprintUpdateRequest(request).Execute()
		checkError(apiError(err))

		watch := utils.NewServicesWatch(client, envId, []string{service.serviceId}, watchFlag)
		ack, _, err := client.BlueprintMainCallsAPI.DeployBlueprint(context.Background(), service.blueprintId).Execute()
		checkError(apiError(err))

		utils.Println(fmt.Sprintf("Request to point %s at %s has been queued (deployment %s)",
			pterm.FgBlue.Sprint(blueprintServiceName), pterm.FgBlue.Sprint(target.GetLatestTag()), ack.GetDeploymentId()))
		watch.Wait(qovery.STATEENUM_DEPLOYED)
	},
}

// A blueprint materializes as a Terraform or a Helm service; both carry the blueprint they come from.
type blueprintService struct {
	serviceId   string
	name        string
	blueprintId string
	iconUri     string
}

func findBlueprintService(client *qovery.APIClient, envId string, name string) (blueprintService, error) {
	terraforms, _, err := client.TerraformsAPI.ListTerraforms(context.Background(), envId).Execute()
	if err != nil {
		return blueprintService{}, err
	}
	helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()
	if err != nil {
		return blueprintService{}, err
	}

	var services []blueprintService
	for _, terraform := range terraforms.GetResults() {
		services = append(services, blueprintService{terraform.Id, terraform.Name, terraform.GetBlueprintId(), terraform.GetIconUri()})
	}
	for _, helm := range helms.GetResults() {
		services = append(services, blueprintService{helm.Id, helm.Name, helm.GetBlueprintId(), helm.GetIconUri()})
	}
	return pickBlueprintService(services, name)
}

func pickBlueprintService(services []blueprintService, name string) (blueprintService, error) {
	for _, service := range services {
		if service.name != name {
			continue
		}
		if service.blueprintId == "" {
			return blueprintService{}, fmt.Errorf("%s is not a blueprint service", name)
		}
		return service, nil
	}
	return blueprintService{}, fmt.Errorf("no blueprint service named %s in this environment", name)
}

func printNewMajors(currentTag string, majors []qovery.BlueprintUpdateNewMajorVersion) {
	if len(majors) == 0 {
		utils.Println(fmt.Sprintf("%s is already on the latest major (%s)", blueprintServiceName, currentTag))
		return
	}
	utils.Println(fmt.Sprintf("%s runs %s. Newer majors:", blueprintServiceName, currentTag))
	data := make([][]string, 0, len(majors))
	for _, major := range majors {
		data = append(data, []string{major.GetServiceVersion(), major.GetLatestTag(), string(major.GetMode()), major.GetUnsupportedReason()})
	}
	checkError(utils.PrintTable([]string{"Major", "Latest tag", "Mode", "Reason"}, data))
	utils.Println("Run again with --major <major> to move to one of them.")
}

func pickMajor(majors []qovery.BlueprintUpdateNewMajorVersion, major string) (qovery.BlueprintUpdateNewMajorVersion, error) {
	for _, candidate := range majors {
		if candidate.GetServiceVersion() == major {
			return candidate, nil
		}
	}
	available := make([]string, 0, len(majors))
	for _, candidate := range majors {
		available = append(available, candidate.GetServiceVersion())
	}
	if len(available) == 0 {
		return qovery.BlueprintUpdateNewMajorVersion{}, fmt.Errorf("major %s is not available: the blueprint is already on the latest major", major)
	}
	return qovery.BlueprintUpdateNewMajorVersion{}, fmt.Errorf("major %s is not available, choose one of: %s", major, strings.Join(available, ", "))
}

func confirmationText(mode qovery.BlueprintMajorSwitchMode, current string, target string) string {
	switch mode {
	case qovery.BLUEPRINTMAJORSWITCHMODE_RELABEL:
		return fmt.Sprintf("The service already runs %s: only Qovery's records change, the service is not touched.", target)
	case qovery.BLUEPRINTMAJORSWITCHMODE_UPGRADE:
		return fmt.Sprintf("This upgrades the service from %s to %s in place on the next deploy (downtime for databases). Review the preview in the Console first if needed.", current, target)
	default:
		return fmt.Sprintf("Moving the service from %s to %s is not supported.", current, target)
	}
}

// Name and icon are required by the API; variables and spec overrides are left out so they stay unchanged.
func buildUpdateRequest(name string, tag string, icon string) qovery.BlueprintUpdateRequest {
	return *qovery.NewBlueprintUpdateRequest(name, tag, icon)
}

// Catalog tags look like AWS/postgres/14/4.6.0; the third segment is the major.
func majorOfTag(tag string) string {
	segments := strings.Split(tag, "/")
	if len(segments) < 3 {
		return tag
	}
	return segments[2]
}

func confirmed(label string) bool {
	prompt := promptui.Select{Label: label, Items: []string{"Yes", "No"}}
	_, result, err := prompt.Run()
	checkError(err)
	return strings.EqualFold(strings.TrimSpace(result), "yes")
}

// Surfaces the API's own message (e.g. why a tag change is refused) instead of the bare status line.
func apiError(err error) error {
	var openAPIError *qovery.GenericOpenAPIError
	if errors.As(err, &openAPIError) && len(openAPIError.Body()) > 0 {
		return fmt.Errorf("%s: %s", openAPIError.Error(), string(openAPIError.Body()))
	}
	return err
}

func init() {
	blueprintCmd.AddCommand(blueprintUpgradeMajorCmd)
	blueprintUpgradeMajorCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	blueprintUpgradeMajorCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	blueprintUpgradeMajorCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	blueprintUpgradeMajorCmd.Flags().StringVarP(&blueprintServiceName, "name", "n", "", "Blueprint service name")
	blueprintUpgradeMajorCmd.Flags().StringVarP(&blueprintUpgradeMajor, "major", "", "", "Major version to move to (e.g. 18). Without it, lists the available majors")
	blueprintUpgradeMajorCmd.Flags().BoolVarP(&blueprintUpgradeSkipConfirmation, "yes", "y", false, "Skip the confirmation prompt")
	blueprintUpgradeMajorCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch the service status until it's ready or an error occurs")
	_ = blueprintUpgradeMajorCmd.MarkFlagRequired("name")
}

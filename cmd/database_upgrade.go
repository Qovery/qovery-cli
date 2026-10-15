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

var databaseUpgradeMajor string
var databaseUpgradeSkipConfirmation bool

var databaseUpgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Move a blueprint database to a newer major version",
	Long: `Move a blueprint database to a newer major version (e.g. PostgreSQL 14 to 18).

Without --major, lists the newer majors and what moving to each one does:
  RELABEL      the database already runs that major (e.g. upgraded on AWS): only Qovery's records change
  UPGRADE      the database is upgraded in place on the next deploy, with downtime
  UNSUPPORTED  the move is refused, with the reason`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		envId := getEnvironmentIdFromContextPanicInCaseOfError(client)

		database, err := findBlueprintDatabase(client, envId, databaseName)
		checkError(err)
		blueprintId := database.terraform.GetBlueprintId()

		availability, _, err := client.BlueprintMainCallsAPI.CheckBlueprintUpdate(context.Background(), blueprintId).Execute()
		checkError(apiError(err))
		currentTag := availability.GetCurrentTag()
		majors := availability.GetNewMajorVersions()

		if databaseUpgradeMajor == "" {
			printNewMajors(currentTag, majors)
			return
		}

		target, err := pickMajor(majors, databaseUpgradeMajor)
		checkError(err)
		if target.GetMode() == qovery.BLUEPRINTMAJORSWITCHMODE_UNSUPPORTED {
			utils.PrintlnError(fmt.Errorf("cannot move %s to major %s: %s", databaseName, target.GetServiceVersion(), target.GetUnsupportedReason()))
			os.Exit(1)
		}

		utils.Println(confirmationText(target.GetMode(), majorOfTag(currentTag), target.GetServiceVersion()))
		if !databaseUpgradeSkipConfirmation && !confirmed("Do you want to proceed? [Yes/No]") {
			utils.Println("Upgrade aborted")
			os.Exit(1)
		}

		blueprint, _, err := client.BlueprintMainCallsAPI.GetBlueprint(context.Background(), blueprintId).Execute()
		checkError(apiError(err))

		request := buildUpdateRequest(blueprint.GetName(), target.GetLatestTag(), database.terraform.GetIconUri())
		_, _, err = client.BlueprintMainCallsAPI.UpdateBlueprint(context.Background(), blueprintId).BlueprintUpdateRequest(request).Execute()
		checkError(apiError(err))

		watch := utils.NewServicesWatch(client, envId, []string{database.terraform.Id}, watchFlag)
		ack, _, err := client.BlueprintMainCallsAPI.DeployBlueprint(context.Background(), blueprintId).Execute()
		checkError(apiError(err))

		utils.Println(fmt.Sprintf("Request to move database %s to %s has been queued (deployment %s)",
			pterm.FgBlue.Sprint(databaseName), pterm.FgBlue.Sprint(target.GetLatestTag()), ack.GetDeploymentId()))
		watch.Wait(qovery.STATEENUM_DEPLOYED)
	},
}

func findBlueprintDatabase(client *qovery.APIClient, envId string, name string) (blueprintDatabase, error) {
	databases, err := listBlueprintDatabases(client, envId, false)
	if err != nil {
		return blueprintDatabase{}, err
	}
	for _, database := range databases {
		if database.terraform.Name == name {
			return database, nil
		}
	}
	return blueprintDatabase{}, fmt.Errorf("no blueprint database named %s in this environment", name)
}

func printNewMajors(currentTag string, majors []qovery.BlueprintUpdateNewMajorVersion) {
	if len(majors) == 0 {
		utils.Println(fmt.Sprintf("%s is already on the latest major (%s)", databaseName, currentTag))
		return
	}
	utils.Println(fmt.Sprintf("%s runs %s. Newer majors:", databaseName, currentTag))
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
		return qovery.BlueprintUpdateNewMajorVersion{}, fmt.Errorf("major %s is not available: the database is already on the latest major", major)
	}
	return qovery.BlueprintUpdateNewMajorVersion{}, fmt.Errorf("major %s is not available, choose one of: %s", major, strings.Join(available, ", "))
}

func confirmationText(mode qovery.BlueprintMajorSwitchMode, current string, target string) string {
	switch mode {
	case qovery.BLUEPRINTMAJORSWITCHMODE_RELABEL:
		return fmt.Sprintf("The database already runs %s: only Qovery's records change, the database is not touched.", target)
	case qovery.BLUEPRINTMAJORSWITCHMODE_UPGRADE:
		return fmt.Sprintf("This upgrades the database from %s to %s in place, with downtime. Review the preview in the Console first if needed.", current, target)
	default:
		return fmt.Sprintf("Moving the database from %s to %s is not supported.", current, target)
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
	databaseCmd.AddCommand(databaseUpgradeCmd)
	databaseUpgradeCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	databaseUpgradeCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	databaseUpgradeCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	databaseUpgradeCmd.Flags().StringVarP(&databaseName, "database", "n", "", "Database Name")
	databaseUpgradeCmd.Flags().StringVarP(&databaseUpgradeMajor, "major", "", "", "Major version to move to (e.g. 18). Without it, lists the available majors")
	databaseUpgradeCmd.Flags().BoolVarP(&databaseUpgradeSkipConfirmation, "yes", "y", false, "Skip the confirmation prompt")
	databaseUpgradeCmd.Flags().BoolVarP(&watchFlag, "watch", "w", false, "Watch the database status until it's ready or an error occurs")
	_ = databaseUpgradeCmd.MarkFlagRequired("database")
}

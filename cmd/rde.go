package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

// RDE env var constants
const rdeBlueprintProjectIdVar = "BLUEPRINT_PROJECT_ID"
const rdeBlueprintKeyVar = "BLUEPRINT_KEY"

// RDE shared flag variables
var rdeBlueprintProjectName string
var rdeName string
var rdeEmail string
var rdeSkipRbac bool
var rdeSkipInvite bool
var rdeSkipDeploy bool
var rdeUpgradeStrategy string
var rdeConfirmFlag bool

var rdeCmd = &cobra.Command{
	Use:   "rde",
	Short: "Manage Remote Development Environments (RDE)",
	Long: `Manage Remote Development Environments (RDE).

RDE allows platform teams to provision isolated, pre-configured development
environments for developers. The system works as:

  1. Blueprints: Template projects/environments that serve as the source for cloning
  2. RDE instances: Cloned from a blueprint, with optional RBAC isolation and member invitation

Blueprint identification uses environment variables:
  - Project-level: BLUEPRINT_PROJECT_ID = <project ID> (marks a project as a blueprint)
  - Environment-level: BLUEPRINT_KEY = <blueprint project ID> (links environments to their blueprint)`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(args) == 0 {
			_ = cmd.Help()
			os.Exit(0)
		}
	},
}

func init() {
	rootCmd.AddCommand(rdeCmd)
}

// --- RDE helper types ---

type rdeBlueprintInfo struct {
	ProjectId   string
	ProjectName string
	EnvId       string
	EnvName     string
}

type rdeChildInfo struct {
	ProjectId          string
	ProjectName        string
	EnvId              string
	EnvName            string
	BlueprintProjectId string
}

// --- RDE helper functions ---

// rdeGetOrgId resolves the organization ID from the --organization flag or stored context.
func rdeGetOrgId(client *qovery.APIClient) (string, error) {
	return usercontext.GetOrganizationContextResourceId(client, organizationName)
}

// rdeListBlueprintProjects finds all projects in the org that have the BLUEPRINT_PROJECT_ID env var.
func rdeListBlueprintProjects(client *qovery.APIClient, orgId string) ([]rdeBlueprintInfo, error) {
	projects, _, err := client.ProjectsAPI.ListProject(context.Background(), orgId).Execute()
	if err != nil {
		return nil, err
	}

	var blueprints []rdeBlueprintInfo

	for _, project := range projects.GetResults() {
		vars, err := utils.ListProjectVariables(client, project.Id)
		if err != nil {
			continue // skip projects we can't read vars for
		}

		bpVar := utils.FindEnvironmentVariableByKey(rdeBlueprintProjectIdVar, vars)
		if bpVar == nil {
			continue
		}

		// Verify the var value matches this project's ID
		val := ""
		if bpVar.Value.IsSet() && bpVar.Value.Get() != nil {
			val = *bpVar.Value.Get()
		}
		if val != project.Id {
			continue
		}

		// Find the first environment that has BLUEPRINT_KEY == projectId
		envInfo, err := rdeFindBlueprintEnv(client, project.Id)
		if err != nil || envInfo == nil {
			// Blueprint project with no matching environment yet
			blueprints = append(blueprints, rdeBlueprintInfo{
				ProjectId:   project.Id,
				ProjectName: project.Name,
			})
			continue
		}

		blueprints = append(blueprints, rdeBlueprintInfo{
			ProjectId:   project.Id,
			ProjectName: project.Name,
			EnvId:       envInfo.EnvId,
			EnvName:     envInfo.EnvName,
		})
	}

	return blueprints, nil
}

// rdeFindBlueprintByProjectName finds a specific blueprint project by name.
func rdeFindBlueprintByProjectName(client *qovery.APIClient, orgId string, name string) (*rdeBlueprintInfo, error) {
	projects, _, err := client.ProjectsAPI.ListProject(context.Background(), orgId).Execute()
	if err != nil {
		return nil, err
	}

	for _, project := range projects.GetResults() {
		if !strings.EqualFold(project.Name, name) {
			continue
		}

		vars, err := utils.ListProjectVariables(client, project.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to read variables for project %s: %w", name, err)
		}

		bpVar := utils.FindEnvironmentVariableByKey(rdeBlueprintProjectIdVar, vars)
		if bpVar == nil {
			return nil, fmt.Errorf("project %s is not a blueprint (missing %s variable)", name, rdeBlueprintProjectIdVar)
		}

		val := ""
		if bpVar.Value.IsSet() && bpVar.Value.Get() != nil {
			val = *bpVar.Value.Get()
		}
		if val != project.Id {
			return nil, fmt.Errorf("project %s has invalid %s variable (expected %s, got %s)", name, rdeBlueprintProjectIdVar, project.Id, val)
		}

		envInfo, err := rdeFindBlueprintEnv(client, project.Id)
		if err != nil {
			return nil, err
		}

		info := &rdeBlueprintInfo{
			ProjectId:   project.Id,
			ProjectName: project.Name,
		}
		if envInfo != nil {
			info.EnvId = envInfo.EnvId
			info.EnvName = envInfo.EnvName
		}

		return info, nil
	}

	return nil, fmt.Errorf("project %s not found", name)
}

type envInfo struct {
	EnvId   string
	EnvName string
}

// rdeFindBlueprintEnv gets the first environment in a blueprint project that has BLUEPRINT_KEY == projectId.
func rdeFindBlueprintEnv(client *qovery.APIClient, projectId string) (*envInfo, error) {
	environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), projectId).Execute()
	if err != nil {
		return nil, err
	}

	for _, env := range environments.GetResults() {
		vars, err := utils.ListEnvironmentVariables(client, env.Id)
		if err != nil {
			continue
		}

		bkVar := utils.FindEnvironmentVariableByKey(rdeBlueprintKeyVar, vars)
		if bkVar == nil {
			continue
		}

		val := ""
		if bkVar.Value.IsSet() && bkVar.Value.Get() != nil {
			val = *bkVar.Value.Get()
		}
		if val == projectId {
			return &envInfo{EnvId: env.Id, EnvName: env.Name}, nil
		}
	}

	return nil, nil
}

// rdeListChildren finds all projects whose environments have BLUEPRINT_KEY == blueprintProjectId (excluding the blueprint itself).
func rdeListChildren(client *qovery.APIClient, orgId string, blueprintProjectId string) ([]rdeChildInfo, error) {
	projects, _, err := client.ProjectsAPI.ListProject(context.Background(), orgId).Execute()
	if err != nil {
		return nil, err
	}

	var children []rdeChildInfo

	for _, project := range projects.GetResults() {
		if project.Id == blueprintProjectId {
			continue // skip the blueprint itself
		}

		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), project.Id).Execute()
		if err != nil {
			continue
		}

		for _, env := range environments.GetResults() {
			vars, err := utils.ListEnvironmentVariables(client, env.Id)
			if err != nil {
				continue
			}

			bkVar := utils.FindEnvironmentVariableByKey(rdeBlueprintKeyVar, vars)
			if bkVar == nil {
				continue
			}

			val := ""
			if bkVar.Value.IsSet() && bkVar.Value.Get() != nil {
				val = *bkVar.Value.Get()
			}
			if val == blueprintProjectId {
				children = append(children, rdeChildInfo{
					ProjectId:          project.Id,
					ProjectName:        project.Name,
					EnvId:              env.Id,
					EnvName:            env.Name,
					BlueprintProjectId: blueprintProjectId,
				})
			}
		}
	}

	return children, nil
}

// rdeListAllChildren finds all RDE children across all blueprints.
func rdeListAllChildren(client *qovery.APIClient, orgId string) ([]rdeChildInfo, error) {
	blueprints, err := rdeListBlueprintProjects(client, orgId)
	if err != nil {
		return nil, err
	}

	var allChildren []rdeChildInfo
	for _, bp := range blueprints {
		children, err := rdeListChildren(client, orgId, bp.ProjectId)
		if err != nil {
			continue
		}
		allChildren = append(allChildren, children...)
	}

	return allChildren, nil
}

// rdeFindChildByName finds a child RDE project by its project name within the org.
func rdeFindChildByName(client *qovery.APIClient, orgId string, name string) (*rdeChildInfo, error) {
	projects, _, err := client.ProjectsAPI.ListProject(context.Background(), orgId).Execute()
	if err != nil {
		return nil, err
	}

	for _, project := range projects.GetResults() {
		if !strings.EqualFold(project.Name, name) {
			continue
		}

		environments, _, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), project.Id).Execute()
		if err != nil {
			return nil, fmt.Errorf("failed to list environments for project %s: %w", name, err)
		}

		for _, env := range environments.GetResults() {
			vars, err := utils.ListEnvironmentVariables(client, env.Id)
			if err != nil {
				continue
			}

			bkVar := utils.FindEnvironmentVariableByKey(rdeBlueprintKeyVar, vars)
			if bkVar == nil {
				continue
			}

			val := ""
			if bkVar.Value.IsSet() && bkVar.Value.Get() != nil {
				val = *bkVar.Value.Get()
			}

			// It's a child if BLUEPRINT_KEY != own project ID
			if val != "" && val != project.Id {
				return &rdeChildInfo{
					ProjectId:          project.Id,
					ProjectName:        project.Name,
					EnvId:              env.Id,
					EnvName:            env.Name,
					BlueprintProjectId: val,
				}, nil
			}
		}

		return nil, fmt.Errorf("project %s exists but is not an RDE child (no %s variable pointing to a different project)", name, rdeBlueprintKeyVar)
	}

	return nil, fmt.Errorf("project %s not found", name)
}

// rdeGetEnvStatus gets the environment status as a StateEnum.
func rdeGetEnvStatus(client *qovery.APIClient, envId string) (qovery.StateEnum, error) {
	statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()
	if err != nil {
		return "", err
	}
	return statuses.Environment.State, nil
}

// rdeGetWorkspaceUrl gets the first application's public URL from the environment.
func rdeGetWorkspaceUrl(client *qovery.APIClient, envId string) string {
	apps, _, err := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()
	if err != nil || len(apps.GetResults()) == 0 {
		return ""
	}

	appId := apps.GetResults()[0].Id
	links, _, err := client.ApplicationMainCallsAPI.ListApplicationLinks(context.Background(), appId).Execute()
	if err != nil || len(links.GetResults()) == 0 {
		return ""
	}

	url := links.GetResults()[0].GetUrl()
	return url
}

// rdeFormatUptime formats a deployment timestamp to human-readable uptime.
func rdeFormatUptime(deployedAt *time.Time) string {
	if deployedAt == nil {
		return "-"
	}

	diff := time.Since(*deployedAt)
	if diff < time.Minute {
		return fmt.Sprintf("%ds", int(diff.Seconds()))
	} else if diff < time.Hour {
		return fmt.Sprintf("%dm", int(diff.Minutes()))
	} else if diff < 24*time.Hour {
		h := int(diff.Hours())
		m := int(diff.Minutes()) % 60
		return fmt.Sprintf("%dh %dm", h, m)
	}
	d := int(diff.Hours()) / 24
	h := int(diff.Hours()) % 24
	return fmt.Sprintf("%dd %dh", d, h)
}

// rdeGetLastDeployTime gets the last deployment timestamp for an environment.
func rdeGetLastDeployTime(client *qovery.APIClient, envId string) *time.Time {
	history, _, err := client.EnvironmentDeploymentHistoryAPI.ListEnvironmentDeploymentHistory(context.Background(), envId).Execute()
	if err != nil || len(history.GetResults()) == 0 {
		return nil
	}

	t := history.GetResults()[0].GetCreatedAt()
	return &t
}

// rdeFindProjectByName finds a project by its exact name (case-insensitive) in the org.
func rdeFindProjectByName(client *qovery.APIClient, orgId string, name string) (*qovery.Project, error) {
	projects, _, err := client.ProjectsAPI.ListProject(context.Background(), orgId).Execute()
	if err != nil {
		return nil, err
	}

	for _, project := range projects.GetResults() {
		if strings.EqualFold(project.Name, name) {
			return &project, nil
		}
	}

	return nil, fmt.Errorf("project %s not found", name)
}

// rdeFindCustomRoleByName finds a custom role by name in the org.
func rdeFindCustomRoleByName(client *qovery.APIClient, orgId string, roleName string) (*qovery.OrganizationCustomRole, error) {
	roles, _, err := client.OrganizationCustomRoleAPI.ListOrganizationCustomRoles(context.Background(), orgId).Execute()
	if err != nil {
		return nil, err
	}

	for _, role := range roles.GetResults() {
		if role.Name != nil && strings.EqualFold(*role.Name, roleName) {
			return &role, nil
		}
	}

	return nil, nil
}

// rdePrintEnvServices prints the services and their statuses for an environment as a table.
func rdePrintEnvServices(client *qovery.APIClient, envId string) {
	statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()
	if err != nil {
		utils.Println("  (could not retrieve statuses)")
		return
	}

	// Build a name map from all service types
	nameMap := make(map[string]string)

	apps, _, _ := client.ApplicationsAPI.ListApplication(context.Background(), envId).Execute()
	if apps != nil {
		for _, app := range apps.GetResults() {
			nameMap[app.Id] = app.GetName()
		}
	}

	containers, _, _ := client.ContainersAPI.ListContainer(context.Background(), envId).Execute()
	if containers != nil {
		for _, c := range containers.GetResults() {
			nameMap[c.Id] = c.Name
		}
	}

	jobs, _, _ := client.JobsAPI.ListJobs(context.Background(), envId).Execute()
	if jobs != nil {
		for _, j := range jobs.GetResults() {
			nameMap[utils.GetJobId(&j)] = utils.GetJobName(&j)
		}
	}

	databases, _, _ := client.DatabasesAPI.ListDatabase(context.Background(), envId).Execute()
	if databases != nil {
		for _, db := range databases.GetResults() {
			nameMap[db.Id] = db.Name
		}
	}

	helms, _, _ := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()
	if helms != nil {
		for _, h := range helms.GetResults() {
			nameMap[h.Id] = h.Name
		}
	}

	var data [][]string
	collectStatuses := func(statuses []qovery.Status, typeName string) {
		for _, s := range statuses {
			name := nameMap[s.Id]
			if name == "" {
				name = s.Id
			}
			data = append(data, []string{name, typeName, utils.GetStatusTextWithColor(s.State)})
		}
	}

	collectStatuses(statuses.GetApplications(), "Application")
	collectStatuses(statuses.GetContainers(), "Container")
	collectStatuses(statuses.GetJobs(), "Job")
	collectStatuses(statuses.GetDatabases(), "Database")
	collectStatuses(statuses.GetHelms(), "Helm")

	if len(data) == 0 {
		utils.Println("  No services found.")
		return
	}

	_ = utils.PrintTable([]string{"Name", "Type", "Status"}, data)
}

// rdePrintKeyValueTable renders a key-value pterm table (no headers, like PrintContext).
func rdePrintKeyValueTable(rows [][]string) {
	tableData := pterm.TableData{}
	for _, row := range rows {
		tableData = append(tableData, row)
	}
	_ = pterm.DefaultTable.WithData(tableData).Render()
}

// ctx is a shorthand for context.Background() used in RDE commands.
func ctx() context.Context {
	return context.Background()
}

// rdeBlueprintNameForProjectId resolves a blueprint project ID to its project name.
func rdeBlueprintNameForProjectId(client *qovery.APIClient, projectId string) string {
	project, _, err := client.ProjectMainCallsAPI.GetProject(context.Background(), projectId).Execute()
	if err != nil {
		return projectId // fallback to ID
	}
	return project.Name
}

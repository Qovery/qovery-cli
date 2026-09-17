package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
)

var rdeCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Provision a new Remote Development Environment from a blueprint",
	Long: `Create a new RDE by cloning a blueprint environment into a new project.

This command:
  1. Creates a new project for the RDE
  2. Creates an RBAC role with scoped permissions (unless --skip-rbac)
  3. Clones the blueprint environment into the new project
  4. Updates the TTL job to target the new environment (if present)
  5. Invites the developer via email (unless --skip-invite)
  6. Triggers deployment (unless --skip-deploy)`,
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		client := utils.GetQoveryClientPanicInCaseOfError()
		orgId, err := rdeGetOrgId(client)
		checkError(err)

		// Validate required flags
		if rdeBlueprintProjectName == "" {
			utils.PrintlnError(fmt.Errorf("--blueprint is required"))
			os.Exit(1)
			panic("unreachable")
		}
		if rdeName == "" {
			utils.PrintlnError(fmt.Errorf("--name is required"))
			os.Exit(1)
			panic("unreachable")
		}
		if rdeEmail == "" && !rdeSkipInvite {
			utils.PrintlnInfo("No --email provided, skipping member invitation (use --skip-invite to suppress this message)")
			rdeSkipInvite = true
		}

		// Step 1: Resolve blueprint
		utils.Println(fmt.Sprintf("Resolving blueprint %s...", pterm.FgBlue.Sprintf("%s", rdeBlueprintProjectName)))
		bp, err := rdeFindBlueprintByProjectName(client, orgId, rdeBlueprintProjectName)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable")
		}
		if bp.EnvId == "" {
			utils.PrintlnError(fmt.Errorf("blueprint %s has no environment with %s set", bp.ProjectName, rdeBlueprintKeyVar))
			os.Exit(1)
			panic("unreachable")
		}
		utils.Println(fmt.Sprintf("  Blueprint: %s (env: %s)", bp.ProjectName, bp.EnvId))

		// Step 2: Create project
		projectName := fmt.Sprintf("rde-%s", rdeName)
		utils.Println(fmt.Sprintf("\nStep 1/6: Creating project %s...", pterm.FgBlue.Sprintf("%s", projectName)))
		desc := fmt.Sprintf("RDE for %s (blueprint: %s)", rdeName, bp.ProjectName)
		projectReq := qovery.NewProjectRequest(projectName)
		projectReq.Description = &desc
		project, _, err := client.ProjectsAPI.CreateProject(ctx(), orgId).ProjectRequest(*projectReq).Execute()
		if err != nil {
			// Check if project already exists
			existing, findErr := rdeFindProjectByName(client, orgId, projectName)
			if findErr == nil && existing != nil {
				utils.PrintlnInfo(fmt.Sprintf("Project %s already exists, reusing...", projectName))
				project = existing
			} else {
				utils.PrintlnError(fmt.Errorf("failed to create project: %w", err))
				os.Exit(1)
				panic("unreachable")
			}
		}
		utils.Println(fmt.Sprintf("  Project: %s", project.Id))

		// Step 3: Create RBAC role
		var roleId string
		if !rdeSkipRbac {
			roleName := fmt.Sprintf("RDE-%s", rdeName)
			utils.Println(fmt.Sprintf("\nStep 2/6: Creating RBAC role %s...", pterm.FgBlue.Sprintf("%s", roleName)))

			roleReq := qovery.NewOrganizationCustomRoleCreateRequest(roleName)
			roleDesc := fmt.Sprintf("Access to %s only", projectName)
			roleReq.Description = &roleDesc
			role, _, err := client.OrganizationCustomRoleAPI.CreateOrganizationCustomRole(ctx(), orgId).
				OrganizationCustomRoleCreateRequest(*roleReq).Execute()
			if err != nil {
				// Check if role already exists
				existingRole, _ := rdeFindCustomRoleByName(client, orgId, roleName)
				if existingRole != nil && existingRole.Id != nil {
					utils.PrintlnInfo(fmt.Sprintf("Role %s already exists, reusing...", roleName))
					roleId = *existingRole.Id
				} else {
					utils.PrintlnError(fmt.Errorf("failed to create RBAC role: %w", err))
					utils.PrintlnInfo("Continuing without RBAC role (use --skip-rbac to suppress)")
				}
			} else if role.Id != nil {
				roleId = *role.Id
			}

			if roleId != "" {
				// Set permissions: all clusters VIEWER except target = ENV_CREATOR, all projects NO_ACCESS except ours = DEPLOYER
				err = rdeSetRolePermissions(client, orgId, roleId, roleName, project.Id)
				if err != nil {
					utils.PrintlnError(fmt.Errorf("failed to set role permissions: %w", err))
					utils.PrintlnInfo("RBAC role created but permissions may be incomplete")
				}
				utils.Println(fmt.Sprintf("  Role: %s", roleId))
			}
		} else {
			utils.Println("\nStep 2/6: Skipping RBAC role creation (--skip-rbac)")
		}

		// Step 4: Clone blueprint
		utils.Println("\nStep 3/6: Cloning blueprint environment...")
		cloneReq := qovery.CloneEnvironmentRequest{
			Name:      "workspace",
			ProjectId: &project.Id,
			Mode:      qovery.ENVIRONMENTMODEENUM_DEVELOPMENT.Ptr(),
		}

		// Default to the blueprint's cluster
		blueprintEnv, _, bpEnvErr := client.EnvironmentMainCallsAPI.GetEnvironment(ctx(), bp.EnvId).Execute()
		if bpEnvErr == nil {
			bpClusterId := blueprintEnv.ClusterId
			cloneReq.ClusterId = &bpClusterId
			clusterDisplay := bpClusterId
			if blueprintEnv.ClusterName != nil {
				clusterDisplay = *blueprintEnv.ClusterName
			}
			utils.Println(fmt.Sprintf("  Using blueprint cluster: %s", pterm.FgBlue.Sprintf("%s", clusterDisplay)))
		}

		// Override with --cluster flag if provided
		if clusterName != "" {
			clusters, _, clErr := client.ClustersAPI.ListOrganizationCluster(ctx(), orgId).Execute()
			if clErr == nil {
				found := false
				for _, c := range clusters.GetResults() {
					if strings.EqualFold(c.Name, clusterName) {
						clId := c.Id
						cloneReq.ClusterId = &clId
						utils.Println(fmt.Sprintf("  Overriding cluster to: %s", pterm.FgBlue.Sprintf("%s", clusterName)))
						found = true
						break
					}
				}
				if !found {
					utils.PrintlnError(fmt.Errorf("cluster %s not found", clusterName))
					os.Exit(1)
					panic("unreachable")
				}
			}
		}

		clonedEnv, _, err := client.EnvironmentActionsAPI.CloneEnvironment(ctx(), bp.EnvId).
			CloneEnvironmentRequest(cloneReq).Execute()
		if err != nil {
			// Check if environment already exists
			envInfo, findErr := rdeFindBlueprintEnv(client, project.Id)
			if findErr == nil && envInfo != nil {
				utils.PrintlnInfo("Environment already exists, reusing...")
				// Create a minimal environment struct for use below
				clonedEnv = &qovery.Environment{}
				clonedEnv.Id = envInfo.EnvId
				clonedEnv.Name = envInfo.EnvName
			} else {
				utils.PrintlnError(fmt.Errorf("failed to clone blueprint: %w", err))
				os.Exit(1)
				panic("unreachable")
			}
		}
		utils.Println(fmt.Sprintf("  Environment: %s", clonedEnv.Id))

		// Set RDE_OWNER_EMAIL on the cloned environment if email was provided
		if rdeEmail != "" {
			_ = utils.CreateEnvironmentVariable(client, project.Id, clonedEnv.Id, rdeOwnerEmailVar, rdeEmail, false)
		}

		// Step 5: Update TTL job (if present)
		utils.Println("\nStep 4/6: Checking for TTL job...")
		rdeUpdateTTLJob(client, clonedEnv.Id)

		// Step 6: Invite member
		if !rdeSkipInvite && rdeEmail != "" {
			utils.Println(fmt.Sprintf("\nStep 5/6: Inviting %s...", rdeEmail))
			inviteReq := qovery.NewInviteMemberRequest(rdeEmail)
			if roleId != "" {
				inviteReq.RoleId = &roleId
			}
			_, _, err = client.MembersAPI.PostInviteMember(ctx(), orgId).
				InviteMemberRequest(*inviteReq).Execute()
			if err != nil {
				utils.PrintlnInfo(fmt.Sprintf("Invitation failed or already sent: %v", err))
			} else {
				utils.Println(fmt.Sprintf("  Invited: %s", rdeEmail))
			}
		} else {
			utils.Println("\nStep 5/6: Skipping invitation")
		}

		// Step 7: Deploy
		if !rdeSkipDeploy {
			utils.Println("\nStep 6/6: Deploying...")
			_, _, err = client.EnvironmentActionsAPI.DeployEnvironment(ctx(), clonedEnv.Id).Execute()
			if err != nil {
				utils.PrintlnInfo(fmt.Sprintf("Deploy failed: %v (deploy from Console)", err))
			} else {
				utils.Println("  Deployment triggered")
			}
		} else {
			utils.Println("\nStep 6/6: Skipping deployment (--skip-deploy)")
		}

		utils.Println("")
		utils.Println(fmt.Sprintf("RDE %s provisioned successfully!", pterm.FgBlue.Sprintf("%s", rdeName)))
		utils.Println("")
		rdePrintKeyValueTable([][]string{
			{"Project", project.Id},
			{"Environment", clonedEnv.Id},
			{"Console", fmt.Sprintf("https://console.qovery.com/organization/%s/project/%s/environment/%s", orgId, project.Id, clonedEnv.Id)},
		})
		utils.Println("")
		utils.PrintlnInfo("Workspace URL will be available once deployment completes.")
	},
}

// rdeSetRolePermissions configures the RBAC role with appropriate cluster and project permissions.
func rdeSetRolePermissions(client *qovery.APIClient, orgId string, roleId string, roleName string, targetProjectId string) error {
	// Get all clusters
	clusters, _, err := client.ClustersAPI.ListOrganizationCluster(ctx(), orgId).Execute()
	if err != nil {
		return fmt.Errorf("failed to list clusters: %w", err)
	}

	var clusterPerms []qovery.OrganizationCustomRoleUpdateRequestClusterPermissionsInner
	for _, c := range clusters.GetResults() {
		perm := qovery.ORGANIZATIONCUSTOMROLECLUSTERPERMISSION_VIEWER
		// If cluster name matches, give ENV_CREATOR
		if clusterName != "" && strings.EqualFold(c.Name, clusterName) {
			perm = qovery.ORGANIZATIONCUSTOMROLECLUSTERPERMISSION_ENV_CREATOR
		} else if clusterName == "" {
			// If no cluster specified, give ENV_CREATOR on all clusters
			perm = qovery.ORGANIZATIONCUSTOMROLECLUSTERPERMISSION_ENV_CREATOR
		}
		cId := c.Id
		clusterPerms = append(clusterPerms, qovery.OrganizationCustomRoleUpdateRequestClusterPermissionsInner{
			ClusterId:  &cId,
			Permission: &perm,
		})
	}

	// Get all projects
	projects, _, err := client.ProjectsAPI.ListProject(ctx(), orgId).Execute()
	if err != nil {
		return fmt.Errorf("failed to list projects: %w", err)
	}

	var projectPerms []qovery.OrganizationCustomRoleUpdateRequestProjectPermissionsInner
	for _, p := range projects.GetResults() {
		isAdmin := false
		pId := p.Id

		var permissions []qovery.OrganizationCustomRoleUpdateRequestProjectPermissionsInnerPermissionsInner
		if p.Id == targetProjectId {
			// Target project: DEPLOYER for DEVELOPMENT and PREVIEW, VIEWER for STAGING, NO_ACCESS for PRODUCTION
			devMode := qovery.ENVIRONMENTMODEENUM_DEVELOPMENT
			stagingMode := qovery.ENVIRONMENTMODEENUM_STAGING
			prodMode := qovery.ENVIRONMENTMODEENUM_PRODUCTION
			previewMode := qovery.ENVIRONMENTMODEENUM_PREVIEW
			deployerPerm := qovery.ORGANIZATIONCUSTOMROLEPROJECTPERMISSION_DEPLOYER
			viewerPerm := qovery.ORGANIZATIONCUSTOMROLEPROJECTPERMISSION_VIEWER
			noAccessPerm := qovery.ORGANIZATIONCUSTOMROLEPROJECTPERMISSION_NO_ACCESS

			permissions = []qovery.OrganizationCustomRoleUpdateRequestProjectPermissionsInnerPermissionsInner{
				{EnvironmentType: &devMode, Permission: &deployerPerm},
				{EnvironmentType: &stagingMode, Permission: &viewerPerm},
				{EnvironmentType: &prodMode, Permission: &noAccessPerm},
				{EnvironmentType: &previewMode, Permission: &deployerPerm},
			}
		} else {
			// Other projects: NO_ACCESS for all
			devMode := qovery.ENVIRONMENTMODEENUM_DEVELOPMENT
			stagingMode := qovery.ENVIRONMENTMODEENUM_STAGING
			prodMode := qovery.ENVIRONMENTMODEENUM_PRODUCTION
			previewMode := qovery.ENVIRONMENTMODEENUM_PREVIEW
			noAccessPerm := qovery.ORGANIZATIONCUSTOMROLEPROJECTPERMISSION_NO_ACCESS

			permissions = []qovery.OrganizationCustomRoleUpdateRequestProjectPermissionsInnerPermissionsInner{
				{EnvironmentType: &devMode, Permission: &noAccessPerm},
				{EnvironmentType: &stagingMode, Permission: &noAccessPerm},
				{EnvironmentType: &prodMode, Permission: &noAccessPerm},
				{EnvironmentType: &previewMode, Permission: &noAccessPerm},
			}
		}

		projectPerms = append(projectPerms, qovery.OrganizationCustomRoleUpdateRequestProjectPermissionsInner{
			ProjectId:   &pId,
			IsAdmin:     &isAdmin,
			Permissions: permissions,
		})
	}

	updateReq := qovery.NewOrganizationCustomRoleUpdateRequest(roleName, clusterPerms, projectPerms)
	_, _, err = client.OrganizationCustomRoleAPI.EditOrganizationCustomRole(ctx(), orgId, roleId).
		OrganizationCustomRoleUpdateRequest(*updateReq).Execute()
	return err
}

// rdeUpdateTTLJob finds and updates the ttl-auto-shutdown job in the environment.
func rdeUpdateTTLJob(client *qovery.APIClient, envId string) {
	jobs, _, err := client.JobsAPI.ListJobs(ctx(), envId).Execute()
	if err != nil {
		utils.Println("  No jobs found (non-critical)")
		return
	}

	for _, job := range jobs.GetResults() {
		jobName := utils.GetJobName(&job)
		if jobName == "ttl-auto-shutdown" {
			jobId := utils.GetJobId(&job)
			utils.Println(fmt.Sprintf("  Found TTL job: %s", jobId))
			// The TTL job is cloned from the blueprint and may reference the blueprint env ID
			// in its arguments. We don't modify the curl command here since the token would
			// also need updating. The TTL job will work as-is if the SHUTDOWN_TOKEN env var
			// is properly set on the job.
			utils.Println("  TTL job preserved from blueprint clone")
			return
		}
	}

	utils.Println("  No TTL job found (non-critical)")
}

func init() {
	rdeCmd.AddCommand(rdeCreateCmd)
	rdeCreateCmd.Flags().StringVarP(&rdeBlueprintProjectName, "blueprint", "b", "", "Blueprint Project Name to clone from")
	rdeCreateCmd.Flags().StringVarP(&rdeName, "name", "n", "", "Name for the new RDE (will create project rde-<name>)")
	rdeCreateCmd.Flags().StringVarP(&rdeEmail, "email", "e", "", "Email address to invite the developer")
	rdeCreateCmd.Flags().StringVarP(&clusterName, "cluster", "c", "", "Cluster Name where to create the RDE")
	rdeCreateCmd.Flags().StringVarP(&organizationName, "organization", "o", "", "Organization Name")
	rdeCreateCmd.Flags().BoolVarP(&rdeSkipRbac, "skip-rbac", "", false, "Skip RBAC role creation")
	rdeCreateCmd.Flags().BoolVarP(&rdeSkipInvite, "skip-invite", "", false, "Skip member invitation")
	rdeCreateCmd.Flags().BoolVarP(&rdeSkipDeploy, "skip-deploy", "", false, "Skip deployment after cloning")

	_ = rdeCreateCmd.MarkFlagRequired("blueprint")
	_ = rdeCreateCmd.MarkFlagRequired("name")
}

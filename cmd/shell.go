package cmd

import (
	"errors"
	"fmt"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Connect to an application container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)
		useContext := false
		currentContext, err := utils.CurrentContext()
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		utils.PrintlnInfo("Current context:")
		if currentContext.ApplicationId != "" && currentContext.ApplicationName != "" &&
			currentContext.EnvironmentId != "" && currentContext.EnvironmentName != "" &&
			currentContext.ProjectId != "" && currentContext.ProjectName != "" &&
			currentContext.OrganizationId != "" && currentContext.OrganizationName != "" {
			if err := utils.PrintlnContext(); err != nil {
				fmt.Println("Context not yet configured.")
			}
			fmt.Println()

			utils.PrintlnInfo("Continue with shell command using this context ?")
			useContext = utils.Validate("context")
			fmt.Println()
		} else {
			if err := utils.PrintlnContext(); err != nil {
				fmt.Println("Context not yet configured.")
				fmt.Println("Unable to use current context for `shell` command.")
				fmt.Println()
			}
		}

		var req *pkg.ShellRequest
		if useContext {
			req, err = shellRequestFromContext(currentContext)
		} else {
			req, err = shellRequestFromSelect()
		}
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		pkg.ExecShell(req)
	},
}

func shellRequestFromSelect() (*pkg.ShellRequest, error) {
	utils.PrintlnInfo("Select organization")
	orga, err := utils.SelectOrganization()
	if err != nil {
		return nil, err
	}

	utils.PrintlnInfo("Select project")
	project, err := utils.SelectProject(orga.ID)
	if err != nil {
		return nil, err
	}

	utils.PrintlnInfo("Select environment")
	env, err := utils.SelectEnvironment(project.ID)
	if err != nil {
		return nil, err
	}

	utils.PrintlnInfo("Select application")
	app, err := utils.SelectApplication(env.ID)
	if err != nil {
		return nil, err
	}

	return &pkg.ShellRequest{
		ApplicationID:  app.ID,
		ProjectID:      project.ID,
		OrganizationID: orga.ID,
		EnvironmentID:  env.ID,
		ClusterID:      env.ClusterID,
	}, nil
}

func shellRequestFromContext(currentContext utils.QoveryContext) (*pkg.ShellRequest, error) {
	token, err := utils.GetAccessToken()
	if err != nil {
		return nil, err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	e, res, err := client.EnvironmentMainCallsApi.GetEnvironment(auth, string(currentContext.EnvironmentId)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while fetching environment. ")
	}

	return &pkg.ShellRequest{
		ApplicationID:  currentContext.ApplicationId,
		ProjectID:      currentContext.ProjectId,
		OrganizationID: currentContext.OrganizationId,
		EnvironmentID:  currentContext.EnvironmentId,
		ClusterID:      utils.Id(e.ClusterId),
	}, nil
}

func init() {
	rootCmd.AddCommand(shellCmd)
}

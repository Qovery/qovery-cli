package cmd

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Connect to an application container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		var shellRequest *pkg.ShellRequest
		var err error
		if len(args) > 0 {
			shellRequest, err = shellRequestWithApplicationUrl(args)
		} else {
			shellRequest, err = shellRequestWithoutArg()
		}
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		pkg.ExecShell(shellRequest)
	},
}
var (
	command           []string
	podName           *string
	podContainerName  *string
	podNameFlag       string
	containerNameFlag string
)

func shellRequestWithoutArg() (*pkg.ShellRequest, error) {
	useContext := false
	currentContext, err := utils.CurrentContext()
	if err != nil {
		return nil, err
	}

	utils.PrintlnInfo("Current context:")
	if currentContext.ServiceId != "" && currentContext.ServiceName != "" &&
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
		return nil, err
	}

	return req, nil
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

	utils.PrintlnInfo("Select service")
	service, err := utils.SelectService(env.ID)
	if err != nil {
		return nil, err
	}

	return &pkg.ShellRequest{
		ServiceID:      service.ID,
		ProjectID:      project.ID,
		OrganizationID: orga.ID,
		EnvironmentID:  env.ID,
		ClusterID:      env.ClusterID,
		PodName:        podName,
		ContainerName:  podContainerName,
		Command:        command,
	}, nil
}

func shellRequestFromContext(currentContext utils.QoveryContext) (*pkg.ShellRequest, error) {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	client := utils.GetQoveryClient(tokenType, token)

	e, res, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), string(currentContext.EnvironmentId)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while fetching environment. ")
	}

	return &pkg.ShellRequest{
		ServiceID:      currentContext.ServiceId,
		ProjectID:      currentContext.ProjectId,
		OrganizationID: currentContext.OrganizationId,
		EnvironmentID:  currentContext.EnvironmentId,
		ClusterID:      utils.Id(e.ClusterId),
		PodName:        podName,
		ContainerName:  podContainerName,
		Command:        command,
	}, nil
}

func shellRequestWithApplicationUrl(args []string) (*pkg.ShellRequest, error) {
	var url = args[0]
	url = strings.Replace(url, "https://console.qovery.com/", "", 1)
	url = strings.Replace(url, "https://new.console.qovery.com/", "", 1)
	urlSplit := strings.Split(url, "/")

	if len(urlSplit) < 8 {
		return nil, errors.New("Wrong URL format: " + url)
	}

	var organizationId = urlSplit[1]
	organization, err := utils.GetOrganizationById(organizationId)
	if err != nil {
		return nil, err
	}

	var projectId = urlSplit[3]
	project, err := utils.GetProjectById(projectId)
	if err != nil {
		return nil, err
	}

	var environmentId = urlSplit[5]
	environment, err := utils.GetEnvironmentById(environmentId)
	if err != nil {
		return nil, err
	}

	environmentServices, err := utils.GetEnvironmentServicesById(environmentId)
	if err != nil {
		return nil, err
	}

	var service utils.Service
	var serviceId = urlSplit[7]
	for _, envService := range environmentServices {
		if envService.ID == serviceId {
			switch envService.Type {

			case utils.ApplicationType:
				applicationAPI, err := utils.GetApplicationById(serviceId)
				if err != nil {
					return nil, err
				}
				service = utils.Service{
					ID:   applicationAPI.ID,
					Name: applicationAPI.Name,
					Type: utils.ApplicationType,
				}

			case utils.ContainerType:
				containerAPI, err := utils.GetContainerById(serviceId)
				if err != nil {
					return nil, err
				}
				service = utils.Service{
					ID:   containerAPI.ID,
					Name: containerAPI.Name,
					Type: utils.ContainerType,
				}

			case utils.JobType:
				jobAPI, err := utils.GetJobById(serviceId)
				if err != nil {
					return nil, err
				}
				service = utils.Service{
					ID:   jobAPI.ID,
					Name: jobAPI.Name,
					Type: utils.JobType,
				}

			default:
				return nil, errors.New("ServiceLevel type `" + string(envService.Type) + "` is not supported for shell")
			}
		}
	}

	_ = pterm.DefaultTable.WithData(pterm.TableData{
		{"Organization", string(organization.Name)},
		{"Project", string(project.Name)},
		{"Environment", string(environment.Name)},
		{"ServiceLevel", string(service.Name)},
		{"ServiceType", string(service.Type)},
	}).Render()

	return &pkg.ShellRequest{
		OrganizationID: organization.ID,
		ProjectID:      project.ID,
		EnvironmentID:  environment.ID,
		ServiceID:      service.ID,
		ClusterID:      environment.ClusterID,
		PodName:        podName,
		ContainerName:  podContainerName,
		Command:        command,
	}, nil
}

func init() {
	var shellCmd = shellCmd
	shellCmd.Flags().StringSliceVarP(&command, "command", "c", []string{"sh"}, "command to launch inside the pod")
	shellCmd.Flags().StringVarP(&podNameFlag, "pod", "p", "", "pod name where to exec into")
	shellCmd.Flags().StringVar(&containerNameFlag, "container", "", "container name inside the pod")

	if podNameFlag != "" {
		podName = &podNameFlag
	}
	if containerNameFlag != "" {
		podContainerName = &containerNameFlag
	}

	rootCmd.AddCommand(shellCmd)
}

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
	"github.com/qovery/qovery-cli/pkg/usercontext"
	"github.com/qovery/qovery-cli/utils"
)

var shellCmd = &cobra.Command{
	Use:   "shell",
	Short: "Connect to an application container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		var shellRequest *pkg.ShellRequest
		var err error
		if strings.TrimSpace(organizationName) != "" || strings.TrimSpace(projectName) != "" || strings.TrimSpace(environmentName) != "" || strings.TrimSpace(serviceName) != "" {
			if strings.TrimSpace(organizationName) == "" {
				utils.PrintlnError(errors.New("organization name is required"))
				return
			}
			if strings.TrimSpace(projectName) == "" {
				utils.PrintlnError(errors.New("project name is required"))
				return
			}
			if strings.TrimSpace(environmentName) == "" {
				utils.PrintlnError(errors.New("environment name is required"))
				return
			}
			if strings.TrimSpace(serviceName) == "" {
				utils.PrintlnError(errors.New("service name is required"))
				return
			}

			shellRequest, err = shellRequestWithContextFlags()
		} else if len(args) == 1 {
			shellRequest, err = shellRequestWithApplicationUrl(args)
		} else {
			shellRequest, err = shellRequestWithoutArg()
		}
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		pkg.ExecShell(shellRequest, "/shell/exec")
	},
}

var (
	command          []string
	podName          string
	podContainerName string
)

func shellRequestWithContextFlags() (*pkg.ShellRequest, error) {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	client := utils.GetQoveryClient(tokenType, token)

	organizationID, err := usercontext.GetOrganizationContextResourceId(client, organizationName)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	projectID, err := getProjectContextResourceId(client, projectName, organizationID)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	environmentID, err := getEnvironmentContextResourceId(client, environmentName, projectID)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	environment, err := utils.GetEnvironmentById(environmentID)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	service, err := getServiceContextResourceId(client, serviceName, environmentID)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return &pkg.ShellRequest{
		ServiceID:      utils.Id(service.ID),
		ProjectID:      utils.Id(projectID),
		OrganizationID: utils.Id(organizationID),
		EnvironmentID:  utils.Id(environmentID),
		ClusterID:      environment.ClusterID,
		PodName:        podName,
		ContainerName:  podContainerName,
		Command:        command,
	}, nil
}

func shellRequestWithoutArg() (*pkg.ShellRequest, error) {
	useContext := false
	currentContext, err := utils.GetCurrentContext()
	if err != nil {
		return nil, err
	}

	utils.PrintlnInfo("Current context:")
	if currentContext.ServiceId != "" && currentContext.ServiceName != "" &&
		currentContext.EnvironmentId != "" && currentContext.EnvironmentName != "" &&
		currentContext.ProjectId != "" && currentContext.ProjectName != "" &&
		currentContext.OrganizationId != "" && currentContext.OrganizationName != "" {
		if err := utils.PrintContext(); err != nil {
			fmt.Println("Context not yet configured.")
		}
		fmt.Println()

		utils.PrintlnInfo("Continue with shell command using this context ?")
		useContext = utils.Validate("context")
		fmt.Println()
	} else {
		if err := utils.PrintContext(); err != nil {
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
	url := args[0]
	url = strings.Replace(url, "https://console.qovery.com/", "", 1)
	url = strings.Replace(url, "https://new.console.qovery.com/", "", 1)
	urlSplit := strings.Split(url, "/")

	if len(urlSplit) < 8 {
		return nil, errors.New("Wrong URL format: " + url)
	}

	organizationId := urlSplit[1]
	organization, err := utils.GetOrganizationById(organizationId)
	if err != nil {
		return nil, err
	}

	projectId := urlSplit[3]
	project, err := utils.GetProjectById(projectId)
	if err != nil {
		return nil, err
	}

	environmentId := urlSplit[5]
	environment, err := utils.GetEnvironmentById(environmentId)
	if err != nil {
		return nil, err
	}

	environmentServices, err := utils.GetEnvironmentServicesById(environmentId)
	if err != nil {
		return nil, err
	}

	var service utils.Service
	serviceId := urlSplit[7]
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

			case utils.DatabaseType:
				db, err := utils.GetDatabaseById(serviceId)
				if err != nil {
					return nil, err
				}
				service = *db

			case utils.HelmType:
				helm, err := utils.GetHelmById(serviceId)
				if err != nil {
					return nil, err
				}
				service = *helm

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
	shellCmd := shellCmd
	shellCmd.Flags().StringSliceVarP(&command, "command", "c", []string{"sh"}, "command to launch inside the pod")
	shellCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	shellCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	shellCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	shellCmd.Flags().StringVarP(&serviceName, "service", "", "", "Service Name")
	shellCmd.Flags().StringVarP(&podName, "pod", "p", "", "pod name where to exec into")
	shellCmd.Flags().StringVar(&podContainerName, "container", "", "container name inside the pod")
	shellCmd.Example = "qovery shell\n" +
		"qovery shell <qovery_console_service_url>\n" +
		"qovery shell --organization <organization_name> --project <project_name> --environment <environment_name> --service <service_name>\n" +
		"qovery shell --organization <organization_name> --project <project_name> --environment <environment_name> --service <service_name> --pod <pod_name> --container <container_name> --command <command>"

	rootCmd.AddCommand(shellCmd)
}

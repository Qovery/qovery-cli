package cmd

import (
	"errors"
	"fmt"
	log "github.com/sirupsen/logrus"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"

	"github.com/qovery/qovery-cli/pkg"
	"github.com/qovery/qovery-cli/utils"
)

var portForwardCmd = &cobra.Command{
	Use:   "port-forward",
	Short: "Port forward a port to an application container",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		if len(ports) == 0 {
			log.Fatal("port flag must be specified at least once")
			return
		}

		var portForwardRequest *pkg.PortForwardRequest
		var err error
		if len(args) > 0 {
			portForwardRequest, err = portForwardRequestWithApplicationUrl(args)
		} else {
			portForwardRequest, err = portForwardRequestWithoutArg()
		}
		if err != nil {
			utils.PrintlnError(err)
			return
		}

		for _, port := range ports {
			ps := strings.Split(port, ":")
			var localPortStr, remotePortStr string
			if len(ps) > 1 {
				localPortStr = ps[0]
				remotePortStr = ps[1]
			} else {
				localPortStr = ps[0]
				remotePortStr = ps[0]
			}

			localPort, err := strconv.ParseUint(localPortStr, 10, 16)
			if err != nil {
				log.Fatal("Invalid local port {} {}", port, err)
			}

			remotePort, err := strconv.ParseUint(remotePortStr, 10, 16)
			if err != nil {
				log.Fatal("Invalid remote port {} {}", port, err)
			}

			req := *portForwardRequest
			req.LocalPort = uint16(localPort)
			req.Port = uint16(remotePort)
			go pkg.ExecPortForward(&req)
		}

		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGINT, syscall.SIGTERM)
		<-done
	},
}
var (
	ports []string
)

func portForwardRequestWithoutArg() (*pkg.PortForwardRequest, error) {
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

		utils.PrintlnInfo("Continue with port-forward command using this context ?")
		useContext = utils.Validate("context")
		fmt.Println()
	} else {
		if err := utils.PrintlnContext(); err != nil {
			fmt.Println("Context not yet configured.")
			fmt.Println("Unable to use current context for `port-forward` command.")
			fmt.Println()
		}
	}

	var req *pkg.PortForwardRequest
	if useContext {
		req, err = portForwardRequestFromContext(currentContext)
	} else {
		req, err = portForwardRequestFromSelect()
	}
	if err != nil {
		return nil, err
	}

	return req, nil
}

func portForwardRequestFromSelect() (*pkg.PortForwardRequest, error) {
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

	return &pkg.PortForwardRequest{
		ServiceID:      service.ID,
		ServiceType:    strings.ToUpper(string(service.Type)),
		ProjectID:      project.ID,
		OrganizationID: orga.ID,
		EnvironmentID:  env.ID,
		ClusterID:      env.ClusterID,
		PodName:        podName,
		Port:           0,
		LocalPort:      0,
	}, nil
}

func portForwardRequestFromContext(currentContext utils.QoveryContext) (*pkg.PortForwardRequest, error) {
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

	return &pkg.PortForwardRequest{
		ServiceID:      currentContext.ServiceId,
		ServiceType:    strings.ToUpper(string(currentContext.ServiceType)),
		ProjectID:      currentContext.ProjectId,
		OrganizationID: currentContext.OrganizationId,
		EnvironmentID:  currentContext.EnvironmentId,
		ClusterID:      utils.Id(e.ClusterId),
		PodName:        podName,
		Port:           0,
		LocalPort:      0,
	}, nil
}

func portForwardRequestWithApplicationUrl(args []string) (*pkg.PortForwardRequest, error) {
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
				return nil, errors.New("ServiceLevel type `" + string(envService.Type) + "` is not supported for port-forward")
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

	return &pkg.PortForwardRequest{
		OrganizationID: organization.ID,
		ProjectID:      project.ID,
		EnvironmentID:  environment.ID,
		ServiceID:      service.ID,
		ServiceType:    strings.ToUpper(string(service.Type)),
		ClusterID:      environment.ClusterID,
		PodName:        podName,
		Port:           0,
		LocalPort:      0,
	}, nil
}

func init() {
	var portForwardCmd = portForwardCmd
	portForwardCmd.Flags().StringVarP(&podName, "pod", "", "", "pod name where to forward traffic")
	portForwardCmd.Flags().StringSliceVarP(&ports, "port", "p", nil, "port that will be forwarded. Format  \"local_port:remote_port\" i.e: 8080:80")
	_ = portForwardCmd.MarkFlagRequired("port")

	rootCmd.AddCommand(portForwardCmd)
}

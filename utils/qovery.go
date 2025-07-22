package utils

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/qovery/qovery-cli/variable"

	"github.com/pterm/pterm"

	"github.com/manifoldco/promptui"
	"github.com/qovery/qovery-client-go"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/net/context"
)

func init() {
	log.SetFormatter(&log.TextFormatter{
		FullTimestamp: true,
	})
}

type Organization struct {
	ID   Id
	Name Name
}

type TokenInformation struct {
	Organization *Organization
	Role         *Role
	Name         string
	Description  string
}

type Role struct {
	ID   string
	Name Name
}

func WebsocketUrl() string {
	if url := os.Getenv("QOVERY_WS_URL"); url != "" {
		return url
	}
	return "wss://ws.qovery.com"
}

func GetQoveryClientPanicInCaseOfError() *qovery.APIClient {
	tokenType, token, err := GetAccessToken()
	CheckError(err)
	return GetQoveryClient(tokenType, token)
}

func CheckError(err error) {
	if err != nil {
		PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func GetQoveryClient(tokenType AccessTokenType, token AccessToken) *qovery.APIClient {
	conf := qovery.NewConfiguration()
	conf.UserAgent = "CLI " + Version
	if url := os.Getenv("QOVERY_API_URL"); url != "" {
		conf.Servers = qovery.ServerConfigurations{
			{
				URL:         url,
				Description: "No description provided",
			},
		}
	}
	conf.DefaultHeader["Authorization"] = GetAuthorizationHeaderValue(tokenType, token)
	conf.Debug = variable.Verbose
	conf.HTTPClient = &http.Client{
		Timeout: time.Second * 60,
	}
	return qovery.NewAPIClient(conf)
}

func SelectRole(organization *Organization) (*Role, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	roles, res, err := client.OrganizationMainCallsAPI.ListOrganizationAvailableRoles(context.Background(), string(organization.ID)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing organizations. ")
	}

	var roleNames []string
	var rolesIds = make(map[string]string)

	for _, role := range roles.GetResults() {
		roleNames = append(roleNames, role.Name)
		rolesIds[role.Name] = role.Id
	}

	if len(roleNames) < 1 {
		return nil, errors.New("no role found")
	}

	fmt.Println("Roles:")
	prompt := promptui.Select{
		Items: roleNames,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(roleNames[index]), strings.ToLower(input))
		},
	}
	_, selectedRole, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return &Role{
		ID:   rolesIds[selectedRole],
		Name: Name(selectedRole),
	}, nil

}

func SelectOrganization() (*Organization, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	organizations, res, err := client.OrganizationMainCallsAPI.ListOrganization(context.Background()).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing organizations. ")
	}

	var organizationNames []string
	var orgs = make(map[string]string)

	for _, org := range organizations.GetResults() {
		organizationNames = append(organizationNames, org.Name)
		orgs[org.Name] = org.Id
	}

	if len(organizationNames) < 1 {
		return nil, errors.New("no organizations found")
	}

	if len(organizationNames) == 1 {
		return &Organization{
			ID:   Id(orgs[organizationNames[0]]),
			Name: Name(organizationNames[0]),
		}, nil
	}

	fmt.Println("Organization:")
	prompt := promptui.Select{
		Items: organizationNames,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(organizationNames[index]), strings.ToLower(input))
		},
		Size: 30,
	}
	_, selectedOrganization, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return &Organization{
		ID:   Id(orgs[selectedOrganization]),
		Name: Name(selectedOrganization),
	}, nil
}

func SelectAndSetOrganization() (*Organization, error) {
	selectedOrganization, err := SelectOrganization()
	if err != nil {
		return nil, err
	}

	err = SetOrganization(selectedOrganization)
	if err != nil {
		PrintlnError(err)
		return nil, err
	}

	return selectedOrganization, nil
}

type Project struct {
	ID   Id
	Name Name
}

func GetOrganizationById(id string) (*Organization, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	organization, res, err := client.OrganizationMainCallsAPI.GetOrganization(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting organization " + id)
	}
	if err != nil {
		return nil, err
	}

	return &Organization{
		ID:   Id(organization.Id),
		Name: Name(organization.Name),
	}, nil
}

func SelectProject(organizationID Id) (*Project, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	p, res, err := client.ProjectsAPI.ListProject(context.Background(), string(organizationID)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing projects. ")
	}

	var projectsNames []string
	var projects = make(map[string]string)

	for _, proj := range p.GetResults() {
		projectsNames = append(projectsNames, proj.Name)
		projects[proj.Name] = proj.Id
	}

	if len(projectsNames) < 1 {
		return nil, errors.New("no projects found")
	}

	if len(projectsNames) == 1 {
		return &Project{
			ID:   Id(projects[projectsNames[0]]),
			Name: Name(projectsNames[0]),
		}, nil
	}

	fmt.Println("Project:")
	prompt := promptui.Select{
		Items: projectsNames,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(projectsNames[index]), strings.ToLower(input))
		},
	}
	_, selectedProject, err := prompt.Run()
	if err != nil {
		PrintlnError(err)
		return nil, err
	}

	return &Project{
		ID:   Id(projects[selectedProject]),
		Name: Name(selectedProject),
	}, nil
}

func SelectAndSetProject(organizationID Id) (*Project, error) {
	selectedProject, err := SelectProject(organizationID)
	if err != nil {
		return nil, err
	}
	err = SetProject(selectedProject)
	if err != nil {
		PrintlnError(err)
		return nil, err
	}

	return selectedProject, nil
}

type Environment struct {
	ID        Id
	ClusterID Id
	Name      Name
}

func GetProjectById(id string) (*Project, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	project, res, err := client.ProjectMainCallsAPI.GetProject(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting project " + id)
	}
	if err != nil {
		return nil, err
	}

	return &Project{
		ID:   Id(project.Id),
		Name: Name(project.Name),
	}, nil
}

func SelectEnvironment(projectID Id) (*Environment, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	e, res, err := client.EnvironmentsAPI.ListEnvironment(context.Background(), string(projectID)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing environments. ")
	}

	var environmentsNames []string
	var environments = make(map[string]qovery.Environment)

	for _, env := range e.GetResults() {
		environmentsNames = append(environmentsNames, env.Name)
		environments[env.Name] = env
	}

	if len(environmentsNames) < 1 {
		return nil, errors.New("no environments found")
	}

	if len(environmentsNames) == 1 {
		return &Environment{
			ID:        Id(environments[environmentsNames[0]].Id),
			Name:      Name(environmentsNames[0]),
			ClusterID: Id(environments[environmentsNames[0]].ClusterId),
		}, nil
	}

	fmt.Println("Environment:")
	prompt := promptui.Select{
		Items: environmentsNames,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(environmentsNames[index]), strings.ToLower(input))
		},
	}
	_, selectedEnvironment, err := prompt.Run()
	if err != nil {
		PrintlnError(err)
		return nil, err
	}
	return &Environment{
		ID:        Id(environments[selectedEnvironment].Id),
		Name:      Name(selectedEnvironment),
		ClusterID: Id(environments[selectedEnvironment].ClusterId),
	}, nil
}

func SelectAndSetEnvironment(projectID Id) (*Environment, error) {
	selectedEnvironment, err := SelectEnvironment(projectID)
	if err != nil {
		return nil, err
	}

	err = SetEnvironment(selectedEnvironment)
	if err != nil {
		PrintlnError(err)
		return nil, err
	}

	return selectedEnvironment, nil
}

func GetEnvironmentById(id string) (*Environment, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	environment, res, err := client.EnvironmentMainCallsAPI.GetEnvironment(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting environment " + id)
	}
	if err != nil {
		return nil, err
	}

	return &Environment{
		ID:        Id(environment.Id),
		ClusterID: Id(environment.ClusterId),
		Name:      Name(environment.Name),
	}, nil
}

type EnvironmentService struct {
	ID   string
	Type ServiceType
}

func GetEnvironmentServicesById(id string) ([]EnvironmentService, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	environmentServices, res, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting environment services" + id)
	}
	if err != nil {
		return nil, err
	}

	var services []EnvironmentService
	for _, service := range environmentServices.Applications {
		services = append(services, EnvironmentService{
			ID:   service.Id,
			Type: ApplicationType,
		})
	}
	for _, service := range environmentServices.Containers {
		services = append(services, EnvironmentService{
			ID:   service.Id,
			Type: ContainerType,
		})
	}
	for _, service := range environmentServices.Jobs {
		services = append(services, EnvironmentService{
			ID:   service.Id,
			Type: JobType,
		})
	}
	for _, service := range environmentServices.Databases {
		services = append(services, EnvironmentService{
			ID:   service.Id,
			Type: DatabaseType,
		})
	}

	for _, service := range environmentServices.Helms {
		services = append(services, EnvironmentService{
			ID:   service.Id,
			Type: HelmType,
		})
	}

	return services, nil
}

type ServiceType string

const (
	ApplicationType ServiceType = "application"
	ContainerType   ServiceType = "container"
	DatabaseType    ServiceType = "database"
	JobType         ServiceType = "job"
	HelmType        ServiceType = "helm"
)

type Service struct {
	ID   Id
	Name Name
	Type ServiceType
}

type Application struct {
	ID   Id
	Name Name
}

func SelectService(environment Id) (*Service, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	apps, res, err := client.ApplicationsAPI.ListApplication(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing services. ")
	}

	containers, res, err := client.ContainersAPI.ListContainer(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing containers. ")
	}

	databases, res, err := client.DatabasesAPI.ListDatabase(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing containers. ")
	}

	jobs, res, err := client.JobsAPI.ListJobs(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing containers. ")
	}

	helms, res, err := client.HelmsAPI.ListHelms(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing helms. ")
	}

	var servicesNames []string
	var services = make(map[string]Service)

	for _, app := range apps.GetResults() {
		servicesNames = append(servicesNames, app.Name)
		services[app.Name] = Service{
			ID:   Id(app.Id),
			Name: Name(app.Name),
			Type: ApplicationType,
		}
	}

	for _, container := range containers.GetResults() {
		servicesNames = append(servicesNames, container.Name)
		services[container.Name] = Service{
			ID:   Id(container.Id),
			Name: Name(container.Name),
			Type: ContainerType,
		}
	}

	for _, database := range databases.GetResults() {
		servicesNames = append(servicesNames, database.Name)
		services[database.Name] = Service{
			ID:   Id(database.Id),
			Name: Name(database.Name),
			Type: DatabaseType,
		}
	}

	for _, job := range jobs.GetResults() {
		if job.CronJobResponse != nil {
			cronJob := job.CronJobResponse
			servicesNames = append(servicesNames, cronJob.Name)
			services[cronJob.Name] = Service{
				ID:   Id(cronJob.Id),
				Name: Name(cronJob.Name),
				Type: JobType,
			}
		}
		if job.LifecycleJobResponse != nil {
			lifecycleJob := job.LifecycleJobResponse
			servicesNames = append(servicesNames, lifecycleJob.Name)
			services[lifecycleJob.Name] = Service{
				ID:   Id(lifecycleJob.Id),
				Name: Name(lifecycleJob.Name),
				Type: JobType,
			}
		}
	}

	for _, helm := range helms.GetResults() {
		servicesNames = append(servicesNames, helm.Name)
		services[helm.Name] = Service{
			ID:   Id(helm.Id),
			Name: Name(helm.Name),
			Type: HelmType,
		}
	}

	if len(servicesNames) < 1 {
		return nil, errors.New("no services found")
	}

	if len(servicesNames) == 1 {
		service := services[servicesNames[0]]
		return &service, nil
	}

	fmt.Println("Services:")
	prompt := promptui.Select{
		Items: servicesNames,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(servicesNames[index]), strings.ToLower(input))
		},
	}
	_, selectedService, err := prompt.Run()
	if err != nil {
		PrintlnError(err)
		return nil, err
	}

	service := services[selectedService]
	return &service, nil
}

func SelectAndSetService(environment Id) (*Service, error) {
	service, err := SelectService(environment)
	if err != nil {
		PrintlnError(err)
		return nil, err
	}
	if err := SetService(service); err != nil {
		PrintlnError(err)
		return nil, err
	}
	return service, err
}

func GetApplicationById(id string) (*Application, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	application, res, err := client.ApplicationMainCallsAPI.GetApplication(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting application " + id)
	}
	if err != nil {
		return nil, err
	}

	return &Application{
		ID:   Id(application.Id),
		Name: Name(application.GetName()),
	}, nil
}

func ResetApplicationContext() error {
	ctx, err := GetCurrentContext()
	if err != nil {
		return err
	}

	ctx.OrganizationName = ""
	ctx.OrganizationId = ""
	ctx.ProjectName = ""
	ctx.ProjectId = ""
	ctx.EnvironmentName = ""
	ctx.EnvironmentId = ""
	ctx.ServiceName = ""
	ctx.ServiceId = ""
	ctx.ServiceType = ApplicationType

	err = StoreContext(ctx)

	return err
}

type Container struct {
	ID   Id
	Name Name
}

func GetContainerById(id string) (*Container, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	container, res, err := client.ContainerMainCallsAPI.GetContainer(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting container " + id)
	}
	if err != nil {
		return nil, err
	}

	return &Container{
		ID:   Id(container.Id),
		Name: Name(container.GetName()),
	}, nil
}

func GetDatabaseById(id string) (*Service, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	database, res, err := client.DatabaseMainCallsAPI.GetDatabase(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting database " + id)
	}
	if err != nil {
		return nil, err
	}

	return &Service{
		ID:   Id(database.Id),
		Name: Name(database.GetName()),
		Type: DatabaseType,
	}, nil
}

func GetHelmById(id string) (*Service, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	helm, res, err := client.HelmMainCallsAPI.GetHelm(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting helm " + id)
	}
	if err != nil {
		return nil, err
	}

	return &Service{
		ID:   Id(helm.Id),
		Name: Name(helm.GetName()),
		Type: HelmType,
	}, nil
}

type Job struct {
	ID   Id
	Name Name
}

func GetJobById(id string) (*Job, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	job, res, err := client.JobMainCallsAPI.GetJob(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting job " + id)
	}
	if err != nil {
		return nil, err
	}

	if job.LifecycleJobResponse != nil {
		return &Job{
			ID:   Id(job.LifecycleJobResponse.Id),
			Name: Name(job.LifecycleJobResponse.GetName()),
		}, nil
	}

	if job.CronJobResponse != nil {
		return &Job{
			ID:   Id(job.CronJobResponse.Id),
			Name: Name(job.CronJobResponse.GetName()),
		}, nil
	}

	return nil, errors.New("invalid job response")
}

func GetAdminUrl() string {
	url, ok := os.LookupEnv("ADMIN_URL")
	if !ok {
		log.Fatal("You must set the Qovery admin root url (ADMIN_URL).")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
	return url
}

func DeleteEnvironmentVariable(application Id, key string) error {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return err
	}

	client := GetQoveryClient(tokenType, token)

	// TODO optimize this call by caching the result?
	envVars, _, err := client.ApplicationEnvironmentVariableAPI.ListApplicationEnvironmentVariable(context.Background(), string(application)).Execute()

	if err != nil {
		return err
	}

	var envVar *qovery.EnvironmentVariable
	for _, mEnvVar := range envVars.GetResults() {
		if mEnvVar.Key == key {
			envVar = &mEnvVar
			break
		}
	}

	if envVar == nil {
		return nil
	}

	res, err := client.ApplicationEnvironmentVariableAPI.DeleteApplicationEnvironmentVariable(context.Background(), string(application), envVar.Id).Execute()

	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("Received "+res.Status+" response while deleting an Environment Variable for application %s with key %s", string(application), key)
	}

	return nil
}

func AddEnvironmentVariable(application Id, key string, value string) error {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return err
	}

	client := GetQoveryClient(tokenType, token)

	_, res, err := client.ApplicationEnvironmentVariableAPI.CreateApplicationEnvironmentVariable(context.Background(), string(application)).EnvironmentVariableRequest(
		qovery.EnvironmentVariableRequest{Key: key, Value: &value},
	).Execute()

	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("Received "+res.Status+" response while adding an environment variable for application %s", string(application))
	}

	return nil
}

func DeleteSecret(application Id, key string) error {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return err
	}

	client := GetQoveryClient(tokenType, token)

	// TODO optimize this call by caching the result?
	secrets, _, err := client.ApplicationSecretAPI.ListApplicationSecrets(context.Background(), string(application)).Execute()

	if err != nil {
		return err
	}

	var secret *qovery.Secret
	for _, mSecret := range secrets.GetResults() {
		if mSecret.Key == key {
			secret = &mSecret
			break
		}
	}

	if secret == nil {
		return nil
	}

	res, err := client.ApplicationSecretAPI.DeleteApplicationSecret(context.Background(), string(application), secret.Id).Execute()

	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("Received "+res.Status+" response while deleting a secret for application %s with key %s", string(application), key)
	}

	return nil
}

func AddSecret(application Id, key string, value string) error {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return err
	}

	client := GetQoveryClient(tokenType, token)

	_, res, err := client.ApplicationSecretAPI.CreateApplicationSecret(context.Background(), string(application)).SecretRequest(
		qovery.SecretRequest{Key: key, Value: &value},
	).Execute()

	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("Received "+res.Status+" response while adding an secret for application %s", string(application))
	}

	return nil
}

func SelectTokenInformation() (*TokenInformation, error) {
	organization, err := SelectOrganization()

	if err != nil {
		return nil, err
	}

	PrintlnInfo("Select Role")
	role, err := SelectRole(organization)
	if err != nil {
		return nil, err
	}

	fmt.Println("Choose a token name")
	promptName := promptui.Prompt{
		Label: "Token name",
	}
	name, err := promptName.Run()

	if err != nil {
		return nil, err
	}

	if len(strings.Trim(name, "")) == 0 {
		return nil, errors.New("token name must not be empty")
	}

	fmt.Println("Choose a token description")
	promptDescription := promptui.Prompt{
		Label: "Token description",
	}
	description, err := promptDescription.Run()

	if err != nil {
		return nil, err
	}

	return &TokenInformation{
		organization,
		role,
		name,
		description,
	}, nil
}

func FindStatus(statuses []qovery.Status, serviceId string) string {
	status := "Unknown"

	for _, s := range statuses {
		if serviceId == s.Id {
			return string(s.State)
		}
	}

	return status
}

func FindStatusTextWithColor(statuses []qovery.Status, serviceId string) string {
	status := "Unknown"

	for _, s := range statuses {
		if serviceId == s.Id {
			return GetStatusTextWithColor(s.State)
		}
	}

	return status
}

func GetEnvironmentStatus(statuses []qovery.EnvironmentStatus, serviceId string) string {
	status := "Unknown"

	for _, s := range statuses {
		if serviceId == s.Id {
			return string(s.State)
		}
	}

	return status
}

func GetEnvironmentStatusWithColor(statuses []qovery.EnvironmentStatus, serviceId string) string {
	status := "Unknown"

	for _, s := range statuses {
		if serviceId == s.Id {
			return GetStatusTextWithColor(s.State)
		}
	}

	return status
}

func GetStatusTextWithColor(s qovery.StateEnum) string {
	var state = string(s)
	var statusMsg string

	if s == qovery.STATEENUM_DEPLOYED || s == qovery.STATEENUM_RESTARTED {
		statusMsg = pterm.FgGreen.Sprintf("%s", state)
	} else if strings.HasSuffix(string(s), "ERROR") {
		statusMsg = pterm.FgRed.Sprintf("%s", state)
	} else if strings.HasSuffix(string(s), "ING") {
		statusMsg = pterm.FgLightBlue.Sprintf("%s", state)
	} else if strings.HasSuffix(string(s), "QUEUED") {
		statusMsg = pterm.FgLightYellow.Sprintf("%s", state)
	} else if s == qovery.STATEENUM_READY {
		statusMsg = pterm.FgYellow.Sprintf("%s", state)
	} else if s == qovery.STATEENUM_STOPPED {
		statusMsg = pterm.FgYellow.Sprintf("%s", state)
	} else {
		statusMsg = string(s)
	}

	return statusMsg
}

func GetClusterStatusTextWithColor(s qovery.ClusterStateEnum) string {
	var statusMsg string

	state := string(s)
	if s == qovery.CLUSTERSTATEENUM_DEPLOYED || s == qovery.CLUSTERSTATEENUM_RESTARTED {
		statusMsg = pterm.FgGreen.Sprintf("%s", state)
	} else if strings.HasSuffix(state, "ERROR") || s == qovery.CLUSTERSTATEENUM_INVALID_CREDENTIALS {
		statusMsg = pterm.FgRed.Sprintf("%s", state)
	} else if strings.HasSuffix(state, "ING") {
		statusMsg = pterm.FgLightBlue.Sprintf("%s", state)
	} else if strings.HasSuffix(state, "QUEUED") {
		statusMsg = pterm.FgLightYellow.Sprintf("%s", state)
	} else if s == qovery.CLUSTERSTATEENUM_READY {
		statusMsg = pterm.FgYellow.Sprintf("%s", state)
	} else if s == qovery.CLUSTERSTATEENUM_STOPPED {
		statusMsg = pterm.FgYellow.Sprintf("%s", state)
	} else {
		statusMsg = state
	}

	return statusMsg
}

func FindByOrganizationName(organizations []qovery.Organization, name string) *qovery.Organization {
	for _, o := range organizations {
		if o.Name == name {
			return &o
		}
	}

	return nil
}

func FindByProjectName(projects []qovery.Project, name string) *qovery.Project {
	for _, p := range projects {
		if p.Name == name {
			return &p
		}
	}

	return nil
}

func FindByEnvironmentName(environments []qovery.Environment, name string) *qovery.Environment {
	for _, e := range environments {
		if e.Name == name {
			return &e
		}
	}

	return nil
}

func FindByApplicationName(applications []qovery.Application, name string) *qovery.Application {
	for _, a := range applications {
		if a.Name == name {
			return &a
		}
	}

	return nil
}

func FindByClusterName(clusters []qovery.Cluster, name string) *qovery.Cluster {
	for _, c := range clusters {
		if c.Name == name {
			return &c
		}
	}

	return nil
}

func FindByContainerName(containers []qovery.ContainerResponse, name string) *qovery.ContainerResponse {
	for _, c := range containers {
		if c.Name == name {
			return &c
		}
	}

	return nil
}

func FindByJobName(jobs []qovery.JobResponse, name string) *qovery.JobResponse {
	for _, j := range jobs {
		if j.CronJobResponse != nil && j.CronJobResponse.Name == name {
			return &j
		}
		if j.LifecycleJobResponse != nil && j.LifecycleJobResponse.Name == name {
			return &j
		}
	}

	return nil
}

func FindByDatabaseName(databases []qovery.Database, name string) *qovery.Database {
	for _, d := range databases {
		if d.Name == name {
			return &d
		}
	}

	return nil
}

func FindByHelmName(helms []qovery.HelmResponse, name string) *qovery.HelmResponse {
	for _, h := range helms {
		if h.Name == name {
			return &h
		}
	}

	return nil
}

func FindByCustomDomainName(customDomains []qovery.CustomDomain, name string) *qovery.CustomDomain {
	for _, d := range customDomains {
		if d.Domain == name {
			return &d
		}
	}

	return nil
}

func WatchEnvironment(envId string, finalServiceState qovery.StateEnum, client *qovery.APIClient) {
	WatchEnvironmentWithOptions(envId, finalServiceState, client, false)
}

func WatchEnvironmentWithOptions(envId string, finalServiceState qovery.StateEnum, client *qovery.APIClient, displaySimpleText bool) {
	for {
		statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if err != nil {
			return
		}

		if displaySimpleText {
			// TODO make something more fancy here to display the status. Use UILIVE or something like that
			log.Println(GetStatusTextWithColor(statuses.Environment.LastDeploymentState))
		} else {
			countStatuses := countStatus(statuses.Applications, finalServiceState) +
				countStatus(statuses.Databases, finalServiceState) +
				countStatus(statuses.Jobs, finalServiceState) +
				countStatus(statuses.Containers, finalServiceState) +
				countStatus(statuses.Helms, finalServiceState)

			totalStatuses := len(statuses.Applications) + len(statuses.Databases) + len(statuses.Jobs) + len(statuses.Containers) + len(statuses.Helms)

			icon := "⏳"
			if countStatuses > 0 {
				icon = "✅"
			}

			// TODO make something more fancy here to display the status. Use UILIVE or something like that
			log.Println(GetStatusTextWithColor(statuses.Environment.LastDeploymentState) + " (" + strconv.Itoa(countStatuses) + "/" + strconv.Itoa(totalStatuses) + " services " + icon + " )")
		}

		if statuses.Environment.LastDeploymentState == qovery.STATEENUM_DEPLOYED ||
			statuses.Environment.LastDeploymentState == qovery.STATEENUM_RESTARTED ||
			statuses.Environment.LastDeploymentState == qovery.STATEENUM_DELETED ||
			statuses.Environment.LastDeploymentState == qovery.STATEENUM_STOPPED ||
			statuses.Environment.LastDeploymentState == qovery.STATEENUM_CANCELED {
			return
		}

		if strings.HasSuffix(string(statuses.Environment.LastDeploymentState), "ERROR") {
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		time.Sleep(3 * time.Second)
	}
}

func WatchContainer(containerId string, envId string, client *qovery.APIClient) {
out:
	for {
		status, _, err := client.ContainerMainCallsAPI.GetContainerStatus(context.Background(), containerId).Execute()

		if err != nil {
			break
		}

		switch WatchStatus(status) {
		case Continue:
		case Stop:
			break out
		case Err:
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		time.Sleep(3 * time.Second)
	}

	log.Println("Check environment status..")

	// check status of environment
	WatchEnvironmentWithOptions(envId, "unused", client, true)
}

func WatchApplication(applicationId string, envId string, client *qovery.APIClient) {
out:
	for {
		status, _, err := client.ApplicationMainCallsAPI.GetApplicationStatus(context.Background(), applicationId).Execute()

		if err != nil {
			break
		}

		switch WatchStatus(status) {
		case Continue:
		case Stop:
			break out
		case Err:
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		time.Sleep(3 * time.Second)
	}

	log.Println("Check environment status..")

	// check status of environment
	WatchEnvironmentWithOptions(envId, "unused", client, true)
}

func WatchDatabase(databaseId string, envId string, client *qovery.APIClient) {
out:
	for {
		status, _, err := client.DatabaseMainCallsAPI.GetDatabaseStatus(context.Background(), databaseId).Execute()

		if err != nil {
			break
		}

		switch WatchStatus(status) {
		case Continue:
		case Stop:
			break out
		case Err:
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		time.Sleep(3 * time.Second)
	}

	log.Println("Check environment status..")

	// check status of environment
	WatchEnvironmentWithOptions(envId, "unused", client, true)
}

func WatchJob(jobId string, envId string, client *qovery.APIClient) {
out:
	for {
		status, _, err := client.JobMainCallsAPI.GetJobStatus(context.Background(), jobId).Execute()

		if err != nil {
			break
		}

		switch WatchStatus(status) {
		case Continue:
		case Stop:
			break out
		case Err:
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		time.Sleep(3 * time.Second)
	}

	log.Println("Check environment status..")

	// check status of environment
	WatchEnvironmentWithOptions(envId, "unused", client, true)
}

func WatchHelm(helmId string, envId string, client *qovery.APIClient) {
out:
	for {
		status, _, err := client.HelmMainCallsAPI.GetHelmStatus(context.Background(), helmId).Execute()

		if err != nil {
			break
		}

		switch WatchStatus(status) {
		case Continue:
		case Stop:
			break out
		case Err:
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		time.Sleep(3 * time.Second)
	}

	log.Println("Check environment status..")

	// check status of environment
	WatchEnvironmentWithOptions(envId, "unused", client, true)
}

type Status int8

const (
	Continue Status = iota
	Stop
	Err
)

func WatchStatus(status *qovery.Status) Status {
	// TODO make something more fancy here to display the status. Use UILIVE or something like that
	log.Println(GetStatusTextWithColor(status.State))

	if status.State == qovery.STATEENUM_DEPLOYED || status.State == qovery.STATEENUM_DELETED ||
		status.State == qovery.STATEENUM_STOPPED || status.State == qovery.STATEENUM_CANCELED ||
		status.State == qovery.STATEENUM_RESTARTED {
		return Stop
	}

	if strings.HasSuffix(string(status.State), "ERROR") {
		return Err
	}

	return Continue
}

func countStatus(statuses []qovery.Status, state qovery.StateEnum) int {
	count := 0

	for _, s := range statuses {
		if s.State == state {
			count++
		}
	}

	return count
}

func GetServiceNameByIdAndType(client *qovery.APIClient, serviceId string, serviceType string) string {
	switch serviceType {
	case "APPLICATION":
		application, _, err := client.ApplicationMainCallsAPI.GetApplication(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return application.GetName()
	case "DATABASE":
		database, _, err := client.DatabaseMainCallsAPI.GetDatabase(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return database.GetName()
	case "CONTAINER":
		container, _, err := client.ContainerMainCallsAPI.GetContainer(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return container.GetName()
	case "JOB":
		job, _, err := client.JobMainCallsAPI.GetJob(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return GetJobName(job)
	case "HELM":
		helm, _, err := client.HelmMainCallsAPI.GetHelm(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return helm.GetName()
	default:
		return "Unknown"
	}
}

func GetDeploymentStageId(client *qovery.APIClient, serviceId string) string {
	sourceDeploymentStage, _, err := client.DeploymentStageMainCallsAPI.GetServiceDeploymentStage(context.Background(), serviceId).Execute()

	if err != nil {
		PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return sourceDeploymentStage.Id
}

func DeployApplications(client *qovery.APIClient, envId string, applicationList []*qovery.Application, commitId string) error {
	if len(applicationList) == 0 {
		return nil
	}

	var applicationsToDeploy []qovery.DeployAllRequestApplicationsInner

	for _, application := range applicationList {

		// if commitId is not set, use the deployed commit id
		applicationCommitId := application.GitRepository.DeployedCommitId
		if commitId != "" {
			// commitId is set, use it
			applicationCommitId = &commitId
		}

		applicationsToDeploy = append(applicationsToDeploy, qovery.DeployAllRequestApplicationsInner{
			ApplicationId: application.Id,
			GitCommitId:   applicationCommitId,
		})
	}

	req := qovery.DeployAllRequest{
		Applications: applicationsToDeploy,
		Databases:    nil,
		Containers:   nil,
		Jobs:         nil,
	}

	return deployAllServices(client, envId, req)
}

func DeployContainers(client *qovery.APIClient, envId string, containerList []*qovery.ContainerResponse, tag string) error {
	if len(containerList) == 0 {
		return nil
	}

	var containersToDeploy []qovery.DeployAllRequestContainersInner
	for _, container := range containerList {

		// if tag is not set, use the deployed commit id
		containerTag := container.Tag
		if tag != "" {
			// tag is set, use it
			containerTag = tag
		}

		containersToDeploy = append(containersToDeploy, qovery.DeployAllRequestContainersInner{
			Id:       container.Id,
			ImageTag: &containerTag,
		})
	}

	req := qovery.DeployAllRequest{
		Applications: nil,
		Databases:    nil,
		Containers:   containersToDeploy,
		Jobs:         nil,
	}

	return deployAllServices(client, envId, req)
}

func DeployJobs(client *qovery.APIClient, envId string, jobList []*qovery.JobResponse, commitId string, tag string) error {
	if len(jobList) == 0 {
		return nil
	}

	var jobsToDeploy []qovery.DeployAllRequestJobsInner

	for _, job := range jobList {

		var docker = GetJobDocker(job)
		var image = GetJobImage(job)

		var mCommitId *string
		var mTag *string

		if docker != nil {
			mCommitId = docker.GitRepository.DeployedCommitId
			if commitId != "" {
				mCommitId = &commitId
			}

		} else {
			mTag = &image.Tag

			if tag != "" {
				mTag = &tag
			}
		}

		var jobId = GetJobId(job)
		jobsToDeploy = append(jobsToDeploy, qovery.DeployAllRequestJobsInner{
			Id:          &jobId,
			ImageTag:    mTag,
			GitCommitId: mCommitId,
		})
	}

	req := qovery.DeployAllRequest{
		Applications: nil,
		Databases:    nil,
		Containers:   nil,
		Jobs:         jobsToDeploy,
	}

	return deployAllServices(client, envId, req)
}

func GetJobDocker(job *qovery.JobResponse) *qovery.JobSourceDockerResponse {
	if job.CronJobResponse != nil && job.CronJobResponse.Source.BaseJobResponseAllOfSourceOneOf1 != nil {
		return &job.CronJobResponse.Source.BaseJobResponseAllOfSourceOneOf1.Docker
	}

	if job.LifecycleJobResponse != nil && job.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf1 != nil {
		return &job.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf1.Docker
	}

	return nil
}

func GetJobImage(job *qovery.JobResponse) *qovery.ContainerSource {
	if job.CronJobResponse != nil && job.CronJobResponse.Source.BaseJobResponseAllOfSourceOneOf != nil {
		return &job.CronJobResponse.Source.BaseJobResponseAllOfSourceOneOf.Image
	}

	if job.LifecycleJobResponse != nil && job.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf != nil {
		return &job.LifecycleJobResponse.Source.BaseJobResponseAllOfSourceOneOf.Image
	}

	return nil
}

func GetJobId(job *qovery.JobResponse) string {
	if job.CronJobResponse != nil {
		return job.CronJobResponse.Id
	}
	if job.LifecycleJobResponse != nil {
		return job.LifecycleJobResponse.Id
	}
	return ""
}

func GetJobName(job *qovery.JobResponse) string {
	if job.CronJobResponse != nil {
		return job.CronJobResponse.Name
	}
	if job.LifecycleJobResponse != nil {
		return job.LifecycleJobResponse.Name
	}
	return ""
}

func DeployDatabases(client *qovery.APIClient, envId string, databaseList []*qovery.Database) error {
	if len(databaseList) == 0 {
		return nil
	}

	var databasesToDeploy []string

	for _, database := range databaseList {
		databasesToDeploy = append(databasesToDeploy, database.Id)
	}

	req := qovery.DeployAllRequest{
		Applications: nil,
		Containers:   nil,
		Databases:    databasesToDeploy,
		Jobs:         nil,
	}

	return deployAllServices(client, envId, req)
}

func DeployHelms(client *qovery.APIClient, envId string, helmList []*qovery.HelmResponse, chartVersion string, chartGitCommitId string, valuesOverrideCommitId string) error {
	if len(helmList) == 0 {
		return nil
	}

	var helmsToDeploy []qovery.DeployAllRequestHelmsInner

	for _, helm := range helmList {

		var gitSource = GetGitSource(helm)
		var helmRepositorySource = GetHelmRepository(helm)

		if gitSource != nil && helmRepositorySource != nil {
			return fmt.Errorf("invalid helm")
		}

		var mCommitId *string
		var mChartVersion *string
		var mValuesOverrideCommitId *string

		if gitSource != nil {
			if chartGitCommitId != "" {
				mCommitId = &chartGitCommitId
			}
		}

		if helmRepositorySource != nil {
			if chartVersion != "" {
				mChartVersion = &chartVersion
			}
		}

		if valuesOverrideCommitId != "" {
			mValuesOverrideCommitId = &valuesOverrideCommitId
		}

		helmsToDeploy = append(helmsToDeploy, qovery.DeployAllRequestHelmsInner{
			Id:                        &helm.Id,
			ChartVersion:              mChartVersion,
			GitCommitId:               mCommitId,
			ValuesOverrideGitCommitId: mValuesOverrideCommitId,
		})
	}

	req := qovery.DeployAllRequest{
		Applications: nil,
		Databases:    nil,
		Containers:   nil,
		Jobs:         nil,
		Helms:        helmsToDeploy,
	}

	return deployAllServices(client, envId, req)
}

func GetGitSource(helm *qovery.HelmResponse) *qovery.HelmSourceGitResponse {
	if helm.Source.HelmResponseAllOfSourceOneOf != nil {
		return &helm.Source.HelmResponseAllOfSourceOneOf.Git
	}

	return nil
}

func GetHelmRepository(helm *qovery.HelmResponse) *qovery.HelmSourceRepositoryResponse {
	if helm.Source.HelmResponseAllOfSourceOneOf1 != nil {
		return &helm.Source.HelmResponseAllOfSourceOneOf1.Repository
	}

	return nil
}

func deployAllServices(client *qovery.APIClient, envId string, req qovery.DeployAllRequest) error {
	_, _, err := client.EnvironmentActionsAPI.DeployAllServices(context.Background(), envId).DeployAllRequest(req).Execute()
	if err != nil {
		return err
	}

	return nil
}

func CancelEnvironmentDeployment(client *qovery.APIClient, envId string, watchFlag bool) error {
	_, _, err := client.EnvironmentActionsAPI.CancelEnvironmentDeployment(context.Background(), envId).Execute()

	if err != nil {
		return err
	}

	if watchFlag {
		WatchEnvironmentWithOptions(envId, qovery.STATEENUM_CANCELED, client, true)
	}

	return nil
}

func IsTerminalState(state qovery.StateEnum) bool {
	return state == qovery.STATEENUM_DEPLOYED || state == qovery.STATEENUM_DELETED ||
		state == qovery.STATEENUM_STOPPED || state == qovery.STATEENUM_CANCELED ||
		state == qovery.STATEENUM_READY || state == qovery.STATEENUM_RESTARTED ||
		strings.HasSuffix(string(state), "ERROR")
}

func IsTerminalClusterState(state qovery.ClusterStateEnum) bool {
	return state == qovery.CLUSTERSTATEENUM_DEPLOYED || state == qovery.CLUSTERSTATEENUM_DELETED ||
		state == qovery.CLUSTERSTATEENUM_STOPPED || state == qovery.CLUSTERSTATEENUM_CANCELED ||
		state == qovery.CLUSTERSTATEENUM_READY || state == qovery.CLUSTERSTATEENUM_RESTARTED ||
		state == qovery.CLUSTERSTATEENUM_INVALID_CREDENTIALS || strings.HasSuffix(string(state), "ERROR")
}

func CancelServiceDeployment(client *qovery.APIClient, envId string, serviceId string, serviceType ServiceType, watchFlag bool) (string, error) {
	statuses, _, err := client.EnvironmentMainCallsAPI.GetEnvironmentStatuses(context.Background(), envId).Execute()

	if err != nil {
		return "", err
	}

	envStatus := statuses.GetEnvironment()

	if IsTerminalState(envStatus.State) {
		// if the environment is in a terminal state, there is nothing to cancel
		return "there is no deployment in progress. Nothing to cancel", nil
	}

	// cancel deployment if the targeted service is a non-terminal state
	switch serviceType {
	case ApplicationType:
		for _, application := range statuses.GetApplications() {
			if application.Id == serviceId && !IsTerminalState(application.State) {
				err := CancelEnvironmentDeployment(client, envId, watchFlag)
				if err != nil {
					return "", err
				}

				return "", nil
			}
		}
	case DatabaseType:
		for _, database := range statuses.GetDatabases() {
			if database.Id == serviceId && !IsTerminalState(database.State) {
				err := CancelEnvironmentDeployment(client, envId, watchFlag)
				if err != nil {
					return "", err
				}

				return "", nil
			}
		}
	case ContainerType:
		for _, container := range statuses.GetContainers() {
			if container.Id == serviceId && !IsTerminalState(container.State) {
				err := CancelEnvironmentDeployment(client, envId, watchFlag)
				if err != nil {
					return "", err
				}

				return "", nil
			}
		}
	case JobType:
		for _, job := range statuses.GetJobs() {
			if job.Id == serviceId && !IsTerminalState(job.State) {
				err := CancelEnvironmentDeployment(client, envId, watchFlag)
				if err != nil {
					return "", err
				}

				return "", nil
			}
		}
	case HelmType:
		for _, helm := range statuses.GetHelms() {
			if helm.Id == serviceId && !IsTerminalState(helm.State) {
				err := CancelEnvironmentDeployment(client, envId, watchFlag)
				if err != nil {
					return "", err
				}

				return "", nil
			}
		}
	}

	PrintlnInfo("waiting for previous deployment to be completed...")

	// sleep here to avoid too many requests
	time.Sleep(5 * time.Second)

	return CancelServiceDeployment(client, envId, serviceId, serviceType, watchFlag)
}

func ToJobRequest(job qovery.JobResponse) qovery.JobRequest {
	var docker = GetJobDocker(&job)
	var image = GetJobImage(&job)

	var sourceImage qovery.JobRequestAllOfSourceImage

	if image != nil {
		sourceImage = qovery.JobRequestAllOfSourceImage{
			ImageName:  &image.ImageName,
			Tag:        &image.Tag,
			RegistryId: image.RegistryId,
		}
	}

	var sourceDocker qovery.JobRequestAllOfSourceDocker

	if docker != nil {
		sourceDockerGitRepository := qovery.ApplicationGitRepositoryRequest{
			Branch:     docker.GitRepository.Branch,
			GitTokenId: docker.GitRepository.GitTokenId,
			RootPath:   docker.GitRepository.RootPath,
			Url:        docker.GitRepository.Url,
		}

		sourceDocker = qovery.JobRequestAllOfSourceDocker{
			DockerfilePath: docker.DockerfilePath,
			GitRepository:  &sourceDockerGitRepository,
		}
	}

	source := qovery.JobRequestAllOfSource{
		Image:  qovery.NullableJobRequestAllOfSourceImage{},
		Docker: qovery.NullableJobRequestAllOfSourceDocker{},
	}

	source.Image.Set(&sourceImage)
	source.Docker.Set(&sourceDocker)

	if job.LifecycleJobResponse != nil {
		var schedule = qovery.JobRequestAllOfSchedule{
			OnStart:       job.LifecycleJobResponse.Schedule.OnStart,
			OnStop:        job.LifecycleJobResponse.Schedule.OnStop,
			OnDelete:      job.LifecycleJobResponse.Schedule.OnDelete,
			LifecycleType: job.LifecycleJobResponse.Schedule.LifecycleType,
			Cronjob:       nil,
		}

		return qovery.JobRequest{
			Name:               job.LifecycleJobResponse.Name,
			Description:        job.LifecycleJobResponse.Description,
			Cpu:                Int32(job.LifecycleJobResponse.Cpu),
			Memory:             Int32(job.LifecycleJobResponse.Memory),
			MaxNbRestart:       job.LifecycleJobResponse.MaxNbRestart,
			MaxDurationSeconds: job.LifecycleJobResponse.MaxDurationSeconds,
			AutoPreview:        Bool(job.LifecycleJobResponse.AutoPreview),
			Port:               job.LifecycleJobResponse.Port,
			Source:             &source,
			Healthchecks:       job.LifecycleJobResponse.Healthchecks,
			Schedule:           &schedule,
			AutoDeploy:         *qovery.NewNullableBool(job.LifecycleJobResponse.AutoDeploy),
		}
	} else {
		var scheduleCronjob = qovery.JobRequestAllOfScheduleCronjob{
			Entrypoint:  job.CronJobResponse.Schedule.Cronjob.Entrypoint,
			Arguments:   job.CronJobResponse.Schedule.Cronjob.Arguments,
			ScheduledAt: job.CronJobResponse.Schedule.Cronjob.ScheduledAt,
		}

		var schedule = qovery.JobRequestAllOfSchedule{
			OnStart:       nil,
			OnStop:        nil,
			OnDelete:      nil,
			LifecycleType: nil,
			Cronjob:       &scheduleCronjob,
		}

		return qovery.JobRequest{
			Name:               job.CronJobResponse.Name,
			Description:        job.CronJobResponse.Description,
			Cpu:                Int32(job.CronJobResponse.Cpu),
			Memory:             Int32(job.CronJobResponse.Memory),
			MaxNbRestart:       job.CronJobResponse.MaxNbRestart,
			MaxDurationSeconds: job.CronJobResponse.MaxDurationSeconds,
			AutoPreview:        Bool(job.CronJobResponse.AutoPreview),
			Port:               job.CronJobResponse.Port,
			Source:             &source,
			Healthchecks:       job.CronJobResponse.Healthchecks,
			Schedule:           &schedule,
			AutoDeploy:         *qovery.NewNullableBool(job.CronJobResponse.AutoDeploy),
		}
	}
}

func GetDuration(startTime time.Time, endTime time.Time) string {
	duration := endTime.Sub(startTime)

	if duration.Minutes() < 1 {
		return fmt.Sprintf("%d seconds", int(duration.Seconds()))
	}

	if duration.Minutes() < 2 {
		return fmt.Sprintf("%d minute and %d seconds", int(duration.Minutes()), int(duration.Seconds())%60)
	}

	if duration.Minutes() > 0 && duration.Seconds() == 0 {
		return fmt.Sprintf("%d minutes", int(duration.Minutes()))
	}

	return fmt.Sprintf("%d minutes and %d seconds", int(duration.Minutes()), int(duration.Seconds())%60)
}

// ShowHelpIfNoArgs shows help and exits if no arguments are provided
func ShowHelpIfNoArgs(cmd *cobra.Command, args []string) {
	if len(args) == 0 {
		_ = cmd.Help()
		os.Exit(0)
	}
}

// ShowHelpIfNoRequiredFlags shows help if required flags are not provided
func ShowHelpIfNoRequiredFlags(cmd *cobra.Command) {
	// Check if any required flags are missing by checking if they were set
	missingFlags := []string{}

	// Check application flag
	if cmd.Flags().Lookup("application") != nil && !cmd.Flags().Lookup("application").Changed {
		missingFlags = append(missingFlags, "application")
	}

	// Check key flag
	if cmd.Flags().Lookup("key") != nil && !cmd.Flags().Lookup("key").Changed {
		missingFlags = append(missingFlags, "key")
	}

	// Check other common required flags
	if cmd.Flags().Lookup("cluster") != nil && !cmd.Flags().Lookup("cluster").Changed {
		missingFlags = append(missingFlags, "cluster")
	}

	if cmd.Flags().Lookup("database") != nil && !cmd.Flags().Lookup("database").Changed {
		missingFlags = append(missingFlags, "database")
	}

	if cmd.Flags().Lookup("container") != nil && !cmd.Flags().Lookup("container").Changed {
		missingFlags = append(missingFlags, "container")
	}

	if len(missingFlags) > 0 {
		_ = cmd.Help()
		os.Exit(0)
	}
}

// ShowHelpIfNoArgsOrRequiredFlags shows help if no arguments or required flags are provided
func ShowHelpIfNoArgsOrRequiredFlags(cmd *cobra.Command, args []string) {
	// First check if no arguments are provided
	if len(args) == 0 {
		_ = cmd.Help()
		os.Exit(0)
	}

	// Then check if required flags are missing
	ShowHelpIfNoRequiredFlags(cmd)
}

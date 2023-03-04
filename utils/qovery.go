package utils

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/pterm/pterm"

	"github.com/manifoldco/promptui"
	"github.com/qovery/qovery-client-go"
	log "github.com/sirupsen/logrus"
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
	Name         string
	Description  string
}

const AdminUrl = "https://api-admin.qovery.com"

func GetQoveryClient(tokenType AccessTokenType, token AccessToken) *qovery.APIClient {
	conf := qovery.NewConfiguration()
	conf.UserAgent = "Qovery CLI"
	conf.DefaultHeader["Authorization"] = GetAuthorizationHeaderValue(tokenType, token)
	return qovery.NewAPIClient(conf)
}

func SelectOrganization() (*Organization, error) {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	client := GetQoveryClient(tokenType, token)

	organizations, res, err := client.OrganizationMainCallsApi.ListOrganization(context.Background()).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing organizations. ")
	}

	var organizationNames []string
	var orgas = make(map[string]string)

	for _, org := range organizations.GetResults() {
		organizationNames = append(organizationNames, org.Name)
		orgas[org.Name] = org.Id
	}

	if len(organizationNames) < 1 {
		return nil, errors.New("No organizations found. ")
	}

	fmt.Println("Organization:")
	prompt := promptui.Select{
		Items: organizationNames,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(organizationNames[index]), strings.ToLower(input))
		},
	}
	_, selectedOrganization, err := prompt.Run()
	if err != nil {
		return nil, err
	}

	return &Organization{
		ID:   Id(orgas[selectedOrganization]),
		Name: Name(selectedOrganization),
	}, nil
}

func SelectAndSetOrganization() (*Organization, error) {
	selectedOrganization, err := SelectOrganization()
	if err != nil {
		PrintlnError(err)
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

	organization, res, err := client.OrganizationMainCallsApi.GetOrganization(context.Background(), id).Execute()
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

	p, res, err := client.ProjectsApi.ListProject(context.Background(), string(organizationID)).Execute()
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
		return nil, errors.New("No projects found. ")
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

	project, res, err := client.ProjectMainCallsApi.GetProject(context.Background(), id).Execute()
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

	e, res, err := client.EnvironmentsApi.ListEnvironment(context.Background(), string(projectID)).Execute()
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
		return nil, errors.New("No environments found. ")
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
		PrintlnError(err)
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

	environment, res, err := client.EnvironmentMainCallsApi.GetEnvironment(context.Background(), id).Execute()
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

	environmentServices, res, err := client.EnvironmentMainCallsApi.GetEnvironmentStatuses(context.Background(), id).Execute()
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

	return services, nil
}

type ServiceType string

const (
	ApplicationType ServiceType = "application"
	ContainerType   ServiceType = "container"
	DatabaseType    ServiceType = "database"
	JobType         ServiceType = "job"
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

	apps, res, err := client.ApplicationsApi.ListApplication(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing services. ")
	}

	containers, res, err := client.ContainersApi.ListContainer(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing containers. ")
	}

	databases, res, err := client.DatabasesApi.ListDatabase(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing containers. ")
	}

	jobs, res, err := client.JobsApi.ListJobs(context.Background(), string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing containers. ")
	}

	var servicesNames []string
	var services = make(map[string]Service)

	for _, app := range apps.GetResults() {
		servicesNames = append(servicesNames, *app.Name)
		services[*app.Name] = Service{
			ID:   Id(app.Id),
			Name: Name(*app.Name),
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
		servicesNames = append(servicesNames, job.Name)
		services[job.Name] = Service{
			ID:   Id(job.Id),
			Name: Name(job.Name),
			Type: JobType,
		}
	}

	if len(servicesNames) < 1 {
		return nil, errors.New("No services found. ")
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

	application, res, err := client.ApplicationMainCallsApi.GetApplication(context.Background(), id).Execute()
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
	ctx, err := CurrentContext()
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

	container, res, err := client.ContainerMainCallsApi.GetContainer(context.Background(), id).Execute()
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

	job, res, err := client.JobMainCallsApi.GetJob(context.Background(), id).Execute()
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while getting job " + id)
	}
	if err != nil {
		return nil, err
	}

	return &Job{
		ID:   Id(job.Id),
		Name: Name(job.GetName()),
	}, nil
}

func CheckAdminUrl() {
	if _, ok := os.LookupEnv("ADMIN_URL"); !ok {
		log.Error("You must set the Qovery admin root url (ADMIN_URL).")
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}
}

func DeleteEnvironmentVariable(application Id, key string) error {
	tokenType, token, err := GetAccessToken()
	if err != nil {
		return err
	}

	client := GetQoveryClient(tokenType, token)

	// TODO optimize this call by caching the result?
	envVars, _, err := client.ApplicationEnvironmentVariableApi.ListApplicationEnvironmentVariable(context.Background(), string(application)).Execute()

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

	res, err := client.ApplicationEnvironmentVariableApi.DeleteApplicationEnvironmentVariable(context.Background(), string(application), envVar.Id).Execute()

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

	_, res, err := client.ApplicationEnvironmentVariableApi.CreateApplicationEnvironmentVariable(context.Background(), string(application)).EnvironmentVariableRequest(
		qovery.EnvironmentVariableRequest{Key: key, Value: value},
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
	secrets, _, err := client.ApplicationSecretApi.ListApplicationSecrets(context.Background(), string(application)).Execute()

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

	res, err := client.ApplicationSecretApi.DeleteApplicationSecret(context.Background(), string(application), secret.Id).Execute()

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

	_, res, err := client.ApplicationSecretApi.CreateApplicationSecret(context.Background(), string(application)).SecretRequest(
		qovery.SecretRequest{Key: key, Value: value},
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

	fmt.Println("Choose a token name")
	promptName := promptui.Prompt{
		Label: "Token name",
	}
	name, err := promptName.Run()

	if err != nil {
		return nil, err
	}

	if len(strings.Trim(name, "")) == 0 {
		return nil, errors.New("Token name must not be empty")
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
		name,
		description,
	}, nil
}

func GetStatus(statuses []qovery.Status, serviceId string) string {
	status := "Unknown"

	for _, s := range statuses {
		if serviceId == s.Id {
			return GetStatusTextWithColor(s)
		}
	}

	return status
}

func GetStatusTextWithColor(s qovery.Status) string {
	var statusMsg string

	if s.State == qovery.STATEENUM_RUNNING {
		statusMsg = pterm.FgGreen.Sprintf(string(s.State))
	} else if strings.HasSuffix(string(s.State), "ERROR") {
		statusMsg = pterm.FgRed.Sprintf(string(s.State))
	} else if strings.HasSuffix(string(s.State), "ING") {
		statusMsg = pterm.FgLightBlue.Sprintf(string(s.State))
	} else if strings.HasSuffix(string(s.State), "QUEUED") {
		statusMsg = pterm.FgLightYellow.Sprintf(string(s.State))
	} else if s.State == qovery.STATEENUM_READY {
		statusMsg = pterm.FgYellow.Sprintf(string(s.State))
	} else {
		statusMsg = string(s.State)
	}

	if s.Message != nil && *s.Message != "" {
		statusMsg += " (" + *s.Message + ")"
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
		if *a.Name == name {
			return &a
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
		if j.Name == name {
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

func WatchEnvironment(envId string, finalServiceState qovery.StateEnum, client *qovery.APIClient) {
	WatchEnvironmentWithOptions(envId, finalServiceState, client, false)
}

func WatchEnvironmentWithOptions(envId string, finalServiceState qovery.StateEnum, client *qovery.APIClient, displaySimpleText bool) {
	for {
		status, _, err := client.EnvironmentMainCallsApi.GetEnvironmentStatus(context.Background(), envId).Execute()

		if err != nil {
			return
		}

		statuses, _, _ := client.EnvironmentMainCallsApi.GetEnvironmentStatuses(context.Background(), envId).Execute()

		if displaySimpleText {
			// TODO make something more fancy here to display the status. Use UILIVE or something like that
			log.Println(GetStatusTextWithColor(*status))
		} else {
			countStatuses := countStatus(statuses.Applications, finalServiceState) + countStatus(statuses.Databases, finalServiceState) +
				countStatus(statuses.Jobs, finalServiceState) + countStatus(statuses.Containers, finalServiceState)

			totalStatuses := len(statuses.Applications) + len(statuses.Databases) + len(statuses.Jobs) + len(statuses.Containers)

			icon := "⏳"
			if countStatuses > 0 {
				icon = "✅"
			}

			// TODO make something more fancy here to display the status. Use UILIVE or something like that
			log.Println(GetStatusTextWithColor(*status) + " (" + strconv.Itoa(countStatuses) + "/" + strconv.Itoa(totalStatuses) + " services " + icon + " )")
		}

		if status.State == qovery.STATEENUM_RUNNING || status.State == qovery.STATEENUM_DELETED ||
			status.State == qovery.STATEENUM_STOPPED || status.State == qovery.STATEENUM_CANCELED {
			return
		}

		if strings.HasSuffix(string(status.State), "ERROR") {
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		time.Sleep(3 * time.Second)
	}
}

func WatchContainer(containerId string, envId string, client *qovery.APIClient) {
out:
	for {
		status, _, err := client.ContainerMainCallsApi.GetContainerStatus(context.Background(), containerId).Execute()

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
		status, _, err := client.ApplicationMainCallsApi.GetApplicationStatus(context.Background(), applicationId).Execute()

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
		status, _, err := client.DatabaseMainCallsApi.GetDatabaseStatus(context.Background(), databaseId).Execute()

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
		status, _, err := client.JobMainCallsApi.GetJobStatus(context.Background(), jobId).Execute()

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
	log.Println(GetStatusTextWithColor(*status))

	if status.State == qovery.STATEENUM_RUNNING || status.State == qovery.STATEENUM_DELETED ||
		status.State == qovery.STATEENUM_STOPPED || status.State == qovery.STATEENUM_CANCELED {
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

func IsEnvironmentInATerminalState(envId string, client *qovery.APIClient) bool {
	status, _, err := client.EnvironmentMainCallsApi.GetEnvironmentStatus(context.Background(), envId).Execute()

	if err != nil {
		return false
	}

	return status.State == qovery.STATEENUM_RUNNING || status.State == qovery.STATEENUM_DELETED ||
		status.State == qovery.STATEENUM_STOPPED || status.State == qovery.STATEENUM_CANCELED ||
		status.State == qovery.STATEENUM_READY || strings.HasSuffix(string(status.State), "ERROR")
}

func GetServiceNameByIdAndType(client *qovery.APIClient, serviceId string, serviceType string) string {
	switch serviceType {
	case "APPLICATION":
		application, _, err := client.ApplicationMainCallsApi.GetApplication(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return application.GetName()
	case "DATABASE":
		database, _, err := client.DatabaseMainCallsApi.GetDatabase(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return database.GetName()
	case "CONTAINER":
		container, _, err := client.ContainerMainCallsApi.GetContainer(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return container.GetName()
	case "JOB":
		job, _, err := client.JobMainCallsApi.GetJob(context.Background(), serviceId).Execute()
		if err != nil {
			return ""
		}
		return job.GetName()
	default:
		return "Unknown"
	}
}

func GetDeploymentStageId(client *qovery.APIClient, serviceId string) string {
	sourceDeploymentStage, _, err := client.DeploymentStageMainCallsApi.GetServiceDeploymentStage(context.Background(), serviceId).Execute()

	if err != nil {
		PrintlnError(err)
		os.Exit(1)
		panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
	}

	return sourceDeploymentStage.Id
}

package utils

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/qovery/qovery-client-go"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"
)

type Organization struct {
	ID   Id
	Name Name
}

func SelectOrganization() (*Organization, error) {
	token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	organizations, res, err := client.OrganizationMainCallsApi.ListOrganization(auth).Execute()
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

func SelectProject(organizationID Id) (*Project, error) {
	token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	p, res, err := client.ProjectsApi.ListProject(auth, string(organizationID)).Execute()
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

func SelectEnvironment(projectID Id) (*Environment, error) {
	token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	e, res, err := client.EnvironmentsApi.ListEnvironment(auth, string(projectID)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing environments. ")
	}

	var environmentsNames []string
	var environments = make(map[string]qovery.EnvironmentResponse)

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

type Application struct {
	ID   Id
	Name Name
}

func SelectApplication(environment Id) (*Application, error) {
	token, err := GetAccessToken()
	if err != nil {
		return nil, err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	a, res, err := client.ApplicationsApi.ListApplication(auth, string(environment)).Execute()
	if err != nil {
		return nil, err
	}
	if res.StatusCode >= 400 {
		return nil, errors.New("Received " + res.Status + " response while listing applications. ")
	}

	var applicationsNames []string
	var applications = make(map[string]string)

	for _, app := range a.GetResults() {
		applicationsNames = append(applicationsNames, *app.Name)
		applications[*app.Name] = app.Id
	}

	if len(applicationsNames) < 1 {
		return nil, errors.New("No applications found. ")
	}

	fmt.Println("Application:")
	prompt := promptui.Select{
		Items: applicationsNames,
		Searcher: func(input string, index int) bool {
			return strings.Contains(strings.ToLower(applicationsNames[index]), strings.ToLower(input))
		},
	}
	_, selectedApplication, err := prompt.Run()
	if err != nil {
		PrintlnError(err)
		return nil, err
	}

	return &Application{
		ID:   Id(applications[selectedApplication]),
		Name: Name(selectedApplication),
	}, nil
}

func SelectAndSetApplication(environment Id) (*Application, error) {
	application, err := SelectApplication(environment)
	if err != nil {
		PrintlnError(err)
		return nil, err
	}
	return application, err
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
	ctx.ApplicationName = ""
	ctx.ApplicationId = ""

	err = StoreContext(ctx)

	return err
}

func CheckAdminUrl() {
	if _, ok := os.LookupEnv("ADMIN_URL"); !ok {
		log.Error("You must set the Qovery admin root url (ADMIN_URL).")
		os.Exit(1)
	}
}

func DeleteEnvironmentVariable(application Id, key string) error {
	token, err := GetAccessToken()
	if err != nil {
		return err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	// TODO optimize this call by caching the result?
	envVars, _, err := client.ApplicationEnvironmentVariableApi.ListApplicationEnvironmentVariable(auth, string(application)).Execute()

	if err != nil {
		return err
	}

	var envVar *qovery.EnvironmentVariableResponse
	for _, mEnvVar := range envVars.GetResults() {
		if mEnvVar.Key == key {
			envVar = &mEnvVar
			break
		}
	}

	if envVar == nil {
		return nil
	}

	res, err := client.ApplicationEnvironmentVariableApi.DeleteApplicationEnvironmentVariable(auth, string(application), envVar.Id).Execute()

	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("Received "+res.Status+" response while deleting an Environment Variable for application %s with key %s", string(application), key)
	}

	return nil
}

func AddEnvironmentVariable(application Id, key string, value string) error {
	token, err := GetAccessToken()
	if err != nil {
		return err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	_, res, err := client.ApplicationEnvironmentVariableApi.CreateApplicationEnvironmentVariable(auth, string(application)).EnvironmentVariableRequest(
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
	token, err := GetAccessToken()
	if err != nil {
		return err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	// TODO optimize this call by caching the result?
	secrets, _, err := client.ApplicationSecretApi.ListApplicationSecrets(auth, string(application)).Execute()

	if err != nil {
		return err
	}

	var secret *qovery.SecretResponse
	for _, mSecret := range secrets.GetResults() {
		if *mSecret.Key == key {
			secret = &mSecret
			break
		}
	}

	if secret == nil {
		return nil
	}

	res, err := client.ApplicationSecretApi.DeleteApplicationSecret(auth, string(application), secret.Id).Execute()

	if err != nil {
		return err
	}

	if res.StatusCode >= 400 {
		return fmt.Errorf("Received "+res.Status+" response while deleting a secret for application %s with key %s", string(application), key)
	}

	return nil
}

func AddSecret(application Id, key string, value string) error {
	token, err := GetAccessToken()
	if err != nil {
		return err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	_, res, err := client.ApplicationSecretApi.CreateApplicationSecret(auth, string(application)).SecretRequest(
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

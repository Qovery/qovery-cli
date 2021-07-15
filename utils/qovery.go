package utils

import (
	"errors"
	"fmt"
	"github.com/manifoldco/promptui"
	"github.com/qovery/qovery-client-go"
	"golang.org/x/net/context"
	"strings"
)

func SelectOrganization() error {
	token, err := GetAccessToken()
	if err != nil {
		return err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	organizations, res, err := client.OrganizationMainCallsApi.ListOrganization(auth).Execute()
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return errors.New("Received " + res.Status + " response while listing organizations. ")
	}

	var organizationNames []string
	var orgas = make(map[string]string)

	for _, org := range organizations.GetResults() {
		organizationNames = append(organizationNames, org.Name)
		orgas[org.Name] = org.Id
	}

	if len(organizationNames) < 1 {
		return errors.New("No organizations found. ")
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
		PrintlnError(err)
		return nil
	}

	err = SetOrganization(Name(selectedOrganization), Id(orgas[selectedOrganization]))
	if err != nil {
		PrintlnError(err)
		return nil
	}

	return nil
}

func SelectProject(organization Id) error {
	token, err := GetAccessToken()
	if err != nil {
		return err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	p, res, err := client.ProjectsApi.ListProject(auth, string(organization)).Execute()
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return errors.New("Received " + res.Status + " response while listing projects. ")
	}

	var projectsNames []string
	var projects = make(map[string]string)

	for _, proj := range p.GetResults() {
		projectsNames = append(projectsNames, proj.Name)
		projects[proj.Name] = proj.Id
	}

	if len(projectsNames) < 1 {
		return errors.New("No projects found. ")
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
		return nil
	}

	err = SetProject(Name(selectedProject), Id(projects[selectedProject]))
	if err != nil {
		PrintlnError(err)
		return nil
	}

	return nil
}

func SelectEnvironment(project Id) error {
	token, err := GetAccessToken()
	if err != nil {
		return err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	e, res, err := client.EnvironmentsApi.ListEnvironment(auth, string(project)).Execute()
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return errors.New("Received " + res.Status + " response while listing environments. ")
	}

	var environmentsNames []string
	var environments = make(map[string]string)

	for _, env := range e.GetResults() {
		environmentsNames = append(environmentsNames, env.Name)
		environments[env.Name] = env.Id
	}

	if len(environmentsNames) < 1 {
		return errors.New("No environments found. ")
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
		return nil
	}

	err = SetEnvironment(Name(selectedEnvironment), Id(environments[selectedEnvironment]))
	if err != nil {
		PrintlnError(err)
		return nil
	}

	return nil
}

func SelectApplication(environment Id) error {
	token, err := GetAccessToken()
	if err != nil {
		return err
	}

	auth := context.WithValue(context.Background(), qovery.ContextAccessToken, string(token))
	client := qovery.NewAPIClient(qovery.NewConfiguration())

	a, res, err := client.ApplicationsApi.ListApplication(auth, string(environment)).Execute()
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return errors.New("Received " + res.Status + " response while listing applications. ")
	}

	var applicationsNames []string
	var applications = make(map[string]string)

	for _, app := range a.GetResults() {
		applicationsNames = append(applicationsNames, *app.Name)
		applications[*app.Name] = app.Id
	}

	if len(applicationsNames) < 1 {
		return errors.New("No applications found. ")
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
		return nil
	}

	err = SetApplication(Name(selectedApplication), Id(applications[selectedApplication]))
	if err != nil {
		PrintlnError(err)
		return nil
	}

	return nil
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

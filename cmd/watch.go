package cmd

import (
	"github.com/qovery/qovery-client-go"

	"github.com/qovery/qovery-cli/utils"
)

// The ids below feed utils.NewServicesWatch, which must be created before the request is sent

func applicationIds(applications []*qovery.Application) []string {
	return utils.Map(applications, func(application *qovery.Application) string { return application.Id })
}

func containerIds(containers []*qovery.ContainerResponse) []string {
	return utils.Map(containers, func(container *qovery.ContainerResponse) string { return container.Id })
}

func databaseIds(databases []*qovery.Database) []string {
	return utils.Map(databases, func(database *qovery.Database) string { return database.Id })
}

func jobIds(jobs []*qovery.JobResponse) []string {
	return utils.Map(jobs, utils.GetJobId)
}

func helmIds(helms []*qovery.HelmResponse) []string {
	return utils.Map(helms, func(helm *qovery.HelmResponse) string { return helm.Id })
}

func terraformIds(terraforms []*qovery.TerraformResponse) []string {
	return utils.Map(terraforms, func(terraform *qovery.TerraformResponse) string { return terraform.Id })
}

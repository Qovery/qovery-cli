package cmd

import (
	"fmt"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
)

// Sync option flags (set in rde_upgrade.go init)
var rdeSyncAll bool
var rdeSyncResources bool
var rdeSyncPorts bool
var rdeSyncHealthchecks bool
var rdeSyncStorage bool

// rdeSyncServicesFromBlueprint reads all services from the blueprint environment and updates
// the matching services in the child environment. Services are matched by name.
// Source/image config is always synced. Additional config is synced based on flags.
func rdeSyncServicesFromBlueprint(client *qovery.APIClient, blueprintEnvId string, childEnvId string) int {
	synced := 0
	synced += rdeSyncContainers(client, blueprintEnvId, childEnvId)
	synced += rdeSyncApplications(client, blueprintEnvId, childEnvId)
	synced += rdeSyncJobs(client, blueprintEnvId, childEnvId)
	synced += rdeSyncHelms(client, blueprintEnvId, childEnvId)
	return synced
}

// --- Container sync ---

func rdeSyncContainers(client *qovery.APIClient, blueprintEnvId string, childEnvId string) int {
	bpContainers, _, err := client.ContainersAPI.ListContainer(ctx(), blueprintEnvId).Execute()
	if err != nil || bpContainers == nil {
		return 0
	}
	childContainers, _, err := client.ContainersAPI.ListContainer(ctx(), childEnvId).Execute()
	if err != nil || childContainers == nil {
		return 0
	}

	bpMap := make(map[string]qovery.ContainerResponse)
	for _, c := range bpContainers.GetResults() {
		bpMap[c.Name] = c
	}

	synced := 0
	for _, child := range childContainers.GetResults() {
		bp, ok := bpMap[child.Name]
		if !ok {
			continue
		}

		// Build storage from child or blueprint
		var storage []qovery.ServiceStorageRequestStorageInner
		srcStorage := child.Storage
		if rdeSyncAll || rdeSyncStorage {
			srcStorage = bp.Storage
		}
		for _, s := range srcStorage {
			storage = append(storage, qovery.ServiceStorageRequestStorageInner{
				Id:         &s.Id,
				Type:       s.Type,
				Size:       s.Size,
				MountPoint: s.MountPoint,
			})
		}

		// Build ports from child or blueprint
		var ports []qovery.ServicePortRequestPortsInner
		srcPorts := child.Ports
		if rdeSyncAll || rdeSyncPorts {
			srcPorts = bp.Ports
		}
		for _, p := range srcPorts {
			ports = append(ports, qovery.ServicePortRequestPortsInner{
				Name:               p.Name,
				InternalPort:       p.InternalPort,
				ExternalPort:       p.ExternalPort,
				PubliclyAccessible: p.PubliclyAccessible,
				IsDefault:          p.IsDefault,
				Protocol:           &p.Protocol,
			})
		}

		cpu := utils.Int32(child.Cpu)
		memory := utils.Int32(child.Memory)
		minInst := utils.Int32(child.MinRunningInstances)
		maxInst := utils.Int32(child.MaxRunningInstances)
		autoscaling := utils.ConvertAutoscalingResponseToRequest(child.Autoscaling)
		if rdeSyncAll || rdeSyncResources {
			cpu = utils.Int32(bp.Cpu)
			memory = utils.Int32(bp.Memory)
			minInst = utils.Int32(bp.MinRunningInstances)
			maxInst = utils.Int32(bp.MaxRunningInstances)
			autoscaling = utils.ConvertAutoscalingResponseToRequest(bp.Autoscaling)
		}

		healthchecks := child.Healthchecks
		if rdeSyncAll || rdeSyncHealthchecks {
			healthchecks = bp.Healthchecks
		}

		req := qovery.ContainerRequest{
			Storage:             storage,
			Ports:               ports,
			Name:                child.Name,
			Description:         child.Description,
			RegistryId:          bp.Registry.Id, // always sync source
			ImageName:           bp.ImageName,   // always sync source
			Tag:                 bp.Tag,         // always sync source
			Arguments:           child.Arguments,
			Entrypoint:          child.Entrypoint,
			Cpu:                 cpu,
			Memory:              memory,
			MinRunningInstances: minInst,
			MaxRunningInstances: maxInst,
			Healthchecks:        healthchecks,
			AutoPreview:         utils.Bool(child.AutoPreview),
			AutoDeploy:          *qovery.NewNullableBool(child.AutoDeploy),
			Autoscaling:         autoscaling,
		}

		_, _, err := client.ContainerMainCallsAPI.EditContainer(ctx(), child.Id).ContainerRequest(req).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("    WARNING: Failed to sync container %s: %v", child.Name, err))
		} else {
			utils.Println(fmt.Sprintf("    Synced container: %s (tag: %s)", pterm.FgBlue.Sprintf("%s", child.Name), bp.Tag))
			synced++
		}
	}

	return synced
}

// --- Application sync ---

func rdeSyncApplications(client *qovery.APIClient, blueprintEnvId string, childEnvId string) int {
	bpApps, _, err := client.ApplicationsAPI.ListApplication(ctx(), blueprintEnvId).Execute()
	if err != nil || bpApps == nil {
		return 0
	}
	childApps, _, err := client.ApplicationsAPI.ListApplication(ctx(), childEnvId).Execute()
	if err != nil || childApps == nil {
		return 0
	}

	bpMap := make(map[string]qovery.Application)
	for _, a := range bpApps.GetResults() {
		bpMap[a.Name] = a
	}

	synced := 0
	for _, child := range childApps.GetResults() {
		bp, ok := bpMap[child.Name]
		if !ok {
			continue
		}

		// Build git repository request from blueprint (always sync source)
		var gitRepo *qovery.ApplicationGitRepositoryRequest
		if bp.GitRepository != nil {
			gitRepo = &qovery.ApplicationGitRepositoryRequest{
				Url:      bp.GitRepository.Url,
				Branch:   bp.GitRepository.Branch,
				RootPath: bp.GitRepository.RootPath,
				Provider: bp.GitRepository.Provider,
			}
		}

		var storage []qovery.ServiceStorageRequestStorageInner
		srcStorage := child.Storage
		if rdeSyncAll || rdeSyncStorage {
			srcStorage = bp.Storage
		}
		for _, s := range srcStorage {
			storage = append(storage, qovery.ServiceStorageRequestStorageInner{
				Id:         &s.Id,
				Type:       s.Type,
				Size:       s.Size,
				MountPoint: s.MountPoint,
			})
		}

		cpu := child.Cpu
		memory := child.Memory
		minInst := child.MinRunningInstances
		maxInst := child.MaxRunningInstances
		if rdeSyncAll || rdeSyncResources {
			cpu = bp.Cpu
			memory = bp.Memory
			minInst = bp.MinRunningInstances
			maxInst = bp.MaxRunningInstances
		}

		healthchecks := child.Healthchecks
		if rdeSyncAll || rdeSyncHealthchecks {
			healthchecks = bp.Healthchecks
		}

		ports := child.Ports
		if rdeSyncAll || rdeSyncPorts {
			ports = bp.Ports
		}

		req := qovery.ApplicationEditRequest{
			Storage:             storage,
			Name:                &child.Name,
			Description:         child.Description,
			GitRepository:       gitRepo,           // always sync source
			BuildMode:           bp.BuildMode,      // always sync source
			DockerfilePath:      bp.DockerfilePath, // always sync source
			Cpu:                 cpu,
			Memory:              memory,
			MinRunningInstances: minInst,
			MaxRunningInstances: maxInst,
			Healthchecks:        healthchecks,
			AutoPreview:         child.AutoPreview,
			Ports:               ports,
			Arguments:           child.Arguments,
			Entrypoint:          child.Entrypoint,
			AutoDeploy:          *qovery.NewNullableBool(child.AutoDeploy),
		}

		_, _, err := client.ApplicationMainCallsAPI.EditApplication(ctx(), child.Id).ApplicationEditRequest(req).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("    WARNING: Failed to sync application %s: %v", child.Name, err))
		} else {
			branch := ""
			if bp.GitRepository != nil && bp.GitRepository.Branch != nil {
				branch = *bp.GitRepository.Branch
			}
			utils.Println(fmt.Sprintf("    Synced application: %s (branch: %s)", pterm.FgBlue.Sprintf("%s", child.Name), branch))
			synced++
		}
	}

	return synced
}

// --- Job sync ---

func rdeSyncJobs(client *qovery.APIClient, blueprintEnvId string, childEnvId string) int {
	bpJobs, _, err := client.JobsAPI.ListJobs(ctx(), blueprintEnvId).Execute()
	if err != nil || bpJobs == nil {
		return 0
	}
	childJobs, _, err := client.JobsAPI.ListJobs(ctx(), childEnvId).Execute()
	if err != nil || childJobs == nil {
		return 0
	}

	bpMap := make(map[string]qovery.JobResponse)
	for _, j := range bpJobs.GetResults() {
		bpMap[utils.GetJobName(&j)] = j
	}

	synced := 0
	for _, childJob := range childJobs.GetResults() {
		childName := utils.GetJobName(&childJob)
		bp, ok := bpMap[childName]
		if !ok {
			continue
		}

		childId := utils.GetJobId(&childJob)
		bpSource := rdeJobResponseToRequestSource(&bp)
		childDetail := rdeExtractJobDetail(&childJob)
		if childDetail == nil || bpSource == nil {
			continue
		}

		cpu := childDetail.cpu
		memory := childDetail.memory
		if rdeSyncAll || rdeSyncResources {
			bpDetail := rdeExtractJobDetail(&bp)
			if bpDetail != nil {
				cpu = bpDetail.cpu
				memory = bpDetail.memory
			}
		}

		healthchecks := childDetail.healthchecks
		if rdeSyncAll || rdeSyncHealthchecks {
			bpDetail := rdeExtractJobDetail(&bp)
			if bpDetail != nil {
				healthchecks = bpDetail.healthchecks
			}
		}

		req := qovery.JobRequest{
			Name:               childName,
			Description:        childDetail.description,
			Cpu:                cpu,
			Memory:             memory,
			MaxNbRestart:       childDetail.maxNbRestart,
			MaxDurationSeconds: childDetail.maxDurationSeconds,
			AutoPreview:        childDetail.autoPreview,
			Port:               childDetail.port,
			Source:             bpSource, // always sync source
			Healthchecks:       healthchecks,
			Schedule:           childDetail.schedule,
			AutoDeploy:         childDetail.autoDeploy,
		}

		_, _, err := client.JobMainCallsAPI.EditJob(ctx(), childId).JobRequest(req).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("    WARNING: Failed to sync job %s: %v", childName, err))
		} else {
			utils.Println(fmt.Sprintf("    Synced job: %s", pterm.FgBlue.Sprintf("%s", childName)))
			synced++
		}
	}

	return synced
}

// rdeJobResponseToRequestSource converts a JobResponse source to a JobRequestAllOfSource.
func rdeJobResponseToRequestSource(job *qovery.JobResponse) *qovery.JobRequestAllOfSource {
	var source qovery.BaseJobResponseAllOfSource
	if job.CronJobResponse != nil {
		source = job.CronJobResponse.Source
	} else if job.LifecycleJobResponse != nil {
		source = job.LifecycleJobResponse.Source
	} else {
		return nil
	}

	result := &qovery.JobRequestAllOfSource{}

	if source.BaseJobResponseAllOfSourceOneOf != nil {
		// Image source
		img := source.BaseJobResponseAllOfSourceOneOf.Image
		reqImg := qovery.NewNullableJobRequestAllOfSourceImage(
			&qovery.JobRequestAllOfSourceImage{
				ImageName:  &img.ImageName,
				Tag:        &img.Tag,
				RegistryId: img.RegistryId,
			},
		)
		result.Image = *reqImg
	} else if source.BaseJobResponseAllOfSourceOneOf1 != nil {
		// Docker/git source
		docker := source.BaseJobResponseAllOfSourceOneOf1.Docker
		var gitRepo *qovery.ApplicationGitRepositoryRequest
		if docker.GitRepository != nil {
			gitRepo = &qovery.ApplicationGitRepositoryRequest{
				Url:      docker.GitRepository.Url,
				Branch:   docker.GitRepository.Branch,
				RootPath: docker.GitRepository.RootPath,
				Provider: docker.GitRepository.Provider,
			}
		}
		reqDocker := qovery.NewNullableJobRequestAllOfSourceDocker(
			&qovery.JobRequestAllOfSourceDocker{
				GitRepository:          gitRepo,
				DockerfilePath:         docker.DockerfilePath,
				DockerfileRaw:          docker.DockerfileRaw,
				DockerTargetBuildStage: docker.DockerTargetBuildStage,
			},
		)
		result.Docker = *reqDocker
	}

	return result
}

// jobDetail holds common fields from CronJobResponse or LifecycleJobResponse.
type jobDetail struct {
	cpu                *int32
	memory             *int32
	description        *string
	maxNbRestart       *int32
	maxDurationSeconds *int32
	autoPreview        *bool
	port               qovery.NullableInt32
	healthchecks       qovery.Healthcheck
	schedule           *qovery.JobRequestAllOfSchedule
	autoDeploy         qovery.NullableBool
}

// rdeExtractJobDetail extracts common fields from a JobResponse.
func rdeExtractJobDetail(job *qovery.JobResponse) *jobDetail {
	if job.CronJobResponse != nil {
		cj := job.CronJobResponse
		cpu := cj.Cpu
		mem := cj.Memory
		autoPreview := utils.Bool(cj.AutoPreview)
		tz := cj.Schedule.Cronjob.Timezone
		schedule := &qovery.JobRequestAllOfSchedule{
			Cronjob: &qovery.JobRequestAllOfScheduleCronjob{
				ScheduledAt: cj.Schedule.Cronjob.ScheduledAt,
				Timezone:    &tz,
				Entrypoint:  cj.Schedule.Cronjob.Entrypoint,
				Arguments:   cj.Schedule.Cronjob.Arguments,
			},
		}
		return &jobDetail{
			cpu: &cpu, memory: &mem,
			description: cj.Description, maxNbRestart: cj.MaxNbRestart,
			maxDurationSeconds: cj.MaxDurationSeconds, autoPreview: autoPreview,
			port: cj.Port, healthchecks: cj.Healthchecks, schedule: schedule,
			autoDeploy: *qovery.NewNullableBool(cj.AutoDeploy),
		}
	} else if job.LifecycleJobResponse != nil {
		lj := job.LifecycleJobResponse
		cpu := lj.Cpu
		mem := lj.Memory
		autoPreview := utils.Bool(lj.AutoPreview)
		schedule := &qovery.JobRequestAllOfSchedule{}
		if lj.Schedule.OnStart != nil {
			schedule.OnStart = &qovery.JobRequestAllOfScheduleOnStart{
				Entrypoint: lj.Schedule.OnStart.Entrypoint,
				Arguments:  lj.Schedule.OnStart.Arguments,
			}
		}
		if lj.Schedule.OnStop != nil {
			schedule.OnStop = &qovery.JobRequestAllOfScheduleOnStart{
				Entrypoint: lj.Schedule.OnStop.Entrypoint,
				Arguments:  lj.Schedule.OnStop.Arguments,
			}
		}
		if lj.Schedule.OnDelete != nil {
			schedule.OnDelete = &qovery.JobRequestAllOfScheduleOnStart{
				Entrypoint: lj.Schedule.OnDelete.Entrypoint,
				Arguments:  lj.Schedule.OnDelete.Arguments,
			}
		}
		return &jobDetail{
			cpu: &cpu, memory: &mem,
			description: lj.Description, maxNbRestart: lj.MaxNbRestart,
			maxDurationSeconds: lj.MaxDurationSeconds, autoPreview: autoPreview,
			port: lj.Port, healthchecks: lj.Healthchecks, schedule: schedule,
			autoDeploy: *qovery.NewNullableBool(lj.AutoDeploy),
		}
	}
	return nil
}

// --- Helm sync ---

func rdeSyncHelms(client *qovery.APIClient, blueprintEnvId string, childEnvId string) int {
	bpHelms, _, err := client.HelmsAPI.ListHelms(ctx(), blueprintEnvId).Execute()
	if err != nil || bpHelms == nil {
		return 0
	}
	childHelms, _, err := client.HelmsAPI.ListHelms(ctx(), childEnvId).Execute()
	if err != nil || childHelms == nil {
		return 0
	}

	bpMap := make(map[string]qovery.HelmResponse)
	for _, h := range bpHelms.GetResults() {
		bpMap[h.Name] = h
	}

	synced := 0
	for _, child := range childHelms.GetResults() {
		bp, ok := bpMap[child.Name]
		if !ok {
			continue
		}

		bpSource := rdeConvertHelmSource(&bp.Source)
		if bpSource == nil {
			continue
		}

		childValues := rdeConvertHelmValuesOverride(&child.ValuesOverride)

		req := qovery.HelmRequest{
			Name:                      child.Name,
			Description:               child.Description,
			TimeoutSec:                child.TimeoutSec,
			AutoDeploy:                child.AutoDeploy,
			Source:                    *bpSource, // always sync source
			Arguments:                 child.Arguments,
			AllowClusterWideResources: &child.AllowClusterWideResources,
			ValuesOverride:            *childValues,
		}

		_, _, err := client.HelmMainCallsAPI.EditHelm(ctx(), child.Id).HelmRequest(req).Execute()
		if err != nil {
			utils.Println(fmt.Sprintf("    WARNING: Failed to sync helm %s: %v", child.Name, err))
		} else {
			utils.Println(fmt.Sprintf("    Synced helm: %s", pterm.FgBlue.Sprintf("%s", child.Name)))
			synced++
		}
	}

	return synced
}

// rdeConvertHelmSource converts a HelmResponseAllOfSource to a HelmRequestAllOfSource.
func rdeConvertHelmSource(src *qovery.HelmResponseAllOfSource) *qovery.HelmRequestAllOfSource {
	if src.HelmResponseAllOfSourceOneOf != nil {
		// Git source
		gitSrc := src.HelmResponseAllOfSourceOneOf.Git
		gitRepo := &qovery.HelmGitRepositoryRequest{
			Url:      gitSrc.GitRepository.Url,
			Branch:   gitSrc.GitRepository.Branch,
			RootPath: gitSrc.GitRepository.RootPath,
		}
		return &qovery.HelmRequestAllOfSource{
			HelmRequestAllOfSourceOneOf: &qovery.HelmRequestAllOfSourceOneOf{
				GitRepository: gitRepo,
			},
		}
	} else if src.HelmResponseAllOfSourceOneOf1 != nil {
		// Repository source
		repoSrc := src.HelmResponseAllOfSourceOneOf1.Repository
		repoId := repoSrc.Repository.Id
		repoNullable := qovery.NullableString{}
		repoNullable.Set(&repoId)
		return &qovery.HelmRequestAllOfSource{
			HelmRequestAllOfSourceOneOf1: &qovery.HelmRequestAllOfSourceOneOf1{
				HelmRepository: &qovery.HelmRequestAllOfSourceOneOf1HelmRepository{
					Repository:   repoNullable,
					ChartName:    &repoSrc.ChartName,
					ChartVersion: &repoSrc.ChartVersion,
				},
			},
		}
	}
	return nil
}

// rdeConvertHelmValuesOverride converts HelmResponseAllOfValuesOverride to HelmRequestAllOfValuesOverride.
func rdeConvertHelmValuesOverride(v *qovery.HelmResponseAllOfValuesOverride) *qovery.HelmRequestAllOfValuesOverride {
	return &qovery.HelmRequestAllOfValuesOverride{
		Set:       v.Set,
		SetString: v.SetString,
		SetJson:   v.SetJson,
	}
}

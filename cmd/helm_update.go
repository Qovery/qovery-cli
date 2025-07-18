package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var helmUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a helm",
	Run: func(cmd *cobra.Command, args []string) {
		utils.Capture(cmd)

		tokenType, token, err := utils.GetAccessToken()
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		client := utils.GetQoveryClient(tokenType, token)
		_, _, envId, err := getOrganizationProjectEnvironmentContextResourcesIds(client)

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helms, _, err := client.HelmsAPI.ListHelms(context.Background(), envId).Execute()

		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		helm := utils.FindByHelmName(helms.GetResults(), helmName)

		if helm == nil {
			utils.PrintlnError(fmt.Errorf("helm %s not found", helmName))
			utils.PrintlnInfo("You can list all helms with: qovery helm list")
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		var ports []qovery.HelmPortRequestPortsInner
		for _, p := range helm.Ports {
			if p.HelmPortResponseWithServiceName != nil {
				portWithServiceName := p.HelmPortResponseWithServiceName
				ports = append(ports, qovery.HelmPortRequestPortsInner{
					Name:         portWithServiceName.Name,
					InternalPort: portWithServiceName.InternalPort,
					ExternalPort: portWithServiceName.ExternalPort,
					ServiceName:  &portWithServiceName.ServiceName,
					Namespace:    portWithServiceName.Namespace,
					Protocol:     &portWithServiceName.Protocol,
				})
			}
		}

		source, err := GetHelmSource(helm, chartName, chartVersion, charGitCommitBranch)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		valuesOverride, err := GetHelmValuesOverride(helm, valuesOverrideCommitBranch)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		autoPreview := qovery.NullableBool{}
		autoPreview.Set(&helm.AutoPreview)
		req := qovery.HelmRequest{
			Ports:                     ports,
			Name:                      helm.Name,
			Description:               helm.Description,
			TimeoutSec:                helm.TimeoutSec,
			AutoPreview:               autoPreview,
			AutoDeploy:                helm.AutoDeploy,
			Source:                    *source,
			Arguments:                 helm.Arguments,
			AllowClusterWideResources: &helm.AllowClusterWideResources,
			ValuesOverride:            *valuesOverride,
		}

		_, res, err := client.HelmMainCallsAPI.EditHelm(context.Background(), helm.Id).HelmRequest(req).Execute()

		if err != nil {
			// print http body error message
			if res.StatusCode != 200 {
				result, _ := io.ReadAll(res.Body)
				utils.PrintlnError(errors.Errorf("status code: %s ; body: %s", res.Status, string(result)))
			}

			utils.PrintlnError(err)

			os.Exit(1)
			panic("unreachable") // staticcheck false positive: https://staticcheck.io/docs/checks#SA5011
		}

		utils.Println(fmt.Sprintf("helm %s updated!", pterm.FgBlue.Sprintf("%s", helmName)))
	},
}

func GetHelmSource(helm *qovery.HelmResponse, chartName string, chartVersion string, charGitCommitBranch string) (*qovery.HelmRequestAllOfSource, error) {

	if git := utils.GetGitSource(helm); git != nil {
		updatedBranch := git.GitRepository.Branch
		if charGitCommitBranch != "" {
			updatedBranch = &charGitCommitBranch
		}

		return &qovery.HelmRequestAllOfSource{
			HelmRequestAllOfSourceOneOf: &qovery.HelmRequestAllOfSourceOneOf{
				GitRepository: &qovery.HelmGitRepositoryRequest{
					Url:        git.GitRepository.Url,
					Branch:     updatedBranch,
					RootPath:   git.GitRepository.RootPath,
					GitTokenId: git.GitRepository.GitTokenId,
				},
			},
		}, nil

	} else if repository := utils.GetHelmRepository(helm); repository != nil {
		updatedChartName := &repository.ChartName
		if chartName != "" {
			updatedChartName = &chartName
		}

		updatedChartVersion := &repository.ChartVersion
		if chartVersion != "" {
			updatedChartVersion = &chartVersion
		}

		repositoryId := qovery.NullableString{}
		repositoryId.Set(&repository.Repository.Id)

		return &qovery.HelmRequestAllOfSource{
			HelmRequestAllOfSourceOneOf: nil,
			HelmRequestAllOfSourceOneOf1: &qovery.HelmRequestAllOfSourceOneOf1{
				HelmRepository: &qovery.HelmRequestAllOfSourceOneOf1HelmRepository{
					Repository:   repositoryId,
					ChartName:    updatedChartName,
					ChartVersion: updatedChartVersion,
				},
			},
		}, nil
	}

	return nil, fmt.Errorf("invalid Helm source")
}

func GetHelmValuesOverride(helm *qovery.HelmResponse, valuesOverrideCommitBranch string) (*qovery.HelmRequestAllOfValuesOverride, error) {
	helmRequest := qovery.HelmRequestAllOfValuesOverride{}
	helmRequest.SetSet(helm.ValuesOverride.Set)
	helmRequest.SetSetString(helm.ValuesOverride.SetString)
	helmRequest.SetSetJson(helm.ValuesOverride.SetJson)
	helmRequest.SetSetJson(helm.ValuesOverride.SetJson)

	if helm.ValuesOverride.File.Get() != nil && helm.ValuesOverride.File.Get().Git.Get() != nil {
		git := helm.ValuesOverride.File.Get().Git.Get()

		updatedBranch := git.GitRepository.Branch
		if valuesOverrideCommitBranch != "" {
			updatedBranch = &valuesOverrideCommitBranch
		}

		updatedFile := qovery.HelmRequestAllOfValuesOverrideFile{}
		updatedFile.SetGit(qovery.HelmRequestAllOfValuesOverrideFileGit{
			Paths: git.Paths,
			GitRepository: qovery.ApplicationGitRepositoryRequest{
				Url:        git.GitRepository.Url,
				Branch:     updatedBranch,
				GitTokenId: git.GitRepository.GitTokenId,
				RootPath:   git.GitRepository.RootPath,
			},
		})
		updatedFile.SetRawNil()
		helmRequest.SetFile(updatedFile)

		return &helmRequest, nil
	} else if helm.ValuesOverride.File.Get() != nil && helm.ValuesOverride.File.Get().Raw.Get() != nil {
		raw := helm.ValuesOverride.File.Get().Raw.Get()

		var values = make([]qovery.HelmRequestAllOfValuesOverrideFileRawValues, len(raw.Values))
		for ix, value := range raw.Values {
			values[ix] = qovery.HelmRequestAllOfValuesOverrideFileRawValues{
				Name:    &value.Name,
				Content: &value.Content,
			}
		}

		updatedFile := qovery.HelmRequestAllOfValuesOverrideFile{}
		updatedFile.SetRaw(qovery.HelmRequestAllOfValuesOverrideFileRaw{
			Values: values,
		})
		helmRequest.SetFile(updatedFile)

		return &helmRequest, nil
	}

	return nil, fmt.Errorf("invalid Helm values orerride")
}

func init() {
	helmCmd.AddCommand(helmUpdateCmd)
	helmUpdateCmd.Flags().StringVarP(&organizationName, "organization", "", "", "Organization Name")
	helmUpdateCmd.Flags().StringVarP(&projectName, "project", "", "", "Project Name")
	helmUpdateCmd.Flags().StringVarP(&environmentName, "environment", "", "", "Environment Name")
	helmUpdateCmd.Flags().StringVarP(&helmName, "helm", "n", "", "helm Name")
	helmUpdateCmd.Flags().StringVarP(&chartName, "chart_name", "", "", "helm chart name")
	helmUpdateCmd.Flags().StringVarP(&chartVersion, "chart_version", "", "", "helm chart version")
	helmUpdateCmd.Flags().StringVarP(&charGitCommitBranch, "chart_git_commit_branch", "", "", "helm chart version")
	helmUpdateCmd.Flags().StringVarP(&valuesOverrideCommitBranch, "values_override_git_commit_branch", "", "", "helm chart version")

	_ = helmUpdateCmd.MarkFlagRequired("helm")
}

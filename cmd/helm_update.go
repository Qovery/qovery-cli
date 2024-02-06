package cmd

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/pterm/pterm"
	"github.com/qovery/qovery-cli/utils"
	"github.com/qovery/qovery-client-go"
	"github.com/spf13/cobra"
	"io"
	"os"
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
			ports = append(ports, qovery.HelmPortRequestPortsInner{
				Name:         p.Name,
				InternalPort: p.InternalPort,
				ExternalPort: p.ExternalPort,
				ServiceName:  p.ServiceName,
				Namespace:    p.Namespace,
				Protocol:     &p.Protocol,
			})
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

		utils.Println(fmt.Sprintf("helm %s updated!", pterm.FgBlue.Sprintf(helmName)))
	},
}

func GetHelmSource(helm *qovery.HelmResponse, chartName string, chartVersion string, charGitCommitBranch string) (*qovery.HelmRequestAllOfSource, error) {
	if helm.Source.HelmResponseAllOfSourceOneOf != nil && helm.Source.HelmResponseAllOfSourceOneOf.Git != nil && helm.Source.HelmResponseAllOfSourceOneOf.Git.GitRepository != nil {
		gitRepository := helm.Source.HelmResponseAllOfSourceOneOf.Git.GitRepository

		updatedBranch := gitRepository.Branch
		if charGitCommitBranch != "" {
			updatedBranch = &charGitCommitBranch
		}

		return &qovery.HelmRequestAllOfSource{
			HelmRequestAllOfSourceOneOf: &qovery.HelmRequestAllOfSourceOneOf{
				GitRepository: &qovery.HelmGitRepositoryRequest{
					Url:        gitRepository.Url,
					Branch:     updatedBranch,
					RootPath:   gitRepository.RootPath,
					GitTokenId: gitRepository.GitTokenId,
				},
			},
			HelmRequestAllOfSourceOneOf1: nil,
		}, nil
	} else if helm.Source.HelmResponseAllOfSourceOneOf1 != nil && helm.Source.HelmResponseAllOfSourceOneOf1.Repository != nil {
		repository := helm.Source.HelmResponseAllOfSourceOneOf1.Repository

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

	return nil, fmt.Errorf("Invalid Helm source")
}

func GetHelmValuesOverride(helm *qovery.HelmResponse, valuesOverrideCommitBranch string) (*qovery.HelmRequestAllOfValuesOverride, error) {
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

		helmRequest := qovery.HelmRequestAllOfValuesOverride{}
		helmRequest.SetSet(helm.ValuesOverride.Set)
		helmRequest.SetSetString(helm.ValuesOverride.SetString)
		helmRequest.SetSetJson(helm.ValuesOverride.SetJson)
		helmRequest.SetSetJson(helm.ValuesOverride.SetJson)
		helmRequest.SetFile(updatedFile)

		return &helmRequest, nil
	} else if helm.ValuesOverride.File.Get() != nil && helm.ValuesOverride.File.Get().Raw.Get() != nil {
		raw := helm.ValuesOverride.File.Get().Raw.Get()

		var values = make([]qovery.HelmRequestAllOfValuesOverrideFileRawValues, len(raw.Values))
		for _, value := range raw.Values {
			values = append(values, qovery.HelmRequestAllOfValuesOverrideFileRawValues{
				Name:    &value.Name,
				Content: &value.Content,
			})
		}

		updatedFile := qovery.HelmRequestAllOfValuesOverrideFile{}
		updatedFile.SetRaw(qovery.HelmRequestAllOfValuesOverrideFileRaw{
			Values: values,
		})

		helmRequest := qovery.HelmRequestAllOfValuesOverride{}
		helmRequest.SetSet(helm.ValuesOverride.Set)
		helmRequest.SetSetString(helm.ValuesOverride.SetString)
		helmRequest.SetSetJson(helm.ValuesOverride.SetJson)
		helmRequest.SetSetJson(helm.ValuesOverride.SetJson)
		helmRequest.SetFile(updatedFile)

		return &helmRequest, nil
	}

	return nil, fmt.Errorf("Invalid Helm values orerride")
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

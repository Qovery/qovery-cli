package pkg

import (
	"fmt"

	"github.com/qovery/qovery-cli/utils"
)

func DeployClustersByBatch(listService AdminClusterListService, deployService AdminClusterBatchDeployService) error {
	clusters, err := listService.SelectClusters()
	if err != nil {
		return err
	}

	utils.Println(fmt.Sprintf("%d clusters to deploy:", len(clusters)))
	err = PrintClustersTable(clusters)
	if err != nil {
		return err
	}

	deployService.PrintParameters()

	utils.Println("Do you want to continue deploy process ?")
	var validated = utils.Validate("deploy")
	if !validated {
		utils.Println("Exiting: Validation failed")
		return nil
	}

	deployResult, err := deployService.Deploy(clusters)
	if err != nil {
		return err
	}

	if len(deployResult.PendingClusters) > 0 {
		utils.Println(fmt.Sprintf("%d clusters not triggered because in non-terminal state (queue not implemented yet):", len(deployResult.PendingClusters)))
		err := PrintClustersTable(deployResult.PendingClusters)
		if err != nil {
			return err
		}
	}

	if len(deployResult.ProcessedClusters) > 0 {
		utils.Println(fmt.Sprintf("%d clusters deployed:", len(clusters)))
		err := PrintClustersTable(deployResult.ProcessedClusters)
		if err != nil {
			return err
		}
	}

	return nil
}

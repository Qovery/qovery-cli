package pkg

import (
	"fmt"

	"github.com/qovery/qovery-cli/utils"
)

func ListAllClusters(listService AdminClusterListService) error {
	clusters, err := listService.SelectClusters()
	if err != nil {
		return err
	}

	utils.Println(fmt.Sprintf("Found %d clusters", len(clusters)))
	err = PrintClustersTable(clusters)
	if err != nil {
		return err
	}
	return nil
}

package cmd

import (
	"context"
	"fmt"
	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"io"
	"net/http"
	"os"
	"strconv"
	"text/tabwriter"
	"time"
)

var clusterLockedCmd = &cobra.Command{
	Use:   "locked",
	Short: "List locked clusters",
	Run: func(cmd *cobra.Command, args []string) {
		clusterLocked()
	},
}

func init() {
	clusterLockedCmd.Flags().StringVarP(&organizationId, "organization-id", "o", "", "Organization ID")
	_ = clusterLockedCmd.MarkFlagRequired("organization-id")

	clusterCmd.AddCommand(clusterLockedCmd)
}

func clusterLocked() {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	client := utils.GetQoveryClient(tokenType, token)
	lockedClusters, res, err := client.OrganizationClusterLockAPI.ListClusterLock(context.Background(), organizationId).Execute()
	if res != nil && res.StatusCode != http.StatusOK {
		result, _ := io.ReadAll(res.Body)
		log.Errorf("Could not list locked clusters : %s. %s", res.Status, string(result))
		return
	}

	if err != nil {
		log.Fatal(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	format := "%s\t | %s\t | %s\t | %s\t | %s\t | %s\n"
	fmt.Fprintf(w, format, "", "cluster_id", "locked_at", "ttl_in_days", "locked_by", "reason")
	for idx, lock := range lockedClusters.Results {
		ttlInDays := "infinite"
		if lock.TtlInDays != nil {
			ttlInDays = strconv.Itoa(int(*lock.TtlInDays))
		}

		fmt.Fprintf(w, format, fmt.Sprintf("%d", idx+1), lock.ClusterId, lock.LockedAt.Format(time.RFC1123), ttlInDays, lock.OwnerName, lock.Reason)
	}
	w.Flush()
}

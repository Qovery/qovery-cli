package pkg

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
)

func LockedClusters() {
	utils.GetAdminUrl()

	res := listLockedClusters()

	if res.StatusCode != http.StatusOK {
		result, _ := io.ReadAll(res.Body)
		log.Errorf("Could not list locked clusters : %s. %s", res.Status, string(result))
		return
	}

	resp := struct {
		Results []struct {
			ClusterId string    `json:"cluster_id"`
			OwnerSub  string    `json:"owner_sub"`
			OwnerName string    `json:"owner_name"`
			Reason    string    `json:"reason"`
			LockedAt  time.Time `json:"locked_at"`
			TtlInDays *int      `json:"ttl_in_days"`
		} `json:"results"`
	}{}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		log.Fatal(err)
	}

	w := tabwriter.NewWriter(os.Stdout, 1, 1, 1, ' ', 0)
	format := "%s\t | %s\t | %s\t | %s\t | %s\t | %s\n"
	fmt.Fprintf(w, format, "", "cluster_id", "locked_at", "locked_by", "reason", "ttl_in_days")
	for idx, lock := range resp.Results {
		ttlInDay := "infinite"
		if lock.TtlInDays != nil {
			ttlInDay = strconv.Itoa(*lock.TtlInDays)
		}

		fmt.Fprintf(w, format, fmt.Sprintf("%d", idx+1), lock.ClusterId, lock.LockedAt.Format(time.RFC1123), lock.OwnerName, lock.Reason, ttlInDay)
	}
	w.Flush()
}

func listLockedClusters() *http.Response {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/cluster/lock", utils.GetAdminUrl())
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	return res
}

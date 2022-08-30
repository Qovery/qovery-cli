package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/qovery/qovery-cli/utils"
	log "github.com/sirupsen/logrus"
)

func LockedClusters() {
	utils.CheckAdminUrl()

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
	format := "%s\t | %s\t | %s\t | %s\t | %s\n"
	fmt.Fprintf(w, format, "", "cluster_id", "locked_at", "locked_by", "reason")
	for idx, lock := range resp.Results {
		fmt.Fprintf(w, format, fmt.Sprintf("%d", idx+1), lock.ClusterId, lock.LockedAt.Format(time.RFC1123), lock.OwnerName, lock.Reason)
	}
	w.Flush()
}

func LockById(clusterId string, reason string) {
	utils.CheckAdminUrl()

	if reason == "" {
		log.Errorf("Lock reason is required")
		return
	}

	if utils.Validate("lock") {
		res := updateLockById(clusterId, reason, http.MethodPost)

		if res.StatusCode != http.StatusOK {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not lock cluster : %s. %s", res.Status, string(result))
		} else {
			fmt.Println("Cluster locked.")
		}
	}
}

func UnockById(clusterId string) {
	utils.CheckAdminUrl()

	if utils.Validate("unlock") {
		res := updateLockById(clusterId, "", http.MethodDelete)

		if res.StatusCode != http.StatusOK {
			result, _ := io.ReadAll(res.Body)
			log.Errorf("Could not unlock cluster : %s. %s", res.Status, string(result))
		} else {
			fmt.Println("Cluster unlocked.")
		}
	}
}

func listLockedClusters() *http.Response {
	authToken, tokenErr := utils.GetAccessToken()
	if tokenErr != nil {
		utils.PrintlnError(tokenErr)
		os.Exit(0)
	}

	url := fmt.Sprintf("%s/cluster/lock", utils.AdminUrl)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(authToken)))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	return res
}

func updateLockById(clusterId string, reason string, method string) *http.Response {
	authToken, tokenErr := utils.GetAccessToken()
	if tokenErr != nil {
		utils.PrintlnError(tokenErr)
		os.Exit(0)
	}

	payload := map[string]string{}
	if method == http.MethodPost {
		payload["reason"] = reason
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Fatal(err)
	}

	url := fmt.Sprintf("%s/cluster/lock/%s", utils.AdminUrl, clusterId)
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(string(authToken)))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	return res
}

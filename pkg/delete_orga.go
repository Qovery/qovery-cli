package pkg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	log "github.com/sirupsen/logrus"

	"github.com/qovery/qovery-cli/utils"
)

type DeleteOrganizationsResponse struct {
	Deleted []string                      `json:"deleted"`
	Failed  []DeleteOrganizationFailure   `json:"failed"`
}

type DeleteOrganizationFailure struct {
	OrganizationID string `json:"organization_id"`
	Reason         string `json:"reason"`
}

func DeleteOrganizations(organizationIds []string, allowFailedClusters bool, dryRunDisabled bool) {
	utils.GetAdminUrl()

	utils.DryRunPrint(dryRunDisabled)

	if len(organizationIds) == 0 {
		log.Error("No organization IDs provided")
		os.Exit(1)
	}

	if !utils.Validate("delete") {
		return
	}

	// Build URL with allowFailedClusters parameter
	url := utils.GetAdminUrl() + "/organizations"
	if allowFailedClusters {
		url += "?allowFailedClusters=true"
	}

	// Prepare JSON body with organization IDs
	body, err := json.Marshal(organizationIds)
	if err != nil {
		log.Fatalf("Failed to marshal organization IDs: %v", err)
	}

	if !dryRunDisabled {
		fmt.Printf("Would delete %d organization(s) (allowFailedClusters=%t):\n", len(organizationIds), allowFailedClusters)
		for _, id := range organizationIds {
			fmt.Printf("  - %s\n", id)
		}
		return
	}

	// Make HTTP request
	res := deleteWithBody(url, http.MethodDelete, true, bytes.NewReader(body))
	if res == nil {
		log.Error("Failed to execute delete request")
		return
	}
	defer res.Body.Close()

	// Handle response
	if res.StatusCode != http.StatusOK {
		result, _ := io.ReadAll(res.Body)
		log.Errorf("Failed to delete organizations (status %d): %s", res.StatusCode, string(result))
		os.Exit(1)
	}

	// Parse response
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		log.Errorf("Failed to read response: %v", err)
		os.Exit(1)
	}

	var response DeleteOrganizationsResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		log.Errorf("Failed to parse response: %v", err)
		os.Exit(1)
	}

	// Display results
	displayDeletionResults(response, len(organizationIds))
}

func displayDeletionResults(response DeleteOrganizationsResponse, totalRequested int) {
	fmt.Println()
	fmt.Println("====================================")
	fmt.Println("   Organization Deletion Results")
	fmt.Println("====================================")
	fmt.Println()

	successCount := len(response.Deleted)
	failureCount := len(response.Failed)

	fmt.Printf("Total requested: %d\n", totalRequested)
	fmt.Printf("Successfully deleted: %d\n", successCount)
	fmt.Printf("Failed to delete: %d\n", failureCount)
	fmt.Println()

	if successCount > 0 {
		fmt.Println("✓ Successfully deleted organizations:")
		for _, id := range response.Deleted {
			fmt.Printf("  ✓ %s\n", id)
		}
		fmt.Println()
	}

	if failureCount > 0 {
		fmt.Println("✗ Failed to delete organizations:")
		for _, failure := range response.Failed {
			fmt.Printf("  ✗ %s\n", failure.OrganizationID)
			fmt.Printf("    Reason: %s\n", failure.Reason)
		}
		fmt.Println()
	}

	if failureCount > 0 {
		fmt.Println("Some deletions failed. Check the reasons above.")
		os.Exit(1)
	} else {
		fmt.Println("All organizations deleted successfully! ✓")
	}
}

func httpDelete(url string, method string, dryRunDisabled bool) *http.Response {
	return deleteWithBody(url, method, dryRunDisabled, nil)
}

func deleteWithBody(url string, method string, dryRunDisabled bool, body io.Reader) *http.Response {
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	if !dryRunDisabled {
		return nil
	}

	req, err := http.NewRequest(method, url, body)
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

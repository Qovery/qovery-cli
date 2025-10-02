package auditlog

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/qovery/qovery-cli/utils"
)

// Response structures for the audit logs API
type AuditLogResponse struct {
	Links  Links        `json:"links"`
	Events []AuditEvent `json:"events"`
}

type Links struct {
	Next string `json:"next"`
}

type AuditEvent struct {
	ID              string `json:"id"`
	Timestamp       string `json:"timestamp"`
	EventType       string `json:"event_type"`
	TargetID        string `json:"target_id"`
	TargetName      string `json:"target_name"`
	TargetType      string `json:"target_type"`
	SubTargetType   string `json:"sub_target_type"`
	Origin          string `json:"origin"`
	TriggeredBy     string `json:"triggered_by"`
	ProjectID       string `json:"project_id"`
	ProjectName     string `json:"project_name"`
	EnvironmentID   string `json:"environment_id"`
	EnvironmentName string `json:"environment_name"`
	EnvironmentType string `json:"environment_type"`
	UserAgent       string `json:"user_agent"`
	Change          string `json:"change"`
}

// DownloadOptions contains parameters for downloading audit logs
type DownloadOptions struct {
	OrganizationID string
	FromDate       string
	ToDate         string
	TokenType      string
	Token          string
}

// Service handles audit log operations
type Service struct{}

// NewService creates a new audit log service
func NewService() *Service {
	return &Service{}
}

// DownloadAuditLogs downloads audit logs and saves them to a CSV file
func (s *Service) DownloadAuditLogs(options DownloadOptions) error {
	// Parse from-date to timestamp
	fromTimestamp, err := dateStringToTimestamp(options.FromDate)
	utils.CheckError(err)

	// Parse to-date to timestamp (if provided, otherwise use current time)
	var toTimestamp int64
	if options.ToDate != "" {
		toTimestamp, err = dateStringToTimestamp(options.ToDate)
		utils.CheckError(err)
	} else {
		toTimestamp = time.Now().Unix()
	}

	// Create output file
	now := time.Now()
	filename := fmt.Sprintf("audit_logs_%s.csv", now.Format("2006-01-02_15-04-05"))
	file, err := os.Create(filename)
	utils.CheckError(err)
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close file: %v\n", closeErr)
		}
	}()

	// Create CSV writer
	csvWriter := csv.NewWriter(file)
	defer csvWriter.Flush()

	// Write CSV header
	err = csvWriter.Write([]string{
		"timestamp",
		"event_type",
		"target_id",
		"target_name",
		"target_type",
		"sub_target_type",
		"origin",
		"triggered_by",
		"project_id",
		"project_name",
		"environment_id",
		"environment_name",
		"environment_type",
		"user_agent",
		"change",
	})
	utils.CheckError(err)

	fmt.Printf("Downloading audit logs to: %s\n", filename)

	var continueToken string
	totalEvents := 0
	httpClient := &http.Client{}

	for {
		// Build API URL
		apiURL := buildAPIURL(options.OrganizationID, fromTimestamp, toTimestamp, continueToken)

		// Make HTTP request
		response, err := makeHTTPRequest(httpClient, apiURL, options.TokenType, options.Token)
		utils.CheckError(err)

		// Process events
		for _, event := range response.Events {
			// Write to CSV (the change field contains JSON string that gets properly escaped)
			err = csvWriter.Write([]string{
				event.Timestamp,
				event.EventType,
				event.TargetID,
				event.TargetName,
				event.TargetType,
				event.SubTargetType,
				event.Origin,
				event.TriggeredBy,
				event.ProjectID,
				event.ProjectName,
				event.EnvironmentID,
				event.EnvironmentName,
				event.EnvironmentType,
				event.UserAgent,
				event.Change,
			})
			utils.CheckError(err)
		}

		totalEvents += len(response.Events)
		fmt.Printf("\r🔄 Processing %d events...", totalEvents)

		// Check if there are more pages
		if response.Links.Next == "" {
			break
		}

		// Parse continue token from next URL
		continueToken, err = extractContinueToken(response.Links.Next)
		if err != nil {
			fmt.Printf("\nWarning: Could not parse continue token from URL: %s, error: %v\n", response.Links.Next, err)
			break
		}
	}

	fmt.Println("\n✅ Download complete!")
	return nil
}

// dateStringToTimestamp converts a date string in ISO-8601 format to Unix timestamp
func dateStringToTimestamp(dateStr string) (int64, error) {
	t, err := time.Parse(time.RFC3339, dateStr)
	utils.CheckError(err)
	return t.Unix(), nil
}

// buildAPIURL constructs the API URL with query parameters
func buildAPIURL(organizationId string, fromTimestamp, toTimestamp int64, continueToken string) string {
	baseURL := fmt.Sprintf("https://api.qovery.com/organization/%s/events", organizationId)

	params := url.Values{}
	params.Add("fromTimestamp", strconv.FormatInt(fromTimestamp, 10))
	params.Add("toTimestamp", strconv.FormatInt(toTimestamp, 10))
	params.Add("pageSize", "100")

	if continueToken != "" {
		params.Add("continueToken", continueToken)
	}

	return baseURL + "?" + params.Encode()
}

// makeHTTPRequest performs the HTTP request and returns the parsed response
func makeHTTPRequest(httpClient *http.Client, apiURL, tokenType, token string) (*AuditLogResponse, error) {
	req, err := http.NewRequest("GET", apiURL, nil)
	utils.CheckError(err)

	// Set authorization header
	req.Header.Set("Authorization", fmt.Sprintf("%s %s", tokenType, token))
	req.Header.Set("Content-Type", "application/json")

	// Make the request
	resp, err := httpClient.Do(req)
	utils.CheckError(err)
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close response body: %v\n", closeErr)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	body, err := io.ReadAll(resp.Body)
	utils.CheckError(err)

	var response AuditLogResponse
	err = json.Unmarshal(body, &response)
	utils.CheckError(err)

	return &response, nil
}

// extractContinueToken parses the continue token from the next URL
func extractContinueToken(nextURL string) (string, error) {
	// Parse the URL
	u, err := url.Parse(nextURL)
	utils.CheckError(err)

	// Extract continue token from query parameters
	continueToken := u.Query().Get("continueToken")
	if continueToken == "" {
		return "", fmt.Errorf("continueToken not found in URL")
	}

	return continueToken, nil
}

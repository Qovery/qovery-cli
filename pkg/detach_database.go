package pkg

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/fatih/color"

	"github.com/qovery/qovery-cli/utils"
)

const detachDatabaseBanner = "Cloud resources (instance, snapshots, subnet/parameter/security groups, KMS keys, IAM roles) will NOT be touched.\n" +
	"Qovery will stop managing and tracking this database, and its built-in variables disappear from every service of the environment at its next deploy."

type DatabaseDetachReport struct {
	DatabaseID        string   `json:"database_id"`
	DatabaseName      string   `json:"database_name"`
	DatabaseType      string   `json:"database_type"`
	DatabaseMode      string   `json:"database_mode"`
	DatabaseVersion   string   `json:"database_version"`
	EnvironmentID     string   `json:"environment_id"`
	VariablesToDelete []string `json:"variables_to_delete"`
	Blockers          []string `json:"blockers"`
	DryRun            bool     `json:"dry_run"`
	Detached          bool     `json:"detached"`
	AuditLogged       bool     `json:"audit_logged"`
}

type databaseDetachRequest struct {
	Reason string `json:"reason"`
	DryRun bool   `json:"dry_run"`
}

// DetachDatabase removes a managed database from Qovery without touching the cloud account.
// It always shows the server-side dry run first and only executes when execute is true and the
// operator confirms.
func DetachDatabase(databaseID string, reason string, execute bool) error {
	if strings.TrimSpace(reason) == "" {
		return errors.New("a reason is required (support ticket, customer request)")
	}
	tokenType, token, err := utils.GetAccessToken(false)
	if err != nil {
		return err
	}
	authorization := utils.GetAuthorizationHeaderValue(tokenType, token)
	adminURL := utils.GetAdminUrl()

	preview, err := requestDatabaseDetach(http.DefaultClient, adminURL, authorization, databaseID, reason, true)
	if err != nil {
		return err
	}
	fmt.Print(formatDatabaseDetachReport(preview))
	if len(preview.Blockers) > 0 {
		return fmt.Errorf("database %s cannot be detached, fix the blockers above first", databaseID)
	}

	utils.DryRunPrint(execute)
	if !execute {
		return nil
	}
	if !utils.Validate("detach") {
		return errors.New("detach not confirmed")
	}

	result, err := requestDatabaseDetach(http.DefaultClient, adminURL, authorization, databaseID, reason, false)
	if err != nil {
		return fmt.Errorf("%w\nThe detach may have been applied anyway: run the command again without -y to check the database before retrying", err)
	}
	fmt.Printf("Database %s (%s) detached from Qovery.\n", result.DatabaseName, result.DatabaseID)
	if !result.AuditLogged {
		return fmt.Errorf("database %s was detached but the audit log could not be written: record it by hand in the support ticket", result.DatabaseID)
	}
	return nil
}

func requestDatabaseDetach(client *http.Client, adminURL string, authorization string, databaseID string, reason string, dryRun bool) (*DatabaseDetachReport, error) {
	body, err := json.Marshal(databaseDetachRequest{Reason: reason, DryRun: dryRun})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, strings.TrimSuffix(adminURL, "/")+"/database/"+url.PathEscape(databaseID)+"/detach", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", authorization)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	payload, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("detach of database %s failed: %s %s", databaseID, res.Status, strings.TrimSpace(string(payload)))
	}

	var report DatabaseDetachReport
	if err := json.Unmarshal(payload, &report); err != nil {
		return nil, fmt.Errorf("unexpected response for database %s: %w", databaseID, err)
	}
	if err := checkDatabaseDetachReport(&report, databaseID, dryRun); err != nil {
		return nil, err
	}
	return &report, nil
}

// checkDatabaseDetachReport rejects a 200 that does not describe what was asked: an empty body
// would otherwise read as a preview without blockers, and a dry run as a done detach.
func checkDatabaseDetachReport(report *DatabaseDetachReport, databaseID string, dryRun bool) error {
	switch {
	case report.DatabaseID != databaseID:
		return fmt.Errorf("unexpected response: it describes database %q, not %q", report.DatabaseID, databaseID)
	case report.DryRun != dryRun:
		return fmt.Errorf("unexpected response for database %s: dry_run is %t, expected %t", databaseID, report.DryRun, dryRun)
	case dryRun && report.Detached:
		return fmt.Errorf("unexpected response for database %s: a dry run reports the database as detached", databaseID)
	case !dryRun && (!report.Detached || len(report.Blockers) > 0):
		return fmt.Errorf("database %s: inconsistent detach outcome (detached: %t, blockers: %s)", databaseID, report.Detached, strings.Join(report.Blockers, "; "))
	}
	return nil
}

func formatDatabaseDetachReport(report *DatabaseDetachReport) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Database:    %s (%s)\n", report.DatabaseName, report.DatabaseID)
	fmt.Fprintf(&b, "Type:        %s %s, mode %s\n", report.DatabaseType, report.DatabaseVersion, report.DatabaseMode)
	fmt.Fprintf(&b, "Environment: %s\n", report.EnvironmentID)
	b.WriteString("Built-in variables to delete:\n")
	if len(report.VariablesToDelete) == 0 {
		b.WriteString("  (none)\n")
	}
	for _, name := range report.VariablesToDelete {
		fmt.Fprintf(&b, "  - %s\n", name)
	}
	if len(report.Blockers) > 0 {
		b.WriteString(color.RedString("Blockers:") + "\n")
		for _, blocker := range report.Blockers {
			fmt.Fprintf(&b, "  - %s\n", blocker)
		}
	}
	b.WriteString(color.YellowString(detachDatabaseBanner) + "\n")
	return b.String()
}

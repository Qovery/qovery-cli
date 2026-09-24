package utils

import (
	"context"
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"github.com/qovery/qovery-client-go"
)

const LatestCommitKeyword = "latest"

// IsLatestCommitKeyword reports whether a git commit id input is the reserved
// `latest` keyword instead of a commit SHA.
func IsLatestCommitKeyword(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), LatestCommitKeyword)
}

// newestCommit returns the commit with the greatest creation date. The API
// documents no ordering, so position in the list is not trusted.
func newestCommit(commits []qovery.Commit) (*qovery.Commit, error) {
	if len(commits) == 0 {
		return nil, fmt.Errorf("no commit found")
	}

	newest := &commits[0]
	for i := range commits {
		if commits[i].CreatedAt.After(newest.CreatedAt) {
			newest = &commits[i]
		}
	}

	return newest, nil
}

func ResolveLatestApplicationCommit(client *qovery.APIClient, applicationId string, serviceName string) (string, error) {
	commits, _, err := client.ApplicationMainCallsAPI.ListApplicationCommit(context.Background(), applicationId).Execute()
	if err != nil {
		return "", fmt.Errorf("cannot list commits of application %s: %w", serviceName, err)
	}

	return resolveLatestCommit(commits, serviceName)
}

func ResolveLatestJobCommit(client *qovery.APIClient, jobId string, serviceName string) (string, error) {
	commits, _, err := client.JobMainCallsAPI.ListJobCommit(context.Background(), jobId).Execute()
	if err != nil {
		return "", fmt.Errorf("cannot list commits of job %s: %w", serviceName, err)
	}

	return resolveLatestCommit(commits, serviceName)
}

func ResolveLatestHelmChartCommit(client *qovery.APIClient, helmId string, serviceName string) (string, error) {
	commits, _, err := client.HelmMainCallsAPI.ListHelmCommit(context.Background(), helmId).Execute()
	if err != nil {
		return "", fmt.Errorf("cannot list commits of helm %s: %w", serviceName, err)
	}

	return resolveLatestCommit(commits, serviceName)
}

func ResolveLatestTerraformCommit(client *qovery.APIClient, terraformId string, serviceName string) (string, error) {
	commits, _, err := client.TerraformMainCallsAPI.ListTerraformCommit(context.Background(), terraformId).Execute()
	if err != nil {
		return "", fmt.Errorf("cannot list commits of terraform %s: %w", serviceName, err)
	}

	return resolveLatestCommit(commits, serviceName)
}

func resolveLatestCommit(commits *qovery.CommitResponseList, serviceName string) (string, error) {
	if commits == nil {
		return "", fmt.Errorf("no commit found for %s", serviceName)
	}

	commit, err := newestCommit(commits.GetResults())
	if err != nil {
		return "", fmt.Errorf("%w for %s", err, serviceName)
	}

	Println(fmt.Sprintf("Resolved latest commit for %s: %s (%s)",
		pterm.FgBlue.Sprintf("%s", serviceName),
		pterm.FgBlue.Sprintf("%s", commit.GitCommitId),
		firstLine(commit.Message),
	))

	return commit.GitCommitId, nil
}

func firstLine(message string) string {
	line, _, _ := strings.Cut(message, "\n")
	return strings.TrimSpace(line)
}

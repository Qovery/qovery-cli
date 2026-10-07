package pkg

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/qovery/qovery-cli/utils"
)

const uuidPattern = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

// An archive is stored under a key made of a uuid, optionally followed by a numeric id.
// Execution ids reach us with a Unix timestamp appended on top of that key. Only a
// trailing 10 digit segment is a timestamp: a shorter one, such as the "-1211" in
// "<uuid>-1211-1791383662", is part of the key itself and must be kept.
var executionIdWithTimestampRegex = regexp.MustCompile(`^(` + uuidPattern + `(?:-\d+)?)-\d{10}$`)
var executionIdRegex = regexp.MustCompile(`^` + uuidPattern + `(?:-\d+)?$`)

type ArchiveTagsResponse struct {
	Key   string
	Value string
}

type ArchiveResponse struct {
	Archive string
	Tags    []ArchiveTagsResponse
}

// normalizeExecutionId returns the key the archive is stored under, and whether the id
// is a shape we recognise. A trailing 10 digit Unix timestamp is dropped; any shorter
// numeric suffix belongs to the key and is kept.
func normalizeExecutionId(executionId string) (string, bool) {
	if matches := executionIdWithTimestampRegex.FindStringSubmatch(executionId); matches != nil {
		return matches[1], true
	}

	return executionId, executionIdRegex.MatchString(executionId)
}

func DownloadS3Archive(executionId string, directory string) {
	normalizedId, ok := normalizeExecutionId(executionId)
	if !ok {
		log.Errorf("Invalid execution id format: '%s'. Expected '<uuid>' or '<uuid>-<id>', optionally followed by a '-<timestamp>'", executionId)
		return
	}

	if normalizedId != executionId {
		log.Warnf("Execution id '%s' ends with a timestamp, stripping it to '%s'", executionId, normalizedId)
		executionId = normalizedId
	}

	fileName := executionId + ".tgz"
	res := download(utils.GetAdminUrl()+"/getS3ArchiveObject", fileName)

	if !strings.Contains(res.Status, "200") {
		result, _ := io.ReadAll(res.Body)
		log.Errorf("Could not download archive for key %s: %s. %s", fileName, res.Status, string(result))
		return
	}

	archiveResponse := ArchiveResponse{}
	err := json.NewDecoder(res.Body).Decode(&archiveResponse)
	if err != nil {
		log.Errorf("Could not decode JSON: %v", err)
		return
	}

	organizationId := findOrganizationInTag(archiveResponse.Tags)
	if organizationId == nil {
		log.Warning("Could not find organization tags")
	}

	path := filepath.Join(directory, fileName)
	location := path
	if !filepath.IsAbs(path) {
		location = "./" + path
	}

	utils.PrintlnInfo(fmt.Sprintf("Would you like to write the file in '%s' ?", location))
	// check if it is the expected org
	if !utils.Validate("") {
		return
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(archiveResponse.Archive)
	if err != nil {
		log.Fatalf("Failed to decode base64 archive: %v", err)
	}

	err = writeFile(path, decodedBytes)
	if err != nil {
		log.Fatalf("Failed to write archive to file: %v", err)
	} else {
		utils.PrintlnInfo(fmt.Sprintf("File '%s' has been written", location))
	}
}

func findOrganizationInTag(tags []ArchiveTagsResponse) *string {
	for _, tag := range tags {
		if tag.Key == "OrganizationLongId" {
			return &tag.Value
		}
	}
	return nil
}

func download(url string, executionId string) *http.Response {
	tokenType, token, err := utils.GetAccessToken(false)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(0)
	}

	content := fmt.Sprintf(`{ "key": "%s" }`, executionId)
	body := bytes.NewBuffer([]byte(content))

	req, err := http.NewRequest(http.MethodGet, url, body)
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

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0644)
}

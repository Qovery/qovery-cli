package pkg

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/qovery/qovery-cli/utils"
)

type ArchiveTagsResponse struct {
	Key   string
	Value string
}

type ArchiveResponse struct {
	Archive string
	Tags    []ArchiveTagsResponse
}

func DownloadS3Archive(executionId string, directory string) {
	fileName := executionId + ".tgz"
	res := download(utils.GetAdminUrl()+"/getS3ArchiveObject", fileName)

	if !strings.Contains(res.Status, "200") {
		result, _ := io.ReadAll(res.Body)
		log.Errorf("Could not download archive for key %s: %s. %s", fileName, res.Status, string(result))
		log.Info("For cluster execution id be sure to remove the last part (it's a timestamp)")
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
	tokenType, token, err := utils.GetAccessToken()
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

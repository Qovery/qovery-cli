package pkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials/ssocreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"github.com/qovery/qovery-cli/utils"
)

const (
	DefaultArchiveProfile = "qovery-archive-reader"
	archiveBucket         = "s3-qovery-user-logs"
	archiveRegion         = "us-east-2"
	engineRequestFileName = "engine-request.json"
)

var errEngineRequestNotInArchive = errors.New("no " + engineRequestFileName + " at the archive root: archives produced before the engine stored the request do not have it")

const uuidPattern = `[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`

// An archive is stored under a key made of a uuid, optionally followed by a numeric id.
// Execution ids reach us with a Unix timestamp appended on top of that key. Only a
// trailing 10 digit segment is a timestamp: a shorter one, such as the "-1211" in
// "<uuid>-1211-1791383662", is part of the key itself and must be kept.
var executionIdWithTimestampRegex = regexp.MustCompile(`^(` + uuidPattern + `(?:-\d+)?)-\d{10}$`)
var executionIdRegex = regexp.MustCompile(`^` + uuidPattern + `(?:-\d+)?$`)

// normalizeExecutionId returns the key the archive is stored under, and whether the id
// is a shape we recognise. A trailing 10 digit Unix timestamp is dropped; any shorter
// numeric suffix belongs to the key and is kept.
func normalizeExecutionId(executionId string) (string, bool) {
	if matches := executionIdWithTimestampRegex.FindStringSubmatch(executionId); matches != nil {
		return matches[1], true
	}

	return executionId, executionIdRegex.MatchString(executionId)
}

func DownloadS3Archive(executionId string, directory string, profile string) error {
	fileName, archive, err := fetchArchive(executionId, profile)
	if err != nil {
		return err
	}

	target := filepath.Join(directory, fileName)
	location := target
	if !filepath.IsAbs(target) {
		location = "./" + target
	}

	utils.PrintlnInfo(fmt.Sprintf("Would you like to write the file in '%s' ?", location))
	if !utils.Validate("") {
		return nil
	}

	err = writePrivateFile(target, archive)
	if err != nil {
		return fmt.Errorf("failed to write archive to file: %w", err)
	}

	utils.PrintlnInfo(fmt.Sprintf("File '%s' has been written. It holds customer secrets: delete it once done", location))
	return nil
}

func PrintS3ArchiveEngineRequest(executionId string, profile string) error {
	fileName, archive, err := fetchArchive(executionId, profile)
	if err != nil {
		return err
	}

	request, err := readEngineRequest(archive)
	if err != nil {
		return fmt.Errorf("could not read the engine request of %s: %w", fileName, err)
	}

	_, err = os.Stdout.Write(append(request, '\n'))
	return err
}

func fetchArchive(executionId string, profile string) (string, []byte, error) {
	normalizedId, ok := normalizeExecutionId(executionId)
	if !ok {
		return "", nil, fmt.Errorf("invalid execution id format: '%s'. Expected '<uuid>' or '<uuid>-<id>', optionally followed by a '-<timestamp>'", executionId)
	}

	fileName := normalizedId + ".tgz"
	archive, err := getArchive(context.Background(), profile, fileName)
	if err != nil {
		return "", nil, errors.New(describeArchiveError(err, profile, fileName))
	}

	if normalizedId != executionId {
		utils.PrintlnInfoToStderr(fmt.Sprintf("Execution id '%s' ends with a timestamp, using archive %s", executionId, fileName))
	}

	return fileName, archive, nil
}

func readEngineRequest(archive []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, err
	}
	defer func() { _ = gz.Close() }()

	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errEngineRequestNotInArchive
		}
		if err != nil {
			return nil, err
		}

		name := path.Clean(strings.TrimPrefix(header.Name, "./"))
		if header.Typeflag == tar.TypeReg && path.Base(name) == engineRequestFileName && strings.Count(name, "/") == 1 {
			request, err := io.ReadAll(tr)
			if err != nil {
				return nil, err
			}
			if _, err := io.Copy(io.Discard, gz); err != nil {
				return nil, err
			}
			return request, nil
		}
	}
}

func writePrivateFile(path string, data []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	err = file.Chmod(0600)
	if err == nil {
		_, err = file.Write(data)
	}

	return errors.Join(err, file.Close())
}

func getArchive(ctx context.Context, profile string, key string) ([]byte, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithSharedConfigProfile(profile), config.WithRegion(archiveRegion))
	if err != nil {
		return nil, err
	}

	object, err := s3.NewFromConfig(cfg).GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(archiveBucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = object.Body.Close() }()

	return io.ReadAll(object.Body)
}

func describeArchiveError(err error, profile string, key string) string {
	var missingProfile config.SharedConfigProfileNotExistError
	if errors.As(err, &missingProfile) {
		return fmt.Sprintf("AWS profile '%s' not found in ~/.aws/config. Profile '%s' needs 'role_arn' set to the archive-reader role and 'source_profile' set to your SSO profile of the qovery-prod account", missingProfile.Profile, profile)
	}

	if isSSOSessionError(err) {
		return "No valid AWS SSO session: run 'aws sso login --profile <your SSO profile of the qovery-prod account>', then retry"
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && apiErr.ErrorCode() == "AccessDenied" {
		return fmt.Sprintf("Access denied to archive %s with AWS profile '%s': either the archive does not exist (archives expire after 30 days) or the profile does not assume the archive-reader role", key, profile)
	}

	return fmt.Sprintf("Could not download archive %s with AWS profile '%s': %v", key, profile, err)
}

func isSSOSessionError(err error) bool {
	var invalidToken *ssocreds.InvalidTokenError
	if errors.As(err, &invalidToken) {
		return true
	}

	var apiErr smithy.APIError
	if errors.As(err, &apiErr) && (apiErr.ErrorCode() == "InvalidGrantException" || apiErr.ErrorCode() == "UnauthorizedException") {
		return true
	}

	return strings.Contains(err.Error(), "SSO token")
}

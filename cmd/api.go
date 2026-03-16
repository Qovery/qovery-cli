package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/qovery/qovery-cli/utils"
)

var apiMethod string
var apiInput string
var apiFields []string
var apiHeaders []string
var apiInclude bool

var apiCmd = &cobra.Command{
	Use:   "api <endpoint>",
	Short: "Make an authenticated request to the Qovery API",
	Long: `Make an authenticated HTTP request to the Qovery API.

EXAMPLES

  # List organizations
  $ qovery api organization

  # Get a specific organization
  $ qovery api organization/<id>

  # List projects in current organization (from context)
  $ qovery api organization/{organizationId}/project

  # Get current environment's services (fully from context)
  $ qovery api organization/{organizationId}/project/{projectId}/environment/{environmentId}/service

  # Create an organization using --field
  $ qovery api organization --field name=my-org --field plan=FREE

  # Pipe body from stdin
  $ echo '{"name":"my-org","plan":"FREE"}' | qovery api organization --input -

  # Send a JSON file as body
  $ qovery api organization/<id>/project --input - < body.json

  # Delete a resource
  $ qovery api organization/<id> --method DELETE

  # Show response headers
  $ qovery api organization --include

  # Add a custom header
  $ qovery api organization -H "X-Request-Id: abc123"

  # Use a staging environment
  $ QOVERY_API_URL=https://staging.api.qovery.com qovery api organization`,
	Args: cobra.ExactArgs(1),
	Run:  runAPI,
}

func init() {
	rootCmd.AddCommand(apiCmd)
	apiCmd.Flags().StringVarP(&apiMethod, "method", "X", "", "HTTP method (GET, POST, PUT, PATCH, DELETE)")
	apiCmd.Flags().StringVar(&apiInput, "input", "", "Body: '-' for stdin (pipe JSON to command)")
	apiCmd.Flags().StringArrayVarP(&apiFields, "field", "f", []string{}, "Add a key=value pair to the JSON body (repeatable, smart type coercion)")
	apiCmd.Flags().StringArrayVarP(&apiHeaders, "header", "H", []string{}, "Additional request headers in 'Key: Value' format (repeatable)")
	apiCmd.Flags().BoolVarP(&apiInclude, "include", "i", false, "Print HTTP response status and headers before body")
}

// isValidHTTPHeaderName reports whether name is a valid HTTP token per RFC 7230.
func isValidHTTPHeaderName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') {
			continue
		}
		switch ch {
		case '!', '#', '$', '%', '&', '\'', '*', '+', '-', '.', '^', '_', '`', '|', '~':
			continue
		default:
			return false
		}
	}
	return true
}

// validateAPIArgs validates all arguments and flag values before any I/O.
// It returns an error describing the first problem found.
func validateAPIArgs(endpoint, method, input string, fields, headers []string) error {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return errors.New("endpoint must be a path (e.g. /organization), not a full URL")
	}
	if input != "" && input != "-" {
		return errors.New(`--input only accepts '-' (stdin); to send a file: qovery api <endpoint> --input - < file.json`)
	}
	if len(fields) > 0 && input != "" {
		return errors.New("--field and --input are mutually exclusive")
	}
	allowed := map[string]bool{"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true}
	if method != "" && !allowed[method] {
		return fmt.Errorf("invalid HTTP method %q: must be one of GET, POST, PUT, PATCH, DELETE", method)
	}
	for _, h := range headers {
		idx := strings.Index(h, ":")
		if idx <= 0 {
			return fmt.Errorf("invalid header %q: must be in 'Key: Value' format", h)
		}
		name := h[:idx]
		if !isValidHTTPHeaderName(name) {
			return fmt.Errorf("invalid header name %q: must be a non-empty HTTP token", name)
		}
	}
	seen := make(map[string]bool)
	for _, f := range fields {
		idx := strings.Index(f, "=")
		if idx == -1 {
			return fmt.Errorf("invalid field %q: must be in 'key=value' format", f)
		}
		key := f[:idx]
		if key == "" {
			return fmt.Errorf("invalid field %q: key must not be empty", f)
		}
		if seen[key] {
			return fmt.Errorf("duplicate field key %q: each key may only appear once", key)
		}
		seen[key] = true
	}
	return nil
}

// writeResponse writes the response status line, headers (if include), and body
// to a single stream: stdout for 2xx responses, stderr for non-2xx.
// Returns true on success (2xx), false on error response.
func writeResponse(resp *http.Response, include bool, stdout, stderr io.Writer) (bool, error) {
	is2xx := resp.StatusCode >= 200 && resp.StatusCode < 300
	out := stdout
	if !is2xx {
		out = stderr
	}

	if include {
		_, _ = fmt.Fprintf(out, "HTTP/%d.%d %s\n", resp.ProtoMajor, resp.ProtoMinor, resp.Status)
		headerKeys := make([]string, 0, len(resp.Header))
		for k := range resp.Header {
			headerKeys = append(headerKeys, k)
		}
		sort.Strings(headerKeys)
		for _, k := range headerKeys {
			for _, v := range resp.Header[k] {
				_, _ = fmt.Fprintf(out, "%s: %s\n", k, v)
			}
		}
		_, _ = fmt.Fprintln(out)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, err
	}
	_, _ = out.Write(body)
	return is2xx, nil
}

// substitutePathPlaceholders replaces {organizationId}, {projectId}, {environmentId}, {serviceId}
// in the path with values from the current Qovery context (best-effort — errors silently ignored).
// Empty context values leave the literal placeholder unchanged.
func substitutePathPlaceholders(path string) string {
	ctx, _ := utils.GetCurrentContext()
	pairs := []struct {
		placeholder string
		value       string
	}{
		{"{organizationId}", string(ctx.OrganizationId)},
		{"{projectId}", string(ctx.ProjectId)},
		{"{environmentId}", string(ctx.EnvironmentId)},
		{"{serviceId}", string(ctx.ServiceId)},
	}
	for _, p := range pairs {
		if p.value != "" {
			path = strings.ReplaceAll(path, p.placeholder, p.value)
		}
	}
	return path
}

// coerceFieldValue applies smart type coercion for --field values.
// Order: bool → int64 → float64 → string.
func coerceFieldValue(v string) any {
	if v == "true" {
		return true
	}
	if v == "false" {
		return false
	}
	if i, err := strconv.ParseInt(v, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(v, 64); err == nil {
		return f
	}
	return v
}

func runAPI(cmd *cobra.Command, args []string) {
	endpoint := args[0]

	if err := validateAPIArgs(endpoint, apiMethod, apiInput, apiFields, apiHeaders); err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	// Parse headers (format already validated)
	parsedHeaders := make(map[string]string)
	for _, h := range apiHeaders {
		idx := strings.Index(h, ":")
		parsedHeaders[h[:idx]] = strings.TrimPrefix(h[idx+1:], " ")
	}

	// Parse fields (format already validated)
	parsedFields := make(map[string]string)
	for _, f := range apiFields {
		idx := strings.Index(f, "=")
		parsedFields[f[:idx]] = f[idx+1:]
	}

	// Determine effective HTTP method
	method := apiMethod
	if method == "" {
		if apiInput != "" || len(apiFields) > 0 {
			method = "POST"
		} else {
			method = "GET"
		}
	}

	// Build the full URL
	path := strings.TrimLeft(endpoint, "/")
	path = substitutePathPlaceholders(path)
	fullURL := utils.GetAPIBaseURL() + "/" + path

	// Build request body
	var body io.Reader
	hasBody := apiInput != "" || len(apiFields) > 0

	switch {
	case apiInput == "-":
		body = os.Stdin
	case len(apiFields) > 0:
		fieldMap := make(map[string]any, len(parsedFields))
		for k, v := range parsedFields {
			fieldMap[k] = coerceFieldValue(v)
		}
		jsonBytes, err := json.Marshal(fieldMap)
		if err != nil {
			utils.PrintlnError(err)
			os.Exit(1)
		}
		body = bytes.NewReader(jsonBytes)
	}

	// Create HTTP request
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	// Get auth token
	tokenType, token, err := utils.GetAccessToken()
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}

	// Set Authorization header
	req.Header.Set("Authorization", utils.GetAuthorizationHeaderValue(tokenType, token))

	// Set default Content-Type when body is expected (flag presence check, not body-nil check)
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}

	// Apply user headers (always wins — applied after defaults)
	for k, v := range parsedHeaders {
		req.Header.Set(k, v)
	}

	// Execute request with 60s timeout
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	defer func() { _ = resp.Body.Close() }()

	ok, err := writeResponse(resp, apiInclude, os.Stdout, os.Stderr)
	if err != nil {
		utils.PrintlnError(err)
		os.Exit(1)
	}
	if !ok {
		os.Exit(1)
	}
}

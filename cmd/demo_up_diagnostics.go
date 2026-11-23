package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
)

const (
	demoDiagnosticRequestTimeout = "10s"
	demoDiagnosticBudget         = 60 * time.Second
	demoDiagnosticMaxBytes       = 32 * 1024
	demoDiskPressureTaint        = "node.kubernetes.io/disk-pressure"
)

const demoSecretQuoted = `(\\")(?:[^"\\]|\\[^"])*\\"|(")(?:[^"\\]|\\.)*"|(')(?:[^'\\]|\\.)*'`

const demoSecretName = `[A-Za-z0-9_.-]*(?:token|secret|password|passwd|jwt|credentials?|api[_-]?key|access[_-]?key|secret[_-]?key|private[_-]?key)(?:\\?["'])?`

var demoSecretPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`), demoRedactedValue},
	{regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://[^/\s:@]+:)[^/\s@]+@`), "${1}" + demoRedactedValue + "@"},
	{regexp.MustCompile(`(?i)(authorization(?:\\?["'])?\s*[:=]\s*)(?:` + demoSecretQuoted + `|[^\r\n]+)`), "${1}${2}${3}${4}" + demoRedactedValue + "${2}${3}${4}"},
	{regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`), "${1}" + demoRedactedValue},
	{regexp.MustCompile(`(?i)(` + demoSecretName + `\s*(?:=\s*|:))(?:` + demoSecretQuoted + `|[^\s"',;}\\]+)`), "${1}${2}${3}${4}" + demoRedactedValue + "${2}${3}${4}"},
	{regexp.MustCompile(`(?i)(` + demoSecretName + `\s*:\s+)(?:` + demoSecretQuoted + `)`), "${1}${2}${3}${4}" + demoRedactedValue + "${2}${3}${4}"},
}

func redactDemoDiagnostic(text string) string {
	for _, secret := range demoSecretPatterns {
		text = secret.pattern.ReplaceAllString(text, secret.replacement)
	}
	return text
}

func demoDiagnosticTail(output []byte, maxLines int) string {
	lines := strings.Split(strings.TrimRight(redactDemoDiagnostic(string(output)), "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	for len(lines) > 1 && len(strings.Join(lines, "\n")) > demoDiagnosticMaxBytes {
		lines = lines[1:]
	}
	return strings.Join(lines, "\n") + "\n"
}

// CollectDiagnostics appends a bounded, redacted view of the cluster to the debug log; it never fails.
func (l *demoLocalCommands) CollectDiagnostics(ctx context.Context) {
	l.collectDiagnostics(ctx, demoDiagnosticBudget)
}

func (l *demoLocalCommands) collectDiagnostics(ctx context.Context, budget time.Duration) {
	startedAt := time.Now()
	_, _ = fmt.Fprintln(l.debugLog, "\n===== Kubernetes diagnostics =====")
	run := func(title string, maxLines int, keep func([]byte) []byte, args ...string) []byte {
		if time.Since(startedAt) >= budget {
			_, _ = fmt.Fprintf(l.debugLog, "%s skipped: diagnostic time budget exhausted\n", title)
			return nil
		}
		output, err := l.runner.RunSilent(ctx, "kubectl", append([]string{"--context", l.kubeContext, "--request-timeout", demoDiagnosticRequestTimeout}, args...)...)
		if err != nil {
			_, _ = fmt.Fprintln(l.debugLog, commandFailed(title, err))
			return nil
		}
		if keep != nil {
			output = keep(output)
		}
		_, _ = fmt.Fprintf(l.debugLog, "--- %s ---\n%s", title, demoDiagnosticTail(output, maxLines))
		return output
	}
	run("kubectl get nodes", 20, nil, "get", "nodes", "--output",
		`custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type=="Ready")].status,DISK_PRESSURE:.status.conditions[?(@.type=="DiskPressure")].status,MEMORY_PRESSURE:.status.conditions[?(@.type=="MemoryPressure")].status,PID_PRESSURE:.status.conditions[?(@.type=="PIDPressure")].status,TAINTS:.spec.taints[*].key`)
	pods := run("kubectl get pods", 100, demoUnhealthyPods, "get", "pods", "--all-namespaces", "--output", "wide")
	run("kubectl get events", 60, nil, "--namespace", "qovery", "get", "events", "--sort-by", ".metadata.creationTimestamp")
	if demoOperatorPodExists(pods) {
		run("kubectl logs", 100, nil, "--namespace", "qovery", "logs", "deploy/qovery-operator", "--tail=100", "--limit-bytes=32768")
	}
	_, _ = fmt.Fprintln(l.debugLog, "===== end of Kubernetes diagnostics =====")
}

func demoOperatorPodExists(pods []byte) bool {
	for _, line := range strings.Split(string(pods), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 1 && fields[0] == "qovery" && strings.HasPrefix(fields[1], "qovery-operator-") {
			return true
		}
	}
	return false
}

func demoUnhealthyPods(pods []byte) []byte {
	var kept []string
	for index, line := range strings.Split(strings.TrimRight(string(pods), "\n"), "\n") {
		fields := strings.Fields(line)
		if index == 0 || len(fields) < 4 || fields[0] == "qovery" || (fields[3] != "Running" && fields[3] != "Completed") {
			kept = append(kept, line)
		}
	}
	return []byte(strings.Join(kept, "\n"))
}

// NodeDiskPressure returns the first node under disk pressure and the creation time of the newest node.
func (l *demoLocalCommands) NodeDiskPressure(ctx context.Context) (string, time.Time, error) {
	output, err := l.runner.RunSilent(ctx, "kubectl", "--context", l.kubeContext, "--request-timeout", demoDiagnosticRequestTimeout, "get", "nodes", "--output",
		`jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.creationTimestamp}{"\t"}{.spec.taints[*].key}{"\t"}{.status.conditions[?(@.type=="DiskPressure")].status}{"\n"}{end}`)
	if err != nil {
		err = commandFailed("kubectl get nodes", err)
		_, _ = fmt.Fprintln(l.debugLog, err)
		return "", time.Time{}, err
	}
	pressure, newest, read := "", time.Time{}, false
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339, fields[1])
		if err != nil {
			continue
		}
		read = true
		if createdAt.After(newest) {
			newest = createdAt
		}
		if pressure == "" && (fields[3] == "True" || containsString(strings.Fields(fields[2]), demoDiskPressureTaint)) {
			pressure = fields[0]
		}
	}
	if !read {
		err := errors.New("kubectl get nodes returned no readable node")
		_, _ = fmt.Fprintln(l.debugLog, err)
		return "", time.Time{}, err
	}
	return pressure, newest, nil
}

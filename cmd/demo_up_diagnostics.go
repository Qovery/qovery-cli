package cmd

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	demoDiagnosticRequestTimeout = "10s"
	demoDiagnosticCommandTimeout = 20 * time.Second
	demoDiagnosticKillGrace      = 2 * time.Second
	demoDiagnosticBudget         = 60 * time.Second
	demoDiagnosticMaxBytes       = 32 * 1024
	demoDiskPressureTaint        = "node.kubernetes.io/disk-pressure"
	demoOperatorNamespace        = "qovery"
)

const (
	demoSecretQuoted      = `(\\")(?:[^"\\]|\\[^"])*\\"|(")(?:[^"\\]|\\.)*"|(')(?:[^'\\]|\\.)*'`
	demoSecretWords       = `token|secret|password|passwd|jwt|credentials?|api[_-]?key|access[_-]?key|secret[_-]?key|private[_-]?key`
	demoSecretName        = `[A-Za-z0-9_.-]*(?:` + demoSecretWords + `)(?:\\?["'])?`
	demoQuotedReplacement = "${1}${2}${3}${4}" + demoRedactedValue + "${2}${3}${4}"
)

var demoSecretPatterns = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]*`), demoRedactedValue},
	{regexp.MustCompile(`([A-Za-z][A-Za-z0-9+.-]*://[^/\s:@]+:)[^/\s@]+@`), "${1}" + demoRedactedValue + "@"},
	{regexp.MustCompile(`(?i)((?:authorization|cookie|set-cookie|(?:x-)?api-key)(?:\\?["'])?\s*:\s*)(?:` + demoSecretQuoted + `|[^\r\n]+)`), demoQuotedReplacement},
	{regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/=-]+`), "${1}" + demoRedactedValue},
	{regexp.MustCompile(`(?i)(--[A-Za-z0-9-]*(?:` + demoSecretWords + `)(?:=|\s+))(?:` + demoSecretQuoted + `|[^\s"']+)`), demoQuotedReplacement},
	{regexp.MustCompile(`(?i)(` + demoSecretName + `\s*[:=]\s*)(?:` + demoSecretQuoted + `|[^\s"',;}]+)`), demoQuotedReplacement},
	{regexp.MustCompile(`(?im)(` + demoSecretName + `\s*[:=]\s*\\?["'])[^"'\r\n]*$`), "${1}" + demoRedactedValue},
}

func redactDemoDiagnostic(text string) string {
	for _, secret := range demoSecretPatterns {
		text = secret.pattern.ReplaceAllString(text, secret.replacement)
	}
	return text
}

// demoValidUTF8 replaces each invalid byte with U+FFFD, as the JSON encoding of the upload does.
func demoValidUTF8(output []byte) string {
	var text strings.Builder
	for len(output) > 0 {
		r, size := utf8.DecodeRune(output)
		if r == utf8.RuneError && size == 1 {
			text.WriteRune(utf8.RuneError)
		} else {
			text.Write(output[:size])
		}
		output = output[size:]
	}
	return text.String()
}

// demoDiagnosticTail keeps the last maxLines lines of the redacted output, then the last whole lines that fit in
// maxBytes (the end of a single longer line), and says how many bytes it cut.
func demoDiagnosticTail(output []byte, maxLines int, maxBytes int) string {
	lines := strings.Split(strings.TrimRight(redactDemoDiagnostic(demoValidUTF8(output)), "\n"), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	text := strings.Join(lines, "\n")
	full := len(text)
	for len(lines) > 1 && len(text) > maxBytes {
		lines = lines[1:]
		text = strings.Join(lines, "\n")
	}
	if len(text) > maxBytes {
		cut := len(text) - maxBytes
		for cut < len(text) && !utf8.RuneStart(text[cut]) {
			cut++
		}
		text = text[cut:]
	}
	if cut := full - len(text); cut > 0 {
		text = fmt.Sprintf("[truncated: %d bytes cut]\n%s", cut, text)
	}
	return text + "\n"
}

// kubectl runs a diagnostic command under its own deadline; timedOut tells that deadline, not ctx, ended it.
func (l *demoLocalCommands) kubectl(ctx context.Context, timeout time.Duration, args ...string) (output []byte, timedOut bool, err error) {
	commandCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	output, err = l.runner.RunSilent(commandCtx, "kubectl", append([]string{"--context", l.kubeContext, "--request-timeout", demoDiagnosticRequestTimeout}, args...)...)
	return output, err != nil && ctx.Err() == nil && commandCtx.Err() != nil, err
}

// CollectDiagnostics appends a bounded, redacted view of the cluster to the debug log; it never fails.
func (l *demoLocalCommands) CollectDiagnostics(ctx context.Context, namespace string) {
	l.collectDiagnostics(ctx, namespace, demoDiagnosticBudget)
}

func (l *demoLocalCommands) collectDiagnostics(ctx context.Context, namespace string, budget time.Duration) {
	deadline := time.Now().Add(budget)
	remainingBytes := demoDiagnosticMaxBytes
	stopped := false
	_, _ = fmt.Fprintln(l.debugLog, "\n===== Kubernetes diagnostics =====")
	run := func(title string, maxLines int, keep func([]byte) []byte, args ...string) []byte {
		remaining := time.Until(deadline)
		switch {
		case stopped:
			return nil
		case ctx.Err() == nil && remaining <= 0:
			stopped = true
			_, _ = fmt.Fprintln(l.debugLog, "diagnostic time budget exhausted")
			return nil
		case remainingBytes <= 0:
			stopped = true
			_, _ = fmt.Fprintln(l.debugLog, "diagnostic size limit reached")
			return nil
		}
		output, timedOut, err := l.kubectl(ctx, min(remaining, demoDiagnosticCommandTimeout), args...)
		switch {
		case ctx.Err() != nil:
			stopped = true
			_, _ = fmt.Fprintln(l.debugLog, "diagnostic interrupted")
			return nil
		case timedOut:
			_, _ = fmt.Fprintf(l.debugLog, "%s timed out\n", title)
			return nil
		case err != nil:
			_, _ = fmt.Fprintln(l.debugLog, commandFailed(title, err))
			return nil
		}
		section := demoDiagnosticTail(keep(output), maxLines, remainingBytes)
		remainingBytes -= len(section)
		_, _ = fmt.Fprintf(l.debugLog, "--- %s ---\n%s", title, section)
		return output
	}
	all := func(output []byte) []byte { return output }
	run("kubectl get nodes", 20, all, "get", "nodes", "--output",
		`custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type=="Ready")].status,DISK_PRESSURE:.status.conditions[?(@.type=="DiskPressure")].status,MEMORY_PRESSURE:.status.conditions[?(@.type=="MemoryPressure")].status,PID_PRESSURE:.status.conditions[?(@.type=="PIDPressure")].status,TAINTS:.spec.taints[*].key`)
	pods := run("kubectl get pods", 100, func(output []byte) []byte { return demoUnhealthyPods(output, namespace) }, "get", "pods", "--all-namespaces", "--output", "wide")
	run("kubectl get events", 60, all, "--namespace", namespace, "get", "events", "--sort-by", ".metadata.creationTimestamp")
	for _, pod := range demoOperatorPods(pods, namespace) {
		logs := []string{"--namespace", namespace, "logs", pod.name, "--tail=100", "--limit-bytes=32768"}
		run("kubectl logs "+pod.name, 100, all, logs...)
		if pod.restarts > 0 {
			run("kubectl logs "+pod.name+" --previous", 100, all, append(logs, "--previous")...)
		}
	}
	_, _ = fmt.Fprintln(l.debugLog, "===== end of Kubernetes diagnostics =====")
}

type demoOperatorPod struct {
	name     string
	restarts int
}

// demoOperatorPods reads `kubectl get pods -o wide` lines: NAMESPACE NAME READY STATUS RESTARTS ...
func demoOperatorPods(pods []byte, namespace string) []demoOperatorPod {
	var found []demoOperatorPod
	for _, line := range strings.Split(string(pods), "\n") {
		fields := strings.Fields(line)
		if len(fields) > 4 && fields[0] == namespace && strings.HasPrefix(fields[1], "qovery-operator-") {
			restarts, _ := strconv.Atoi(fields[4])
			found = append(found, demoOperatorPod{name: fields[1], restarts: restarts})
		}
	}
	return found
}

func demoUnhealthyPods(pods []byte, namespace string) []byte {
	var kept []string
	for index, line := range strings.Split(strings.TrimRight(string(pods), "\n"), "\n") {
		fields := strings.Fields(line)
		if index == 0 || len(fields) < 4 || fields[0] == namespace || (fields[3] != "Running" && fields[3] != "Completed") || (fields[3] != "Completed" && demoPodNotReady(fields[2])) {
			kept = append(kept, line)
		}
	}
	return []byte(strings.Join(kept, "\n"))
}

func demoPodNotReady(column string) bool {
	readyText, totalText, found := strings.Cut(column, "/")
	ready, readyErr := strconv.Atoi(readyText)
	total, totalErr := strconv.Atoi(totalText)
	return found && readyErr == nil && totalErr == nil && ready < total
}

// NodeDiskPressure returns the first node under disk pressure and the creation time of the newest node.
func (l *demoLocalCommands) NodeDiskPressure(ctx context.Context, timeout time.Duration) (string, time.Time, error) {
	output, timedOut, err := l.kubectl(ctx, timeout, "get", "nodes", "--output",
		`jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.creationTimestamp}{"\t"}{.spec.taints[*].key}{"\t"}{.status.conditions[?(@.type=="DiskPressure")].status}{"\n"}{end}`)
	if timedOut {
		err = errors.New("kubectl get nodes timed out")
	} else if err != nil {
		err = commandFailed("kubectl get nodes", err)
	}
	if err != nil {
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

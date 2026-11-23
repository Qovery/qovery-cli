package cmd

import (
	"bytes"
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testDemoJWT = "eyJhbGciOiJIUzI1NiJ9.eyJjbHVzdGVyIjoiZGVtbyJ9.c2lnbmF0dXJl"

type scriptedDemoRunner struct {
	calls    []string
	timeouts []time.Duration
	respond  func(ctx context.Context, command string) ([]byte, error)
}

func (r *scriptedDemoRunner) LookPath(string) error { return nil }
func (r *scriptedDemoRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.record(ctx, "run", name, args)
}
func (r *scriptedDemoRunner) RunQuiet(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.record(ctx, "quiet", name, args)
}
func (r *scriptedDemoRunner) RunUnlogged(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.record(ctx, "unlogged", name, args)
}
func (r *scriptedDemoRunner) RunSilent(ctx context.Context, name string, args ...string) ([]byte, error) {
	if deadline, ok := ctx.Deadline(); ok {
		r.timeouts = append(r.timeouts, time.Until(deadline))
	}
	return r.record(ctx, "silent", name, args)
}

func (r *scriptedDemoRunner) record(ctx context.Context, mode string, name string, args []string) ([]byte, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, mode+": "+command)
	return r.respond(ctx, command)
}

const testDemoKubectl = "silent: kubectl --context k3d-local-demo-user --request-timeout 10s "

func testDemoPods() string {
	return "NAMESPACE     NAME                             READY   STATUS             RESTARTS      AGE   IP          NODE\n" +
		"kube-system   coredns-1                        1/1     Running            0             1h    10.0.0.2    server-0\n" +
		"ingress       ingress-1                        0/1     ImagePullBackOff   0             1h    10.0.0.3    server-0\n" +
		"ingress       ingress-2                        1/2     Running            0             1h    10.0.0.7    server-0\n" +
		"jobs          migrate-1                        0/1     Completed          0             1h    10.0.0.4    server-0\n" +
		"qovery-ops    qovery-operator-7d9c-x2x         0/1     CrashLoopBackOff   4 (30s ago)   9m    10.0.0.5    server-0\n" +
		"qovery-ops    qovery-helper-1                  1/1     Running            0             9m    10.0.0.6    server-0\n"
}

func TestDemoCollectDiagnosticsWritesARedactedBoundedView(t *testing.T) {
	runner := &scriptedDemoRunner{respond: func(_ context.Context, command string) ([]byte, error) {
		switch {
		case strings.Contains(command, " get nodes "):
			return []byte("NAME       READY   DISK_PRESSURE   MEMORY_PRESSURE   PID_PRESSURE   TAINTS\nserver-0   True    True            False             False          node.kubernetes.io/disk-pressure\n"), nil
		case strings.Contains(command, " get pods "):
			return []byte(testDemoPods()), nil
		case strings.Contains(command, " get events "):
			return []byte("LAST SEEN   TYPE      REASON    OBJECT                         MESSAGE\n10s         Warning   BackOff   pod/qovery-operator-7d9c-x2x   Back-off restarting failed container\n"), nil
		case strings.HasSuffix(command, " --previous"):
			return []byte("panic: password=abc\\def\n"), nil
		case strings.Contains(command, " logs "):
			return []byte("starting\nconnecting with Authorization: Bearer opaque-value\njwt=" + testDemoJWT + "\n"), nil
		}
		return nil, nil
	}}
	var log bytes.Buffer
	local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &log}

	local.CollectDiagnostics(context.Background(), "qovery-ops")

	assert.Equal(t, []string{
		testDemoKubectl + `get nodes --output custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type=="Ready")].status,DISK_PRESSURE:.status.conditions[?(@.type=="DiskPressure")].status,MEMORY_PRESSURE:.status.conditions[?(@.type=="MemoryPressure")].status,PID_PRESSURE:.status.conditions[?(@.type=="PIDPressure")].status,TAINTS:.spec.taints[*].key`,
		testDemoKubectl + "get pods --all-namespaces --output wide",
		testDemoKubectl + "--namespace qovery-ops get events --sort-by .metadata.creationTimestamp",
		testDemoKubectl + "--namespace qovery-ops logs qovery-operator-7d9c-x2x --tail=100 --limit-bytes=32768",
		testDemoKubectl + "--namespace qovery-ops logs qovery-operator-7d9c-x2x --tail=100 --limit-bytes=32768 --previous",
	}, runner.calls)
	for _, timeout := range runner.timeouts {
		assert.InDelta(t, demoDiagnosticCommandTimeout.Seconds(), timeout.Seconds(), 1)
	}
	assert.Len(t, runner.timeouts, 5)
	assert.Equal(t, `
===== Kubernetes diagnostics =====
--- kubectl get nodes ---
NAME       READY   DISK_PRESSURE   MEMORY_PRESSURE   PID_PRESSURE   TAINTS
server-0   True    True            False             False          node.kubernetes.io/disk-pressure
--- kubectl get pods ---
NAMESPACE     NAME                             READY   STATUS             RESTARTS      AGE   IP          NODE
ingress       ingress-1                        0/1     ImagePullBackOff   0             1h    10.0.0.3    server-0
ingress       ingress-2                        1/2     Running            0             1h    10.0.0.7    server-0
qovery-ops    qovery-operator-7d9c-x2x         0/1     CrashLoopBackOff   4 (30s ago)   9m    10.0.0.5    server-0
qovery-ops    qovery-helper-1                  1/1     Running            0             9m    10.0.0.6    server-0
--- kubectl get events ---
LAST SEEN   TYPE      REASON    OBJECT                         MESSAGE
10s         Warning   BackOff   pod/qovery-operator-7d9c-x2x   Back-off restarting failed container
--- kubectl logs qovery-operator-7d9c-x2x ---
starting
connecting with Authorization: <redacted>
jwt=<redacted>
--- kubectl logs qovery-operator-7d9c-x2x --previous ---
panic: password=<redacted>
===== end of Kubernetes diagnostics =====
`, log.String())
}

func TestDemoCollectDiagnosticsLogsFailuresAndSkipsOperatorLogsWithoutAnOperatorPod(t *testing.T) {
	runner := &scriptedDemoRunner{respond: func(_ context.Context, command string) ([]byte, error) {
		if strings.Contains(command, " get nodes ") {
			return []byte("The connection to the server was refused"), errors.New("exit status 1")
		}
		return []byte(strings.Repeat("line\n", 500)), nil
	}}
	var log bytes.Buffer
	local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &log}

	local.CollectDiagnostics(context.Background(), "qovery")

	assert.Len(t, runner.calls, 3)
	assert.Equal(t, "\n===== Kubernetes diagnostics =====\nkubectl get nodes failed\n--- kubectl get pods ---\n"+strings.Repeat("line\n", 100)+
		"--- kubectl get events ---\n"+strings.Repeat("line\n", 60)+"===== end of Kubernetes diagnostics =====\n", log.String())
}

func TestDemoCollectDiagnosticsStaysWithinItsBudget(t *testing.T) {
	runner := &scriptedDemoRunner{respond: func(ctx context.Context, command string) ([]byte, error) {
		<-ctx.Done()
		return nil, errors.New("signal: interrupt")
	}}
	var log bytes.Buffer
	local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &log}

	local.collectDiagnostics(context.Background(), "qovery", 100*time.Millisecond)

	assert.Len(t, runner.calls, 1)
	assert.LessOrEqual(t, runner.timeouts[0], 100*time.Millisecond)
	assert.Equal(t, "\n===== Kubernetes diagnostics =====\nkubectl get nodes timed out\ndiagnostic time budget exhausted\n===== end of Kubernetes diagnostics =====\n", log.String())

	runner.calls, runner.timeouts = nil, nil
	log.Reset()
	local.collectDiagnostics(context.Background(), "qovery", 0)
	assert.Empty(t, runner.calls)
	assert.Contains(t, log.String(), "\ndiagnostic time budget exhausted\n===== end")
}

func TestDemoCollectDiagnosticsStopsCleanlyWhenInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	runner := &scriptedDemoRunner{respond: func(_ context.Context, command string) ([]byte, error) {
		if strings.Contains(command, " get pods ") {
			cancel()
			return nil, errors.New("signal: interrupt")
		}
		return []byte("ok\n"), nil
	}}
	var log bytes.Buffer
	local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &log}

	local.CollectDiagnostics(ctx, "qovery")

	assert.Len(t, runner.calls, 2)
	assert.Equal(t, "\n===== Kubernetes diagnostics =====\n--- kubectl get nodes ---\nok\ndiagnostic interrupted\n===== end of Kubernetes diagnostics =====\n", log.String())
}

func TestDemoDiagnosticTailCapsLinesAndBytesAfterRedaction(t *testing.T) {
	assert.Equal(t, "c\nd\n", demoDiagnosticTail([]byte("a\nb\nc\nd\n"), 2, 100))
	assert.Equal(t, "[truncated: 4 bytes cut]\nc\nd\n", demoDiagnosticTail([]byte("a\nb\nc\nd\n"), 10, 3))
	assert.Equal(t, "[truncated: 253952 bytes cut]\n"+strings.Repeat("y", 32*1024)+"\n", demoDiagnosticTail([]byte(strings.Repeat("y", 280*1024)), 100, 32*1024))
	assert.Equal(t, "[truncated: 3 bytes cut]\n"+strings.Repeat("y", 99)+"\n", demoDiagnosticTail([]byte("aé"+strings.Repeat("y", 99)), 1, 100))
	assert.Equal(t, "token=<redacted>\n", demoDiagnosticTail([]byte("token="+strings.Repeat("s", 1000)), 1, 100))
}

func TestDemoDiagnosticTailReplacesEachInvalidByte(t *testing.T) {
	assert.Equal(t, "a\uFFFD\uFFFDb\uFFFDc\uFFFD\uFFFD\uFFFDd\n", demoDiagnosticTail([]byte("a\xe2\x82b\xffc\xed\xa0\x80d"), 10, 100))
}

func TestDemoCollectDiagnosticsSharesOneSizeLimit(t *testing.T) {
	line := strings.Repeat("x", 299)
	runner := &scriptedDemoRunner{respond: func(_ context.Context, command string) ([]byte, error) {
		output := strings.Repeat(line+"\n", 200)
		if strings.Contains(command, " get pods ") {
			output += "qovery qovery-operator-1 0/1 CrashLoopBackOff 3 1m\n"
		}
		return []byte(output), nil
	}}
	var log bytes.Buffer
	local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &log}

	local.CollectDiagnostics(context.Background(), "qovery")

	assert.Len(t, runner.calls, 2)
	assert.LessOrEqual(t, log.Len(), demoDiagnosticMaxBytes+512)
	assert.Contains(t, log.String(), "--- kubectl get pods ---\n[truncated: 3000 bytes cut]\n")
	assert.True(t, strings.HasSuffix(log.String(), "\ndiagnostic size limit reached\n===== end of Kubernetes diagnostics =====\n"))
}

func TestRedactDemoDiagnostic(t *testing.T) {
	for _, test := range []struct{ input, expected string }{
		{"jwt " + testDemoJWT + " end", "jwt <redacted> end"},
		{"Authorization: Bearer abc.DEF-123", "Authorization: <redacted>"},
		{"authorization: Basic dXNlcjpwYXNz trailing", "authorization: <redacted>"},
		{`{"Authorization":"Token abc","level":"info"}`, `{"Authorization":"<redacted>","level":"info"}`},
		{"X-Api-Key: abc def", "X-Api-Key: <redacted>"},
		{"Api-Key: abc", "Api-Key: <redacted>"},
		{"Cookie: session=abc; theme=dark", "Cookie: <redacted>"},
		{"calling with Bearer abc.def", "calling with Bearer <redacted>"},
		{"pulling https://robot:s3cret@registry.example/v2/ failed", "pulling https://<redacted>@registry.example/v2/ failed"},
		{"redis://:opaque-password@cache.example:6379/0", "redis://<redacted>@cache.example:6379/0"},
		{"https://opaque-token@api.example/path", "https://<redacted>@api.example/path"},
		{"https://api.example/path/user@example", "https://api.example/path/user@example"},
		{"kubectl --token abc get pods", "kubectl --token <redacted> get pods"},
		{"kubectl --token \"opaque-token", "kubectl --token \"<redacted>"},
		{"helm --password='opaque-password\r\nnext entry", "helm --password='<redacted>\r\nnext entry"},
		{`helm --password "a b" --namespace x`, `helm --password "<redacted>" --namespace x`},
		{"--api-key=k1 next", "--api-key=<redacted> next"},
		{`{"token":"abc","level":"info"}`, `{"token":"<redacted>","level":"info"}`},
		{`{\"token\":\"abc def\",\"level\":\"info\"}`, `{\"token\":\"<redacted>\",\"level\":\"info\"}`},
		{`{"message":"{\"password\":\"abc\\\"opaque-suffix\"}","level":"info"}`, `{"message":"{\"password\":\"<redacted>\"}","level":"info"}`},
		{`{"message":"{\"password\":\"abc\\\\\"}","level":"info"}`, `{"message":"{\"password\":\"<redacted>\"}","level":"info"}`},
		{`password="abc\"SECRET" user=admin`, `password="<redacted>" user=admin`},
		{`password=abc\def next`, `password=<redacted> next`},
		{`token="abc def`, `token="<redacted>`},
		{`password="abc'opaque-suffix`, `password="<redacted>`},
		{`secretToken: "my pass word" next`, `secretToken: "<redacted>" next`},
		{"password: hunter2 next", "password: <redacted> next"},
		{"refresh token: context deadline exceeded", "refresh token: <redacted> deadline exceeded"},
		{"token:abc next", "token:<redacted> next"},
		{"AWS_SECRET_ACCESS_KEY=AKIA/def+1 next", "AWS_SECRET_ACCESS_KEY=<redacted> next"},
		{"api-key = k1; next", "api-key = <redacted>; next"},
		{"private_key='-----BEGIN KEY-----'", "private_key='<redacted>'"},
		{"client_credentials=xyz", "client_credentials=<redacted>"},
		{"secret \"qovery-operator\" not found", "secret \"qovery-operator\" not found"},
		{"tokenRefreshInterval=5s monkey=3", "tokenRefreshInterval=5s monkey=3"},
	} {
		assert.Equal(t, test.expected, redactDemoDiagnostic(test.input), test.input)
	}
}

func TestDemoNodeDiskPressureReadsTaintsAndConditions(t *testing.T) {
	created := time.Date(2026, 8, 19, 9, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		output string
		node   string
	}{
		{"server-0\t2026-08-19T09:00:00Z\tnode.kubernetes.io/disk-pressure\tFalse\n", "server-0"},
		{"agent-0\t2026-08-19T08:00:00Z\t\tFalse\nserver-0\t2026-08-19T09:00:00Z\tnode-role.kubernetes.io/control-plane\tTrue\n", "server-0"},
		{"W1123 warning on stderr\nserver-0\t2026-08-19T09:00:00Z\tnode.kubernetes.io/memory-pressure\tFalse\n", ""},
	} {
		runner := &scriptedDemoRunner{respond: func(context.Context, string) ([]byte, error) { return []byte(test.output), nil }}
		local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &bytes.Buffer{}}

		node, newest, err := local.NodeDiskPressure(context.Background(), 7*time.Second)

		require.NoError(t, err)
		assert.Equal(t, test.node, node)
		assert.Equal(t, created, newest)
		assert.Equal(t, []string{testDemoKubectl + `get nodes --output jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.creationTimestamp}{"\t"}{.spec.taints[*].key}{"\t"}{.status.conditions[?(@.type=="DiskPressure")].status}{"\n"}{end}`}, runner.calls)
		assert.InDelta(t, 7, runner.timeouts[0].Seconds(), 1)
	}

	var log bytes.Buffer
	local := demoLocalCommands{runner: &scriptedDemoRunner{respond: func(context.Context, string) ([]byte, error) { return []byte("refused"), errors.New("exit status 1") }}, debugLog: &log}
	_, _, err := local.NodeDiskPressure(context.Background(), time.Second)
	require.Error(t, err)
	assert.Equal(t, "kubectl get nodes failed\n", log.String())

	log.Reset()
	local.runner = &scriptedDemoRunner{respond: func(ctx context.Context, _ string) ([]byte, error) {
		<-ctx.Done()
		return nil, errors.New("signal: interrupt")
	}}
	_, _, err = local.NodeDiskPressure(context.Background(), 50*time.Millisecond)
	require.Error(t, err)
	assert.Equal(t, "kubectl get nodes timed out\n", log.String())
}

func TestDemoExecRunnerRunSilentKeepsTheTerminalCleanAndKillsAtTheDeadline(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("demo is not supported directly on Windows")
	}
	var terminal, log bytes.Buffer
	runner := demoExecRunner{out: &terminal, log: &log, debug: true}

	output, err := runner.RunSilent(context.Background(), "/bin/sh", "-c", "printf out; printf err >&2; exit 7")

	require.Error(t, err)
	assert.Equal(t, "outerr", string(output))
	assert.Empty(t, terminal.String())
	assert.Equal(t, "$ /bin/sh -c printf out; printf err >&2; exit 7\n", log.String())

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	startedAt := time.Now()
	_, err = runner.RunSilent(ctx, "/bin/sh", "-c", "trap '' INT; sleep 10")
	require.Error(t, err)
	assert.Less(t, time.Since(startedAt), demoDiagnosticKillGrace+2*time.Second)
}

func TestDemoUpCollectsDiagnosticsAfterTheFailedPhaseWithoutChangingIt(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	custom := testDemoBootstrap()
	custom.Namespace = "qovery-system"
	for _, test := range []struct {
		name      string
		api       *fakeDemoAPI
		local     *fakeDemoLocal
		phase     string
		code      string
		namespace string
	}{
		{"operator bootstrap, API", &fakeDemoAPI{configureErr: errors.New("boom")}, &fakeDemoLocal{}, demoPhaseOperatorBootstrap, demoErrorQoveryAPI, "qovery"},
		{"operator bootstrap, helm", &fakeDemoAPI{bootstrap: custom}, &fakeDemoLocal{installErr: errors.New("exit status 1")}, demoPhaseOperatorBootstrap, demoErrorOperatorInstall, "qovery-system"},
		{"operator heartbeat", &fakeDemoAPI{}, &fakeDemoLocal{}, demoPhaseOperatorHeartbeat, demoErrorOperatorHeartbeatTimeout, "qovery"},
		{"platform catalog", &fakeDemoAPI{operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)}, deployStatus: "DEPLOYMENT_ERROR"}, &fakeDemoLocal{}, demoPhasePlatformCatalogDeployment, demoErrorClusterDeployment, "qovery"},
		{"workload verification", &fakeDemoAPI{operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)}}, &fakeDemoLocal{workloadsErr: errors.New("missing")}, demoPhaseWorkloadVerification, demoErrorWorkloadVerification, "qovery"},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			events := []string{}
			clock := &fakeDemoClock{now: now}
			test.api.events, test.local.events = &events, &events
			test.local.onDiagnostics = func() {
				clock.now = clock.now.Add(time.Hour)
				cancel()
			}
			progress := &recordingDemoProgress{}
			orchestrator := demoOrchestrator{api: test.api, local: test.local, clock: clock, out: &bytes.Buffer{}, progress: progress}

			err := orchestrator.Up(ctx, testDemoConfig())

			require.Error(t, err)
			assert.NotErrorIs(t, err, errDemoInterrupted)
			assert.Equal(t, "diagnostics:"+test.namespace, events[len(events)-1])
			last := progress.phases[len(progress.phases)-1]
			assert.Equal(t, []string{test.phase, demoPhaseResultFailed, test.code}, []string{last.Phase, last.Result, last.ErrorCode})
			assert.Less(t, last.Duration, time.Hour)
		})
	}
}

func TestDemoUpSkipsDiagnosticsWhenInterruptedOrBeforeTheLocalCluster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	events := []string{}
	orchestrator := demoOrchestrator{
		api:   &fakeDemoAPI{events: &events},
		local: &fakeDemoLocal{events: &events},
		clock: &cancelingDemoClock{fakeDemoClock: fakeDemoClock{now: time.Now()}, cancel: cancel},
		out:   &bytes.Buffer{},
	}
	require.ErrorIs(t, orchestrator.Up(ctx, testDemoConfig()), errDemoInterrupted)
	assert.NotContains(t, strings.Join(events, ","), "diagnostics")

	events = []string{}
	orchestrator = demoOrchestrator{api: &fakeDemoAPI{events: &events}, local: &fakeDemoLocal{events: &events, k3dErr: errors.New("boom")}, clock: &fakeDemoClock{now: time.Now()}, out: &bytes.Buffer{}}
	require.Error(t, orchestrator.Up(context.Background(), testDemoConfig()))
	assert.NotContains(t, strings.Join(events, ","), "diagnostics")
}

type sleepRecordingDemoClock struct {
	fakeDemoClock
	sleeps []time.Duration
}

func (c *sleepRecordingDemoClock) Sleep(ctx context.Context, duration time.Duration) error {
	c.sleeps = append(c.sleeps, duration)
	return c.fakeDemoClock.Sleep(ctx, duration)
}

func TestDemoUpWaitsForDiskPressureToClearBeforeTheOperatorBootstrap(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	old := now.Add(-time.Hour)
	for _, test := range []struct {
		name     string
		reads    []fakeDemoNodeRead
		sleeps   []time.Duration
		failure  bool
		messages bool
	}{
		{"old healthy node: one read", []fakeDemoNodeRead{{createdAt: old}}, nil, false, false},
		{"read failure is ignored", []fakeDemoNodeRead{{err: errors.New("kubectl get nodes failed")}}, nil, false, false},
		{"read failure while waiting stops the wait", []fakeDemoNodeRead{{node: "server-0", createdAt: old}, {err: errors.New("kubectl get nodes failed")}}, []time.Duration{10 * time.Second}, false, true},
		{"fresh node read again at 30s", []fakeDemoNodeRead{{createdAt: now.Add(-5 * time.Second)}, {}}, []time.Duration{25 * time.Second}, false, false},
		{"node in the future: wait capped at 30s", []fakeDemoNodeRead{{createdAt: now.Add(time.Hour)}, {}}, []time.Duration{30 * time.Second}, false, false},
		{"pressure clears", []fakeDemoNodeRead{{node: "server-0", createdAt: old}, {node: "server-0"}, {}}, []time.Duration{10 * time.Second, 10 * time.Second}, false, true},
		{"pressure stays", []fakeDemoNodeRead{{node: "server-0", createdAt: old}}, nil, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			events := []string{}
			var out bytes.Buffer
			clock := &sleepRecordingDemoClock{fakeDemoClock: fakeDemoClock{now: now}}
			api := &fakeDemoAPI{events: &events, operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now.Add(10 * time.Minute))}}
			progress := &recordingDemoProgress{}
			local := &fakeDemoLocal{events: &events, diskPressure: test.reads}
			orchestrator := demoOrchestrator{api: api, local: local, clock: clock, out: &out, progress: progress}

			err := orchestrator.Up(context.Background(), testDemoConfig())

			assert.Equal(t, test.messages, strings.Contains(out.String(), "Node server-0 is under disk pressure: free some space now; Kubernetes can take up to 6 minutes to see it. Waiting up to 7m0s.\n"))
			if !test.failure {
				require.NoError(t, err)
				if len(test.sleeps) > 0 {
					assert.Equal(t, test.sleeps, clock.sleeps[:len(test.sleeps)])
				}
				for _, timeout := range local.readTimeouts {
					assert.Equal(t, demoNodeReadTimeout, timeout)
				}
				return
			}
			require.Error(t, err)
			code, safeMessage := demoFailureDetails(err)
			assert.Equal(t, demoErrorDockerDiskPressure, code)
			assert.Equal(t, "Docker is running out of disk space: Kubernetes cannot start new pods.", safeMessage)
			assert.Contains(t, err.Error(), "still under disk pressure (node.kubernetes.io/disk-pressure) after 7m0s")
			assert.Contains(t, err.Error(), "`docker builder prune -f` and `docker image prune -a -f`, or increase the disk size in Docker Desktop (Settings > Resources), then run `qovery demo up` again")
			assert.Len(t, clock.sleeps, 42)
			assert.Equal(t, 42, len(local.readTimeouts))
			assert.Equal(t, 10*time.Second, local.readTimeouts[41])
			assert.Equal(t, demoNodeReadTimeout, local.readTimeouts[40])
			assert.Equal(t, demoPhaseLocalCluster, progress.phases[len(progress.phases)-1].Phase)
			assert.NotContains(t, events, "legacy-release")
			assert.NotContains(t, strings.Join(events, ","), "diagnostics")
		})
	}
}

type cancelingDemoProgress struct {
	recordingDemoProgress
	cancel context.CancelFunc
}

func (p *cancelingDemoProgress) PhaseFinished(execution demoPhaseExecution) {
	p.recordingDemoProgress.PhaseFinished(execution)
	if execution.Result == demoPhaseResultFailed {
		p.cancel()
	}
}

func TestDemoUpClassifiesAPhaseOnceBeforeReportingIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	progress := &cancelingDemoProgress{cancel: cancel}
	orchestrator := demoOrchestrator{api: &fakeDemoAPI{}, local: &fakeDemoLocal{k3dErr: errors.New("boom")}, clock: &fakeDemoClock{now: time.Now()}, out: &bytes.Buffer{}, progress: progress}

	err := orchestrator.Up(ctx, testDemoConfig())

	require.Error(t, err)
	assert.NotErrorIs(t, err, errDemoInterrupted)
	assert.Equal(t, demoPhaseResultFailed, progress.phases[len(progress.phases)-1].Result)
}

func TestDemoCollectDiagnosticsTargetsTheGivenKubeconfig(t *testing.T) {
	runner := &scriptedDemoRunner{respond: func(context.Context, string) ([]byte, error) { return nil, nil }}
	local := newDemoLocalCommands(runner, "", "qls-a", "vcluster_qls-a", "/kube/qls-a.yaml")
	local.debugLog = &bytes.Buffer{}

	local.CollectDiagnostics(context.Background(), "qovery")

	assert.NotEmpty(t, runner.calls)
	for _, call := range runner.calls {
		assert.True(t, strings.HasPrefix(call, "silent: kubectl --context vcluster_qls-a --kubeconfig /kube/qls-a.yaml --request-timeout 10s "), call)
	}
}

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
	calls   []string
	respond func(command string) ([]byte, error)
}

func (r *scriptedDemoRunner) LookPath(string) error { return nil }
func (r *scriptedDemoRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.record("run", name, args)
}
func (r *scriptedDemoRunner) RunQuiet(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.record("quiet", name, args)
}
func (r *scriptedDemoRunner) RunUnlogged(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.record("unlogged", name, args)
}
func (r *scriptedDemoRunner) RunSilent(ctx context.Context, name string, args ...string) ([]byte, error) {
	return r.record("silent", name, args)
}

func (r *scriptedDemoRunner) record(mode string, name string, args []string) ([]byte, error) {
	command := strings.Join(append([]string{name}, args...), " ")
	r.calls = append(r.calls, mode+": "+command)
	return r.respond(command)
}

const testDemoKubectl = "silent: kubectl --context k3d-local-demo-user --request-timeout 10s "

func TestDemoCollectDiagnosticsWritesARedactedBoundedView(t *testing.T) {
	runner := &scriptedDemoRunner{respond: func(command string) ([]byte, error) {
		switch {
		case strings.Contains(command, " get nodes "):
			return []byte("NAME       READY   DISK_PRESSURE   MEMORY_PRESSURE   PID_PRESSURE   TAINTS\nserver-0   True    True            False             False          node.kubernetes.io/disk-pressure\n"), nil
		case strings.Contains(command, " get pods "):
			return []byte("NAMESPACE     NAME                             READY   STATUS             RESTARTS   AGE   IP          NODE\n" +
				"kube-system   coredns-1                        1/1     Running            0          1h    10.0.0.2    server-0\n" +
				"ingress       ingress-1                        0/1     ImagePullBackOff   0          1h    10.0.0.3    server-0\n" +
				"jobs          migrate-1                        0/1     Completed          0          1h    10.0.0.4    server-0\n" +
				"qovery        qovery-operator-7d9c-x2x         0/1     CrashLoopBackOff   4          9m    10.0.0.5    server-0\n" +
				"qovery        qovery-helper-1                  1/1     Running            0          9m    10.0.0.6    server-0\n"), nil
		case strings.Contains(command, " get events "):
			return []byte("LAST SEEN   TYPE      REASON    OBJECT                         MESSAGE\n10s         Warning   BackOff   pod/qovery-operator-7d9c-x2x   Back-off restarting failed container\n"), nil
		case strings.Contains(command, " logs "):
			return []byte("starting\nconnecting with Authorization: Bearer opaque-value\njwt=" + testDemoJWT + "\npanic: invalid token: expired\n"), nil
		}
		return nil, nil
	}}
	var log bytes.Buffer
	local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &log}

	local.CollectDiagnostics(context.Background())

	assert.Equal(t, []string{
		testDemoKubectl + `get nodes --output custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type=="Ready")].status,DISK_PRESSURE:.status.conditions[?(@.type=="DiskPressure")].status,MEMORY_PRESSURE:.status.conditions[?(@.type=="MemoryPressure")].status,PID_PRESSURE:.status.conditions[?(@.type=="PIDPressure")].status,TAINTS:.spec.taints[*].key`,
		testDemoKubectl + "get pods --all-namespaces --output wide",
		testDemoKubectl + "--namespace qovery get events --sort-by .metadata.creationTimestamp",
		testDemoKubectl + "--namespace qovery logs deploy/qovery-operator --tail=100 --limit-bytes=32768",
	}, runner.calls)
	assert.Equal(t, `
===== Kubernetes diagnostics =====
--- kubectl get nodes ---
NAME       READY   DISK_PRESSURE   MEMORY_PRESSURE   PID_PRESSURE   TAINTS
server-0   True    True            False             False          node.kubernetes.io/disk-pressure
--- kubectl get pods ---
NAMESPACE     NAME                             READY   STATUS             RESTARTS   AGE   IP          NODE
ingress       ingress-1                        0/1     ImagePullBackOff   0          1h    10.0.0.3    server-0
qovery        qovery-operator-7d9c-x2x         0/1     CrashLoopBackOff   4          9m    10.0.0.5    server-0
qovery        qovery-helper-1                  1/1     Running            0          9m    10.0.0.6    server-0
--- kubectl get events ---
LAST SEEN   TYPE      REASON    OBJECT                         MESSAGE
10s         Warning   BackOff   pod/qovery-operator-7d9c-x2x   Back-off restarting failed container
--- kubectl logs ---
starting
connecting with Authorization: <redacted>
jwt=<redacted>
panic: invalid token: expired
===== end of Kubernetes diagnostics =====
`, log.String())
}

func TestDemoCollectDiagnosticsLogsFailuresSkipsOperatorLogsAndKeepsToItsBudget(t *testing.T) {
	runner := &scriptedDemoRunner{respond: func(command string) ([]byte, error) {
		if strings.Contains(command, " get nodes ") {
			return []byte("The connection to the server was refused"), errors.New("exit status 1")
		}
		return []byte(strings.Repeat("line\n", 500)), nil
	}}
	var log bytes.Buffer
	local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &log}

	local.CollectDiagnostics(context.Background())

	assert.Len(t, runner.calls, 3)
	assert.Contains(t, log.String(), "\nkubectl get nodes failed\n--- kubectl get pods ---\n"+strings.Repeat("line\n", 100)+"--- kubectl get events ---")
	assert.NotContains(t, log.String(), "connection to the server")
	assert.Contains(t, log.String(), "--- kubectl get events ---\n"+strings.Repeat("line\n", 60)+"=====")

	runner.calls = nil
	log.Reset()
	local.collectDiagnostics(context.Background(), 0)
	assert.Empty(t, runner.calls)
	assert.Contains(t, log.String(), "kubectl get nodes skipped: diagnostic time budget exhausted\n")
}

func TestDemoDiagnosticTailCapsLinesAndBytes(t *testing.T) {
	assert.Equal(t, "c\nd\n", demoDiagnosticTail([]byte("a\nb\nc\nd\n"), 2))
	line := strings.Repeat("x", 1000)
	tail := demoDiagnosticTail([]byte(strings.Repeat(line+"\n", 100)), 100)
	assert.LessOrEqual(t, len(tail), demoDiagnosticMaxBytes+1)
	assert.True(t, strings.HasPrefix(tail, line+"\n"))
}

func TestRedactDemoDiagnostic(t *testing.T) {
	for _, test := range []struct{ input, expected string }{
		{"jwt " + testDemoJWT + " end", "jwt <redacted> end"},
		{"Authorization: Bearer abc.DEF-123", "Authorization: <redacted>"},
		{"authorization: Basic dXNlcjpwYXNz trailing", "authorization: <redacted>"},
		{`{"Authorization":"Token abc","level":"info"}`, `{"Authorization":"<redacted>","level":"info"}`},
		{"calling with Bearer abc.def", "calling with Bearer <redacted>"},
		{"pulling https://robot:s3cret@registry.example/v2/ failed", "pulling https://robot:<redacted>@registry.example/v2/ failed"},
		{`{"token":"abc","level":"info"}`, `{"token":"<redacted>","level":"info"}`},
		{`{\"token\":\"abc def\",\"level\":\"info\"}`, `{\"token\":\"<redacted>\",\"level\":\"info\"}`},
		{`password="abc\"SECRET" user=admin`, `password="<redacted>" user=admin`},
		{`secretToken: "my pass word" next`, `secretToken: "<redacted>" next`},
		{"token:abc next", "token:<redacted> next"},
		{"AWS_SECRET_ACCESS_KEY=AKIA/def+1 next", "AWS_SECRET_ACCESS_KEY=<redacted> next"},
		{"api-key = k1; next", "api-key = <redacted>; next"},
		{"private_key='-----BEGIN KEY-----'", "private_key='<redacted>'"},
		{"client_credentials=xyz", "client_credentials=<redacted>"},
		{"refresh token: context deadline exceeded", "refresh token: context deadline exceeded"},
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
		newest time.Time
	}{
		{"server-0\t2026-08-19T09:00:00Z\tnode.kubernetes.io/disk-pressure\tFalse\n", "server-0", created},
		{"agent-0\t2026-08-19T08:00:00Z\t\tFalse\nserver-0\t2026-08-19T09:00:00Z\tnode-role.kubernetes.io/control-plane\tTrue\n", "server-0", created},
		{"W1123 warning on stderr\nserver-0\t2026-08-19T09:00:00Z\tnode.kubernetes.io/memory-pressure\tFalse\n", "", created},
	} {
		runner := &scriptedDemoRunner{respond: func(string) ([]byte, error) { return []byte(test.output), nil }}
		local := demoLocalCommands{runner: runner, kubeContext: "k3d-local-demo-user", debugLog: &bytes.Buffer{}}

		node, newest, err := local.NodeDiskPressure(context.Background())

		require.NoError(t, err)
		assert.Equal(t, test.node, node)
		assert.Equal(t, test.newest, newest)
		assert.Equal(t, []string{testDemoKubectl + `get nodes --output jsonpath={range .items[*]}{.metadata.name}{"\t"}{.metadata.creationTimestamp}{"\t"}{.spec.taints[*].key}{"\t"}{.status.conditions[?(@.type=="DiskPressure")].status}{"\n"}{end}`}, runner.calls)
	}

	var log bytes.Buffer
	local := demoLocalCommands{runner: &scriptedDemoRunner{respond: func(string) ([]byte, error) { return []byte("refused"), errors.New("exit status 1") }}, debugLog: &log}
	_, _, err := local.NodeDiskPressure(context.Background())
	require.Error(t, err)
	assert.Equal(t, "kubectl get nodes failed\n", log.String())
}

func TestDemoExecRunnerRunSilentKeepsTheTerminalAndTheLogClean(t *testing.T) {
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
}

func TestDemoUpCollectsDiagnosticsAfterTheFailedPhaseWithoutChangingIt(t *testing.T) {
	now := time.Date(2026, 8, 19, 10, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name  string
		api   *fakeDemoAPI
		local *fakeDemoLocal
		phase string
		code  string
	}{
		{"operator bootstrap", &fakeDemoAPI{}, &fakeDemoLocal{installErr: errors.New("exit status 1")}, demoPhaseOperatorBootstrap, demoErrorOperatorInstall},
		{"operator heartbeat", &fakeDemoAPI{}, &fakeDemoLocal{}, demoPhaseOperatorHeartbeat, demoErrorOperatorHeartbeatTimeout},
		{"platform catalog", &fakeDemoAPI{operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)}, deployStatus: "DEPLOYMENT_ERROR"}, &fakeDemoLocal{}, demoPhasePlatformCatalogDeployment, demoErrorClusterDeployment},
		{"workload verification", &fakeDemoAPI{operatorStatuses: []demoOperatorStatus{readyDemoOperatorStatus(now)}}, &fakeDemoLocal{workloadsErr: errors.New("missing")}, demoPhaseWorkloadVerification, demoErrorWorkloadVerification},
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
			assert.Equal(t, "diagnostics", events[len(events)-1])
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
	assert.NotContains(t, events, "diagnostics")

	events = []string{}
	orchestrator = demoOrchestrator{api: &fakeDemoAPI{events: &events}, local: &fakeDemoLocal{events: &events, k3dErr: errors.New("boom")}, clock: &fakeDemoClock{now: time.Now()}, out: &bytes.Buffer{}}
	require.Error(t, orchestrator.Up(context.Background(), testDemoConfig()))
	assert.NotContains(t, events, "diagnostics")
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
			orchestrator := demoOrchestrator{api: api, local: &fakeDemoLocal{events: &events, diskPressure: test.reads}, clock: clock, out: &out, progress: progress}

			err := orchestrator.Up(context.Background(), testDemoConfig())

			assert.Equal(t, test.messages, strings.Contains(out.String(), "Node server-0 is under disk pressure; waiting up to 5m0s for it to clear"))
			if !test.failure {
				require.NoError(t, err)
				if test.sleeps != nil {
					assert.Equal(t, test.sleeps, clock.sleeps[:len(test.sleeps)])
				}
				return
			}
			require.Error(t, err)
			code, safeMessage := demoFailureDetails(err)
			assert.Equal(t, demoErrorDockerDiskPressure, code)
			assert.Equal(t, "Docker is running out of disk space: Kubernetes cannot start new pods.", safeMessage)
			assert.Contains(t, err.Error(), "still under disk pressure (node.kubernetes.io/disk-pressure) after 5m0s")
			assert.Contains(t, err.Error(), "`docker builder prune -f` and `docker image prune -a -f`, or increase the disk size in Docker Desktop (Settings > Resources), then run `qovery demo up` again")
			assert.Len(t, clock.sleeps, 30)
			assert.Equal(t, demoPhaseLocalCluster, progress.phases[len(progress.phases)-1].Phase)
			assert.NotContains(t, events, "legacy-release")
			assert.NotContains(t, events, "diagnostics")
		})
	}
}

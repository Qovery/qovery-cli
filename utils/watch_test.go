package utils

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/qovery/qovery-client-go"
)

// watchTrace is what /statusesWithStages returned, poll after poll, for real requests sent to a
// test environment (utils/testdata/watch). "before" is the execution of each service right
// before the request was sent, as NewServicesWatch records it.
type watchTrace struct {
	Description string            `json:"description"`
	FinalState  qovery.StateEnum  `json:"final_state"`
	ServiceIds  []string          `json:"service_ids"`
	Before      map[string]string `json:"before"`
	Polls       []struct {
		T          float64                       `json:"t"`
		WithStages map[string]traceServiceStatus `json:"with_stages"`
	} `json:"polls"`
}

type traceServiceStatus struct {
	State       qovery.StateEnum `json:"state"`
	ExecutionId string           `json:"execution_id"`
}

func loadTrace(t *testing.T, name string) watchTrace {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "watch", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	var trace watchTrace
	if err := json.Unmarshal(raw, &trace); err != nil {
		t.Fatal(err)
	}
	return trace
}

func withStages(containers map[string]traceServiceStatus) *qovery.EnvironmentStatusesWithStages {
	var list []qovery.Status
	for id, s := range containers {
		executionId := s.ExecutionId
		list = append(list, qovery.Status{Id: id, State: s.State, ExecutionId: &executionId})
	}
	return &qovery.EnvironmentStatusesWithStages{Stages: []qovery.DeploymentStageWithServicesStatuses{{Containers: list}}}
}

func TestServicesTrackerOnRealTraces(t *testing.T) {
	tests := []struct {
		name     string
		trace    string
		only     []string
		expected Status
	}{
		{"redeploy already deployed", "redeploy_already_deployed", nil, Stop},
		{"queued behind a running deployment", "queued_behind_running_deployment", nil, Stop},
		// the Hyperline incident: every watched service waits behind a deployment of other
		// services, and /statuses keeps showing them DEPLOYED from their previous execution
		{"only services queued behind another deployment", "queued_behind_running_deployment", []string{"svc-06", "svc-07", "svc-08", "svc-09", "svc-10"}, Stop},
		{"restart", "restart", nil, Stop},
		{"stop", "stop", nil, Stop},
		{"stop already stopped", "stop_already_stopped", nil, Stop},
		{"deploy error", "deploy_error", nil, Err},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			trace := loadTrace(t, test.trace)
			if test.only != nil {
				trace.ServiceIds = test.only
			}
			tracker := newServicesTracker(trace.ServiceIds, trace.Before)

			for i, poll := range trace.Polls {
				status, done, err := tracker.update(withStages(poll.WithStages), trace.FinalState)

				allDone := true
				for _, id := range trace.ServiceIds {
					s := poll.WithStages[id]
					if s.State != trace.FinalState || s.ExecutionId == trace.Before[id] {
						allDone = false
					}
				}
				if status == Stop && !allDone {
					t.Fatalf("poll %d at %.1fs: Stop while services were not all done (%d/%d)", i, poll.T, done, len(trace.ServiceIds))
				}
				if status != Continue {
					if status != test.expected {
						t.Fatalf("poll %d at %.1fs: got %v (err %v), want %v", i, poll.T, status, err, test.expected)
					}
					return
				}
			}
			t.Fatalf("the watch never ended, want %v", test.expected)
		})
	}
}

func TestServicesTracker(t *testing.T) {
	type poll map[string]traceServiceStatus
	s := func(state qovery.StateEnum, executionId string) traceServiceStatus {
		return traceServiceStatus{State: state, ExecutionId: executionId}
	}

	tests := []struct {
		name          string
		before        map[string]string
		finalState    qovery.StateEnum
		polls         []poll
		expected      Status
		expectedError string
	}{
		{
			name:       "previous final state with the same execution is not done",
			before:     map[string]string{"a": "env-1"},
			finalState: qovery.STATEENUM_DEPLOYED,
			polls:      []poll{{"a": s(qovery.STATEENUM_DEPLOYED, "env-1")}, {"a": s(qovery.STATEENUM_DEPLOYED, "env-1")}},
			expected:   Continue,
		},
		{
			name:       "new execution in the final state is done even if never seen in progress",
			before:     map[string]string{"a": "env-1"},
			finalState: qovery.STATEENUM_DEPLOYED,
			polls:      []poll{{"a": s(qovery.STATEENUM_DEPLOYED, "env-2")}},
			expected:   Stop,
		},
		{
			// a previous request was running when the watch started: its end must not count
			name:       "previous execution in progress then done is not done",
			before:     map[string]string{"a": "env-1"},
			finalState: qovery.STATEENUM_DEPLOYED,
			polls: []poll{
				{"a": s(qovery.STATEENUM_DEPLOYING, "env-1")},
				{"a": s(qovery.STATEENUM_DEPLOYED, "env-1")},
			},
			expected: Continue,
		},
		{
			name:       "previous execution in progress, then new execution done",
			before:     map[string]string{"a": "env-1"},
			finalState: qovery.STATEENUM_DEPLOYED,
			polls: []poll{
				{"a": s(qovery.STATEENUM_DEPLOYING, "env-1")},
				{"a": s(qovery.STATEENUM_DEPLOYMENT_QUEUED, "env-1")},
				{"a": s(qovery.STATEENUM_DEPLOYING, "env-2")},
				{"a": s(qovery.STATEENUM_DEPLOYED, "env-2")},
			},
			expected: Stop,
		},
		{
			name:       "error of the previous execution is ignored",
			before:     map[string]string{"a": "env-1"},
			finalState: qovery.STATEENUM_DEPLOYED,
			polls:      []poll{{"a": s(qovery.STATEENUM_DEPLOYMENT_ERROR, "env-1")}},
			expected:   Continue,
		},
		{
			name:          "error of the new execution fails the watch",
			before:        map[string]string{"a": "env-1"},
			finalState:    qovery.STATEENUM_DEPLOYED,
			polls:         []poll{{"a": s(qovery.STATEENUM_DEPLOYMENT_QUEUED, "env-1")}, {"a": s(qovery.STATEENUM_DEPLOYMENT_ERROR, "env-2")}},
			expected:      Err,
			expectedError: "service a is in state DEPLOYMENT_ERROR",
		},
		{
			name:          "canceled fails the watch",
			before:        map[string]string{"a": "env-1"},
			finalState:    qovery.STATEENUM_STOPPED,
			polls:         []poll{{"a": s(qovery.STATEENUM_STOP_QUEUED, "env-1")}, {"a": s(qovery.STATEENUM_CANCELED, "env-2")}},
			expected:      Err,
			expectedError: "service a is in state CANCELED",
		},
		{
			name:          "another final state fails the watch",
			before:        map[string]string{"a": "env-1"},
			finalState:    qovery.STATEENUM_STOPPED,
			polls:         []poll{{"a": s(qovery.STATEENUM_DEPLOYED, "env-2")}},
			expected:      Err,
			expectedError: "service a ended in state DEPLOYED instead of STOPPED",
		},
		{
			name:       "deleted service disappears",
			before:     map[string]string{"a": "env-1"},
			finalState: qovery.STATEENUM_DELETED,
			polls:      []poll{{"a": s(qovery.STATEENUM_DELETE_QUEUED, "env-1")}, {}},
			expected:   Stop,
		},
		{
			name:          "service missing when the watch started is not counted as deleted",
			before:        map[string]string{},
			finalState:    qovery.STATEENUM_DELETED,
			polls:         []poll{{}},
			expected:      Err,
			expectedError: "service a was not in the environment when the watch started",
		},
		{
			name:          "missing service fails the watch when not deleting",
			before:        map[string]string{"a": "env-1"},
			finalState:    qovery.STATEENUM_DEPLOYED,
			polls:         []poll{{}},
			expected:      Err,
			expectedError: "service a no longer exists in the environment",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tracker := newServicesTracker([]string{"a"}, test.before)
			var status Status
			var err error
			for _, p := range test.polls {
				status, _, err = tracker.update(withStages(p), test.finalState)
				if status != Continue {
					break
				}
			}
			if status != test.expected {
				t.Fatalf("status = %v, want %v", status, test.expected)
			}
			if got := errorMessage(err); got != test.expectedError {
				t.Fatalf("error = %q, want %q", got, test.expectedError)
			}
		})
	}
}

func TestWatchServicesRetriesApiErrors(t *testing.T) {
	deployed := withStages(map[string]traceServiceStatus{"a": {State: qovery.STATEENUM_DEPLOYED, ExecutionId: "env-2"}})
	deploying := withStages(map[string]traceServiceStatus{"a": {State: qovery.STATEENUM_DEPLOYING, ExecutionId: "env-2"}})
	apiError := errors.New("503 Service Unavailable")

	type response struct {
		statuses *qovery.EnvironmentStatusesWithStages
		err      error
	}

	tests := []struct {
		name          string
		responses     []response
		expected      Status
		expectedCalls int
	}{
		{
			name:          "transient errors are retried",
			responses:     []response{{err: apiError}, {err: apiError}, {statuses: deploying}, {err: apiError}, {statuses: deployed}},
			expected:      Stop,
			expectedCalls: 5,
		},
		{
			name: "errors in a row fail the watch",
			responses: []response{
				{statuses: deploying},
				{err: apiError}, {err: apiError}, {err: apiError}, {err: apiError}, {err: apiError},
				{statuses: deployed},
			},
			expected:      Err,
			expectedCalls: 1 + maxConsecutiveStatusErrors,
		},
		{
			name: "a success resets the error count",
			responses: []response{
				{err: apiError}, {err: apiError}, {err: apiError}, {err: apiError}, {statuses: deploying},
				{err: apiError}, {err: apiError}, {err: apiError}, {err: apiError}, {statuses: deployed},
			},
			expected:      Stop,
			expectedCalls: 10,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			fetch := func() (*qovery.EnvironmentStatusesWithStages, error) {
				if calls >= len(test.responses) {
					t.Fatalf("watchServices fetched more than the %d prepared responses", len(test.responses))
				}
				r := test.responses[calls]
				calls++
				return r.statuses, r.err
			}

			tracker := newServicesTracker([]string{"a"}, map[string]string{"a": "env-1"})
			got := watchServices(fetch, func() {}, func() bool { return false }, tracker, qovery.STATEENUM_DEPLOYED)
			if got != test.expected {
				t.Errorf("watchServices = %v, want %v", got, test.expected)
			}
			if calls != test.expectedCalls {
				t.Errorf("watchServices fetched %d times, want %d", calls, test.expectedCalls)
			}
		})
	}
}

func errorMessage(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func TestWatchServicesTimeout(t *testing.T) {
	deploying := withStages(map[string]traceServiceStatus{"a": {State: qovery.STATEENUM_DEPLOYING, ExecutionId: "env-2"}})
	calls := 0
	fetch := func() (*qovery.EnvironmentStatusesWithStages, error) {
		calls++
		return deploying, nil
	}
	expired := func() bool { return calls >= 3 }

	tracker := newServicesTracker([]string{"a"}, map[string]string{"a": "env-1"})
	if got := watchServices(fetch, func() {}, expired, tracker, qovery.STATEENUM_DEPLOYED); got != Err {
		t.Fatalf("watchServices = %v, want Err once the deadline is passed", got)
	}
	if calls != 3 {
		t.Fatalf("watchServices fetched %d times, want 3", calls)
	}
}

func TestSnapshotExecutions(t *testing.T) {
	statuses := withStages(map[string]traceServiceStatus{"a": {State: qovery.STATEENUM_DEPLOYED, ExecutionId: "env-1"}})
	apiError := errors.New("503 Service Unavailable")

	t.Run("transient errors are retried", func(t *testing.T) {
		calls := 0
		fetch := func() (*qovery.EnvironmentStatusesWithStages, error) {
			calls++
			if calls < 3 {
				return nil, apiError
			}
			return statuses, nil
		}
		before, err := snapshotExecutions(fetch, func() {})
		if err != nil {
			t.Fatalf("snapshotExecutions returned %v", err)
		}
		if before["a"] != "env-1" {
			t.Fatalf("snapshot = %v, want a: env-1", before)
		}
	})

	t.Run("errors in a row fail before the request is sent", func(t *testing.T) {
		calls := 0
		fetch := func() (*qovery.EnvironmentStatusesWithStages, error) {
			calls++
			return nil, apiError
		}
		if _, err := snapshotExecutions(fetch, func() {}); err == nil {
			t.Fatal("snapshotExecutions returned no error, want one")
		}
		if calls != maxConsecutiveStatusErrors {
			t.Fatalf("snapshotExecutions fetched %d times, want %d", calls, maxConsecutiveStatusErrors)
		}
	})
}

func TestWatchTimeout(t *testing.T) {
	tests := []struct {
		value    string
		expected time.Duration
	}{
		{"", defaultWatchTimeout},
		{"2h", 2 * time.Hour},
		{"90m", 90 * time.Minute},
		{"not-a-duration", defaultWatchTimeout},
		{"-5m", defaultWatchTimeout},
	}
	for _, test := range tests {
		t.Setenv("QOVERY_CLI_WATCH_TIMEOUT", test.value)
		if got := watchTimeout(); got != test.expected {
			t.Errorf("watchTimeout() with %q = %v, want %v", test.value, got, test.expected)
		}
	}
}

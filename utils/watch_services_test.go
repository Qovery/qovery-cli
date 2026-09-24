package utils

import (
	"errors"
	"testing"

	"github.com/qovery/qovery-client-go"
)

func TestSelectedServicesStatus(t *testing.T) {
	tests := []struct {
		name          string
		statuses      qovery.EnvironmentStatuses
		serviceIds    []string
		expectedState qovery.StateEnum
		expected      Status
		expectedDone  int
		expectedError string
	}{
		{
			name: "unrelated service still deploying is ignored",
			statuses: qovery.EnvironmentStatuses{
				Applications: []qovery.Status{statusOf("app1", qovery.STATEENUM_RESTARTED), statusOf("other", qovery.STATEENUM_DEPLOYING)},
				Containers:   []qovery.Status{statusOf("ctr1", qovery.STATEENUM_RESTARTED)},
			},
			serviceIds:    []string{"app1", "ctr1"},
			expectedState: qovery.STATEENUM_RESTARTED,
			expected:      Stop,
			expectedDone:  2,
		},
		{
			name: "unrelated service in error is ignored",
			statuses: qovery.EnvironmentStatuses{
				Containers: []qovery.Status{statusOf("ctr1", qovery.STATEENUM_RESTARTED), statusOf("other", qovery.STATEENUM_DEPLOYMENT_ERROR)},
			},
			serviceIds:    []string{"ctr1"},
			expectedState: qovery.STATEENUM_RESTARTED,
			expected:      Stop,
			expectedDone:  1,
		},
		{
			name: "one selected service still restarting",
			statuses: qovery.EnvironmentStatuses{
				Containers: []qovery.Status{statusOf("ctr1", qovery.STATEENUM_RESTARTED), statusOf("ctr2", qovery.STATEENUM_RESTARTING)},
			},
			serviceIds:    []string{"ctr1", "ctr2"},
			expectedState: qovery.STATEENUM_RESTARTED,
			expected:      Continue,
			expectedDone:  1,
		},
		{
			name: "one selected service in error",
			statuses: qovery.EnvironmentStatuses{
				Databases: []qovery.Status{statusOf("db1", qovery.STATEENUM_RESTARTING), statusOf("db2", qovery.STATEENUM_RESTART_ERROR)},
			},
			serviceIds:    []string{"db1", "db2"},
			expectedState: qovery.STATEENUM_RESTARTED,
			expected:      Err,
			expectedDone:  0,
			expectedError: "service db2 is in state RESTART_ERROR",
		},
		{
			name: "selected services across every service type",
			statuses: qovery.EnvironmentStatuses{
				Applications: []qovery.Status{statusOf("app1", qovery.STATEENUM_DEPLOYED)},
				Containers:   []qovery.Status{statusOf("ctr1", qovery.STATEENUM_DEPLOYED)},
				Databases:    []qovery.Status{statusOf("db1", qovery.STATEENUM_DEPLOYED)},
				Jobs:         []qovery.Status{statusOf("job1", qovery.STATEENUM_DEPLOYED)},
				Helms:        []qovery.Status{statusOf("helm1", qovery.STATEENUM_DEPLOYED)},
				Terraforms:   []qovery.Status{statusOf("tf1", qovery.STATEENUM_DEPLOYING)},
			},
			serviceIds:    []string{"app1", "ctr1", "db1", "job1", "helm1", "tf1"},
			expectedState: qovery.STATEENUM_DEPLOYED,
			expected:      Continue,
			expectedDone:  5,
		},
		{
			name: "service still in its previous final state is not done",
			statuses: qovery.EnvironmentStatuses{
				Applications: []qovery.Status{statusOf("app1", qovery.STATEENUM_STOPPED), statusOf("app2", qovery.STATEENUM_DEPLOYED)},
			},
			serviceIds:    []string{"app1", "app2"},
			expectedState: qovery.STATEENUM_STOPPED,
			expected:      Continue,
			expectedDone:  1,
		},
		{
			name: "deleted service no longer listed counts as done when deleting",
			statuses: qovery.EnvironmentStatuses{
				Applications: []qovery.Status{statusOf("app2", qovery.STATEENUM_DELETED)},
			},
			serviceIds:    []string{"app1", "app2"},
			expectedState: qovery.STATEENUM_DELETED,
			expected:      Stop,
			expectedDone:  2,
		},
		{
			name: "missing service is an error when not deleting",
			statuses: qovery.EnvironmentStatuses{
				Applications: []qovery.Status{statusOf("app2", qovery.STATEENUM_STOPPED)},
			},
			serviceIds:    []string{"app1", "app2"},
			expectedState: qovery.STATEENUM_STOPPED,
			expected:      Err,
			expectedDone:  0,
			expectedError: "service app1 no longer exists in the environment",
		},
		{
			name: "service still deployed from the previous deployment is not done",
			statuses: qovery.EnvironmentStatuses{
				Containers: []qovery.Status{queuedStatusOf("ctr1", qovery.STATEENUM_DEPLOYED), queuedStatusOf("ctr2", qovery.STATEENUM_DEPLOYED)},
			},
			serviceIds:    []string{"ctr1", "ctr2"},
			expectedState: qovery.STATEENUM_DEPLOYED,
			expected:      Continue,
			expectedDone:  0,
		},
		{
			name: "a queued request id alone keeps the service pending",
			statuses: qovery.EnvironmentStatuses{
				Containers: []qovery.Status{requestedStatusOf("ctr1", qovery.STATEENUM_DEPLOYED, "req-1"), statusOf("ctr2", qovery.STATEENUM_DEPLOYED)},
			},
			serviceIds:    []string{"ctr1", "ctr2"},
			expectedState: qovery.STATEENUM_DEPLOYED,
			expected:      Continue,
			expectedDone:  1,
		},
		{
			name: "a service deployed with nothing queued is done",
			statuses: qovery.EnvironmentStatuses{
				Containers: []qovery.Status{statusOf("ctr1", qovery.STATEENUM_DEPLOYED), statusOf("ctr2", qovery.STATEENUM_DEPLOYED)},
			},
			serviceIds:    []string{"ctr1", "ctr2"},
			expectedState: qovery.STATEENUM_DEPLOYED,
			expected:      Stop,
			expectedDone:  2,
		},
		{
			name: "a queued service that failed is still an error",
			statuses: qovery.EnvironmentStatuses{
				Containers: []qovery.Status{statusOf("ctr1", qovery.STATEENUM_DEPLOYED), queuedStatusOf("ctr2", qovery.STATEENUM_DEPLOYMENT_ERROR)},
			},
			serviceIds:    []string{"ctr1", "ctr2"},
			expectedState: qovery.STATEENUM_DEPLOYED,
			expected:      Err,
			expectedDone:  1,
			expectedError: "service ctr2 is in state DEPLOYMENT_ERROR",
		},
		{
			name: "canceled selected service is an error",
			statuses: qovery.EnvironmentStatuses{
				Containers: []qovery.Status{statusOf("ctr1", qovery.STATEENUM_DEPLOYED), statusOf("ctr2", qovery.STATEENUM_CANCELED)},
			},
			serviceIds:    []string{"ctr1", "ctr2"},
			expectedState: qovery.STATEENUM_DEPLOYED,
			expected:      Err,
			expectedDone:  1,
			expectedError: "service ctr2 is in state CANCELED",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, done, err := selectedServicesStatus(&test.statuses, test.serviceIds, test.expectedState)
			if got != test.expected {
				t.Errorf("selectedServicesStatus status = %v, want %v", got, test.expected)
			}
			if done != test.expectedDone {
				t.Errorf("selectedServicesStatus done = %d, want %d", done, test.expectedDone)
			}
			if gotError := errorMessage(err); gotError != test.expectedError {
				t.Errorf("selectedServicesStatus error = %q, want %q", gotError, test.expectedError)
			}
		})
	}
}

func statusOf(id string, state qovery.StateEnum) qovery.Status {
	return qovery.Status{Id: id, State: state}
}

// queuedStatusOf is a service whose deployment request is accepted but not started yet:
// it still reports the state of its previous deployment
func queuedStatusOf(id string, state qovery.StateEnum) qovery.Status {
	status := statusOf(id, state)
	status.DeploymentRequestsCount = 1
	return status
}

func requestedStatusOf(id string, state qovery.StateEnum, requestId string) qovery.Status {
	status := statusOf(id, state)
	status.DeploymentRequestId = *qovery.NewNullableString(&requestId)
	return status
}

// TestSelectedServicesStatusIgnoresNullRequestId guards the IsSet() vs Get() distinction:
// the API always sends deployment_request_id, so IsSet() is true even when it is null
func TestSelectedServicesStatusIgnoresNullRequestId(t *testing.T) {
	status := statusOf("ctr1", qovery.STATEENUM_DEPLOYED)
	status.DeploymentRequestId = *qovery.NewNullableString(nil)
	statuses := qovery.EnvironmentStatuses{Containers: []qovery.Status{status}}

	got, done, err := selectedServicesStatus(&statuses, []string{"ctr1"}, qovery.STATEENUM_DEPLOYED)
	if got != Stop || done != 1 || err != nil {
		t.Errorf("selectedServicesStatus = (%v, %d, %v), want (Stop, 1, nil)", got, done, err)
	}
}

func TestWatchServicesLoop(t *testing.T) {
	deployed := &qovery.EnvironmentStatuses{Applications: []qovery.Status{statusOf("app1", qovery.STATEENUM_DEPLOYED)}}
	deploying := &qovery.EnvironmentStatuses{Applications: []qovery.Status{statusOf("app1", qovery.STATEENUM_DEPLOYING)}}
	// deploy requested, engine not started yet: still DEPLOYED from the previous deployment
	queued := &qovery.EnvironmentStatuses{Applications: []qovery.Status{queuedStatusOf("app1", qovery.STATEENUM_DEPLOYED)}}
	apiError := errors.New("503 Service Unavailable")

	type response struct {
		statuses *qovery.EnvironmentStatuses
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
			name: "the watch does not stop while the deployment is still queued",
			responses: []response{
				{statuses: queued}, {statuses: queued}, {statuses: deploying}, {statuses: deployed},
			},
			expected:      Stop,
			expectedCalls: 4,
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
			fetch := func() (*qovery.EnvironmentStatuses, error) {
				r := test.responses[calls]
				calls++
				return r.statuses, r.err
			}

			got := watchServices(fetch, func() {}, []string{"app1"}, qovery.STATEENUM_DEPLOYED)
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

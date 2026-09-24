package utils

import (
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
			name: "missing service is not done when not deleting",
			statuses: qovery.EnvironmentStatuses{
				Applications: []qovery.Status{statusOf("app2", qovery.STATEENUM_STOPPED)},
			},
			serviceIds:    []string{"app1", "app2"},
			expectedState: qovery.STATEENUM_STOPPED,
			expected:      Continue,
			expectedDone:  1,
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
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, done := selectedServicesStatus(&test.statuses, test.serviceIds, test.expectedState)
			if got != test.expected {
				t.Errorf("selectedServicesStatus status = %v, want %v", got, test.expected)
			}
			if done != test.expectedDone {
				t.Errorf("selectedServicesStatus done = %d, want %d", done, test.expectedDone)
			}
		})
	}
}

func statusOf(id string, state qovery.StateEnum) qovery.Status {
	return qovery.Status{Id: id, State: state}
}

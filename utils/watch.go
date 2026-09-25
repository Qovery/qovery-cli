package utils

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/qovery/qovery-client-go"
	log "github.com/sirupsen/logrus"
)

// maxConsecutiveStatusErrors bounds how long a watch retries a failing status API before
// giving up, about 15 seconds with the 3 seconds poll interval
const maxConsecutiveStatusErrors = 5

// ServicesWatch follows the services targeted by one request until each of them is done with it.
//
// It reads /statusesWithStages rather than /statuses: only the former reports a service as
// *_QUEUED while a request for it waits in the environment queue. /statuses keeps showing the
// state of the previous execution (e.g. DEPLOYED), which made watchers exit before the new
// request even started.
type ServicesWatch struct {
	client     *qovery.APIClient
	envId      string
	serviceIds []string
	before     map[string]string
}

// NewServicesWatch must be called before the request is sent: it records the execution of each
// service at that time, so that Wait can tell a finished new execution from the previous one.
// It returns nil when enabled is false, and Wait on a nil watch does nothing.
func NewServicesWatch(client *qovery.APIClient, envId string, serviceIds []string, enabled bool) *ServicesWatch {
	if !enabled {
		return nil
	}

	w := &ServicesWatch{client: client, envId: envId, serviceIds: serviceIds}
	// without this snapshot, Wait only relies on seeing each service queued or in progress
	if statuses, err := w.fetch(); err == nil {
		w.before = executionIds(statuses)
	}

	return w
}

// Wait blocks until every service reached finalServiceState for this request, and exits 1 if
// one of them fails, is canceled, ends in another state or disappears
func (w *ServicesWatch) Wait(finalServiceState qovery.StateEnum) {
	if w == nil {
		return
	}

	tracker := newServicesTracker(w.serviceIds, w.before)
	sleep := func() { time.Sleep(3 * time.Second) }
	if watchServices(w.fetch, sleep, tracker, finalServiceState) == Err {
		os.Exit(1)
	}
}

func (w *ServicesWatch) fetch() (*qovery.EnvironmentStatusesWithStages, error) {
	statuses, _, err := w.client.EnvironmentMainCallsAPI.GetEnvironmentStatusesWithStages(context.Background(), w.envId).Execute()
	return statuses, err
}

func watchServices(
	fetch func() (*qovery.EnvironmentStatusesWithStages, error),
	sleep func(),
	tracker *servicesTracker,
	finalServiceState qovery.StateEnum,
) Status {
	consecutiveErrors := 0
	for {
		statuses, err := fetch()

		// a transient API error must not end the watch as a success
		if err != nil {
			consecutiveErrors++
			if consecutiveErrors >= maxConsecutiveStatusErrors {
				PrintlnErrorToStderr(fmt.Errorf("cannot get the services status after %d attempts: %w", consecutiveErrors, err))
				return Err
			}
			sleep()
			continue
		}
		consecutiveErrors = 0

		status, done, err := tracker.update(statuses, finalServiceState)

		icon := "⏳"
		if status == Stop {
			icon = "✅"
		}
		// TODO make something more fancy here to display the status. Use UILIVE or something like that
		log.Println(fmt.Sprintf("%d/%d services %s %s", done, len(tracker.serviceIds), GetStatusTextWithColor(finalServiceState), icon))

		if err != nil {
			PrintlnErrorToStderr(err)
		}
		if status != Continue {
			return status
		}

		sleep()
	}
}

type servicesTracker struct {
	serviceIds []string
	// execution id of each service when the watch started, nil if it could not be fetched
	before map[string]string
	// services seen queued or in progress since the watch started
	started map[string]bool
}

func newServicesTracker(serviceIds []string, before map[string]string) *servicesTracker {
	return &servicesTracker{serviceIds: serviceIds, before: before, started: make(map[string]bool)}
}

// update returns Err with the reason as soon as one service failed, was canceled, ended in
// another state or disappeared, Stop once they all reached finalServiceState, and how many did.
//
// A final state only counts once the service handled the request: it was seen queued or in
// progress, or its execution changed since the watch started. Otherwise the final state is
// still the one of the previous execution.
func (t *servicesTracker) update(statuses *qovery.EnvironmentStatusesWithStages, finalServiceState qovery.StateEnum) (Status, int, error) {
	byId := make(map[string]qovery.Status)
	for _, s := range allStatuses(statuses) {
		byId[s.Id] = s
	}

	done := 0
	for _, id := range t.serviceIds {
		s, found := byId[id]
		// a deleted service disappears from the environment statuses
		if !found {
			if finalServiceState == qovery.STATEENUM_DELETED {
				done++
				continue
			}
			return Err, done, fmt.Errorf("service %s no longer exists in the environment", id)
		}

		if !isFinalState(s.State) && !isErrorState(s.State) {
			t.started[id] = true
			continue
		}

		handled := t.started[id] || (t.before != nil && s.GetExecutionId() != t.before[id])
		if !handled {
			continue
		}

		switch {
		case s.State == finalServiceState:
			done++
		case isErrorState(s.State), s.State == qovery.STATEENUM_CANCELED:
			return Err, done, fmt.Errorf("service %s is in state %s", id, s.State)
		default:
			return Err, done, fmt.Errorf("service %s ended in state %s instead of %s", id, s.State, finalServiceState)
		}
	}

	if done == len(t.serviceIds) {
		return Stop, done, nil
	}

	return Continue, done, nil
}

func allStatuses(statuses *qovery.EnvironmentStatusesWithStages) []qovery.Status {
	var all []qovery.Status
	for _, stage := range statuses.Stages {
		for _, list := range [][]qovery.Status{
			stage.Applications, stage.Containers, stage.Databases,
			stage.Jobs, stage.Helms, stage.Terraforms, stage.AgenticWorkflows,
		} {
			all = append(all, list...)
		}
	}
	return all
}

func executionIds(statuses *qovery.EnvironmentStatusesWithStages) map[string]string {
	ids := make(map[string]string)
	for _, s := range allStatuses(statuses) {
		ids[s.Id] = s.GetExecutionId()
	}
	return ids
}

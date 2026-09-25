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

// defaultWatchTimeout bounds a whole watch, so a request that never runs cannot block a CI job
// forever. QOVERY_CLI_WATCH_TIMEOUT (a Go duration such as "2h") overrides it for long builds.
const defaultWatchTimeout = time.Hour

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
// Without that record the watch cannot be reliable, so it exits 1 before any request is sent.
// It returns nil when enabled is false, and Wait on a nil watch does nothing.
func NewServicesWatch(client *qovery.APIClient, envId string, serviceIds []string, enabled bool) *ServicesWatch {
	if !enabled {
		return nil
	}

	w := &ServicesWatch{client: client, envId: envId, serviceIds: serviceIds}
	before, err := snapshotExecutions(w.fetch, func() { time.Sleep(3 * time.Second) })
	if err != nil {
		PrintlnErrorToStderr(fmt.Errorf("cannot watch the services, no request was sent: %w", err))
		os.Exit(1)
	}
	w.before = before

	return w
}

func snapshotExecutions(fetch func() (*qovery.EnvironmentStatusesWithStages, error), sleep func()) (map[string]string, error) {
	var err error
	for attempt := 1; attempt <= maxConsecutiveStatusErrors; attempt++ {
		var statuses *qovery.EnvironmentStatusesWithStages
		if statuses, err = fetch(); err == nil {
			return executionIds(statuses), nil
		}
		if attempt < maxConsecutiveStatusErrors {
			sleep()
		}
	}
	return nil, fmt.Errorf("cannot get the services status after %d attempts: %w", maxConsecutiveStatusErrors, err)
}

// Wait blocks until every service reached finalServiceState for this request, and exits 1 if
// one of them fails, is canceled, ends in another state or disappears
func (w *ServicesWatch) Wait(finalServiceState qovery.StateEnum) {
	if w == nil {
		return
	}

	tracker := newServicesTracker(w.serviceIds, w.before)
	sleep := func() { time.Sleep(3 * time.Second) }
	deadline := time.Now().Add(watchTimeout())
	expired := func() bool { return time.Now().After(deadline) }
	if watchServices(w.fetch, sleep, expired, tracker, finalServiceState) == Err {
		os.Exit(1)
	}
}

func watchTimeout() time.Duration {
	value := os.Getenv("QOVERY_CLI_WATCH_TIMEOUT")
	if value == "" {
		return defaultWatchTimeout
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		PrintlnErrorToStderr(fmt.Errorf("invalid QOVERY_CLI_WATCH_TIMEOUT %q, using %s", value, defaultWatchTimeout))
		return defaultWatchTimeout
	}
	return timeout
}

func (w *ServicesWatch) fetch() (*qovery.EnvironmentStatusesWithStages, error) {
	statuses, _, err := w.client.EnvironmentMainCallsAPI.GetEnvironmentStatusesWithStages(context.Background(), w.envId).Execute()
	return statuses, err
}

func watchServices(
	fetch func() (*qovery.EnvironmentStatusesWithStages, error),
	sleep func(),
	expired func() bool,
	tracker *servicesTracker,
	finalServiceState qovery.StateEnum,
) Status {
	consecutiveErrors := 0
	for {
		if expired() {
			PrintlnErrorToStderr(fmt.Errorf("the services did not reach %s before the watch timeout, set QOVERY_CLI_WATCH_TIMEOUT to wait longer", finalServiceState))
			return Err
		}

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
	// execution id of each service when the watch started
	before map[string]string
}

func newServicesTracker(serviceIds []string, before map[string]string) *servicesTracker {
	return &servicesTracker{serviceIds: serviceIds, before: before}
}

// update returns Err with the reason as soon as one service failed, was canceled, ended in
// another state or disappeared, Stop once they all reached finalServiceState, and how many did.
//
// A state only counts once it comes from a new execution: every request starts one, so a final
// or error state under the execution recorded before the request belongs to a previous request.
// The environment queue is FIFO, and /statusesWithStages reports a service as *_QUEUED while a
// request for it waits, whatever the state of its current execution. So an execution that ends
// while our request still waits (a request sent before ours) is never seen in a final state.
// A request sent after ours runs after it and hides our end the same way: the watch then waits
// for that request and reports the state the service ends in.
func (t *servicesTracker) update(statuses *qovery.EnvironmentStatusesWithStages, finalServiceState qovery.StateEnum) (Status, int, error) {
	byId := make(map[string]qovery.Status)
	for _, s := range allStatuses(statuses) {
		byId[s.Id] = s
	}

	done := 0
	for _, id := range t.serviceIds {
		s, found := byId[id]
		if !found {
			if _, existed := t.before[id]; !existed {
				return Err, done, fmt.Errorf("service %s was not in the environment when the watch started", id)
			}
			// a deleted service disappears from the environment statuses
			if finalServiceState == qovery.STATEENUM_DELETED {
				done++
				continue
			}
			return Err, done, fmt.Errorf("service %s no longer exists in the environment", id)
		}

		// queued, in progress, or still the state of a previous execution
		if s.GetExecutionId() == t.before[id] || (!isFinalState(s.State) && !isErrorState(s.State)) {
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

package helper

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/events"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/klog/v2"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"

	. "github.com/rh-ecosystem-edge/eco-gotests/tests/cnf/ran/internal/raninittools"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/cnf/ran/talm/internal/tsparams"
)

// cguEventScopeAnnotation is the annotation TALM sets on ClusterGroupUpgrade events to indicate whether the event is
// global, batch, or cluster scoped. See TALM-events-test-plan.md.
const cguEventScopeAnnotation = "cgu.openshift.io/event-type"

// cguRegardingKind is the regarding.kind value TALM sets on every ClusterGroupUpgrade event.
const cguRegardingKind = "ClusterGroupUpgrade"

// GetCGUEvents lists events.k8s.io/v1 events regarding ClusterGroupUpgrade resources in tsparams.TestNamespace on
// the hub cluster, sorted by creation timestamp (oldest first). When cguName is non-empty, results are further
// filtered to events regarding that specific CGU (filtering in-process in addition to the server-side field
// selector, in case regarding.name filtering isn't honored by a given API server version). This mirrors the
// GetCGUEvents helper described in TALM-events-implementation-plan.md and is used here as a debug preview of real
// TALM event behavior ahead of implementing real assertions with it.
func GetCGUEvents(cguName string) ([]*eventsv1.Event, error) {
	fieldSet := fields.Set{"regarding.kind": cguRegardingKind}
	if cguName != "" {
		fieldSet["regarding.name"] = cguName
	}

	builders, err := events.ListEventV1s(HubAPIClient,
		runtimeclient.InNamespace(tsparams.TestNamespace),
		runtimeclient.MatchingFieldsSelector{Selector: fieldSet.AsSelector()})
	if err != nil {
		return nil, err
	}

	cguEvents := make([]*eventsv1.Event, 0, len(builders))

	for _, builder := range builders {
		if builder.Object == nil {
			continue
		}

		if cguName != "" && builder.Object.Regarding.Name != cguName {
			continue
		}

		cguEvents = append(cguEvents, builder.Object)
	}

	sort.Slice(cguEvents, func(i, j int) bool {
		return cguEvents[i].CreationTimestamp.Before(&cguEvents[j].CreationTimestamp)
	})

	return cguEvents, nil
}

// PrintCGUEvents is a temporary debugging helper that logs all ClusterGroupUpgrade events in tsparams.TestNamespace
// on the hub cluster. It is meant to be called from AfterEach blocks as a full-state safety-net snapshot, so events
// are captured even when a test fails for reasons unrelated to events (AfterEach always runs, even after a failed
// Expect). See TALM-events-test-plan.md / TALM-events-implementation-plan.md while investigating TALM CGU event
// behavior; this and PrintCGUEventsCheckpoint should be removed once real event assertions are implemented.
func PrintCGUEvents() {
	cguEvents, err := GetCGUEvents("")
	if err != nil {
		klog.V(tsparams.LogLevel).Infof("Failed to get CGU events in the %s namespace: %v", tsparams.TestNamespace, err)

		return
	}

	klog.V(tsparams.LogLevel).Infof(
		"CGU events in the %s namespace:\n%s", tsparams.TestNamespace, formatCGUEvents(cguEvents))
}

// PrintCGUEventsCheckpoint is a temporary debugging helper that logs the ClusterGroupUpgrade events currently
// present for cguName, labeled with checkpoint (the test milestone just reached) and expected (the event
// reasons/scopes TALM-events-test-plan.md / TALM-events-implementation-plan.md say should be present at this
// point), so actual behavior can be compared against the plans before real assertions are coded.
//
// This never fails the test: if fetching events errors, the error is logged and the function returns without
// panicking, so a broken event fetch can never mask or replace a real test failure. Call it between a milestone
// wait (e.g. WaitForCondition) and that wait's own Expect(err) assertion, so the checkpoint is captured on a
// best-effort basis even if the wait itself timed out.
func PrintCGUEventsCheckpoint(checkpoint, cguName string, expected ...string) {
	cguEvents, err := GetCGUEvents(cguName)
	if err != nil {
		klog.V(tsparams.LogLevel).Infof(
			"[%s] Failed to get CGU events for %q in the %s namespace: %v",
			checkpoint, cguName, tsparams.TestNamespace, err)

		return
	}

	klog.V(tsparams.LogLevel).Infof(
		"[%s] CGU %q events - expected: [%s], actual:\n%s",
		checkpoint, cguName, strings.Join(expected, ", "), formatCGUEvents(cguEvents))
}

// ClearCGUEvents deletes all ClusterGroupUpgrade events in tsparams.TestNamespace on the hub cluster. It is meant to
// be called from BeforeEach blocks so that events fetched later in the test only reflect what happened during the
// current test rather than accumulating across the whole suite run.
//
// This issues a single server-side DeleteCollection request (via the embedded controller-runtime client's
// DeleteAllOf, the Go equivalent of `oc delete events -n <namespace> --field-selector=...`) rather than listing
// events and deleting each one individually. Deleting one event at a time via a builder's Delete() sends a per-object
// DELETE request that silently treats a 404 as success, which can mask cases where nothing was actually deleted;
// DeleteAllOf avoids that by scoping a single collection-level delete to the namespace and field selector, matching
// the manual `oc delete events` workflow this replaces. Deletion is best-effort and logged rather than fatal, since
// it is a debug convenience rather than part of the test's real setup.
func ClearCGUEvents() {
	if err := eventsv1.AddToScheme(HubAPIClient.Scheme()); err != nil {
		klog.V(tsparams.LogLevel).Infof("Failed to attach the events/v1 scheme for clearing CGU events: %v", err)

		return
	}

	err := HubAPIClient.DeleteAllOf(context.TODO(), &eventsv1.Event{},
		runtimeclient.InNamespace(tsparams.TestNamespace),
		runtimeclient.MatchingFieldsSelector{Selector: fields.Set{"regarding.kind": cguRegardingKind}.AsSelector()})
	if err != nil {
		klog.V(tsparams.LogLevel).Infof(
			"Failed to clear CGU events in the %s namespace: %v", tsparams.TestNamespace, err)

		return
	}

	klog.V(tsparams.LogLevel).Infof("Cleared CGU events in the %s namespace", tsparams.TestNamespace)
}

// formatCGUEvents renders events as a compact, human-readable multi-line summary for debug logging: creation time,
// type, reason, scope annotation, regarding name, and note per event.
func formatCGUEvents(cguEvents []*eventsv1.Event) string {
	if len(cguEvents) == 0 {
		return "  (none)"
	}

	lines := make([]string, 0, len(cguEvents))

	for _, event := range cguEvents {
		scope := event.Annotations[cguEventScopeAnnotation]
		if scope == "" {
			scope = "-"
		}

		lines = append(lines, fmt.Sprintf(
			"  %s  %-7s %-36s scope=%-7s regarding=%-24s note=%s",
			event.CreationTimestamp.Format(time.RFC3339), event.Type, event.Reason, scope,
			event.Regarding.Name, event.Note))
	}

	return strings.Join(lines, "\n")
}

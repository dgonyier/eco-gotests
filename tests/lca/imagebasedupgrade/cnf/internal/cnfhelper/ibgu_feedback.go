package cnfhelper

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/ibgu"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/lca"
	ibguv1alpha1 "github.com/rh-ecosystem-edge/eco-goinfra/pkg/schemes/imagebasedgroupupgrades/v1alpha1"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/lca/imagebasedupgrade/cnf/internal/cnfinittools"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/lca/imagebasedupgrade/cnf/internal/cnfparams"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"
	mwv1 "open-cluster-management.io/api/work/v1"
	goclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	prepInProgressFeedbackName     = "prepInProgressConditionMessage"
	upgradeInProgressFeedbackName  = "upgradeInProgressConditionMessage"
	rollbackInProgressFeedbackName = "rollbackInProgressConditionMessage"
	idleConditionReasonName        = "idleConditionReason"

	prepInProgressFeedbackPath     = `.status.conditions[?(@.type=="PrepInProgress")].message`
	upgradeInProgressFeedbackPath  = `.status.conditions[?(@.type=="UpgradeInProgress")].message`
	rollbackInProgressFeedbackPath = `.status.conditions[?(@.type=="RollbackInProgress")].message`
	idleReasonFeedbackPath         = `.status.conditions[?(@.type=="Idle")].reason`

	// genericIBGUTimeoutMessage is what TALM writes when LCA condition feedback is missing.
	// OCPBUGS-134615 dropped that feedback because of a trailing apostrophe in the JSONPath.
	genericIBGUTimeoutMessage = "Action did not successfully complete before the timeout " +
		"specified in the rolloutStrategy"

	manifestWorkReplicaSetLabel = "work.open-cluster-management.io/manifestworkreplicaset"
	// clusterGroupUpgradeLabel is set by TALM on the ManifestWork for one CGU plan item.
	clusterGroupUpgradeLabel = "openshift-cluster-group-upgrades/clusterGroupUpgrade"
)

// spokeConditionField selects which ImageBasedUpgrade condition field is compared to ManifestWork feedback.
type spokeConditionField int

// spokeTextHistory is the non-empty condition text seen while a feedback check runs.
// TALM copies whichever message is current when it reads status, so a later sample
// replaces the one that was current when the ManifestWork first updated.
type spokeTextHistory struct {
	texts   []string
	sawWork bool
}

const (
	spokeConditionMessage spokeConditionField = iota
	spokeConditionReason
)

var manifestWorkReplicaSetGVR = schema.GroupVersionResource{
	Group:    "work.open-cluster-management.io",
	Version:  "v1alpha1",
	Resource: "manifestworkreplicasets",
}

type manifestWorkReplicaSetDocument struct {
	Spec struct {
		ManifestWorkTemplate struct {
			ManifestConfigs []struct {
				FeedbackRules []struct {
					JSONPaths []struct {
						Name string `json:"name"`
						Path string `json:"path"`
					} `json:"jsonPaths"`
				} `json:"feedbackRules"`
			} `json:"manifestConfigs"`
		} `json:"manifestWorkTemplate"`
	} `json:"spec"`
}

// WaitForPrepFeedbackPath waits until the Prep ManifestWorkReplicaSet JSONPath for
// prepInProgressConditionMessage is the LCA PrepInProgress message path.
func WaitForPrepFeedbackPath(ibguName string, timeout time.Duration) error {
	return waitForFeedbackJSONPath(
		ibguName, ibguv1alpha1.Prep, prepInProgressFeedbackName, prepInProgressFeedbackPath, timeout)
}

// WaitForUpgradeFeedbackPath waits until the Upgrade ManifestWorkReplicaSet JSONPath for
// upgradeInProgressConditionMessage is the LCA UpgradeInProgress message path.
func WaitForUpgradeFeedbackPath(ibguName string, timeout time.Duration) error {
	return waitForFeedbackJSONPath(
		ibguName, ibguv1alpha1.Upgrade, upgradeInProgressFeedbackName, upgradeInProgressFeedbackPath, timeout)
}

// WaitForRollbackFeedbackPath waits until the Rollback ManifestWorkReplicaSet JSONPath for
// rollbackInProgressConditionMessage is the LCA RollbackInProgress message path.
func WaitForRollbackFeedbackPath(ibguName string, timeout time.Duration) error {
	return waitForFeedbackJSONPath(
		ibguName, ibguv1alpha1.Rollback, rollbackInProgressFeedbackName, rollbackInProgressFeedbackPath, timeout)
}

// WaitForIdleReasonFeedbackPath waits until the action ManifestWorkReplicaSet JSONPath for
// idleConditionReason is the LCA Idle reason path. action is Abort or a Finalize action.
func WaitForIdleReasonFeedbackPath(ibguName, action string, timeout time.Duration) error {
	return waitForFeedbackJSONPath(ibguName, action, idleConditionReasonName, idleReasonFeedbackPath, timeout)
}

// WaitForPrepInProgressFeedback waits until a hub ManifestWork for the Prep replica set reports
// prepInProgressConditionMessage equal to the spoke PrepInProgress message.
// If Prep completes first, the rollout timeout was too long to reproduce a timed-out Prep.
func WaitForPrepInProgressFeedback(ibguName, clusterNamespace string, timeout time.Duration) (string, error) {
	return waitForSpokeFeedback(
		ibguName, ibguv1alpha1.Prep, clusterNamespace,
		prepInProgressFeedbackName, conditionPrepInProgress, spokeConditionMessage,
		SpokePrepCompleted, timeout)
}

// WaitForUpgradeInProgressFeedback waits until a hub ManifestWork for the Upgrade replica set reports
// upgradeInProgressConditionMessage equal to the spoke UpgradeInProgress message.
func WaitForUpgradeInProgressFeedback(ibguName, clusterNamespace string, timeout time.Duration) (string, error) {
	return waitForSpokeFeedback(
		ibguName, ibguv1alpha1.Upgrade, clusterNamespace,
		upgradeInProgressFeedbackName, conditionUpgradeInProgress, spokeConditionMessage,
		"", timeout)
}

// WaitForRollbackInProgressFeedback waits until a hub ManifestWork for the Rollback replica set reports
// rollbackInProgressConditionMessage equal to the spoke RollbackInProgress message.
func WaitForRollbackInProgressFeedback(ibguName, clusterNamespace string, timeout time.Duration) (string, error) {
	return waitForSpokeFeedback(
		ibguName, ibguv1alpha1.Rollback, clusterNamespace,
		rollbackInProgressFeedbackName, conditionRollbackInProgress, spokeConditionMessage,
		"", timeout)
}

// WaitForIdleReasonFeedback waits until a hub ManifestWork for the action replica set reports
// idleConditionReason equal to the spoke Idle condition reason.
func WaitForIdleReasonFeedback(ibguName, action, clusterNamespace string, timeout time.Duration) (string, error) {
	return waitForSpokeFeedback(
		ibguName, action, clusterNamespace,
		idleConditionReasonName, conditionIdle, spokeConditionReason,
		"", timeout)
}

// WaitForFailedActionMessage waits until the IBGU reports a failed action whose message contains
// LCA condition text and is not only the generic rollout-timeout fallback.
// wantMessage is the first text observed on the spoke. Later condition messages are recorded
// too, because TALM joins every ConditionMessage feedback value at timeout into one string.
func WaitForFailedActionMessage(
	ibguName, clusterName, action, wantMessage string, timeout time.Duration) error {
	if err := validateHubClient(ibguName); err != nil {
		return err
	}

	if clusterName == "" {
		return fmt.Errorf("cluster name is empty")
	}

	if wantMessage == "" {
		return fmt.Errorf("expected failed action message is empty")
	}

	builder := ibgu.NewIbguBuilder(cnfinittools.TargetHubAPIClient, ibguName, IbguNamespace)
	if builder == nil {
		return fmt.Errorf("failed to build IBGU client for %s", ibguName)
	}

	var (
		lastStatus string
		history    spokeTextHistory
	)

	history.add(wantMessage)

	pollErr := wait.PollUntilContextTimeout(
		context.TODO(), ibguPollInterval, timeout, true,
		func(_ context.Context) (bool, error) {
			recordActionConditionTexts(action, &history)

			object, err := builder.Get()
			if err != nil {
				lastStatus = err.Error()
				klog.V(cnfparams.CNFLogLevel).Infof("waiting for IBGU %s failed action %s: %v", ibguName, action, err)

				return false, nil
			}

			message, found := failedActionMessage(object, clusterName, action)
			lastStatus = fmt.Sprintf("%s failedAction=%q observed=%q", formatIbgu(object), message, history.texts)

			if failedActionHasLCAText(message, found, &history) {
				return true, nil
			}

			if failErr := unexpectedFailedActionOutcome(
				ibguName, action, message, found, object, &history); failErr != nil {
				return false, failErr
			}

			klog.V(cnfparams.CNFLogLevel).Infof("waiting for IBGU failed action %s; current: %s", action, lastStatus)

			return false, nil
		})
	if pollErr != nil {
		return fmt.Errorf("IBGU %s action %s did not report LCA condition text within %s: %w (last: %s)",
			ibguName, action, timeout, pollErr, lastStatus)
	}

	return nil
}

// waitForFeedbackJSONPath waits until the action ManifestWorkReplicaSet has feedbackName
// with expectedPath and no trailing apostrophe.
func waitForFeedbackJSONPath(ibguName, action, feedbackName, expectedPath string, timeout time.Duration) error {
	if err := validateHubClient(ibguName); err != nil {
		return err
	}

	if action == "" {
		return fmt.Errorf("ibgu action is empty")
	}

	replicaSetName := manifestWorkReplicaSetName(ibguName, action)

	var lastStatus string

	pollErr := wait.PollUntilContextTimeout(
		context.TODO(), ibguPollInterval, timeout, true,
		func(_ context.Context) (bool, error) {
			feedbackPath, err := manifestWorkReplicaSetFeedbackPath(replicaSetName, feedbackName)
			if err != nil {
				if k8serrors.IsNotFound(err) {
					lastStatus = err.Error()
					klog.V(cnfparams.CNFLogLevel).Infof(
						"waiting for ManifestWorkReplicaSet %s: %v", replicaSetName, err)

					return false, nil
				}

				return false, err
			}

			if feedbackPath != expectedPath || strings.HasSuffix(feedbackPath, "'") {
				return false, fmt.Errorf(
					"ManifestWorkReplicaSet %s feedback %s path is %q, want %q",
					replicaSetName, feedbackName, feedbackPath, expectedPath)
			}

			return true, nil
		})
	if pollErr != nil {
		return fmt.Errorf(
			"ManifestWorkReplicaSet %s feedback %s path was not %q within %s: %w (last: %s)",
			replicaSetName, feedbackName, expectedPath, timeout, pollErr, lastStatus)
	}

	return nil
}

// waitForSpokeFeedback waits until ManifestWork feedback equals the live spoke IBU condition field.
// failIf, when set, fails the wait if the spoke reaches that state before the feedback matches.
func waitForSpokeFeedback(
	ibguName, action, clusterNamespace, feedbackName, conditionType string,
	field spokeConditionField, failIf SpokeIBUState, timeout time.Duration,
) (string, error) {
	if err := validateHubClient(ibguName); err != nil {
		return "", err
	}

	if clusterNamespace == "" {
		return "", fmt.Errorf("cluster namespace is empty")
	}

	replicaSetName := manifestWorkReplicaSetName(ibguName, action)

	var (
		lastStatus string
		matched    string
		history    spokeTextHistory
	)

	pollErr := wait.PollUntilContextTimeout(
		context.TODO(), ibguPollInterval, timeout, true,
		func(_ context.Context) (bool, error) {
			var (
				done      bool
				sampleErr error
			)

			matched, done, sampleErr = spokeFeedbackPoll(
				clusterNamespace, replicaSetName, feedbackName, conditionType, field, failIf, &history)
			if sampleErr != nil {
				return false, sampleErr
			}

			if done {
				return true, nil
			}

			lastStatus = matched
			klog.V(cnfparams.CNFLogLevel).Infof("waiting for ManifestWork feedback %s; current: %s", feedbackName, lastStatus)

			return false, nil
		})
	if pollErr != nil {
		return "", fmt.Errorf(
			"ManifestWork in %s did not report %s equal to spoke %s within %s: %w (last: %s)",
			clusterNamespace, feedbackName, conditionType, timeout, pollErr, lastStatus)
	}

	return matched, nil
}

// spokeFeedbackPoll compares one sample of spoke condition text with hub ManifestWork feedback.
// The string result is the matched feedback, or a status description when still waiting.
func spokeFeedbackPoll(
	clusterNamespace, replicaSetName, feedbackName, conditionType string,
	field spokeConditionField, failIf SpokeIBUState, history *spokeTextHistory,
) (string, bool, error) {
	want, imageUpgrade, pullStatus := spokeConditionSample(conditionType, field, history)
	if pullStatus != "" {
		return pullStatus, false, nil
	}

	values, belonging, listErr := manifestWorkFeedbackValues(clusterNamespace, replicaSetName, feedbackName)
	if listErr != nil {
		return listErr.Error(), false, nil
	}

	history.noteWork(belonging)

	if matched := history.equal(values); matched != "" {
		return matched, true, nil
	}

	if imageUpgrade != nil && failIf != "" && matchesSpokeState(imageUpgrade, failIf) {
		return "", false, fmt.Errorf(
			"rollout timeout was too long: spoke IBU reached %s before ManifestWork reported %s (%s)",
			failIf, feedbackName, formatIBU(imageUpgrade))
	}

	if feedbackNoLongerPossible(imageUpgrade, belonging, history) {
		return "", false, fmt.Errorf(
			"spoke %s was cleared and no ManifestWork for %s remains; observed %q",
			conditionType, replicaSetName, history.texts)
	}

	return feedbackWaitStatus(
		clusterNamespace, replicaSetName, values, conditionType, want, imageUpgrade), false, nil
}

// spokeConditionSample reads the live spoke condition text. After the spoke API drops the
// ImageBasedUpgrade during pivot, earlier non-empty text stays in history so ManifestWork
// feedback can still be compared to a message that was on the spoke.
func spokeConditionSample(
	conditionType string, field spokeConditionField, history *spokeTextHistory,
) (string, *lca.ImageBasedUpgradeBuilder, string) {
	imageUpgrade, err := pullSpokeIBU()
	if err != nil {
		if history.len() > 0 {
			return "", nil, ""
		}

		return "", nil, err.Error()
	}

	want := spokeConditionText(imageUpgrade, conditionType, field)
	history.add(want)

	return want, imageUpgrade, ""
}

// feedbackNoLongerPossible reports that a ManifestWork which had been created is gone.
// The check stops instead of polling until timeout after TALM deletes the work.
func feedbackNoLongerPossible(
	imageUpgrade *lca.ImageBasedUpgradeBuilder, belonging int, history *spokeTextHistory,
) bool {
	return imageUpgrade != nil && history.sawWork && belonging == 0 && history.len() > 0
}

// failedActionHasLCAText reports that TALM copied LCA condition text into the failed action.
func failedActionHasLCAText(message string, found bool, history *spokeTextHistory) bool {
	return found && message != "" && message != genericIBGUTimeoutMessage && history.containsFragment(message)
}

// unexpectedFailedActionOutcome fail-fasts when the IBGU can no longer gain the LCA condition text.
// A generic timeout message is final: TALM does not replace the first failed-action entry.
func unexpectedFailedActionOutcome(
	ibguName, action, message string, found bool, object *ibguv1alpha1.ImageBasedGroupUpgrade,
	history *spokeTextHistory,
) error {
	if found && message == genericIBGUTimeoutMessage {
		return fmt.Errorf(
			"IBGU %s action %s failed with only the generic timeout message %q",
			ibguName, action, message)
	}

	if !ibguReasonIs(object, reasonCompleted) {
		return nil
	}

	if !found {
		return fmt.Errorf(
			"IBGU %s completed before action %s timed out; the rollout timeout is too long to leave a failed action",
			ibguName, action)
	}

	return fmt.Errorf(
		"IBGU %s action %s completed with %q, which does not include observed LCA text %q",
		ibguName, action, message, history.texts)
}

// recordActionConditionTexts stores the current LCA messages for action.
func recordActionConditionTexts(action string, history *spokeTextHistory) {
	imageUpgrade, err := pullSpokeIBU()
	if err != nil {
		return
	}

	for _, sample := range conditionSamplesForAction(action) {
		history.add(spokeConditionText(imageUpgrade, sample.conditionType, sample.field))
	}
}

// conditionSample is one IBU condition field TALM may copy into a failed action.
type conditionSample struct {
	conditionType string
	field         spokeConditionField
}

// conditionSamplesForAction lists the condition fields whose text TALM joins for action.
func conditionSamplesForAction(action string) []conditionSample {
	switch action {
	case ibguv1alpha1.Prep:
		return []conditionSample{
			{conditionPrepInProgress, spokeConditionMessage},
			{conditionPrepCompleted, spokeConditionMessage},
		}
	case ibguv1alpha1.Upgrade:
		return []conditionSample{
			{conditionUpgradeInProgress, spokeConditionMessage},
			{conditionUpgradeCompleted, spokeConditionMessage},
		}
	case ibguv1alpha1.Rollback:
		return []conditionSample{
			{conditionRollbackInProgress, spokeConditionMessage},
			{conditionRollbackCompleted, spokeConditionMessage},
		}
	default:
		return []conditionSample{
			{conditionIdle, spokeConditionReason},
			{conditionIdle, spokeConditionMessage},
		}
	}
}

func (history *spokeTextHistory) add(text string) {
	if history == nil {
		return
	}

	text = strings.TrimSpace(text)
	if text == "" {
		return
	}

	for _, existing := range history.texts {
		if existing == text {
			return
		}
	}

	history.texts = append(history.texts, text)
}

func (history *spokeTextHistory) len() int {
	if history == nil {
		return 0
	}

	return len(history.texts)
}

func (history *spokeTextHistory) noteWork(belonging int) {
	if history != nil && belonging > 0 {
		history.sawWork = true
	}
}

// equal returns the feedback value that exactly matches a recorded spoke condition text.
func (history *spokeTextHistory) equal(values []string) string {
	if history == nil {
		return ""
	}

	for _, value := range values {
		for _, text := range history.texts {
			if value == text {
				return value
			}
		}
	}

	return ""
}

// containsFragment reports whether message includes a recorded spoke condition text.
// TALM joins several feedback values with newlines, so the failed action is not one field.
func (history *spokeTextHistory) containsFragment(message string) bool {
	if history == nil {
		return false
	}

	for _, text := range history.texts {
		if text != "" && strings.Contains(message, text) {
			return true
		}
	}

	return false
}

// failedActionMessage returns the failed action message for clusterName and action.
func failedActionMessage(object *ibguv1alpha1.ImageBasedGroupUpgrade, clusterName, action string) (string, bool) {
	if object == nil {
		return "", false
	}

	for _, clusterState := range object.Status.Clusters {
		if clusterState.Name != clusterName {
			continue
		}

		for _, failedAction := range clusterState.FailedActions {
			if failedAction.Action == action {
				return failedAction.Message, true
			}
		}
	}

	return "", false
}

// manifestWorkReplicaSetName is the TALM ManifestWorkReplicaSet name for one IBGU action.
func manifestWorkReplicaSetName(ibguName, action string) string {
	return strings.ToLower(ibguName + "-" + action)
}

// validateHubClient rejects an empty IBGU name or a missing hub client.
func validateHubClient(ibguName string) error {
	if cnfinittools.TargetHubAPIClient == nil {
		return fmt.Errorf("target hub API client is nil")
	}

	if ibguName == "" {
		return fmt.Errorf("ibgu name is empty")
	}

	return nil
}

// manifestWorkReplicaSetFeedbackPath reads one feedback JSONPath from the hub replica set.
// The v1alpha1 type is not in the vendored OCM API, so the object is read with the dynamic client.
func manifestWorkReplicaSetFeedbackPath(replicaSetName, feedbackName string) (string, error) {
	if cnfinittools.TargetHubAPIClient == nil || cnfinittools.TargetHubAPIClient.Interface == nil {
		return "", fmt.Errorf("target hub dynamic client is nil")
	}

	replicaSet, err := cnfinittools.TargetHubAPIClient.Resource(manifestWorkReplicaSetGVR).
		Namespace(IbguNamespace).
		Get(context.TODO(), replicaSetName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get ManifestWorkReplicaSet %s/%s: %w", IbguNamespace, replicaSetName, err)
	}

	return feedbackPathFromReplicaSet(replicaSet, feedbackName)
}

// feedbackPathFromReplicaSet returns the JSONPath stored for feedbackName.
func feedbackPathFromReplicaSet(replicaSet *unstructured.Unstructured, feedbackName string) (string, error) {
	if replicaSet == nil {
		return "", fmt.Errorf("manifestworkreplicaset object is nil")
	}

	encoded, err := json.Marshal(replicaSet.Object)
	if err != nil {
		return "", fmt.Errorf("failed to encode ManifestWorkReplicaSet: %w", err)
	}

	var decoded manifestWorkReplicaSetDocument
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		return "", fmt.Errorf("failed to decode ManifestWorkReplicaSet: %w", err)
	}

	for _, manifestConfig := range decoded.Spec.ManifestWorkTemplate.ManifestConfigs {
		for _, feedbackRule := range manifestConfig.FeedbackRules {
			for _, jsonPath := range feedbackRule.JSONPaths {
				if jsonPath.Name == feedbackName {
					return jsonPath.Path, nil
				}
			}
		}
	}

	return "", fmt.Errorf("ManifestWorkReplicaSet feedback %s is missing", feedbackName)
}

// manifestWorkFeedbackValues lists non-empty feedback strings for replicaSetName in the spoke namespace.
// belonging is how many ManifestWorks were selected for that replica set.
func manifestWorkFeedbackValues(
	clusterNamespace, replicaSetName, feedbackName string,
) ([]string, int, error) {
	works, err := listSpokeManifestWorks(clusterNamespace)
	if err != nil {
		return nil, 0, err
	}

	var (
		values    []string
		belonging int
	)

	for workIndex := range works {
		work := &works[workIndex]
		if !workBelongsToReplicaSet(work, replicaSetName) {
			continue
		}

		belonging++

		values = append(values, feedbackStrings(work, feedbackName)...)
	}

	return values, belonging, nil
}

// listSpokeManifestWorks returns hub ManifestWorks in the managed cluster namespace.
func listSpokeManifestWorks(clusterNamespace string) ([]mwv1.ManifestWork, error) {
	if cnfinittools.TargetHubAPIClient == nil {
		return nil, fmt.Errorf("target hub API client is nil")
	}

	if err := cnfinittools.TargetHubAPIClient.AttachScheme(mwv1.Install); err != nil {
		return nil, fmt.Errorf("failed to attach ManifestWork scheme: %w", err)
	}

	workList := &mwv1.ManifestWorkList{}

	err := cnfinittools.TargetHubAPIClient.List(
		context.TODO(), workList, goclient.InNamespace(clusterNamespace))
	if err != nil {
		return nil, fmt.Errorf("failed to list ManifestWorks in %s: %w", clusterNamespace, err)
	}

	return workList.Items, nil
}

// workBelongsToReplicaSet reports whether the ManifestWork was generated for replicaSetName.
// The OCM replica-set controller names the work "<IBGU namespace>.<replica set>-<suffix>".
// TALM also labels it with the ClusterGroupUpgrade "<replica set>-<plan index>".
func workBelongsToReplicaSet(work *mwv1.ManifestWork, replicaSetName string) bool {
	if work == nil || replicaSetName == "" {
		return false
	}

	if manifestWorkNameMatchesReplicaSet(work.Name, replicaSetName) {
		return true
	}

	if work.Labels == nil {
		return false
	}

	owner := work.Labels[manifestWorkReplicaSetLabel]
	if owner == replicaSetName || owner == IbguNamespace+"."+replicaSetName {
		return true
	}

	cguName := work.Labels[clusterGroupUpgradeLabel]

	return cguName == replicaSetName || strings.HasPrefix(cguName, replicaSetName+"-")
}

// manifestWorkNameMatchesReplicaSet reports whether workName is the replica set's ManifestWork.
func manifestWorkNameMatchesReplicaSet(workName, replicaSetName string) bool {
	if workName == replicaSetName || strings.HasPrefix(workName, replicaSetName+"-") {
		return true
	}

	return strings.HasPrefix(workName, IbguNamespace+"."+replicaSetName+"-")
}

// feedbackWaitStatus describes the current feedback sample, including hub works that mention
// the replica set when none of them contributed a value.
func feedbackWaitStatus(
	clusterNamespace, replicaSetName string, values []string, conditionType, want string,
	imageUpgrade *lca.ImageBasedUpgradeBuilder,
) string {
	ibu := "imagebasedupgrade unavailable"
	if imageUpgrade != nil {
		ibu = formatIBU(imageUpgrade)
	}

	status := fmt.Sprintf("feedback=%v spoke %s=%q (%s)", values, conditionType, want, ibu)
	if len(values) != 0 {
		return status
	}

	return status + " " + manifestWorksMentioningReplicaSet(clusterNamespace, replicaSetName)
}

// manifestWorksMentioningReplicaSet lists spoke ManifestWorks whose name or TALM label contains replicaSetName.
func manifestWorksMentioningReplicaSet(clusterNamespace, replicaSetName string) string {
	works, err := listSpokeManifestWorks(clusterNamespace)
	if err != nil {
		return "works=" + err.Error()
	}

	var related []string

	for workIndex := range works {
		work := &works[workIndex]
		if !strings.Contains(work.Name, replicaSetName) &&
			!strings.Contains(work.Labels[clusterGroupUpgradeLabel], replicaSetName) &&
			!strings.Contains(work.Labels[manifestWorkReplicaSetLabel], replicaSetName) {
			continue
		}

		related = append(related, fmt.Sprintf("%s mwrs=%q cgu=%q %s",
			work.Name,
			work.Labels[manifestWorkReplicaSetLabel],
			work.Labels[clusterGroupUpgradeLabel],
			describeWorkFeedback(work)))
	}

	if len(related) == 0 {
		return fmt.Sprintf("works=none mentioning %s (%d in %s)", replicaSetName, len(works), clusterNamespace)
	}

	return "works=[" + strings.Join(related, "; ") + "]"
}

// feedbackStrings collects non-empty status feedback values with feedbackName.
// JSONPath filters can be stored as a raw JSON string as well as a plain string.
func feedbackStrings(work *mwv1.ManifestWork, feedbackName string) []string {
	if work == nil {
		return nil
	}

	var values []string

	for _, manifest := range work.Status.ResourceStatus.Manifests {
		for _, feedback := range manifest.StatusFeedbacks.Values {
			if feedback.Name != feedbackName {
				continue
			}

			values = append(values, feedbackValueTexts(feedback.Value)...)
		}
	}

	return values
}

// feedbackValueTexts returns the string forms of one feedback field.
func feedbackValueTexts(value mwv1.FieldValue) []string {
	if value.String != nil {
		return nonEmptyTexts([]string{*value.String})
	}

	if value.JsonRaw == nil {
		return nil
	}

	raw := strings.TrimSpace(*value.JsonRaw)
	if raw == "" {
		return nil
	}

	var one string
	if err := json.Unmarshal([]byte(raw), &one); err == nil {
		return nonEmptyTexts([]string{one})
	}

	var many []string
	if err := json.Unmarshal([]byte(raw), &many); err == nil {
		return nonEmptyTexts(many)
	}

	return nil
}

// nonEmptyTexts trims and drops blank feedback strings.
func nonEmptyTexts(texts []string) []string {
	var values []string

	for _, text := range texts {
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		values = append(values, text)
	}

	return values
}

// describeWorkFeedback summarizes status feedback names on one ManifestWork.
func describeWorkFeedback(work *mwv1.ManifestWork) string {
	if work == nil {
		return "statusFeedback=[]"
	}

	var parts []string

	for _, manifest := range work.Status.ResourceStatus.Manifests {
		for _, feedback := range manifest.StatusFeedbacks.Values {
			parts = append(parts, feedback.Name+"="+feedbackValuePreview(feedback.Value))
			if len(parts) == 4 {
				return "statusFeedback=[" + strings.Join(parts, ", ") + ", ...]"
			}
		}
	}

	if len(parts) == 0 {
		return "statusFeedback=[]"
	}

	return "statusFeedback=[" + strings.Join(parts, ", ") + "]"
}

// feedbackValuePreview is a short log form of one feedback field.
func feedbackValuePreview(value mwv1.FieldValue) string {
	texts := feedbackValueTexts(value)
	if len(texts) == 0 {
		return fmt.Sprintf("%q", string(value.Type))
	}

	text := texts[0]
	if len(text) > 60 {
		text = text[:60] + "..."
	}

	return fmt.Sprintf("%q", text)
}

// spokeConditionText returns the spoke condition message or reason.
func spokeConditionText(
	imageUpgrade *lca.ImageBasedUpgradeBuilder, conditionType string, field spokeConditionField,
) string {
	if field == spokeConditionReason {
		return strings.TrimSpace(conditionReason(imageUpgrade, conditionType))
	}

	condition, found := findCondition(imageUpgrade, conditionType)
	if !found {
		return ""
	}

	return strings.TrimSpace(condition.Message)
}

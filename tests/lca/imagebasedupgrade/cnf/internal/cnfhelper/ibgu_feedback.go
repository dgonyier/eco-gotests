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
)

// spokeConditionField selects which ImageBasedUpgrade condition field is compared to ManifestWork feedback.
type spokeConditionField int

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
// wantMessage and is not only the generic rollout-timeout fallback.
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

	var lastStatus string

	pollErr := wait.PollUntilContextTimeout(
		context.TODO(), ibguPollInterval, timeout, true,
		func(_ context.Context) (bool, error) {
			object, err := builder.Get()
			if err != nil {
				lastStatus = err.Error()
				klog.V(cnfparams.CNFLogLevel).Infof("waiting for IBGU %s failed action %s: %v", ibguName, action, err)

				return false, nil
			}

			message, found := failedActionMessage(object, clusterName, action)
			lastStatus = fmt.Sprintf("%s failedAction=%q", formatIbgu(object), message)

			if found && message != genericIBGUTimeoutMessage && strings.Contains(message, wantMessage) {
				return true, nil
			}

			if failErr := unexpectedFailedActionOutcome(ibguName, action, message, found, object); failErr != nil {
				return false, failErr
			}

			klog.V(cnfparams.CNFLogLevel).Infof("waiting for IBGU failed action %s; current: %s", action, lastStatus)

			return false, nil
		})
	if pollErr != nil {
		return fmt.Errorf("IBGU %s action %s did not report %q within %s: %w (last: %s)",
			ibguName, action, wantMessage, timeout, pollErr, lastStatus)
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
	)

	pollErr := wait.PollUntilContextTimeout(
		context.TODO(), ibguPollInterval, timeout, true,
		func(_ context.Context) (bool, error) {
			var (
				done      bool
				sampleErr error
			)

			matched, done, sampleErr = spokeFeedbackPoll(
				clusterNamespace, replicaSetName, feedbackName, conditionType, field, failIf)
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
	field spokeConditionField, failIf SpokeIBUState,
) (string, bool, error) {
	imageUpgrade, err := pullSpokeIBU()
	if err != nil {
		return err.Error(), false, nil
	}

	want := spokeConditionText(imageUpgrade, conditionType, field)

	values, listErr := manifestWorkFeedbackValues(clusterNamespace, replicaSetName, feedbackName)
	if listErr != nil {
		return listErr.Error(), false, nil
	}

	for _, value := range values {
		if value == want && want != "" {
			return value, true, nil
		}
	}

	if failIf != "" && matchesSpokeState(imageUpgrade, failIf) {
		return "", false, fmt.Errorf(
			"rollout timeout was too long: spoke IBU reached %s before ManifestWork reported %s (%s)",
			failIf, feedbackName, formatIBU(imageUpgrade))
	}

	return fmt.Sprintf("feedback=%v spoke %s=%q (%s)", values, conditionType, want, formatIBU(imageUpgrade)), false, nil
}

// unexpectedFailedActionOutcome fail-fasts when the IBGU can no longer gain the LCA condition text.
func unexpectedFailedActionOutcome(
	ibguName, action, message string, found bool, object *ibguv1alpha1.ImageBasedGroupUpgrade,
) error {
	if !ibguReasonIs(object, reasonCompleted) {
		return nil
	}

	if !found {
		return fmt.Errorf(
			"IBGU %s completed before action %s timed out; the rollout timeout is too long to leave a failed action",
			ibguName, action)
	}

	if message == genericIBGUTimeoutMessage {
		return fmt.Errorf(
			"IBGU %s action %s failed with only the generic timeout message %q",
			ibguName, action, message)
	}

	return nil
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
func manifestWorkFeedbackValues(clusterNamespace, replicaSetName, feedbackName string) ([]string, error) {
	works, err := listSpokeManifestWorks(clusterNamespace)
	if err != nil {
		return nil, err
	}

	var values []string

	for workIndex := range works {
		work := &works[workIndex]
		if !workBelongsToReplicaSet(work, replicaSetName) {
			continue
		}

		values = append(values, feedbackStrings(work, feedbackName)...)
	}

	return values, nil
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
func workBelongsToReplicaSet(work *mwv1.ManifestWork, replicaSetName string) bool {
	if work == nil {
		return false
	}

	if work.Labels != nil {
		owner := work.Labels[manifestWorkReplicaSetLabel]
		if owner == IbguNamespace+"."+replicaSetName || owner == replicaSetName {
			return true
		}
	}

	return strings.HasPrefix(work.Name, replicaSetName)
}

// feedbackStrings collects non-empty status feedback values with feedbackName.
func feedbackStrings(work *mwv1.ManifestWork, feedbackName string) []string {
	if work == nil {
		return nil
	}

	var values []string

	for _, manifest := range work.Status.ResourceStatus.Manifests {
		for _, feedback := range manifest.StatusFeedbacks.Values {
			if feedback.Name != feedbackName || feedback.Value.String == nil {
				continue
			}

			text := strings.TrimSpace(*feedback.Value.String)
			if text == "" {
				continue
			}

			values = append(values, text)
		}
	}

	return values
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

package upgrade_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/rh-ecosystem-edge/eco-goinfra/pkg/reportxml"
	ibguv1alpha1 "github.com/rh-ecosystem-edge/eco-goinfra/pkg/schemes/imagebasedgroupupgrades/v1alpha1"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/lca/imagebasedupgrade/cnf/internal/cnfclusterinfo"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/lca/imagebasedupgrade/cnf/internal/cnfhelper"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/lca/imagebasedupgrade/cnf/upgrade-talm/internal/tsparams"
)

const (
	// prepFeedbackRolloutTimeoutMinutes is shorter than stateroot and precache on the
	// spoke, so the CGU times out while PrepInProgress still has a message.
	prepFeedbackRolloutTimeoutMinutes = 2
	abortAfterPrepTimeoutMinutes      = 20

	manifestWorkReplicaSetPathTimeout = 3 * time.Minute
	prepInProgressFeedbackTimeout     = 12 * time.Minute
	prepFailedActionTimeout           = 20 * time.Minute
	abortIdleReasonFeedbackTimeout    = 20 * time.Minute
	abortToIdleTimeout                = 20 * time.Minute
)

var _ = Describe(
	"Verifying IBGU surfaces LCA condition feedback when Prep times out",
	Label(tsparams.LabelPrepTimeoutFeedback), func() {
		BeforeEach(func() {
			By("Ensuring the spoke IBU is Idle and ready for Prep")

			Expect(cnfhelper.EnsureSpokeReadyForNextIbgu()).To(Succeed(),
				"spoke IBU was not Idle with Prep in validNextStages before prep timeout feedback")

			By("Fetching target sno cluster name")

			err := cnfclusterinfo.PreUpgradeClusterInfo.SaveClusterInfo()
			Expect(err).ToNot(HaveOccurred(), "Failed to extract target sno cluster name")

			tsparams.TargetSnoClusterName = cnfclusterinfo.PreUpgradeClusterInfo.Name
		})

		AfterEach(func() {
			deleteIbgusAndRecoverIfFailed()
		})

		// OCPBUGS-134615: a stray quote in the Prep feedback JSONPath dropped the LCA
		// PrepInProgress message from ManifestWork status and from IBGU failedActions.
		// Abort returns the spoke to Idle for another run.
		It("Surfaces the LCA PrepInProgress message when Prep times out", reportxml.ID("no-testcase"), func() {
			By("Creating Prep IBGU with a short rollout timeout, followed by Abort")

			_, err := cnfhelper.NewIbgu(tsparams.PrepTimeoutIbguName).
				WithPlan([]string{ibguv1alpha1.Prep}, 1, prepFeedbackRolloutTimeoutMinutes).
				WithPlan([]string{ibguv1alpha1.Abort}, 1, abortAfterPrepTimeoutMinutes).
				Create()
			Expect(err).ToNot(HaveOccurred(), "Failed to create prep timeout IBGU")

			By("Verifying the Prep ManifestWorkReplicaSet feedback JSONPath")

			Expect(cnfhelper.WaitForPrepFeedbackPath(tsparams.PrepTimeoutIbguName, manifestWorkReplicaSetPathTimeout)).
				To(Succeed(), "Prep ManifestWorkReplicaSet feedback path is malformed")

			By("Waiting until Prep ManifestWork reports the spoke PrepInProgress message")

			prepMessage, err := cnfhelper.WaitForPrepInProgressFeedback(
				tsparams.PrepTimeoutIbguName, tsparams.TargetSnoClusterName, prepInProgressFeedbackTimeout)
			Expect(err).NotTo(HaveOccurred(),
				"Prep ManifestWork did not report the spoke PrepInProgress message")

			By("Waiting until the timed-out IBGU failed action contains that LCA message")

			Expect(cnfhelper.WaitForFailedActionMessage(
				tsparams.PrepTimeoutIbguName, tsparams.TargetSnoClusterName, ibguv1alpha1.Prep,
				prepMessage, prepFailedActionTimeout)).To(Succeed(),
				"IBGU failedActions did not include the spoke PrepInProgress message")

			By("Verifying the Abort ManifestWorkReplicaSet Idle reason JSONPath")

			Expect(cnfhelper.WaitForIdleReasonFeedbackPath(
				tsparams.PrepTimeoutIbguName, ibguv1alpha1.Abort, manifestWorkReplicaSetPathTimeout)).
				To(Succeed(), "Abort ManifestWorkReplicaSet feedback path is malformed")

			By("Waiting until Abort ManifestWork reports the spoke Idle reason")

			_, err = cnfhelper.WaitForIdleReasonFeedback(
				tsparams.PrepTimeoutIbguName, ibguv1alpha1.Abort,
				tsparams.TargetSnoClusterName, abortIdleReasonFeedbackTimeout)
			Expect(err).NotTo(HaveOccurred(),
				"Abort ManifestWork did not report the spoke Idle reason")

			By("Waiting until spoke IBU returns to Idle")

			Expect(cnfhelper.WaitForSpokeIBU(cnfhelper.SpokeIdle, abortToIdleTimeout)).To(Succeed(),
				"spoke IBU did not return to Idle after Abort")

			By("Waiting until the spoke is healthy after prep timeout")

			Expect(cnfhelper.WaitUntilHealthy(cnfhelper.DefaultHealthTimeout)).To(Succeed(),
				"spoke cluster was not healthy after prep timeout")
		})
	})

package tsparams

import (
	lcav1 "github.com/openshift-kni/lifecycle-agent/api/imagebasedupgrade/v1"
	cguv1alpha1 "github.com/openshift-kni/cluster-group-upgrades-operator/pkg/api/clustergroupupgrades/v1alpha1"
	configv1 "github.com/openshift/api/config/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/openshift-kni/k8sreporter"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/lca/imagebasedupgrade/cnf/internal/cnfparams"
	"k8s.io/utils/ptr"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	policiesv1beta1 "open-cluster-management.io/governance-policy-propagator/api/v1beta1"
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"
)

const (
	// HubTestNamespace is the hub test namespace used for log capture (matches talm test suite).
	HubTestNamespace = "talm-test"
	// OpenshiftOperatorNamespace is the namespace where operators are (matches talm test suite).
	OpenshiftOperatorNamespace = "openshift-operators"
	// AcmOperatorNamespace is the acm operator namespace used for hub log capture.
	AcmOperatorNamespace = "open-cluster-management"

	// LCANamespace is the namespace used by the lifecycle-agent.
	LCANamespace = "openshift-lifecycle-agent"

	// LCAWorkloadName is the name used for creating resources needed to backup workload app.
	LCAWorkloadName = "ibu-workload-app"

	// LCAOADPNamespace is the namespace used by the OADP operator.
	LCAOADPNamespace = "openshift-adp"

	// LCAKlusterletNamespace is the namespace that contains the klusterlet.
	LCAKlusterletNamespace = "open-cluster-management-agent"
)

var (
	// Labels represents the range of labels that can be used for test cases selection.
	Labels = append(cnfparams.Labels, LabelSuite)

	// ReporterHubNamespacesToDump tells the reporter which namespaces on the hub to collect pod logs from (values from talm test suite).
	ReporterHubNamespacesToDump = map[string]string{
		HubTestNamespace:           "",
		OpenshiftOperatorNamespace: "",
		AcmOperatorNamespace:       "",
	}

	// ReporterHubCRsToDump is the CRs the reporter should dump on the hub (values from talm test suite).
	ReporterHubCRsToDump = []k8sreporter.CRData{
		{Cr: &corev1.NamespaceList{}},
		{Cr: &corev1.PodList{}},
		{Cr: &cguv1alpha1.ClusterGroupUpgradeList{}, Namespace: ptr.To(HubTestNamespace)},
		{Cr: &cguv1alpha1.PreCachingConfigList{}, Namespace: ptr.To(HubTestNamespace)},
		{Cr: &policiesv1.PolicyList{}},
		{Cr: &policiesv1.PlacementBindingList{}, Namespace: ptr.To(HubTestNamespace)},
		{Cr: &placementrulev1.PlacementRuleList{}, Namespace: ptr.To(HubTestNamespace)},
		{Cr: &policiesv1beta1.PolicySetList{}, Namespace: ptr.To(HubTestNamespace)},
	}

	// ReporterSpokeNamespacesToDump tells the reporter from where to collect logs on the spoke.
	ReporterSpokeNamespacesToDump = map[string]string{
		LCANamespace:           "lca",
		LCAWorkloadName:        "workload",
		LCAKlusterletNamespace: "klusterlet",
	}

	// ReporterSpokeCRsToDump tells the reporter what CRs to dump on the spoke.
	ReporterSpokeCRsToDump = []k8sreporter.CRData{
		{Cr: &corev1.PodList{}},
		{Cr: &batchv1.JobList{}},
		{Cr: &corev1.ConfigMapList{}},
		{Cr: &appsv1.DeploymentList{}},
		{Cr: &corev1.ServiceList{}},
		{Cr: &lcav1.ImageBasedUpgradeList{}},
		{Cr: &configv1.ClusterOperatorList{}},
	}

	// TargetSnoClusterName is the name of target sno cluster.
	TargetSnoClusterName string

	// ClusterLabelSelector is the cluster label passed to IBGUs.
	ClusterLabelSelector = map[string]string{"common": "true"}
)

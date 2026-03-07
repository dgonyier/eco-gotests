package tsparams

import (
	cguv1alpha1 "github.com/openshift-kni/cluster-group-upgrades-operator/pkg/api/clustergroupupgrades/v1alpha1"
	"github.com/openshift-kni/k8sreporter"
	lcasgv1 "github.com/openshift-kni/lifecycle-agent/api/seedgenerator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
	policiesv1 "open-cluster-management.io/governance-policy-propagator/api/v1"
	policiesv1beta1 "open-cluster-management.io/governance-policy-propagator/api/v1beta1"
	placementrulev1 "open-cluster-management.io/multicloud-operators-subscription/pkg/apis/apps/placementrule/v1"
)

const (
	// LabelSuite represents seedgeneration label that can be used for test cases selection.
	LabelSuite = "seedgeneration"

	// LCANamespace is the namespace used by the lifecycle-agent.
	LCANamespace = "openshift-lifecycle-agent"

	// HubTestNamespace is the hub test namespace used for log capture (matches talm test suite).
	HubTestNamespace = "talm-test"
	// OpenshiftOperatorNamespace is the namespace where operators are (matches talm test suite).
	OpenshiftOperatorNamespace = "openshift-operators"
	// AcmOperatorNamespace is the acm operator namespace used for seed hub log capture.
	AcmOperatorNamespace = "open-cluster-management"
)

var (
	// ReporterHubNamespacesToDump tells the reporter which namespaces on the seed hub to collect pod logs from (values from talm test suite).
	ReporterHubNamespacesToDump = map[string]string{
		HubTestNamespace:           "",
		OpenshiftOperatorNamespace: "",
		AcmOperatorNamespace:       "",
	}

	// ReporterHubCRsToDump is the CRs the reporter should dump on the seed hub (values from talm test suite).
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

	// ReporterSpokeNamespacesToDump tells the reporter from where to collect logs on the seed spoke.
	ReporterSpokeNamespacesToDump = map[string]string{
		LCANamespace: "openshift-lifecycle-agent",
	}

	// ReporterSpokeCRsToDump tells the reporter what CRs to dump on the seed spoke.
	ReporterSpokeCRsToDump = []k8sreporter.CRData{
		{Cr: &corev1.PodList{}, Namespace: ptr.To(LCANamespace)},
		{Cr: &corev1.SecretList{}, Namespace: ptr.To(LCANamespace)},
		{Cr: &lcasgv1.SeedGeneratorList{}},
	}
)

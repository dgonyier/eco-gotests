package tsparams

import (
	bmhv1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	cguv1alpha1 "github.com/openshift-kni/cluster-group-upgrades-operator/pkg/api/clustergroupupgrades/v1alpha1"
	hivev1 "github.com/rh-ecosystem-edge/eco-goinfra/pkg/schemes/hive/api/v1"
	ibiv1alpha1 "github.com/rh-ecosystem-edge/eco-goinfra/pkg/schemes/imagebasedinstall/api/hiveextensions/v1alpha1"
	siteconfigv1alpha1 "github.com/rh-ecosystem-edge/eco-goinfra/pkg/schemes/siteconfig/v1alpha1"

	"github.com/openshift-kni/k8sreporter"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/lca/imagebasedinstall/mgmt/internal/mgmtparams"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
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
)

var (
	// Labels represents the range of labels that can be used for test cases selection.
	Labels = append(mgmtparams.Labels, LabelSuite)

	// RHACMNamespace holds the namespace for ACM resources.
	RHACMNamespace = "rhacm"

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
		mgmtparams.IBIONamespace: "ibio",
	}

	// ReporterSpokeCRsToDump tells the reporter what CRs to dump on the spoke.
	ReporterSpokeCRsToDump = []k8sreporter.CRData{
		{Cr: &corev1.PodList{}},
		{Cr: &corev1.SecretList{}},
		{Cr: &corev1.ConfigMapList{}},
		{Cr: &appsv1.DeploymentList{}},
		{Cr: &corev1.ServiceList{}},
		{Cr: &ibiv1alpha1.ImageClusterInstallList{}},
		{Cr: &hivev1.ClusterImageSetList{}},
		{Cr: &hivev1.ClusterDeploymentList{}},
		{Cr: &siteconfigv1alpha1.ClusterInstanceList{}},
		{Cr: &bmhv1alpha1.BareMetalHostList{}},
	}
)

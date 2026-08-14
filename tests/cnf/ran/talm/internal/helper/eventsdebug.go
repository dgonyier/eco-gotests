package helper

import (
	"os"
	"os/exec"

	. "github.com/rh-ecosystem-edge/eco-gotests/tests/cnf/ran/internal/raninittools"
	"github.com/rh-ecosystem-edge/eco-gotests/tests/cnf/ran/talm/internal/tsparams"
	"k8s.io/klog/v2"
)

// fallbackOcPaths are checked, in order, if "oc" cannot be resolved from PATH. Different environments (bare host
// vs. running inside a pod) have mounted the oc binary at different locations during debugging.
var fallbackOcPaths = []string{"/clusterconfigs/oc", "/usr/local/bin/oc", "/usr/bin/oc"}

// PrintCGUEvents is a temporary debugging helper that prints all ClusterGroupUpgrade events in the
// tsparams.TestNamespace namespace on the hub cluster. It is meant to be called from AfterEach blocks per
// TALM-events-test-plan.md while investigating TALM CGU event behavior, and should be removed once real event
// assertions are implemented.
func PrintCGUEvents() {
	ocPath := resolveOcPath()

	hubKubeconfig := RANConfig.HubKubeconfig
	if _, statErr := os.Stat(hubKubeconfig); statErr != nil {
		klog.V(tsparams.LogLevel).Infof("Hub kubeconfig %q not found or inaccessible: %v", hubKubeconfig, statErr)
	}

	getEventsCmd := exec.Command(ocPath, "get", "event.v1.events.k8s.io",
		"-n", tsparams.TestNamespace,
		"--field-selector", "regarding.kind==ClusterGroupUpgrade",
		"--sort-by", "{.metadata.creationTimestamp}")

	getEventsCmd.Env = append(os.Environ(), "KUBECONFIG="+hubKubeconfig)

	output, err := getEventsCmd.CombinedOutput()
	if err != nil {
		klog.V(tsparams.LogLevel).Infof(
			"Failed to get CGU events in the %s namespace (oc=%s, KUBECONFIG=%s): %v\noutput: %s",
			tsparams.TestNamespace, ocPath, hubKubeconfig, err, output)
	} else {
		klog.V(tsparams.LogLevel).Infof("CGU events in the %s namespace:\n%s", tsparams.TestNamespace, output)
	}
}

// resolveOcPath finds the oc binary via PATH, falling back to a list of known mount locations seen across
// different debugging environments (bare host vs. running inside a pod).
func resolveOcPath() string {
	if pathFromLookup, lookErr := exec.LookPath("oc"); lookErr == nil {
		return pathFromLookup
	}

	for _, candidate := range fallbackOcPaths {
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
	}

	klog.V(tsparams.LogLevel).Infof(
		"Could not resolve oc from PATH or fallback locations %v, defaulting to bare 'oc'", fallbackOcPaths)

	return "oc"
}

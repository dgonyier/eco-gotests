package tests

import (
	"github.com/golang/glog"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift-kni/eco-goinfra/pkg/bmh"
	"github.com/openshift-kni/eco-goinfra/pkg/reportxml"
	"github.com/openshift-kni/eco-gotests/tests/cnf/ran/gitopsztp/internal/tsparams"
	"github.com/openshift-kni/eco-gotests/tests/cnf/ran/internal/rancluster"
	. "github.com/openshift-kni/eco-gotests/tests/cnf/ran/internal/raninittools"
	"github.com/openshift-kni/eco-gotests/tests/cnf/ran/internal/version"
)

var _ = Describe("ZTP BIOS Configuration Tests", Label(tsparams.LabelBiosDayZeroTests), func() {
	var (
		spokeClusterName string
		nodeNames        []string
	)

	// 75196 - Check if spoke has required BIOS setting values applied
	It("Verifies SNO spoke has required BIOS setting values applied", reportxml.ID("75196"), func() {
		versionInRange, err := version.IsVersionStringInRange(RANConfig.ZTPVersion, "4.17", "")
		Expect(err).ToNot(HaveOccurred(), "Failed to check if ZTP version is in range")

		if !versionInRange {
			Skip("ZTP BIOS configuration tests require ZTP version of least 4.17")
		}

		spokeClusterName, err = rancluster.GetSpokeClusterName(HubAPIClient, Spoke1APIClient)
		Expect(err).ToNot(HaveOccurred(), "Failed to get SNO cluster name")
		glog.V(tsparams.LogLevel).Infof("cluster name: %s", spokeClusterName)

		nodeNames, err = rancluster.GetNodeNames(Spoke1APIClient)
		Expect(err).ToNot(HaveOccurred(), "Failed to get node names")
		glog.V(tsparams.LogLevel).Infof("Node names: %v", nodeNames)

		By("getting HFS for spoke")
		hfs, err := bmh.PullHFS(HubAPIClient, nodeNames[0], spokeClusterName)
		Expect(err).ToNot(
			HaveOccurred(),
			"Failed to get HFS for spoke %s in cluster %s",
			nodeNames[0],
			spokeClusterName,
		)

		hfsObject, err := hfs.Get()
		Expect(err).ToNot(
			HaveOccurred(),
			"Failed to get HFS Obj for spoke %s in cluster %s",
			nodeNames[0],
			spokeClusterName,
		)

		By("comparing requsted BIOS settings to actual BIOS settings")
		hfsRequestedSettings := hfsObject.Spec.Settings
		hfsCurrentSettings := hfsObject.Status.Settings

		if len(hfsRequestedSettings) == 0 {
			Skip("hfs.spec.settings map is empty")
		}

		Expect(hfsCurrentSettings).ToNot(
			BeEmpty(),
			"hfs.spec.settings map is not empty, but hfs.status.settings map is empty",
		)

		allSettingsMatch := true
		for param, value := range hfsRequestedSettings {
			setting, ok := hfsCurrentSettings[param]
			if !ok {
				glog.V(tsparams.LogLevel).Infof("Current settings does not have param %s", param)

				continue
			}

			requestedSetting := value.String()
			if requestedSetting == setting {
				glog.V(tsparams.LogLevel).Infof("Requested setting matches current: %s=%s", param, setting)
			} else {
				glog.V(tsparams.LogLevel).Infof(
					"Requested setting %s value %s does not match current value %s",
					param,
					requestedSetting,
					setting)
				allSettingsMatch = false
			}

		}

		Expect(allSettingsMatch).To(BeTrueBecause("One or more requested settings does not match current settings"))
	})

})

/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import (
	"fmt"
	"strings"

	"github.com/openshift-kni/eco-goinfra/pkg/schemes/kmm/v1beta2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	VerificationSuccess    = v1beta2.VerificationSuccess
	VerificationFailure    = v1beta2.VerificationFailure
	VerificationInProgress = v1beta2.VerificationInProgress
	VerificationStageImage = v1beta2.VerificationStageImage
	VerificationStageDone  = v1beta2.VerificationStageDone
)

// PreflightValidationSpec describes the desired state of the resource, such as the kernel version
// that Module CRs need to be verified against as well as the debug configuration of the logs
// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
// +kubebuilder:validation:Required
// +kubebuilder:object:generate=false
type PreflightValidationSpec = v1beta2.PreflightValidationSpec

// +kubebuilder:object:generate=false
type CRStatus = v1beta2.CRBaseStatus

// PreflightValidationStatus is the most recently observed status of the PreflightValidation.
// It is populated by the system and is read-only.
// More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#spec-and-status
type PreflightValidationStatus struct {
	// CRStatuses contain observations about each Module's preflight upgradability validation
	// +patchMergeKey=type
	// +patchStrategy=merge
	// +optional
	CRStatuses map[string]*CRStatus `json:"crStatuses,omitempty" patchStrategy:"merge" patchMergeKey:"type"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// PreflightValidation initiates a preflight validations for all Modules on the current Kubernetes cluster.
// +kubebuilder:resource:path=preflightvalidations,scope=Cluster,shortName=pfv
// +kubebuilder:deprecatedversion
// +operator-sdk:csv:customresourcedefinitions:displayName="Preflight Validation"
type PreflightValidation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// +kubebuilder:validation:Required

	Spec   PreflightValidationSpec   `json:"spec,omitempty"`
	Status PreflightValidationStatus `json:"status,omitempty"`
}

func (p *PreflightValidation) ConvertTo(dstRaw conversion.Hub) error {
	dst := dstRaw.(*v1beta2.PreflightValidation)

	dst.ObjectMeta = p.ObjectMeta
	dst.Spec = p.Spec

	var err error

	dst.Status, err = v1beta2StatusFromV1beta1(p.Status)
	if err != nil {
		return fmt.Errorf("error while converting status: %v", err)
	}

	return nil
}

func (p *PreflightValidation) ConvertFrom(srcRaw conversion.Hub) error {
	src := srcRaw.(*v1beta2.PreflightValidation)

	p.ObjectMeta = src.ObjectMeta
	p.Spec = src.Spec
	p.Status = v1beta1StatusFromV1beta2(src.Status)

	return nil
}

// +kubebuilder:object:root=true

// PreflightValidationList is a list of PreflightValidation objects.
type PreflightValidationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`

	// List of PreflightValidation. More info:
	// https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md
	Items []PreflightValidation `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PreflightValidation{}, &PreflightValidationList{})
}

func v1beta1StatusFromV1beta2(s v1beta2.PreflightValidationStatus) PreflightValidationStatus {
	res := PreflightValidationStatus{}

	if count := len(s.Modules); count > 0 {
		res.CRStatuses = make(map[string]*CRStatus, count)

		for _, v := range s.Modules {
			v := v
			res.CRStatuses[v.Namespace+"/"+v.Name] = &v.CRBaseStatus

			// This may lead to collisions, but at least we preserve backwards compatibility.
			res.CRStatuses[v.Name] = &v.CRBaseStatus
		}
	}

	return res
}

func v1beta2StatusFromV1beta1(s PreflightValidationStatus) (v1beta2.PreflightValidationStatus, error) {
	res := v1beta2.PreflightValidationStatus{}

	if count := len(s.CRStatuses); count > 0 {
		res.Modules = make([]v1beta2.PreflightValidationModuleStatus, 0, count)

		for k, v := range s.CRStatuses {
			namespace, name, ok := strings.Cut(k, "/")

			if !ok || v == nil {
				// Elements whose key is not a namespace name or that are nil are invalid.
				return v1beta2.PreflightValidationStatus{}, nil
			}

			status := v1beta2.PreflightValidationModuleStatus{
				CRBaseStatus: *v,
				Namespace:    namespace,
				Name:         name,
			}

			res.Modules = append(res.Modules, status)
		}
	}

	return res, nil
}

/*
Copyright 2022.

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

package v1alpha1

import (
	"strings"

	"github.com/itayankri/OSRM-Operator/internal/status"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

type OSRMInstanceSpec struct {
	PBFURL            string                       `json:"pbfUrl,omitempty"`
	OSRMProfile       string                       `json:"osrmProfile"`
	Replicas          *int32                       `json:"replicas,omitempty"`
	MinReplicas       *int32                       `json:"minReplicas,omitempty"`
	MaxReplicas       *int32                       `json:"maxReplicas,omitempty"`
	Image             *string                      `json:"image,omitempty"`
	Resources         *corev1.ResourceRequirements `json:"resources,omitempty"`
	Service           ServiceSpec                  `json:"service,omitempty"`
	Persistence       PersistenceSpec              `json:"persistence,omitempty"`
	MapBuilder        MapBuilderSpec               `json:"mapBuilder,omitempty"`
	SpeedUpdates      *SpeedUpdatesSpec            `json:"speedUpdates,omitempty"`
	OSRMRoutedOptions *OSRMRoutedOptions           `json:"osrmRoutedOptions,omitempty"`
}

func (spec *OSRMInstanceSpec) GetImage() string {
	if spec.Image != nil {
		return *spec.Image
	}
	return defaultImage
}

func (spec *OSRMInstanceSpec) GetPbfFileName() string {
	split := strings.Split(spec.PBFURL, "/")
	return split[len(split)-1]
}

func (spec *OSRMInstanceSpec) GetOsrmFileName() string {
	return strings.ReplaceAll(spec.GetPbfFileName(), "osm.pbf", "osrm")
}

func (spec *OSRMInstanceSpec) GetMinAvailable() *intstr.IntOrString {
	if spec.MinReplicas != nil {
		return &intstr.IntOrString{IntVal: *spec.MinReplicas - 1}
	}
	return &intstr.IntOrString{IntVal: 1}
}

func (spec *OSRMInstanceSpec) GetResources() *corev1.ResourceRequirements {
	if spec.Resources == nil {
		return &corev1.ResourceRequirements{}
	}
	return spec.Resources
}

type OSRMInstanceStatus struct {
	// Paused is true when the operator notices paused annotation.
	Paused bool `json:"paused,omitempty"`

	// ObservedGeneration is the latest generation observed by the operator.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	Phase Phase `json:"phase,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (s *OSRMInstanceStatus) SetConditions(resources []runtime.Object) {
	var oldAvailableCondition *metav1.Condition
	var oldAllReplicasReadyCondition *metav1.Condition
	var oldReconciliationSuccessCondition *metav1.Condition

	for _, condition := range s.Conditions {
		switch condition.Type {
		case status.ConditionAllReplicasReady:
			oldAllReplicasReadyCondition = condition.DeepCopy()
		case status.ConditionAvailable:
			oldAvailableCondition = condition.DeepCopy()
		case status.ConditionReconciliationSuccess:
			oldReconciliationSuccessCondition = condition.DeepCopy()
		}
	}

	var reconciliationSuccessCondition metav1.Condition
	if oldReconciliationSuccessCondition != nil {
		reconciliationSuccessCondition = *oldReconciliationSuccessCondition
	} else {
		reconciliationSuccessCondition = status.ReconcileSuccessCondition(metav1.ConditionUnknown, "Initialising", "")
	}

	availableCondition := status.AvailableCondition(resources, oldAvailableCondition)
	allReplicasReadyCondition := status.AllReplicasReadyCondition(resources, oldAllReplicasReadyCondition)
	s.Conditions = []metav1.Condition{
		availableCondition,
		allReplicasReadyCondition,
		reconciliationSuccessCondition,
	}
}

func (s *OSRMInstanceStatus) SetCondition(condition metav1.Condition) {
	for i := range s.Conditions {
		if s.Conditions[i].Type == condition.Type {
			if s.Conditions[i].Status != condition.Status {
				s.Conditions[i].LastTransitionTime = metav1.Now()
			}
			s.Conditions[i].Status = condition.Status
			s.Conditions[i].Reason = condition.Reason
			s.Conditions[i].Message = condition.Message
			break
		}
	}
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type OSRMInstance struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OSRMInstanceSpec   `json:"spec,omitempty"`
	Status OSRMInstanceStatus `json:"status,omitempty"`
}

func (instance *OSRMInstance) ChildResourceName(component string, suffix string) string {
	nameWithComponent := strings.TrimSuffix(strings.Join([]string{instance.ObjectMeta.Name, component}, "-"), "-")
	return strings.TrimSuffix(strings.Join([]string{nameWithComponent, suffix}, "-"), "-")
}

func (instance *OSRMInstance) GetImage() string                 { return instance.Spec.GetImage() }
func (instance *OSRMInstance) GetPbfURL() string                { return instance.Spec.PBFURL }
func (instance *OSRMInstance) GetPbfFileName() string           { return instance.Spec.GetPbfFileName() }
func (instance *OSRMInstance) GetOsrmFileName() string          { return instance.Spec.GetOsrmFileName() }
func (instance *OSRMInstance) GetPersistence() *PersistenceSpec { return &instance.Spec.Persistence }
func (instance *OSRMInstance) GetMapBuilder() *MapBuilderSpec   { return &instance.Spec.MapBuilder }
func (instance *OSRMInstance) GetService() ServiceSpec          { return instance.Spec.Service }

//+kubebuilder:object:root=true

type OSRMInstanceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OSRMInstance `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OSRMInstance{}, &OSRMInstanceList{})
}

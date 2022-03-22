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

	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultImage = "osrm/osrm-backend"

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OSRMClusterSpec defines the desired state of OSRMCluster
type OSRMClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	PBFURL      string          `json:"pbfUrl,omitempty"`
	Profiles    ProfilesSpec    `json:"profiles,omitempty"`
	Ingress     *IngressSpec    `json:"ingress,omitempty"`
	Image       *string         `json:"image,omitempty"`
	Persistence PersistenceSpec `json:"persistence,omitempty"`
}

func (spec *OSRMClusterSpec) GetImage() string {
	if spec.Image != nil {
		return *spec.Image
	}
	return defaultImage
}

func (spec *OSRMClusterSpec) GetPbfFileName() string {
	split := strings.Split(spec.PBFURL, "/")
	return split[len(split)-1]
}

type ProfilesSpec struct {
	Driving *ProfileSpec `json:"driving,omitempty"`
	Cycling *ProfileSpec `json:"cycling,omitempty"`
	Foot    *ProfileSpec `json:"foot,omitempty"`
}

type ProfileSpec struct {
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	MaxReplicas *int32 `json:"maxReplicas,omitempty"`
}

type IngressSpec struct {
	IngressClassName *string  `json:"ingressClassName,omitempty"`
	IPName           string   `json:"ipName,omitempty"`
	ExposingServices []string `json:"exposingServices,omitempty"`
}

type PersistenceSpec struct {
	StorageClassName string             `json:"storageClassName,omitempty"`
	Storage          *resource.Quantity `json:"storage,omitempty"`
}

// OSRMClusterStatus defines the observed state of OSRMCluster
type OSRMClusterStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (clusterStatus *OSRMClusterStatus) SetCondition(
	conditionType string,
	conditionStatus metav1.ConditionStatus,
	reason string,
	messages ...string,
) {
	for i := range clusterStatus.Conditions {
		if clusterStatus.Conditions[i].Type == conditionType {
			if clusterStatus.Conditions[i].Status != conditionStatus {
				clusterStatus.Conditions[i].LastTransitionTime = metav1.Now()
			}
			clusterStatus.Conditions[i].Status = conditionStatus
			clusterStatus.Conditions[i].Reason = reason
			clusterStatus.Conditions[i].Message = strings.Join(messages, ". ")
			break
		}
	}
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// OSRMCluster is the Schema for the osrmclusters API
type OSRMCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OSRMClusterSpec   `json:"spec,omitempty"`
	Status OSRMClusterStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// OSRMClusterList contains a list of OSRMCluster
type OSRMClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []OSRMCluster `json:"items"`
}

func init() {
	SchemeBuilder.Register(&OSRMCluster{}, &OSRMClusterList{})
}

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
	resource "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const defaultImage = "ghcr.io/project-osrm/osrm-backend:v5.27.1"
const defaultSpeedUpdatesFetcherImage = "itayankri/osrm-speed-updates:osrm-v5.27.1"
const defaultBuilderImage = "itayankri/osrm-builder:osrm-v5.27.1"

const OperatorPausedAnnotation = "osrm.itayankri/operator.paused"

// Phase is the current phase of the deployment
type Phase string

const (
	// PhaseBuildingMap signals that the map building phase is in progress
	PhaseBuildingMap Phase = "BuildingMap"

	// PhaseBuildingNewMap signals that a new map is being built
	PhaseUpdatingMap Phase = "UpdatingMap"

	// PhaseWaitingForMap signals that the deployment is waiting for the map to be ready
	PhaseRedepoloyingWorkers Phase = "RedeployingWorkers"

	// PhaseDeployingWorkers signals that the workers are being deployed
	PhaseDeployingWorkers Phase = "DeployingWorkers"

	// PhaseWorkersDeployed signals that the resources are successfully deployed
	PhaseWorkersDeployed Phase = "WorkersDeployed"

	// PhaseWorkersRedeployed signals that the workers are being redeployed
	PhaseWorkersRedeployed Phase = "WorkersRedeployed"

	// PhaseEmpty is an uninitialized phase
	PhaseEmpty Phase = ""
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// OSRMClusterSpec defines the desired state of OSRMCluster
type OSRMClusterSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	PBFURL      string          `json:"pbfUrl,omitempty"`
	Profiles    ProfilesSpec    `json:"profiles,omitempty"`
	Service     ServiceSpec     `json:"service,omitempty"`
	Image       *string         `json:"image,omitempty"`
	Persistence PersistenceSpec `json:"persistence,omitempty"`
	MapBuilder  MapBuilderSpec  `json:"mapBuilder,omitempty"`
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

func (spec *OSRMClusterSpec) GetOsrmFileName() string {
	return strings.ReplaceAll(spec.GetPbfFileName(), "osm.pbf", "osrm")
}

type ProfilesSpec []*ProfileSpec

type ProfileSpec struct {
	Name             string                       `json:"name,omitempty"`
	EndpointName     string                       `json:"endpointName,omitempty"`
	Replicas         *int32                       `json:"replicas,omitempty"`
	InternalEndpoint *string                      `json:"internalEndpoint,omitempty"`
	OSRMProfile      *string                      `json:"osrmProfile,omitempty"`
	MinReplicas      *int32                       `json:"minReplicas,omitempty"`
	MaxReplicas      *int32                       `json:"maxReplicas,omitempty"`
	Resources        *corev1.ResourceRequirements `json:"resources,omitempty"`
	SpeedUpdates     *SpeedUpdatesSpec            `json:"speedUpdates,omitempty"`
}

func (spec *ProfileSpec) GetMinAvailable() *intstr.IntOrString {
	if spec.MinReplicas != nil {
		return &intstr.IntOrString{IntVal: *spec.MinReplicas}
	}

	return &intstr.IntOrString{IntVal: 1}
}

func (spec *ProfileSpec) GetSpeedUpdatesImage() string {
	if spec.SpeedUpdates != nil && spec.SpeedUpdates.Image != nil {
		return *spec.SpeedUpdates.Image
	}
	return defaultSpeedUpdatesFetcherImage
}

func (spec *ProfileSpec) GetResources() *corev1.ResourceRequirements {
	if spec.Resources == nil {
		return &corev1.ResourceRequirements{}
	}
	return spec.Resources
}

func (spec *ProfileSpec) GetProfile() string {
	if spec.OSRMProfile == nil {
		return spec.Name
	}
	return *spec.OSRMProfile
}

func (spec *ProfileSpec) GetInternalEndpoint() string {
	if spec.InternalEndpoint == nil {
		return spec.EndpointName
	}
	return *spec.InternalEndpoint
}

type MapBuilderSpec struct {
	Image            *string                      `json:"image,omitempty"`
	ExtractOptions   *string                      `json:"extractOptions,omitempty"`
	PartitionOptions *string                      `json:"partitionOptions,omitempty"`
	CustomizeOptions *string                      `json:"customizeOptions,omitempty"`
	Resources        *corev1.ResourceRequirements `json:"resources,omitempty"`
}

func (spec *MapBuilderSpec) GetImage() string {
	if spec.Image != nil {
		return *spec.Image
	}
	return defaultBuilderImage
}

func (spec *MapBuilderSpec) GetResources() *corev1.ResourceRequirements {
	if spec.Resources == nil {
		return &corev1.ResourceRequirements{
			Requests: map[corev1.ResourceName]resource.Quantity{
				"memory": resource.MustParse("1Gi"),
				"cpu":    resource.MustParse("1"),
			},
		}
	}
	return spec.Resources
}

type ServiceSpec struct {
	Type             *corev1.ServiceType `json:"type,omitempty"`
	Annotations      map[string]string   `json:"annotations,omitempty"`
	ExposingServices []string            `json:"exposingServices,omitempty"`
	LoadBalancerIP   *string             `json:"loadBalancerIP,omitempty"`
}

func (spec *ServiceSpec) GetType() corev1.ServiceType {
	if spec.Type != nil {
		return *spec.Type
	}
	return corev1.ServiceTypeClusterIP
}

type SpeedUpdatesSpec struct {
	Suspend   *bool                        `json:"suspend,omitempty"`
	URL       string                       `json:"url,omitempty"`
	Schedule  string                       `json:"schedule,omitempty"`
	Image     *string                      `json:"image,omitempty"`
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	Env       []corev1.EnvVar              `json:"env,omitempty"`
}

func (spec *SpeedUpdatesSpec) GetResources() *corev1.ResourceRequirements {
	if spec.Resources == nil {
		return &corev1.ResourceRequirements{}
	}
	return spec.Resources
}

func (spec *SpeedUpdatesSpec) GetEnv() []corev1.EnvVar {
	if spec.Env == nil {
		return []corev1.EnvVar{}
	}
	return spec.Env
}

type PersistenceSpec struct {
	StorageClassName string                             `json:"storageClassName,omitempty"`
	Storage          *resource.Quantity                 `json:"storage,omitempty"`
	AccessMode       *corev1.PersistentVolumeAccessMode `json:"accessMode,omitempty"`
}

func (spec *PersistenceSpec) GetAccessMode() corev1.PersistentVolumeAccessMode {
	if spec.AccessMode != nil {
		return *spec.AccessMode
	}
	return corev1.ReadWriteMany
}

// OSRMClusterStatus defines the observed state of OSRMCluster
type OSRMClusterStatus struct {
	// Paused is true when the operator notices paused annotation.
	Paused bool `json:"paused,omitempty"`

	// ObservedGeneration is the latest generation observed by the operator.
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`

	Phase Phase `json:"phase,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (osrmClusterStatus *OSRMClusterStatus) SetConditions(resources []runtime.Object) {
	var oldAvailableCondition *metav1.Condition
	var oldAllReplicasReadyCondition *metav1.Condition
	var oldReconciliationSuccessCondition *metav1.Condition

	for _, condition := range osrmClusterStatus.Conditions {
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
	osrmClusterStatus.Conditions = []metav1.Condition{
		availableCondition,
		allReplicasReadyCondition,
		reconciliationSuccessCondition,
	}
}

func (status *OSRMClusterStatus) SetCondition(condition metav1.Condition) {
	for i := range status.Conditions {
		if status.Conditions[i].Type == condition.Type {
			if status.Conditions[i].Status != condition.Status {
				status.Conditions[i].LastTransitionTime = metav1.Now()
			}
			status.Conditions[i].Status = condition.Status
			status.Conditions[i].Reason = condition.Reason
			status.Conditions[i].Message = condition.Message
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

func (cluster *OSRMCluster) ChildResourceName(service string, suffix string) string {
	nameWithService := strings.TrimSuffix(strings.Join([]string{cluster.ObjectMeta.Name, service}, "-"), "-")
	return strings.TrimSuffix(strings.Join([]string{nameWithService, suffix}, "-"), "-")
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

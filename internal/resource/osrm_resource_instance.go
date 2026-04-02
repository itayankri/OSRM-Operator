package resource

import (
	osrmv1alpha1 "github.com/itayankri/OSRM-Operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// OSRMResourceInstance is the interface that both *OSRMCluster and *OSRMInstance satisfy,
// allowing builders to work with either CRD type.
type OSRMResourceInstance interface {
	metav1.Object
	runtime.Object
	ChildResourceName(component string, suffix string) string
	GetImage() string
	GetPbfURL() string
	GetPbfFileName() string
	GetOsrmFileName() string
	GetPersistence() *osrmv1alpha1.PersistenceSpec
	GetMapBuilder() *osrmv1alpha1.MapBuilderSpec
	GetService() osrmv1alpha1.ServiceSpec
}

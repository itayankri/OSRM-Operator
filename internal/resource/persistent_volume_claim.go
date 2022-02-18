package resource

import "sigs.k8s.io/controller-runtime/pkg/client"

type PersistentVolumeClaimBuilder struct {
	*OSRMResourceBuilder
}

func (builder *OSRMResourceBuilder) PersistentVolumeClaim() *PersistentVolumeClaimBuilder {
	return &PersistentVolumeClaimBuilder{builder}
}

func (builder *PersistentVolumeClaimBuilder) Build() (client.Object, error) {
	return nil, nil
}

func (builder *PersistentVolumeClaimBuilder) Update(object client.Object) error {
	return nil
}
